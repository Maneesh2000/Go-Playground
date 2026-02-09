// Module 13, example 3: signals and the graceful-shutdown pattern.
//
// Run with: go run 03_signals_shutdown.go
// Then press Ctrl-C (SIGINT) — or from another terminal: kill -TERM <pid>.
// If you do nothing, the demo shuts itself down after 10 seconds.
//
// The flow (this is THE pattern for daemons and servers):
//
//	SIGTERM/SIGINT ──► signal.NotifyContext ──► ctx.Done() closes
//	                                                │
//	                              ┌─────────────────┼──────────────┐
//	                              ▼                 ▼              ▼
//	                        HTTP server        worker loop     (anything else
//	                        srv.Shutdown()     returns          watching ctx)
//	                              │
//	                              ▼
//	                        cleanup: flush, close, remove pidfile → exit 0
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	fmt.Printf("pid %d — press Ctrl-C to shut down gracefully (auto-stop in 10s)\n", os.Getpid())

	// NotifyContext turns "a signal arrived" into "this context is done".
	// One line replaces the old channel + goroutine boilerplate.
	// os.Interrupt is SIGINT (Ctrl-C); syscall.SIGTERM is what `kill`,
	// Kubernetes, systemd, and Docker send first.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop() // restores default signal behavior (2nd Ctrl-C kills hard)

	// Demo-only: cancel automatically after 10s so the example always ends.
	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	var wg sync.WaitGroup

	// ------------------ component 1: an HTTP server ---------------------
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello — try Ctrl-C in the server's terminal")
	})
	srv := &http.Server{Addr: "127.0.0.1:8091", Handler: mux}

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("http: listening on http://127.0.0.1:8091")
		// ListenAndServe blocks until Shutdown/Close; ErrServerClosed is
		// the EXPECTED error on a clean shutdown — not a failure.
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "http:", err)
		}
	}()

	// ------------------ component 2: a background worker ----------------
	// A typical "do something every interval" loop that exits when ctx does.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Println("worker: heartbeat (pretend to process a batch)")
			case <-ctx.Done():
				fmt.Println("worker: ctx done, finishing current batch and exiting")
				time.Sleep(200 * time.Millisecond) // simulate finishing in-flight work
				fmt.Println("worker: clean exit")
				return
			}
		}
	}()

	// ---------------------- block until a signal ------------------------
	<-ctx.Done()
	fmt.Println("\nmain: shutdown requested —", ctx.Err())

	// Give in-flight HTTP requests a deadline to finish. Note this is a
	// FRESH context — the signal ctx is already cancelled, and Shutdown
	// with a dead context wouldn't wait at all.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintln(os.Stderr, "http shutdown:", err)
	} else {
		fmt.Println("http: drained and stopped")
	}

	// Wait for every component to confirm it exited, THEN clean up.
	wg.Wait()
	fmt.Println("main: all components stopped — flushing logs, closing DB (pretend)")
	fmt.Println("main: bye (exit 0)")
}
