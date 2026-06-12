package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/amura/codearena/internal/auth"
	"github.com/amura/codearena/internal/models"
)

// wsUpgrader upgrades the browser connection. Origin is allowed broadly for the
// dev/lab setup; the JWT (query param) is the real access control.
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// handleWorkspaceWS is the Eval-style reverse proxy: it authenticates the user,
// checks they own the workspace, resumes it if hibernated, dials the in-pod
// agent, and copies WebSocket frames between the browser and the agent.
//
// Browsers can't set Authorization headers on WebSockets, so the JWT arrives as
// ?token=... (same convention as the run-events /ws endpoint).
func (s *Server) handleWorkspaceWS(w http.ResponseWriter, r *http.Request) {
	if s.workspaces == nil {
		http.Error(w, "workspaces not enabled", http.StatusServiceUnavailable)
		return
	}
	userID, err := auth.ParseToken(s.cfg.JWTSecret, r.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, "invalid or missing token", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	ws, err := s.store.GetWorkspace(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "workspace not found", http.StatusNotFound)
		return
	}

	// Resume-on-open: ensure the workspace is running before we try to reach it.
	if ws.Status != models.WorkspaceRunning {
		if err := s.workspaces.Start(r.Context(), id); err != nil {
			http.Error(w, "could not start workspace", http.StatusInternalServerError)
			return
		}
		_ = s.store.SetWorkspaceStatus(r.Context(), id, models.WorkspaceRunning)
	}

	// Dial the agent first (with retry for a cold/resuming pod) so a failure is a
	// clean HTTP error instead of an opened-then-closed browser socket.
	agentConn, err := s.dialAgent(r.Context(), s.workspaces.AgentEndpoint(id))
	if err != nil {
		slog.Error("dial workspace agent", "workspace", id, "error", err)
		http.Error(w, "workspace agent unreachable (still starting?)", http.StatusBadGateway)
		return
	}
	defer agentConn.Close()

	browserConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the error
	}
	defer browserConn.Close()
	slog.Info("workspace proxy open", "workspace", id, "user", userID)

	proxyWS(browserConn, agentConn)
	slog.Info("workspace proxy closed", "workspace", id, "user", userID)
}

// dialAgent connects to ws://<endpoint>/agent, retrying while the pod finishes
// starting (first resume may need an image pull), up to a deadline.
func (s *Server) dialAgent(ctx context.Context, endpoint string) (*websocket.Conn, error) {
	url := "ws://" + endpoint + "/agent"
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for {
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return nil, lastErr
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// proxyWS copies frames in both directions until either side closes. Each conn
// has exactly one reader and one writer goroutine, which is gorilla-safe.
func proxyWS(browser, agent *websocket.Conn) {
	done := make(chan struct{}, 2)
	pipe := func(dst, src *websocket.Conn) {
		defer func() { done <- struct{}{} }()
		for {
			mt, data, err := src.ReadMessage()
			if err != nil {
				return
			}
			if err := dst.WriteMessage(mt, data); err != nil {
				return
			}
		}
	}
	go pipe(agent, browser) // browser -> agent
	go pipe(browser, agent) // agent -> browser
	<-done                  // first side to close tears down both (defers Close)
}
