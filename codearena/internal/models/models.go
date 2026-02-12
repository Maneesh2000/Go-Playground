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
