package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func inTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			_ = tx.Rollback(rollbackCtx)
			cancel()
			panic(recovered)
		}
		if err != nil {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			rollbackErr := tx.Rollback(rollbackCtx)
			cancel()
			if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
