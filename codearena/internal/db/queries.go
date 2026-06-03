package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/amura/codearena/internal/models"
)

// Sentinel errors that let handlers map DB failures to HTTP status codes
// without importing pgx directly.
var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
)

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrDuplicate
	}
	return err
}

// --- users ---

func (s *Store) CreateUser(ctx context.Context, username, email, passwordHash string) (models.User, error) {
	var u models.User
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, password_hash, created_at`,
		username, email, passwordHash,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, mapErr(err)
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	err := s.Pool.QueryRow(ctx, `
		SELECT id, username, email, password_hash, created_at
		FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, mapErr(err)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	var u models.User
	err := s.Pool.QueryRow(ctx, `
		SELECT id, username, email, password_hash, created_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, mapErr(err)
}

// --- runs ---

const runColumns = `id::text, user_id, language, code, status, output, error,
	exit_code, runtime_ms, created_at, updated_at`

func scanRun(row pgx.Row) (models.Run, error) {
	var r models.Run
	err := row.Scan(&r.ID, &r.UserID, &r.Language, &r.Code, &r.Status, &r.Output,
		&r.Error, &r.ExitCode, &r.RuntimeMS, &r.CreatedAt, &r.UpdatedAt)
	return r, mapErr(err)
}

func (s *Store) CreateRun(ctx context.Context, userID int64, language, code string) (models.Run, error) {
	return scanRun(s.Pool.QueryRow(ctx, `
		INSERT INTO runs (user_id, language, code)
		VALUES ($1, $2, $3)
		RETURNING `+runColumns,
		userID, language, code))
}

func (s *Store) GetRun(ctx context.Context, id string) (models.Run, error) {
	return scanRun(s.Pool.QueryRow(ctx, `
		SELECT `+runColumns+` FROM runs WHERE id = $1`, id))
}

// ListUserRuns returns the caller's most recent runs (newest first) with a
// short code snippet instead of the full source.
func (s *Store) ListUserRuns(ctx context.Context, userID int64, limit int) ([]models.RunSummary, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id::text, status, exit_code, runtime_ms, created_at, left(code, 200)
		FROM runs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.RunSummary{}
	for rows.Next() {
		var r models.RunSummary
		if err := rows.Scan(&r.ID, &r.Status, &r.ExitCode, &r.RuntimeMS, &r.CreatedAt, &r.Snippet); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetRunStatus updates only the status column (e.g. queued -> running).
func (s *Store) SetRunStatus(ctx context.Context, id, status string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE runs SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

// FinishRun records the final outcome of a run.
func (s *Store) FinishRun(ctx context.Context, id, status, output, errMsg string, exitCode, runtimeMS int) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE runs
		SET status = $2, output = $3, error = $4, exit_code = $5,
		    runtime_ms = $6, updated_at = now()
		WHERE id = $1`,
		id, status, output, errMsg, exitCode, runtimeMS)
	return err
}

// --- workspaces ---

const workspaceColumns = `id::text, user_id, name, image, status, created_at, updated_at`

func scanWorkspace(row pgx.Row) (models.Workspace, error) {
	var w models.Workspace
	err := row.Scan(&w.ID, &w.UserID, &w.Name, &w.Image, &w.Status, &w.CreatedAt, &w.UpdatedAt)
	return w, mapErr(err)
}

// CreateWorkspace inserts a new workspace row in the "creating" state.
func (s *Store) CreateWorkspace(ctx context.Context, userID int64, name, image string) (models.Workspace, error) {
	return scanWorkspace(s.Pool.QueryRow(ctx, `
		INSERT INTO workspaces (user_id, name, image, status)
		VALUES ($1, $2, $3, $4)
		RETURNING `+workspaceColumns,
		userID, name, image, models.WorkspaceCreating))
}

// GetWorkspace returns a workspace by id scoped to its owner (defense in depth:
// callers must pass the authenticated user so one user can't read another's).
func (s *Store) GetWorkspace(ctx context.Context, id string, userID int64) (models.Workspace, error) {
	return scanWorkspace(s.Pool.QueryRow(ctx, `
		SELECT `+workspaceColumns+` FROM workspaces WHERE id = $1 AND user_id = $2`, id, userID))
}

// ListUserWorkspaces returns the caller's workspaces, newest first.
func (s *Store) ListUserWorkspaces(ctx context.Context, userID int64) ([]models.Workspace, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT `+workspaceColumns+` FROM workspaces WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Workspace{}
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// SetWorkspaceStatus updates the lifecycle status (creating/running/stopped/error).
func (s *Store) SetWorkspaceStatus(ctx context.Context, id, status string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE workspaces SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

// DeleteWorkspace removes the row (Kubernetes objects are torn down separately).
func (s *Store) DeleteWorkspace(ctx context.Context, id string, userID int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
