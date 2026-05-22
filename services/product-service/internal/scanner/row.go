package scanner


import "github.com/jackc/pgx/v5"

func Row[T Scannable](row pgx.Row, factory ScannableFactory[T]) (T, error) {
	t := factory()
	err := t.FromRow(row)
	return t, err
}