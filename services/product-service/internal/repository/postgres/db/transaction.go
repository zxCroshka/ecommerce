package db

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Transaction(ctx context.Context, pool *pgxpool.Pool, f func(tx pgx.Tx) error) (err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		slog.Error("tx begin error", "err", err)
		return err
	}
	defer func() {
		rollback := func() {
			if rerr := tx.Rollback(ctx); rerr != nil {
				slog.Error("tx rollback error", "err", rerr)
			}
		}
		if p := recover(); p != nil {
			rollback()
			panic(p)
		}
		if err != nil {
			rollback()
		}
	}()
	err = f(tx)
	if err != nil {
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("tx commit error: %v", "err", err)
		return err
	}
	return nil
}
