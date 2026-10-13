package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgTransactor struct {
	pool *pgxpool.Pool
}

func NewPgTransactor(pool *pgxpool.Pool) *PgTransactor {
	return &PgTransactor{pool: pool}
}

func (t *PgTransactor) WithinTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
