// Package runner orchestrates the worker: consume run ids from Kafka,
// execute the program, stream output events, and persist the final verdict.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/amura/codearena/internal/db"
	"github.com/amura/codearena/internal/executor"
	"github.com/amura/codearena/internal/models"
	"github.com/amura/codearena/internal/queue"
)

// MaxOutputBytes caps the combined output stored in runs.output (64KB).
const MaxOutputBytes = 64 * 1024

// Worker consumes the runs topic, executes each run and produces run-events.
type Worker struct {
	Store       *db.Store
	Executor    executor.Executor
	Reader      *kafka.Reader
	Writer      *kafka.Writer
	TimeLimitMS int // per-run wall-clock budget (RUN_TIMEOUT_MS)
}

// Run processes messages until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	slog.Info("run worker started", "time_limit_ms", w.TimeLimitMS)
	for {
		msg, err := w.Reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			slog.Error("fetch message", "error", err)
			// Transient broker error; back off briefly and keep consuming.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
			continue
		}

		w.handle(ctx, msg.Value)

		if err := w.Reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			slog.Error("commit message", "error", err)
		}
	}
}

// handle executes a single run. Failures are absorbed (logged and, where
// possible, recorded as internal_error) so one bad message never kills the
// consume loop.
func (w *Worker) handle(ctx context.Context, raw []byte) {
	var msg queue.RunMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		slog.Error("malformed run message, skipping", "error", err, "raw", string(raw))
		return
	}
	log := slog.With("run_id", msg.RunID)

	run, err := w.Store.GetRun(ctx, msg.RunID)
	if err != nil {
		log.Error("load run", "error", err)
		return
	}

	// queued -> running: persist and notify the user's terminal.
	if err := w.Store.SetRunStatus(ctx, run.ID, models.StatusRunning); err != nil {
		log.Error("set status running", "error", err)
	}
	w.publish(ctx, queue.RunEvent{
		RunID:  run.ID,
		UserID: run.UserID,
		Type:   queue.EventStatus,
		Status: models.StatusRunning,
	})

	// emit streams each output chunk to Kafka live and accumulates the
	// combined output (capped) for the runs.output column. Executors may call
	// it from multiple goroutines, hence the mutex.
	var (
		mu  sync.Mutex
		buf strings.Builder
	)
	emit := func(stream, data string) {
		mu.Lock()
		if room := MaxOutputBytes - buf.Len(); room > 0 {
			if len(data) > room {
				buf.WriteString(data[:room])
			} else {
				buf.WriteString(data)
			}
		}
		mu.Unlock()

		w.publish(ctx, queue.RunEvent{
			RunID:  run.ID,
			UserID: run.UserID,
			Type:   queue.EventChunk,
			Stream: stream,
			Data:   data,
		})
	}

	log.Info("executing", "user_id", run.UserID, "code_bytes", len(run.Code))
	res, err := w.Executor.Execute(ctx, executor.ExecRequest{
		Code:        run.Code,
		TimeLimitMS: w.TimeLimitMS,
	}, emit)
	if err != nil {
		log.Error("execute", "error", err)
		res = executor.ExecResult{
			Status:   models.StatusInternalError,
			ExitCode: -1,
			ErrorMsg: "internal error while running the program",
		}
	}

	mu.Lock()
	output := buf.String()
	mu.Unlock()

	w.finish(ctx, run, res, output)
	log.Info("finished", "status", res.Status, "exit_code", res.ExitCode, "runtime_ms", res.RuntimeMS)
}

// finish persists the verdict and publishes the done event. It uses a
// background-derived context so a mid-run shutdown still records results.
func (w *Worker) finish(ctx context.Context, run models.Run, res executor.ExecResult, output string) {
	saveCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		saveCtx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	if err := w.Store.FinishRun(saveCtx, run.ID, res.Status, output,
		res.ErrorMsg, res.ExitCode, res.RuntimeMS); err != nil {
		slog.Error("persist run result", "run_id", run.ID, "error", err)
	}

	w.publish(saveCtx, queue.RunEvent{
		RunID:     run.ID,
		UserID:    run.UserID,
		Type:      queue.EventDone,
		Status:    res.Status,
		ExitCode:  res.ExitCode,
		RuntimeMS: res.RuntimeMS,
		Error:     res.ErrorMsg,
	})
}

func (w *Worker) publish(ctx context.Context, ev queue.RunEvent) {
	if err := queue.Publish(ctx, w.Writer, ev.RunID, ev); err != nil && ctx.Err() == nil {
		slog.Error("publish run event", "run_id", ev.RunID, "type", ev.Type, "error", err)
	}
}
