// Package db provides the Postgres connection pool, the migration runner,
// and all SQL queries used by the application.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectWithRetry opens a pgx pool and verifies connectivity, retrying with
// backoff until maxWait elapses. This lets the binaries boot before Postgres
// is ready (e.g. under docker-compose without ordering guarantees).
func ConnectWithRetry(ctx context.Context, url string, maxWait time.Duration) (*pgxpool.Pool, error) {
	deadline := time.Now().Add(maxWait)
	backoff := time.Second

	var lastErr error
	for {
		pool, err := pgxpool.New(ctx, url)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = pool.Ping(pingCtx)
			cancel()
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("database not reachable after %s: %w", maxWait, lastErr)
		}
		slog.Warn("database not ready, retrying", "error", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
}

// Store wraps the connection pool with typed query methods.
type Store struct {
	Pool *pgxpool.Pool
}

// NewStore creates a Store backed by the given pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool}
}
