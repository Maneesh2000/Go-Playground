package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amura/codearena/internal/auth"
)

// Migrate applies all *.sql files in dir (sorted by filename) that have not
// yet been recorded in schema_migrations. A missing directory is tolerated
// with a warning so the server can run before the infra pieces are in place.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("migrations directory missing, skipping migrations", "dir", dir)
			return nil
		}
		return fmt.Errorf("read migrations dir: %w", err)
	}

	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		// Apply the migration and record it in a single transaction so a
		// partial failure never marks the file as applied.
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
		slog.Info("applied migration", "file", name)
	}
	return nil
}

// EnsureDemoUser inserts the well-known demo account if it does not exist.
func EnsureDemoUser(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := auth.HashPassword("demo123")
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ('demo', 'demo@codearena.dev', $1)
		ON CONFLICT DO NOTHING`, hash)
	if err != nil {
		return fmt.Errorf("ensure demo user: %w", err)
	}
	return nil
}
