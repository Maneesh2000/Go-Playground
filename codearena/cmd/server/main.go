// Command server runs the CodeArena playground API: REST + websocket hub +
// static frontend, producing runs to Kafka and consuming live run events.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/amura/codearena/internal/config"
	"github.com/amura/codearena/internal/db"
	"github.com/amura/codearena/internal/httpapi"
	"github.com/amura/codearena/internal/queue"
	"github.com/amura/codearena/internal/ws"
)

const dependencyBootBudget = 60 * time.Second

// kafkaPublisher adapts a kafka writer to the httpapi.RunPublisher interface.
type kafkaPublisher struct {
	writer *kafka.Writer
}

func (p *kafkaPublisher) PublishRun(ctx context.Context, runID string) error {
	return queue.Publish(ctx, p.writer, runID, queue.RunMessage{RunID: runID})
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Postgres (retry so compose ordering doesn't matter).
	pool, err := db.ConnectWithRetry(ctx, cfg.DatabaseURL, dependencyBootBudget)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	store := db.NewStore(pool)

	if err := db.Migrate(ctx, pool, cfg.MigrationsDir); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}
	if err := db.EnsureDemoUser(ctx, pool); err != nil {
		// Non-fatal: the users table may not exist yet if migrations are absent.
		slog.Warn("ensure demo user", "error", err)
	}

	// Kafka topics: ensure in the background so HTTP can start serving
	// immediately even if Kafka comes up late.
	go func() {
		if err := queue.EnsureTopicsWithRetry(ctx, cfg.KafkaBrokers, dependencyBootBudget,
			queue.TopicRuns, queue.TopicRunEvents); err != nil {
			slog.Error("ensure kafka topics", "error", err)
		}
	}()

	// Producer for new runs; consumer for live run events.
	writer := queue.NewWriter(cfg.KafkaBrokers, queue.TopicRuns)
	defer writer.Close()
	reader := queue.NewReader(cfg.KafkaBrokers, queue.TopicRunEvents, "codearena-server")
	defer reader.Close()

	hub := ws.NewHub()
	go consumeRunEvents(ctx, reader, hub)

	api := httpapi.New(cfg, store, hub, &kafkaPublisher{writer: writer}, "./web")
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		slog.Error("http server failed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown", "error", err)
	}
	slog.Info("server stopped")
}

// consumeRunEvents forwards run events (status/chunk/done) from Kafka to the
// owning user's websockets. The reader retries internally, so this simply
// loops until the context is cancelled.
func consumeRunEvents(ctx context.Context, reader *kafka.Reader, hub *ws.Hub) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("fetch run event", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		var ev queue.RunEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			slog.Error("malformed run event, skipping", "error", err, "raw", string(msg.Value))
		} else {
			hub.SendToUser(ev.UserID, "run_event", ev)
			if ev.Type != queue.EventChunk { // chunk events would spam the log
				slog.Info("forwarded run event",
					"run_id", ev.RunID, "user_id", ev.UserID, "type", ev.Type, "status", ev.Status)
			}
		}

		if err := reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			slog.Error("commit run event", "error", err)
		}
	}
}
