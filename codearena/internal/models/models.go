// Package models defines the shared data types used across the application.
package models

import "time"

// Run statuses. These values are a fixed contract with the database and the
// frontend.
const (
	StatusQueued            = "queued"
	StatusRunning           = "running"
	StatusSuccess           = "success"
	StatusCompileError      = "compile_error"
	StatusRuntimeError      = "runtime_error"
	StatusTimeLimitExceeded = "time_limit_exceeded"
	StatusInternalError     = "internal_error"
)

// User mirrors the users table. PasswordHash is never serialized.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Run mirrors the runs table: one execution of an arbitrary user program.
type Run struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Language  string    `json:"language"`
	Code      string    `json:"code"`
	Status    string    `json:"status"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	ExitCode  int       `json:"exit_code"`
	RuntimeMS int       `json:"runtime_ms"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RunSummary is the list-view projection of a run (recent-runs sidebar).
type RunSummary struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	ExitCode  int       `json:"exit_code"`
	RuntimeMS int       `json:"runtime_ms"`
	CreatedAt time.Time `json:"created_at"`
	Snippet   string    `json:"snippet"`
}

// Workspace statuses. A workspace is "running" when its Deployment is scaled to
// 1 and "stopped" when hibernated (scaled to 0, PVC retained).
const (
	WorkspaceCreating = "creating"
	WorkspaceRunning  = "running"
	WorkspaceStopped  = "stopped"
	WorkspaceError    = "error"
)

// Workspace mirrors the workspaces table: one persistent, per-user project.
// It maps to a Deployment + PVC + Service + Ingress managed by internal/workspace.
type Workspace struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// PreviewURL is populated by the API layer (not stored) from config.
	PreviewURL string `json:"preview_url,omitempty"`
}
