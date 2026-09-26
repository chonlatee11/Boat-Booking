// Package pgx is the sole constructor for this project's Postgres pools and
// the shared transaction helper (D-02). Its package name is intentionally
// "pgx" (matching github.com/jackc/pgx/v5); callers import it aliased as
// bbpgx to avoid the name clash — see pkg/outbox for the pattern.
package pgx

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a connection pool to dsn and verifies it with a Ping. This is
// the only sanctioned pgxpool.New call site outside pkg/testenv (forbidigo,
// PLAT-02).
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn) //nolint:forbidigo // pkg/pgx is the sanctioned constructor (D-02)
	if err != nil {
		return nil, fmt.Errorf("pgx: new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgx: ping: %w", err)
	}
	return pool, nil
}

// WithTx runs fn inside a transaction: begins, defers a rollback (a no-op
// after a successful commit), runs fn, and commits on success.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pgx: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pgx: commit: %w", err)
	}
	return nil
}
