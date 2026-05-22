package scanner

import "github.com/jackc/pgx/v5"

func Rows[T Scannable](rows pgx.Rows, factory ScannableFactory[T]) ([]T, error) {
	res := []T{}
	for rows.Next() {
		t := factory()
		if err := t.FromRow(rows); err != nil {
			return []T{}, err
		}
		res = append(res, t)
	}

	return res, nil
}