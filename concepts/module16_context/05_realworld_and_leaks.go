// Module 16, Example 5 — context in the real world:
//  1. net/http — the server's r.Context() is cancelled when the client
//     disconnects (demoed with httptest + a client whose ctx times out);
//  2. os/signal.NotifyContext — graceful shutdown on Ctrl-C/SIGTERM
//     (self-sends SIGTERM after 2s so the demo terminates on its own);
//  3. the classic goroutine LEAK when nobody listens to Done(), made
//     visible with runtime.NumGoroutine(), then fixed.
//
// Run with: go run 05_realworld_and_leaks.go
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// ---------------------------------------------------------------------------
// Part 1 — net/http: the server sees the client hang up via r.Context()
// ---------------------------------------------------------------------------
// Every incoming request carries a context: r.Context(). net/http cancels it
// when the client disconnects, the client's timeout fires, or ServeHTTP
// returns. A handler doing anything slow (DB call, upstream fetch) should
// pass r.Context() down so all that work stops the instant it's pointless.
func httpDemo() {
	// handlerSaw lets main observe what happened inside the handler.
	handlerSaw := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context() // cancelled by net/http when the client goes away
		select {
		case <-time.After(2 * time.Second): // pretend: slow DB query
			fmt.Fprintln(w, "report ready")
			handlerSaw <- nil
		case <-ctx.Done():
			// Client gone — stop burning CPU/DB time on an unwanted response.
			handlerSaw <- ctx.Err()
		}
	}))
	defer srv.Close()

	// The CLIENT side of the same story: attach a 300ms timeout to the
	// request via its context. http.NewRequestWithContext threads it through
	// the whole exchange (dial, write, read).
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)

	_, err := http.DefaultClient.Do(req)
	fmt.Println("client:  Do() error:", err) // ...context deadline exceeded
	fmt.Println("client:  timed out?", errors.Is(err, context.DeadlineExceeded))
	fmt.Println("handler: r.Context() reported:", <-handlerSaw) // context canceled

	// Same idea, batteries included, in other stdlib APIs (prose, not run here):
	//   db.QueryContext(ctx, ...)            — database/sql cancels the query
	//   exec.CommandContext(ctx, "sleep", "9") — kills the child process on cancel
}

// ---------------------------------------------------------------------------
// Part 2 — os/signal.NotifyContext: graceful shutdown
// ---------------------------------------------------------------------------
// NotifyContext turns OS signals into context cancellation: the returned ctx
// is cancelled on the first SIGINT/SIGTERM. Your whole program becomes one
// context tree rooted here — servers, workers, pollers all wind down from a
// single Ctrl-C. This IS the modern main() skeleton for services.
func signalDemo() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM) // os.Interrupt == Ctrl-C (SIGINT)
	defer stop() // restores default signal behavior (2nd Ctrl-C then kills hard)

	// So the demo ends without a human: send OURSELVES a SIGTERM after 2s.
	// In real life you delete these three lines and let Ctrl-C / the
	// orchestrator (Kubernetes sends SIGTERM) do it.
	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("  (self-sending SIGTERM)")
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	fmt.Println("working... press Ctrl-C to stop (or wait 2s for the self-SIGTERM)")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			fmt.Println("  ...doing periodic work")
		case <-ctx.Done():
			// Graceful shutdown: flush buffers, close listeners, drain queues.
			fmt.Println("  signal received — shutting down cleanly:", ctx.Err())
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Part 3 — goroutine leaks: nobody listening to Done()
// ---------------------------------------------------------------------------

// LEAKY: the worker sends its result on an UNBUFFERED channel and ignores
// ctx. If the caller gives up (timeout) and stops receiving, the send blocks
// forever — the goroutine (and everything it references) can never be
// collected. Repeat per request and the process slowly drowns.
func leakyFetch(ctx context.Context) (string, error) {
	result := make(chan string) // unbuffered: send blocks until someone receives
	go func() {
		time.Sleep(200 * time.Millisecond) // slow work
		result <- "data"                   // <-- blocks FOREVER once caller left
	}()
	select {
	case r := <-result:
		return r, nil
	case <-ctx.Done():
		return "", ctx.Err() // caller bails... but the goroutine stays behind
	}
}

// FIXED: two independent fixes, use either (or both):
//
//	a) buffer the channel (cap 1) so the final send never blocks;
//	b) have the goroutine ALSO select on ctx.Done() so it can abort early.
func fixedFetch(ctx context.Context) (string, error) {
	result := make(chan string, 1) // (a) send completes even with no receiver
	go func() {
		select {
		case <-time.After(200 * time.Millisecond):
			result <- "data"
		case <-ctx.Done(): // (b) stop working as soon as nobody cares
			return
		}
	}()
	select {
	case r := <-result:
		return r, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func leakDemo() {
	measure := func(name string, fetch func(context.Context) (string, error)) {
		before := runtime.NumGoroutine()
		for i := 0; i < 20; i++ { // 20 requests that all time out
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			_, _ = fetch(ctx)
			cancel()
		}
		time.Sleep(400 * time.Millisecond) // let well-behaved goroutines finish
		after := runtime.NumGoroutine()
		fmt.Printf("  %-11s goroutines before=%d after=%d (stuck: %d)\n",
			name+":", before, after, after-before)
	}
	measure("leakyFetch", leakyFetch) // stuck: ~20 — one zombie per request
	measure("fixedFetch", fixedFetch) // stuck: ~0
}

func main() {
	fmt.Println("== 1. net/http: client timeout cancels the server handler ==")
	httpDemo()

	fmt.Println("\n== 2. goroutine leak vs fix (runtime.NumGoroutine) ==")
	leakDemo()

	fmt.Println("\n== 3. signal.NotifyContext: graceful shutdown ==")
	signalDemo()

	fmt.Println("\nall demos finished")
}
