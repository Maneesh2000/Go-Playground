// Command agent runs inside a workspace pod. It serves the workspace filesystem
// and an interactive PTY over a WebSocket at /agent, which the control-plane
// server dials and proxies to the browser IDE. It also serves /healthz.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"github.com/amura/codearena/internal/agent"
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	port := getenv("AGENT_PORT", "8081")
	root := getenv("WORKSPACE_ROOT", "/workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		slog.Error("create workspace root", "root", root, "error", err)
		os.Exit(1)
	}

	// The control plane (in-cluster) is the only client; it authenticates the
	// browser before proxying, so origin checks here are not the trust boundary.
	upgrader := websocket.Upgrader{
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
		CheckOrigin:     func(*http.Request) bool { return true },
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /agent", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("upgrade", "error", err)
			return
		}
		slog.Info("agent session opened", "remote", r.RemoteAddr)
		agent.Serve(ws, root)
		slog.Info("agent session closed", "remote", r.RemoteAddr)
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("workspace agent listening", "addr", srv.Addr, "root", root)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("agent server failed", "error", err)
		os.Exit(1)
	}
}
