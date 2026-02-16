// Command worker runs the CodeArena playground executor: it consumes run ids
// from Kafka, executes the programs (locally or in Kubernetes Jobs), streams
// output events and records verdicts in Postgres.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amura/codearena/internal/config"
	"github.com/amura/codearena/internal/db"
	"github.com/amura/codearena/internal/executor"
	"github.com/amura/codearena/internal/queue"
	"github.com/amura/codearena/internal/runner"
)

const dependencyBootBudget = 60 * time.Second

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

	// Kafka topics must exist before consuming; retry within the boot budget.
	if err := queue.EnsureTopicsWithRetry(ctx, cfg.KafkaBrokers, dependencyBootBudget,
		queue.TopicRuns, queue.TopicRunEvents); err != nil {
		slog.Error("ensure kafka topics", "error", err)
		os.Exit(1)
	}

	// Pick the execution backend.
	var exec executor.Executor
	switch cfg.Executor {
	case "k8s":
		k8sExec, err := executor.NewK8sExecutor(cfg.K8sNamespace, cfg.RunnerImage)
		if err != nil {
			slog.Error("initialize k8s executor", "error", err)
			os.Exit(1)
		}
		exec = k8sExec
		slog.Info("using kubernetes executor", "namespace", cfg.K8sNamespace, "image", cfg.RunnerImage)
	case "local":
		exec = executor.NewLocalExecutor()
		slog.Info("using local executor")
	default:
		slog.Error("unknown EXECUTOR value", "executor", cfg.Executor)
		os.Exit(1)
	}

	reader := queue.NewReader(cfg.KafkaBrokers, queue.TopicRuns, "codearena-workers")
	defer reader.Close()
	writer := queue.NewWriter(cfg.KafkaBrokers, queue.TopicRunEvents)
	defer writer.Close()

	worker := &runner.Worker{
		Store:       store,
		Executor:    exec,
		Reader:      reader,
		Writer:      writer,
		TimeLimitMS: cfg.RunTimeoutMS,
	}
	if err := worker.Run(ctx); err != nil {
		slog.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
	slog.Info("worker stopped")
}
