package product

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/db"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/scanner"
)

type Storage struct {
	db db.DBTX
}

func New(db db.DBTX) *Storage {
	return &Storage{db: db}
}

func (s *Storage) WithTX(tx pgx.Tx) *Storage {
	return &Storage{db: tx}
}

func (s *Storage) Insert(
	ctx context.Context,
	name, description string,
	price, stock int64,
	category string,
	images []string,
	isActive bool,
	createdAt time.Time,
	updatedAt time.Time,
) (int64, error) {
	const op = "storage.postgres.product.Insert"
	stmt := `INSERT INTO productservice.products(name,description,price,stock,category,images,is_active,created_at,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING id `
	var id int64
	if err := s.db.QueryRow(ctx, stmt, name, description, price, stock, category, images, isActive, createdAt, updatedAt).Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return 0, fmt.Errorf("%s: %w", op, customerrors.ErrProductExists)
			}
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return id, nil
}

func (s *Storage) UpdateProductFields(
	ctx context.Context,
	productID int64,
	patch domain.ProductPatch,
) error {
	const op = "storage.postgres.product.UpdateProductFields"

	exists, err := s.productExists(ctx, productID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if !exists {
		return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
	}

	if patch.Empty() {
		return fmt.Errorf("%s: no fields to update", op)
	}

	stmt := `UPDATE productservice.products SET
		name = COALESCE($1, name), description = COALESCE($2, description),
		price = COALESCE($3, price), stock = COALESCE($4, stock),
		category = COALESCE($5, category),
		images = CASE WHEN $6 THEN $7 ELSE images END,
		is_active = COALESCE($8, is_active), updated_at = NOW()
		WHERE id = $9`
	result, err := s.db.Exec(ctx, stmt, patch.Name, patch.Description, patch.Price, patch.Stock,
		patch.Category, patch.ImagesSet, patch.Images, patch.IsActive, productID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
	}

	return nil
}

func (s *Storage) ListProducts(ctx context.Context, req domain.ProductListRequest) ([]*domain.Product, int64, error) {
	const op = "storage.postgres.product.ListProducts"
	baseStmt := `FROM productservice.products WHERE 1=1`
	args := []any{}
	argCounter := 1
	if req.Filter.Category != nil {
		baseStmt += fmt.Sprintf(" AND category=$%d", argCounter)
		args = append(args, *req.Filter.Category)
		argCounter++
	}
	if req.Filter.IsActive != nil {
		baseStmt += fmt.Sprintf(" AND is_active=$%d", argCounter)
		args = append(args, *req.Filter.IsActive)
		argCounter++
	}

	countStmt := `SELECT COUNT(*)` + baseStmt
	var total int64
	if err := s.db.QueryRow(ctx, countStmt, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, err)
	}

	if total == 0 {
		return []*domain.Product{}, 0, nil
	}
	sortField := "id"
	switch req.Sort {
	case domain.SortByPrice:
		sortField = "price"
	case domain.SortByCreatedAt:
		sortField = "created_at"
	case domain.SortByName:
		sortField = "name"
	default:
		sortField = "id"
	}

	sortOrder := string(req.Order)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "ASC"
	}
	stmt := fmt.Sprintf(`
		SELECT id,name,description,price,stock,category,images,is_active,created_at,updated_at
		%s 
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, baseStmt, sortField, sortOrder, argCounter, argCounter+1)

	args = append(args, req.Limit, req.Offset)

	rows, err := s.db.Query(ctx, stmt, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, err)
	}
	res, err := scanner.Rows(rows, domain.ProductFactory)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, err)
	}
	return res, total, nil
}

func (s *Storage) SoftDelete(ctx context.Context, productID int64) error {
	const op = "storage.postgres.product.SoftDelete"
	stmt := "UPDATE productservice.products SET is_active=false, updated_at=NOW() WHERE id=$1 AND is_active=true"
	result, err := s.db.Exec(ctx, stmt, productID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
	}
	return nil
}

func (s *Storage) GetProduct(ctx context.Context, productID int64) (*domain.Product, error) {
	const op = "storage.postgres.product.GetProduct"

	stmt := `SELECT id, name, description, price, stock, category, images, is_active, created_at,updated_at
	         FROM productservice.products WHERE id = $1`

	row := s.db.QueryRow(ctx, stmt, productID)
	product := domain.ProductFactory()
	if err := product.FromRow(row); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return product, nil
}

func (s *Storage) ReserveStock(ctx context.Context, productID int64, quantity int64) (int64, error) {
	const op = "storage.postgres.product.ReserveStock"

	var newStock int64
	stmt := `UPDATE productservice.products
		SET stock = stock - $1, updated_at = NOW()
		WHERE id = $2 AND is_active = true AND stock >= $1
		RETURNING stock`
	if err := s.db.QueryRow(ctx, stmt, quantity, productID).Scan(&newStock); err == nil {
		return newStock, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	var active bool
	var stock int64
	if err := s.db.QueryRow(ctx, `SELECT is_active, stock FROM productservice.products WHERE id=$1`, productID).Scan(&active, &stock); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	if !active {
		return 0, fmt.Errorf("%s: %w", op, customerrors.ErrProductInactive)
	}
	return 0, fmt.Errorf("%s: %w", op, customerrors.ErrInsufficientStock)
}

func (s *Storage) ReleaseStock(ctx context.Context, productID int64, quantity int64) (int64, error) {
	const op = "storage.postgres.product.ReleaseStock"

	var newStock int64
	stmt := `UPDATE productservice.products SET stock = stock + $1, updated_at = NOW() WHERE id = $2 RETURNING stock`
	if err := s.db.QueryRow(ctx, stmt, quantity, productID).Scan(&newStock); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return newStock, nil
}

func (s *Storage) productExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	stmt := `SELECT EXISTS(SELECT 1 FROM productservice.products WHERE id=$1)`
	err := s.db.QueryRow(ctx, stmt, id).Scan(&exists)
	return exists, err
}
