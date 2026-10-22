// Package db owns swosh's PostgreSQL schema and connections to it.
//
// The schema lives in migrations/ as golang-migrate files that are embedded
// into the binary, so Migrate can bring any database up to date. The queries
// in queries/ are compiled into the sqlc sub-package; run `make generate`
// after changing either directory.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a connection pool to databaseURL and checks that the database
// is reachable.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
