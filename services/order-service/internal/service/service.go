package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

type Config struct {
	Currency             string
	MaxItems             int
	MaxIdempotencyLength int
	LeaseTimeout         time.Duration
	FinalizeTimeout      time.Duration
	CompensationTimeout  time.Duration
	CartCleanupTimeout   time.Duration
}

type Repository interface {
	CreatePending(context.Context, domain.NewOrder, uuid.UUID) (*domain.Order, bool, error)
	GetByIdempotency(context.Context, int64, string) (*domain.Order, error)
	GetByIDForUser(context.Context, int64, int64) (*domain.Order, error)
	GetByID(context.Context, int64) (*domain.Order, error)
	ListForUser(context.Context, int64, int, int) ([]*domain.Order, int64, error)
	TryClaimPending(context.Context, int64, uuid.UUID, time.Duration) (bool, error)
	RefreshPendingLease(context.Context, int64, uuid.UUID) error
	ReleasePendingLease(context.Context, int64, uuid.UUID, string) error
	MarkFailed(context.Context, int64, uuid.UUID, string) error
	ConfirmWithOutbox(context.Context, *domain.Order, uuid.UUID) (bool, error)
	ListRecoverablePending(context.Context, time.Time, time.Time, int) ([]int64, error)
}

type CartClient interface {
	Snapshot(context.Context, int64) (*domain.CartSnapshot, error)
	ClearIfUnchanged(context.Context, int64, int64) (bool, error)
}

type ProductClient interface {
	GetProduct(context.Context, int64) (*domain.Product, error)
	ReserveStock(context.Context, string, int64, int64) error
	ReleaseStock(context.Context, string, int64) error
}

type CreateResult struct {
	Order   *domain.Order
	Created bool
}

type OrderService struct {
	log      *slog.Logger
	repo     Repository
	cart     CartClient
	products ProductClient
	config   Config
}

func New(
	log *slog.Logger,
	repo Repository,
	cart CartClient,
	products ProductClient,
	config Config,
) (*OrderService, error) {
	if repo == nil || cart == nil || products == nil {
		return nil, fmt.Errorf("Order Service dependencies are required")
	}
	if log == nil {
		log = slog.Default()
	}
	config.Currency = strings.ToUpper(strings.TrimSpace(config.Currency))
	if len(config.Currency) != 3 || config.MaxItems <= 0 || config.MaxIdempotencyLength <= 0 ||
		config.LeaseTimeout <= 0 || config.FinalizeTimeout <= 0 ||
		config.CompensationTimeout <= 0 || config.CartCleanupTimeout <= 0 {
		return nil, fmt.Errorf("invalid Order Service config")
	}
	return &OrderService{log: log, repo: repo, cart: cart, products: products, config: config}, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, idempotencyKey string) (*CreateResult, error) {
	identity, ok := auth.UserIdentityFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthenticated
	}
	key, err := s.validateIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByIdempotency(ctx, identity.UserID, key)
	switch {
	case err == nil:
		return s.continueOrder(ctx, existing, false)
	case !errors.Is(err, domain.ErrOrderNotFound):
		return nil, fmt.Errorf("find idempotent order: %w", err)
	}

	snapshot, err := s.cart.Snapshot(ctx, identity.UserID)
	if err != nil {
		return nil, fmt.Errorf("get cart snapshot: %w", err)
	}
	input, err := s.buildOrder(ctx, identity.UserID, key, snapshot)
	if err != nil {
		return nil, err
	}

	owner := uuid.New()
	order, created, err := s.repo.CreatePending(ctx, input, owner)
	if err != nil {
		return nil, err
	}
	if !created {
		return s.continueOrder(ctx, order, false)
	}

	confirmed, err := s.processPending(ctx, order, owner)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Order: confirmed, Created: true}, nil
}

func (s *OrderService) continueOrder(
	ctx context.Context,
	order *domain.Order,
	created bool,
) (*CreateResult, error) {
	switch order.Status {
	case domain.StatusConfirmed:
		s.clearConfirmedCart(ctx, order)
		return &CreateResult{Order: order, Created: created}, nil
	case domain.StatusFailed:
		return nil, fmt.Errorf("%w: %s", domain.ErrOrderFailed, order.FailureReason)
	case domain.StatusPending:
		owner := uuid.New()
		claimed, err := s.repo.TryClaimPending(ctx, order.ID, owner, s.config.LeaseTimeout)
		if err != nil {
			return nil, err
		}
		if !claimed {
			latest, getErr := s.repo.GetByID(ctx, order.ID)
			if getErr != nil {
				return nil, getErr
			}
			if latest.Status != domain.StatusPending {
				return s.continueOrder(ctx, latest, created)
			}
			s.log.Info("idempotent order workflow is already owned", "order_id", order.ID)
			return &CreateResult{Order: latest, Created: created}, nil
		}
		confirmed, err := s.processPending(ctx, order, owner)
		if err != nil {
			return nil, err
		}
		return &CreateResult{Order: confirmed, Created: created}, nil
	default:
		return nil, domain.ErrInvalidTransition
	}
}

func (s *OrderService) buildOrder(
	ctx context.Context,
	userID int64,
	idempotencyKey string,
	snapshot *domain.CartSnapshot,
) (domain.NewOrder, error) {
	if snapshot == nil || snapshot.Revision <= 0 || len(snapshot.Items) == 0 {
		return domain.NewOrder{}, domain.ErrCartEmpty
	}
	if len(snapshot.Items) > s.config.MaxItems {
		return domain.NewOrder{}, fmt.Errorf("%w: cart has too many items", domain.ErrInvalidOrder)
	}

	items := append([]domain.CartItem(nil), snapshot.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ProductID < items[j].ProductID })
	seen := make(map[int64]struct{}, len(items))
	result := domain.NewOrder{
		UserID:         userID,
		Currency:       s.config.Currency,
		IdempotencyKey: idempotencyKey,
		CartRevision:   snapshot.Revision,
		Items:          make([]domain.Item, 0, len(items)),
	}

	for _, cartItem := range items {
		if cartItem.ProductID <= 0 || cartItem.Quantity <= 0 {
			return domain.NewOrder{}, domain.ErrInvalidOrder
		}
		if _, duplicate := seen[cartItem.ProductID]; duplicate {
			return domain.NewOrder{}, domain.ErrInvalidOrder
		}
		seen[cartItem.ProductID] = struct{}{}

		product, err := s.products.GetProduct(ctx, cartItem.ProductID)
		if err != nil {
			return domain.NewOrder{}, fmt.Errorf("load product %d: %w", cartItem.ProductID, err)
		}
		if product == nil || product.ID != cartItem.ProductID || !product.IsActive || product.Price < 0 {
			return domain.NewOrder{}, fmt.Errorf("%w: product %d", domain.ErrProductUnavailable, cartItem.ProductID)
		}
		if product.Price != 0 && cartItem.Quantity > math.MaxInt64/product.Price {
			return domain.NewOrder{}, domain.ErrAmountOverflow
		}
		lineTotal := product.Price * cartItem.Quantity
		if result.TotalAmount > math.MaxInt64-lineTotal {
			return domain.NewOrder{}, domain.ErrAmountOverflow
		}
		result.TotalAmount += lineTotal
		result.Items = append(result.Items, domain.Item{
			ProductID:   product.ID,
			ProductName: product.Name,
			UnitPrice:   product.Price,
			Quantity:    cartItem.Quantity,
			LineTotal:   lineTotal,
		})
	}
	return result, nil
}

func (s *OrderService) processPending(
	ctx context.Context,
	order *domain.Order,
	owner uuid.UUID,
) (*domain.Order, error) {
	reservationID := fmt.Sprintf("order:%d", order.ID)
	for _, item := range order.Items {
		if err := s.repo.RefreshPendingLease(ctx, order.ID, owner); err != nil {
			return nil, err
		}
		if err := s.products.ReserveStock(ctx, reservationID, item.ProductID, item.Quantity); err != nil {
			if isPermanentReservationError(err) {
				return nil, s.failAndCompensate(ctx, order, owner, reservationID, err)
			}
			s.leavePending(ctx, order.ID, owner, err)
			return nil, fmt.Errorf("%w: reserve product %d: %v", domain.ErrDownstream, item.ProductID, err)
		}
	}

	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.FinalizeTimeout)
	_, err := s.repo.ConfirmWithOutbox(finalizeCtx, order, owner)
	cancel()
	if err != nil {
		s.leavePending(ctx, order.ID, owner, err)
		return nil, err
	}

	loadCtx, loadCancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.FinalizeTimeout)
	confirmed, err := s.repo.GetByID(loadCtx, order.ID)
	loadCancel()
	if err != nil {
		order.Status = domain.StatusConfirmed
		confirmed = order
	}
	s.clearConfirmedCart(ctx, confirmed)
	return confirmed, nil
}

func isPermanentReservationError(err error) bool {
	return errors.Is(err, domain.ErrInsufficientStock) ||
		errors.Is(err, domain.ErrProductUnavailable) ||
		errors.Is(err, domain.ErrInvalidOrder)
}

func (s *OrderService) failAndCompensate(
	ctx context.Context,
	order *domain.Order,
	owner uuid.UUID,
	reservationID string,
	cause error,
) error {
	compensationCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		s.config.CompensationTimeout,
	)
	defer cancel()

	var compensationErr error
	for _, item := range order.Items {
		if err := s.products.ReleaseStock(compensationCtx, reservationID, item.ProductID); err != nil {
			compensationErr = errors.Join(compensationErr, fmt.Errorf("release product %d: %w", item.ProductID, err))
		}
	}
	if compensationErr != nil {
		releaseErr := s.repo.ReleasePendingLease(compensationCtx, order.ID, owner, compensationErr.Error())
		return errors.Join(domain.ErrCompensationPending, cause, compensationErr, releaseErr)
	}
	if err := s.repo.MarkFailed(compensationCtx, order.ID, owner, cause.Error()); err != nil {
		return errors.Join(domain.ErrCompensationPending, cause, err)
	}
	return fmt.Errorf("%w: %v", domain.ErrOrderFailed, cause)
}

func (s *OrderService) leavePending(ctx context.Context, orderID int64, owner uuid.UUID, cause error) {
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.FinalizeTimeout)
	defer cancel()
	if err := s.repo.ReleasePendingLease(finalizeCtx, orderID, owner, cause.Error()); err != nil {
		s.log.Error("failed to release pending order workflow lease", "order_id", orderID, "error", err)
	}
	s.log.Warn("order remains pending and will be retried", "order_id", orderID, "error", cause)
}

func (s *OrderService) clearConfirmedCart(ctx context.Context, order *domain.Order) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.config.CartCleanupTimeout)
	defer cancel()
	cleared, err := s.cart.ClearIfUnchanged(cleanupCtx, order.UserID, order.CartRevision)
	if err != nil {
		s.log.Warn("confirmed order cart cleanup will be retried", "order_id", order.ID, "error", err)
		return
	}
	if !cleared {
		s.log.Info("cart changed after snapshot; newer cart was preserved", "order_id", order.ID)
	}
}

func (s *OrderService) GetOrder(ctx context.Context, orderID int64) (*domain.Order, error) {
	identity, ok := auth.UserIdentityFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthenticated
	}
	if orderID <= 0 {
		return nil, domain.ErrInvalidOrder
	}
	return s.repo.GetByIDForUser(ctx, orderID, identity.UserID)
}

func (s *OrderService) ListOrders(
	ctx context.Context,
	limit, offset int,
) ([]*domain.Order, int64, error) {
	identity, ok := auth.UserIdentityFromContext(ctx)
	if !ok {
		return nil, 0, domain.ErrUnauthenticated
	}
	if limit <= 0 || offset < 0 {
		return nil, 0, domain.ErrInvalidOrder
	}
	return s.repo.ListForUser(ctx, identity.UserID, limit, offset)
}

func (s *OrderService) RecoverPending(ctx context.Context, orderID int64) error {
	owner := uuid.New()
	claimed, err := s.repo.TryClaimPending(ctx, orderID, owner, s.config.LeaseTimeout)
	if err != nil || !claimed {
		return err
	}
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	_, err = s.processPending(ctx, order, owner)
	if errors.Is(err, domain.ErrOrderFailed) {
		return nil
	}
	return err
}

func (s *OrderService) ListRecoverablePending(
	ctx context.Context,
	recoveryAge time.Duration,
	limit int,
) ([]int64, error) {
	now := time.Now().UTC()
	return s.repo.ListRecoverablePending(
		ctx,
		now.Add(-recoveryAge),
		now.Add(-s.config.LeaseTimeout),
		limit,
	)
}

func (s *OrderService) validateIdempotencyKey(value string) (string, error) {
	key := strings.TrimSpace(value)
	if key == "" || len(key) > s.config.MaxIdempotencyLength || !idempotencyKeyPattern.MatchString(key) {
		return "", domain.ErrInvalidIdempotency
	}
	return key, nil
}
