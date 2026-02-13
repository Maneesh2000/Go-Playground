// Package httpapi implements the REST API, websocket endpoint and static
// file serving for the CodeArena server.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/amura/codearena/internal/config"
	"github.com/amura/codearena/internal/db"
	"github.com/amura/codearena/internal/ws"
)

// RunPublisher enqueues a run id for execution.
type RunPublisher interface {
	PublishRun(ctx context.Context, runID string) error
}

// Server wires config, storage, websocket hub and the Kafka producer into an
// http.Handler.
type Server struct {
	cfg       config.Config
	store     *db.Store
	hub       *ws.Hub
	publisher RunPublisher
	staticDir string
}

// New creates the API server. staticDir is the directory served at "/"
// (normally ./web, owned by the frontend agent).
func New(cfg config.Config, store *db.Store, hub *ws.Hub, publisher RunPublisher, staticDir string) *Server {
	return &Server{cfg: cfg, store: store, hub: hub, publisher: publisher, staticDir: staticDir}
}

// Handler builds the full route table with middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/register", s.handleRegister)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("GET /ws", s.handleWS)

	// Authenticated endpoints.
	mux.Handle("GET /api/me", s.auth(s.handleMe))
	mux.Handle("POST /api/runs", s.auth(s.handleCreateRun))
	mux.Handle("GET /api/runs", s.auth(s.handleListRuns))
	mux.Handle("GET /api/runs/{id}", s.auth(s.handleGetRun))

	// Static SPA at the root; everything above is more specific and wins.
	mux.Handle("/", s.spaHandler())

	return chain(mux, recoverMiddleware, corsMiddleware, loggingMiddleware)
}

// spaHandler serves files from the static dir and falls back to index.html
// for client-side routes. API and WS paths never reach it via fallback.
func (s *Server) spaHandler() http.Handler {
	fileServer := http.FileServer(http.Dir(s.staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/ws") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}

		// Serve the file if it exists on disk.
		clean := filepath.Join(s.staticDir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for anything else.
		index := filepath.Join(s.staticDir, "index.html")
		if _, err := os.Stat(index); err == nil {
			http.ServeFile(w, r, index)
			return
		}
		http.NotFound(w, r)
	})
}

// --- small JSON helpers shared by all handlers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	return dec.Decode(v)
}
