package scanner

import "github.com/jackc/pgx/v5"

type Scannable interface {
	FromRow(pgx.Row) error
}

type ScannableFactory[T Scannable] func() T
