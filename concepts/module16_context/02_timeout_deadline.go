// Module 16, Example 2 — WithTimeout and WithDeadline: racing slow work
// against ctx.Done(), inspecting Err(), and how a timeout at the top of a
// call tree cancels everything underneath it.
//
// Run with: go run 02_timeout_deadline.go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// slowOperation pretends to be an RPC/DB call that takes `cost` to finish.
// The canonical shape: RACE the work against ctx.Done() in a select.
// Whichever happens first wins; if the context wins, we return ctx.Err().
func slowOperation(ctx context.Context, name string, cost time.Duration) error {
	select {
	case <-time.After(cost): // the "work" (in real code: a channel from the work)
		fmt.Printf("  %s: finished after %v\n", name, cost)
		return nil
	case <-ctx.Done(): // the context expired or was cancelled first
		fmt.Printf("  %s: abandoned (%v)\n", name, ctx.Err())
		return ctx.Err() // convention: return ctx.Err(), don't invent your own
	}
}

// A three-level call tree. Note that NOBODY below the top adds a timeout —
// they just pass ctx down. The top-level deadline governs all of them.
//
//	handle (150ms budget)
//	   └── fetchUser
//	          ├── queryDB      (fast, fits in budget)
//	          └── fetchAvatar  (slow, blows the budget → cancelled)
func handle(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel() // release the timer even if we finish early — ALWAYS
	return fetchUser(ctx)
}

func fetchUser(ctx context.Context) error {
	if err := slowOperation(ctx, "queryDB", 60*time.Millisecond); err != nil {
		return fmt.Errorf("queryDB: %w", err)
	}
	if err := slowOperation(ctx, "fetchAvatar", 500*time.Millisecond); err != nil {
		return fmt.Errorf("fetchAvatar: %w", err)
	}
	return nil
}

// cpuBoundLoop shows the OTHER way to honor a context: work that never blocks
// on a channel (pure computation) can't select on Done(), so it polls
// ctx.Err() every iteration (or every N iterations if checks are costly).
func cpuBoundLoop(ctx context.Context) (int, error) {
	sum := 0
	for i := 0; ; i++ {
		if err := ctx.Err(); err != nil { // non-blocking "am I cancelled?"
			return sum, err
		}
		sum += i
		time.Sleep(10 * time.Millisecond) // stand-in for a chunk of real work
	}
}

func main() {
	// ---- WithTimeout vs WithDeadline ----------------------------------------
	// They are the same mechanism expressed two ways:
	//   WithTimeout(ctx, 2*time.Second)          — relative: "2s from NOW"
	//   WithDeadline(ctx, someAbsoluteTime)       — absolute: "until 14:05:00"
	// WithTimeout(ctx, d) is literally WithDeadline(ctx, time.Now().Add(d)).
	// Use WithDeadline when the wall-clock instant comes from elsewhere
	// (e.g. an upstream service told you "respond by T").
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Callees can inspect the budget via Deadline():
	if dl, ok := ctx.Deadline(); ok {
		fmt.Printf("deadline in ~%v\n", time.Until(dl).Round(time.Millisecond))
	}

	// Race 1: work (40ms) beats the deadline (100ms) — completes.
	_ = slowOperation(ctx, "fast-call", 40*time.Millisecond)
	// Race 2: work (300ms) loses to the remaining ~60ms — abandoned.
	err := slowOperation(ctx, "slow-call", 300*time.Millisecond)

	// ---- Inspecting Err() after expiry ---------------------------------------
	// After the deadline fires: ctx.Err() == context.DeadlineExceeded.
	// After a manual cancel(): ctx.Err() == context.Canceled.
	// Use errors.Is — timeouts often arrive wrapped in %w chains.
	fmt.Println("ctx.Err() =", ctx.Err())
	fmt.Println("timed out?", errors.Is(err, context.DeadlineExceeded)) // true
	fmt.Println("cancelled?", errors.Is(err, context.Canceled))         // false

	// ---- Timeout cancelling a whole call tree --------------------------------
	// The 150ms budget lives ONLY in handle(); queryDB fits, fetchAvatar is
	// cut off mid-flight. One deadline, enforced everywhere ctx reaches.
	fmt.Println("\ncall tree with a 150ms budget:")
	if err := handle(context.Background()); err != nil {
		fmt.Println("handle returned:", err) // fetchAvatar: context deadline exceeded
	}

	// ---- Polling ctx.Err() in CPU-bound loops --------------------------------
	fmt.Println("\ncpu-bound loop under an 80ms deadline:")
	loopCtx, loopCancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer loopCancel()
	sum, err := cpuBoundLoop(loopCtx)
	fmt.Printf("loop stopped with sum=%d, err=%v\n", sum, err)

	// ---- WithDeadline directly ------------------------------------------------
	// Same behavior, absolute time. Also: deriving a LONGER deadline from a
	// shorter parent has no effect — the parent's (sooner) deadline wins.
	dlCtx, dlCancel := context.WithDeadline(context.Background(),
		time.Now().Add(50*time.Millisecond))
	defer dlCancel()
	looser, looserCancel := context.WithTimeout(dlCtx, time.Hour) // wish granted? no.
	defer looserCancel()
	if dl, ok := looser.Deadline(); ok {
		fmt.Printf("\nchild asked for 1h, effective deadline is still ~%v away\n",
			time.Until(dl).Round(10*time.Millisecond))
	}
}
