package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/amura/codearena/internal/auth"
	"github.com/amura/codearena/internal/db"
	"github.com/amura/codearena/internal/models"
)

const (
	maxCodeBytes = 64 * 1024
	runsListCap  = 20
)

type userResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func toUserResponse(u models.User) userResponse {
	return userResponse{ID: u.ID, Username: u.Username, Email: u.Email}
}

// POST /api/register
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username, email and password are required")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.internalError(w, "hash password", err)
		return
	}

	user, err := s.store.CreateUser(r.Context(), req.Username, req.Email, hash)
	if errors.Is(err, db.ErrDuplicate) {
		writeError(w, http.StatusConflict, "username or email already taken")
		return
	}
	if err != nil {
		s.internalError(w, "create user", err)
		return
	}

	token, err := auth.GenerateToken(s.cfg.JWTSecret, user.ID)
	if err != nil {
		s.internalError(w, "generate token", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token": token,
		"user":  toUserResponse(user),
	})
}

// POST /api/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	user, err := s.store.GetUserByUsername(r.Context(), strings.TrimSpace(req.Username))
	if errors.Is(err, db.ErrNotFound) || (err == nil && !auth.CheckPassword(user.PasswordHash, req.Password)) {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		s.internalError(w, "load user", err)
		return
	}

	token, err := auth.GenerateToken(s.cfg.JWTSecret, user.ID)
	if err != nil {
		s.internalError(w, "generate token", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  toUserResponse(user),
	})
}

// GET /api/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())
	user, err := s.store.GetUserByID(r.Context(), userID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "user no longer exists")
		return
	}
	if err != nil {
		s.internalError(w, "load user", err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

// POST /api/runs
func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())

	var req struct {
		Code     string `json:"code"`
		Language string `json:"language"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "code must not be empty")
		return
	}
	if len(req.Code) > maxCodeBytes {
		writeError(w, http.StatusBadRequest, "code must be at most 64KB")
		return
	}
	if req.Language == "" {
		req.Language = "go"
	}

	run, err := s.store.CreateRun(r.Context(), userID, req.Language, req.Code)
	if err != nil {
		s.internalError(w, "create run", err)
		return
	}

	if err := s.publisher.PublishRun(r.Context(), run.ID); err != nil {
		slog.Error("publish run", "run_id", run.ID, "error", err)
		// The worker will never see it; mark it dead so the client isn't left
		// waiting on a queued run forever.
		if uerr := s.store.SetRunStatus(r.Context(), run.ID, models.StatusInternalError); uerr != nil {
			slog.Error("mark run internal_error", "run_id", run.ID, "error", uerr)
		}
		writeJSON(w, http.StatusAccepted, map[string]string{
			"id":     run.ID,
			"status": models.StatusInternalError,
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"id":     run.ID,
		"status": models.StatusQueued,
	})
}

// GET /api/runs/{id} — polling fallback for the UI.
func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())
	id := r.PathValue("id")

	run, err := s.store.GetRun(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) || (err == nil && run.UserID != userID) {
		// Hide other users' runs behind a 404.
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if err != nil {
		s.internalError(w, "load run", err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// GET /api/runs — the caller's 20 most recent runs, newest first.
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())

	runs, err := s.store.ListUserRuns(r.Context(), userID, runsListCap)
	if err != nil {
		s.internalError(w, "list runs", err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

// GET /ws?token=<jwt>
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing token query parameter")
		return
	}
	userID, err := auth.ParseToken(s.cfg.JWTSecret, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	s.hub.ServeWS(w, r, userID)
}

func (s *Server) internalError(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
