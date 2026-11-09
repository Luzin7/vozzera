package chat

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgTransactor struct {
	pool *pgxpool.Pool
}

func NewPgTransactor(pool *pgxpool.Pool) *PgTransactor {
	return &PgTransactor{pool: pool}
}

func (t *PgTransactor) WithinTx(ctx context.Context, limits TxLimits, fn func(Repository) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := applyTxLimits(ctx, tx, limits); err != nil {
		return err
	}

	if err := fn(New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func applyTxLimits(ctx context.Context, tx pgx.Tx, limits TxLimits) error {
	if err := setLocalTimeout(ctx, tx, "lock_timeout", limits.LockTimeout); err != nil {
		return err
	}
	return setLocalTimeout(ctx, tx, "statement_timeout", limits.StatementTimeout)
}

func setLocalTimeout(ctx context.Context, tx pgx.Tx, setting string, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	_, err := tx.Exec(ctx, "SET LOCAL "+setting+" = '"+strconv.FormatInt(timeout.Milliseconds(), 10)+"ms'")
	return err
}
