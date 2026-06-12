package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/amura/codearena/internal/db"
	"github.com/amura/codearena/internal/models"
)

// handleCreateWorkspace provisions a new workspace: it inserts the row, then
// creates the Kubernetes objects. If the k8s create fails the row is marked
// "error" so the user can retry/delete.
func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())
	if s.workspaces == nil {
		writeError(w, http.StatusServiceUnavailable, "workspaces are not enabled on this server")
		return
	}

	var req struct {
		Name  string `json:"name"`
		Image string `json:"image"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	ws, err := s.store.CreateWorkspace(r.Context(), userID, req.Name, req.Image)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create workspace")
		return
	}

	if err := s.workspaces.Create(r.Context(), ws); err != nil {
		slog.Error("provision workspace", "id", ws.ID, "error", err)
		_ = s.store.SetWorkspaceStatus(r.Context(), ws.ID, models.WorkspaceError)
		ws.Status = models.WorkspaceError
		s.decorate(&ws)
		writeJSON(w, http.StatusInternalServerError, ws)
		return
	}
	_ = s.store.SetWorkspaceStatus(r.Context(), ws.ID, models.WorkspaceRunning)
	ws.Status = models.WorkspaceRunning
	s.decorate(&ws)
	writeJSON(w, http.StatusCreated, ws)
}

// handleListWorkspaces returns the caller's workspaces.
func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())
	list, err := s.store.ListUserWorkspaces(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list workspaces")
		return
	}
	for i := range list {
		s.decorate(&list[i])
	}
	writeJSON(w, http.StatusOK, list)
}

// handleGetWorkspace returns one workspace owned by the caller.
func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())
	ws, err := s.store.GetWorkspace(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		s.writeStoreErr(w, err)
		return
	}
	s.decorate(&ws)
	writeJSON(w, http.StatusOK, ws)
}

// handleStartWorkspace resumes a hibernated workspace.
func (s *Server) handleStartWorkspace(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, models.WorkspaceRunning, func(id string) error {
		return s.workspaces.Start(r.Context(), id)
	})
}

// handleStopWorkspace hibernates a workspace (scale to 0, keep the PVC).
func (s *Server) handleStopWorkspace(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, models.WorkspaceStopped, func(id string) error {
		return s.workspaces.Stop(r.Context(), id)
	})
}

// handleDeleteWorkspace tears down the Kubernetes objects and the row.
func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFrom(r.Context())
	id := r.PathValue("id")
	if _, err := s.store.GetWorkspace(r.Context(), id, userID); err != nil {
		s.writeStoreErr(w, err)
		return
	}
	if s.workspaces != nil {
		if err := s.workspaces.Delete(r.Context(), id); err != nil {
			slog.Error("delete workspace objects", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "could not delete workspace resources")
			return
		}
	}
	if err := s.store.DeleteWorkspace(r.Context(), id, userID); err != nil {
		s.writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// transition validates ownership, runs a k8s scale op, and records the new status.
func (s *Server) transition(w http.ResponseWriter, r *http.Request, status string, op func(id string) error) {
	userID, _ := userIDFrom(r.Context())
	id := r.PathValue("id")
	ws, err := s.store.GetWorkspace(r.Context(), id, userID)
	if err != nil {
		s.writeStoreErr(w, err)
		return
	}
	if s.workspaces == nil {
		writeError(w, http.StatusServiceUnavailable, "workspaces are not enabled on this server")
		return
	}
	if err := op(id); err != nil {
		slog.Error("workspace transition", "id", id, "status", status, "error", err)
		writeError(w, http.StatusInternalServerError, "could not update workspace")
		return
	}
	_ = s.store.SetWorkspaceStatus(r.Context(), id, status)
	ws.Status = status
	s.decorate(&ws)
	writeJSON(w, http.StatusOK, ws)
}

// decorate fills derived, non-stored fields (the preview URL) for API responses.
func (s *Server) decorate(ws *models.Workspace) {
	if s.workspaces != nil {
		ws.PreviewURL = s.workspaces.PreviewURL(ws.ID)
	}
}

func (s *Server) writeStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}
