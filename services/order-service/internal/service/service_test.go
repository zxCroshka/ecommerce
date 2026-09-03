package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
)

type memoryRepository struct {
	mu          sync.Mutex
	nextID      int64
	orders      map[int64]*domain.Order
	byKey       map[string]int64
	outboxCount int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		nextID: 1,
		orders: make(map[int64]*domain.Order),
		byKey:  make(map[string]int64),
	}
}

func (r *memoryRepository) CreatePending(_ context.Context, input domain.NewOrder, owner uuid.UUID) (*domain.Order, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%d/%s", input.UserID, input.IdempotencyKey)
	if id, ok := r.byKey[key]; ok {
		return cloneOrder(r.orders[id]), false, nil
	}
	now := time.Now().UTC()
	order := &domain.Order{
		ID:             r.nextID,
		UserID:         input.UserID,
		Status:         domain.StatusPending,
		TotalAmount:    input.TotalAmount,
		Currency:       input.Currency,
		IdempotencyKey: input.IdempotencyKey,
		CartRevision:   input.CartRevision,
		Items:          append([]domain.Item(nil), input.Items...),
		ProcessingBy:   owner.String(),
		ProcessingAt:   &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	r.nextID++
	r.orders[order.ID] = order
	r.byKey[key] = order.ID
	return cloneOrder(order), true, nil
}

func (r *memoryRepository) GetByIdempotency(_ context.Context, userID int64, key string) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[fmt.Sprintf("%d/%s", userID, key)]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return cloneOrder(r.orders[id]), nil
}

func (r *memoryRepository) GetByIDForUser(_ context.Context, orderID, userID int64) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[orderID]
	if !ok || order.UserID != userID {
		return nil, domain.ErrOrderNotFound
	}
	return cloneOrder(order), nil
}

func (r *memoryRepository) GetByID(_ context.Context, orderID int64) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[orderID]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return cloneOrder(order), nil
}

func (r *memoryRepository) ListForUser(_ context.Context, userID int64, limit, offset int) ([]*domain.Order, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	orders := make([]*domain.Order, 0)
	for _, order := range r.orders {
		if order.UserID == userID {
			orders = append(orders, cloneOrder(order))
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].ID > orders[j].ID })
	total := int64(len(orders))
	if offset >= len(orders) {
		return []*domain.Order{}, total, nil
	}
	end := min(offset+limit, len(orders))
	return orders[offset:end], total, nil
}

func (r *memoryRepository) TryClaimPending(_ context.Context, orderID int64, owner uuid.UUID, staleAfter time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[orderID]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	if order.Status != domain.StatusPending {
		return false, nil
	}
	if order.ProcessingBy != "" && order.ProcessingAt != nil && time.Since(*order.ProcessingAt) < staleAfter {
		return false, nil
	}
	now := time.Now().UTC()
	order.ProcessingBy = owner.String()
	order.ProcessingAt = &now
	return true, nil
}

func (r *memoryRepository) RefreshPendingLease(_ context.Context, orderID int64, owner uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[orderID]
	if !ok || order.Status != domain.StatusPending || order.ProcessingBy != owner.String() {
		return domain.ErrWorkflowLeaseLost
	}
	now := time.Now().UTC()
	order.ProcessingAt = &now
	return nil
}

func (r *memoryRepository) ReleasePendingLease(_ context.Context, orderID int64, owner uuid.UUID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[orderID]
	if !ok || order.Status != domain.StatusPending || order.ProcessingBy != owner.String() {
		return domain.ErrWorkflowLeaseLost
	}
	order.ProcessingBy = ""
	order.ProcessingAt = nil
	order.FailureReason = reason
	return nil
}

func (r *memoryRepository) MarkFailed(_ context.Context, orderID int64, owner uuid.UUID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[orderID]
	if !ok || order.Status != domain.StatusPending || order.ProcessingBy != owner.String() {
		return domain.ErrInvalidTransition
	}
	order.Status = domain.StatusFailed
	order.ProcessingBy = ""
	order.ProcessingAt = nil
	order.FailureReason = reason
	order.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memoryRepository) ConfirmWithOutbox(_ context.Context, input *domain.Order, owner uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[input.ID]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	if order.Status == domain.StatusConfirmed {
		return false, nil
	}
	if order.Status != domain.StatusPending || order.ProcessingBy != owner.String() {
		return false, domain.ErrInvalidTransition
	}
	order.Status = domain.StatusConfirmed
	order.ProcessingBy = ""
	order.ProcessingAt = nil
	order.FailureReason = ""
	order.UpdatedAt = time.Now().UTC()
	r.outboxCount++
	return true, nil
}

func (r *memoryRepository) ListRecoverablePending(_ context.Context, olderThan, staleBefore time.Time, limit int) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]int64, 0, limit)
	for _, order := range r.orders {
		if order.Status == domain.StatusPending && !order.UpdatedAt.After(olderThan) &&
			(order.ProcessingBy == "" || order.ProcessingAt == nil || order.ProcessingAt.Before(staleBefore)) {
			ids = append(ids, order.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}

func (r *memoryRepository) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.orders)
}

func (r *memoryRepository) eventCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outboxCount
}

func cloneOrder(order *domain.Order) *domain.Order {
	if order == nil {
		return nil
	}
	copyValue := *order
	copyValue.Items = append([]domain.Item(nil), order.Items...)
	if order.ProcessingAt != nil {
		processingAt := *order.ProcessingAt
		copyValue.ProcessingAt = &processingAt
	}
	return &copyValue
}

type fakeCart struct {
	mu         sync.Mutex
	snapshot   *domain.CartSnapshot
	clear      bool
	clearErr   error
	clearCalls int
}

func (c *fakeCart) Snapshot(context.Context, int64) (*domain.CartSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshot == nil {
		return nil, domain.ErrCartEmpty
	}
	result := *c.snapshot
	result.Items = append([]domain.CartItem(nil), c.snapshot.Items...)
	return &result, nil
}

func (c *fakeCart) ClearIfUnchanged(context.Context, int64, int64) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearCalls++
	return c.clear, c.clearErr
}

type reservation struct {
	quantity int64
	released bool
}

type fakeProducts struct {
	mu             sync.Mutex
	products       map[int64]*domain.Product
	stock          map[int64]int64
	reservations   map[string]reservation
	reserveErrors  map[int64][]error
	releaseErrors  map[int64][]error
	reserveApplied map[int64]int
	releaseApplied map[int64]int
}

func newFakeProducts() *fakeProducts {
	return &fakeProducts{
		products:       make(map[int64]*domain.Product),
		stock:          make(map[int64]int64),
		reservations:   make(map[string]reservation),
		reserveErrors:  make(map[int64][]error),
		releaseErrors:  make(map[int64][]error),
		reserveApplied: make(map[int64]int),
		releaseApplied: make(map[int64]int),
	}
}

func (p *fakeProducts) GetProduct(_ context.Context, productID int64) (*domain.Product, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	product, ok := p.products[productID]
	if !ok {
		return nil, domain.ErrProductUnavailable
	}
	copyValue := *product
	return &copyValue, nil
}

func reservationKey(reservationID string, productID int64) string {
	return fmt.Sprintf("%s/%d", reservationID, productID)
}

func (p *fakeProducts) ReserveStock(_ context.Context, reservationID string, productID, quantity int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := reservationKey(reservationID, productID)
	if existing, ok := p.reservations[key]; ok {
		if existing.released || existing.quantity != quantity {
			return domain.ErrInvalidOrder
		}
		return nil
	}
	if values := p.reserveErrors[productID]; len(values) > 0 {
		err := values[0]
		p.reserveErrors[productID] = values[1:]
		return err
	}
	if p.stock[productID] < quantity {
		return domain.ErrInsufficientStock
	}
	p.stock[productID] -= quantity
	p.reservations[key] = reservation{quantity: quantity}
	p.reserveApplied[productID]++
	return nil
}

func (p *fakeProducts) ReleaseStock(_ context.Context, reservationID string, productID int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if values := p.releaseErrors[productID]; len(values) > 0 {
		err := values[0]
		p.releaseErrors[productID] = values[1:]
		return err
	}
	key := reservationKey(reservationID, productID)
	existing, ok := p.reservations[key]
	if !ok || existing.released {
		return nil
	}
	existing.released = true
	p.reservations[key] = existing
	p.stock[productID] += existing.quantity
	p.releaseApplied[productID]++
	return nil
}

func newTestOrderService(t *testing.T, repo *memoryRepository, cart *fakeCart, products *fakeProducts) *OrderService {
	t.Helper()
	result, err := New(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		repo,
		cart,
		products,
		Config{
			Currency:             "USD",
			MaxItems:             100,
			MaxIdempotencyLength: 128,
			LeaseTimeout:         time.Second,
			FinalizeTimeout:      time.Second,
			CompensationTimeout:  time.Second,
			CartCleanupTimeout:   time.Second,
		},
	)
	require.NoError(t, err)
	return result
}

func userContext(userID int64) context.Context {
	return auth.WithUserIdentity(context.Background(), auth.UserIdentity{UserID: userID, Role: "user"})
}

func fixture() (*memoryRepository, *fakeCart, *fakeProducts) {
	repo := newMemoryRepository()
	cart := &fakeCart{
		clear: true,
		snapshot: &domain.CartSnapshot{
			Revision: 7,
			Items:    []domain.CartItem{{ProductID: 1, Quantity: 2}, {ProductID: 2, Quantity: 1}},
		},
	}
	products := newFakeProducts()
	products.products[1] = &domain.Product{ID: 1, Name: "first", Price: 125, IsActive: true}
	products.products[2] = &domain.Product{ID: 2, Name: "second", Price: 350, IsActive: true}
	products.stock[1] = 10
	products.stock[2] = 10
	return repo, cart, products
}

func TestCreateOrderSuccess(t *testing.T) {
	repo, cart, products := fixture()
	orders := newTestOrderService(t, repo, cart, products)

	result, err := orders.CreateOrder(userContext(42), "checkout-1")
	require.NoError(t, err)
	require.True(t, result.Created)
	require.Equal(t, domain.StatusConfirmed, result.Order.Status)
	require.Equal(t, int64(600), result.Order.TotalAmount)
	require.Equal(t, int64(8), products.stock[1])
	require.Equal(t, int64(9), products.stock[2])
	require.Equal(t, 1, cart.clearCalls)
	require.Equal(t, 1, repo.eventCount())
}

func TestPriceSnapshotDoesNotChangeWithProduct(t *testing.T) {
	repo, cart, products := fixture()
	orders := newTestOrderService(t, repo, cart, products)
	created, err := orders.CreateOrder(userContext(42), "price-snapshot")
	require.NoError(t, err)

	products.mu.Lock()
	products.products[1].Price = 9999
	products.mu.Unlock()
	stored, err := orders.GetOrder(userContext(42), created.Order.ID)
	require.NoError(t, err)
	require.Equal(t, int64(125), stored.Items[0].UnitPrice)
	require.Equal(t, int64(600), stored.TotalAmount)
}

func TestDuplicateIdempotencyKeyDoesNotReserveTwice(t *testing.T) {
	repo, cart, products := fixture()
	orders := newTestOrderService(t, repo, cart, products)

	first, err := orders.CreateOrder(userContext(42), "same-key")
	require.NoError(t, err)
	second, err := orders.CreateOrder(userContext(42), "same-key")
	require.NoError(t, err)
	require.Equal(t, first.Order.ID, second.Order.ID)
	require.False(t, second.Created)
	require.Equal(t, 1, products.reserveApplied[1])
	require.Equal(t, 1, products.reserveApplied[2])
	require.Equal(t, 1, repo.eventCount())
	require.Equal(t, 1, repo.count())
}

func TestInsufficientStockCompensatesAndPreservesCart(t *testing.T) {
	repo, cart, products := fixture()
	products.stock[2] = 0
	orders := newTestOrderService(t, repo, cart, products)

	_, err := orders.CreateOrder(userContext(42), "insufficient")
	require.ErrorIs(t, err, domain.ErrOrderFailed)
	stored, getErr := repo.GetByIdempotency(context.Background(), 42, "insufficient")
	require.NoError(t, getErr)
	require.Equal(t, domain.StatusFailed, stored.Status)
	require.Equal(t, int64(10), products.stock[1])
	require.Equal(t, 1, products.releaseApplied[1])
	require.Zero(t, cart.clearCalls)
	require.Zero(t, repo.eventCount())
}

func TestPartialCompensationIsIdempotentlyRetried(t *testing.T) {
	repo, cart, products := fixture()
	products.stock[2] = 0
	products.releaseErrors[1] = []error{errors.New("temporary release failure")}
	orders := newTestOrderService(t, repo, cart, products)

	_, err := orders.CreateOrder(userContext(42), "compensate-retry")
	require.ErrorIs(t, err, domain.ErrCompensationPending)
	pending, getErr := repo.GetByIdempotency(context.Background(), 42, "compensate-retry")
	require.NoError(t, getErr)
	require.Equal(t, domain.StatusPending, pending.Status)
	require.Equal(t, int64(8), products.stock[1])

	_, err = orders.CreateOrder(userContext(42), "compensate-retry")
	require.ErrorIs(t, err, domain.ErrOrderFailed)
	failed, getErr := repo.GetByIdempotency(context.Background(), 42, "compensate-retry")
	require.NoError(t, getErr)
	require.Equal(t, domain.StatusFailed, failed.Status)
	require.Equal(t, int64(10), products.stock[1])
	require.Equal(t, 1, products.reserveApplied[1])
	require.Equal(t, 1, products.releaseApplied[1])
	require.Zero(t, cart.clearCalls)
}

func TestRetryPendingOrderReusesExistingReservations(t *testing.T) {
	repo, cart, products := fixture()
	orders := newTestOrderService(t, repo, cart, products)
	owner := uuid.New()
	pending, created, err := repo.CreatePending(context.Background(), domain.NewOrder{
		UserID:         42,
		TotalAmount:    600,
		Currency:       "USD",
		IdempotencyKey: "crash-retry",
		CartRevision:   7,
		Items: []domain.Item{
			{ProductID: 1, ProductName: "first", UnitPrice: 125, Quantity: 2, LineTotal: 250},
			{ProductID: 2, ProductName: "second", UnitPrice: 350, Quantity: 1, LineTotal: 350},
		},
	}, owner)
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, products.ReserveStock(context.Background(), "order:1", 1, 2))
	require.NoError(t, repo.ReleasePendingLease(context.Background(), pending.ID, owner, "simulated crash"))

	result, err := orders.CreateOrder(userContext(42), "crash-retry")
	require.NoError(t, err)
	require.False(t, result.Created)
	require.Equal(t, domain.StatusConfirmed, result.Order.Status)
	require.Equal(t, 1, products.reserveApplied[1])
	require.Equal(t, 1, products.reserveApplied[2])
	require.Equal(t, 1, repo.eventCount())
}

func TestChangedCartIsNotCleared(t *testing.T) {
	repo, cart, products := fixture()
	cart.clear = false
	orders := newTestOrderService(t, repo, cart, products)

	result, err := orders.CreateOrder(userContext(42), "cart-changed")
	require.NoError(t, err)
	require.Equal(t, domain.StatusConfirmed, result.Order.Status)
	require.Equal(t, 1, cart.clearCalls)
	require.False(t, cart.clear)
}

func TestCartCleanupFailureDoesNotRollBackConfirmedOrder(t *testing.T) {
	repo, cart, products := fixture()
	cart.clearErr = errors.New("Cart unavailable")
	orders := newTestOrderService(t, repo, cart, products)

	result, err := orders.CreateOrder(userContext(42), "cart-cleanup-retry")
	require.NoError(t, err)
	require.Equal(t, domain.StatusConfirmed, result.Order.Status)
	require.Equal(t, 1, repo.eventCount())
	require.Equal(t, 1, cart.clearCalls)

	cart.clearErr = nil
	result, err = orders.CreateOrder(userContext(42), "cart-cleanup-retry")
	require.NoError(t, err)
	require.False(t, result.Created)
	require.Equal(t, 2, cart.clearCalls, "retrying the confirmed idempotency key retries Cart cleanup")
}

func TestGetOrderCannotReadAnotherUsersOrder(t *testing.T) {
	repo, cart, products := fixture()
	orders := newTestOrderService(t, repo, cart, products)
	created, err := orders.CreateOrder(userContext(42), "private-order")
	require.NoError(t, err)

	_, err = orders.GetOrder(userContext(99), created.Order.ID)
	require.ErrorIs(t, err, domain.ErrOrderNotFound)
	foreign, total, err := orders.ListOrders(userContext(99), 20, 0)
	require.NoError(t, err)
	require.Empty(t, foreign)
	require.Zero(t, total)
}

func TestConcurrentDuplicateCreateProducesOneOrder(t *testing.T) {
	repo, cart, products := fixture()
	orders := newTestOrderService(t, repo, cart, products)

	const workers = 20
	errorsByWorker := make(chan error, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			_, err := orders.CreateOrder(userContext(42), "concurrent-key")
			errorsByWorker <- err
		}()
	}
	wait.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		require.NoError(t, err)
	}
	require.Equal(t, 1, repo.count())
	require.Equal(t, 1, products.reserveApplied[1])
	require.Equal(t, 1, products.reserveApplied[2])
	require.Equal(t, 1, repo.eventCount())
}

func TestRecoveryStopsGracefully(t *testing.T) {
	repo, cart, products := fixture()
	orders := newTestOrderService(t, repo, cart, products)
	recovery, err := NewRecovery(nil, orders, RecoveryConfig{
		PollInterval: time.Hour,
		RecoveryAge:  time.Hour,
		OrderTimeout: time.Second,
		BatchSize:    10,
	})
	require.NoError(t, err)
	require.NoError(t, recovery.Start(context.Background()))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, recovery.Stop(ctx))
}
