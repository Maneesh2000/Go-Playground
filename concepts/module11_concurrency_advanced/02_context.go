// Module 11, example 2: context.Context — cancellation, timeout, and values.
//
// Run with: go run 02_context.go
//
// A context forms a TREE. Deriving a child (WithCancel/WithTimeout/WithValue)
// hangs it under its parent; cancelling a parent cancels every descendant.
// This is how "the request timed out" propagates instantly to the database
// call, the cache lookup, and every goroutine spawned for that request.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ctxKey is a private type for context values. Using an unexported type means
// no other package can construct our key, so collisions are impossible.
type ctxKey string

const requestIDKey ctxKey = "requestID"

// slowOperation pretends to do `work` that takes `d`. It respects ctx: if the
// context is cancelled first, it stops early and reports why.
// Convention: ctx is ALWAYS the first parameter.
func slowOperation(ctx context.Context, name string, d time.Duration) error {
	// Pull the request ID back out of the context. Values are for
	// request-scoped metadata like this — NOT for ordinary arguments.
	reqID, _ := ctx.Value(requestIDKey).(string)

	select {
	case <-time.After(d):
		fmt.Printf("  [req %s] %-10s finished after %v\n", reqID, name, d)
		return nil
	case <-ctx.Done():
		// ctx.Err() tells us WHY: context.Canceled or context.DeadlineExceeded.
		fmt.Printf("  [req %s] %-10s aborted: %v\n", reqID, name, ctx.Err())
		return ctx.Err()
	}
}

// handleRequest simulates an HTTP handler: it derives a per-request timeout
// from the incoming context and fans work out to helpers. Every helper shares
// the SAME deadline — when it fires, they all see Done() close at once.
func handleRequest(ctx context.Context, id string, budget time.Duration) {
	ctx = context.WithValue(ctx, requestIDKey, id)

	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel() // ALWAYS call cancel — releases the timer even on success

	fmt.Printf("request %s: budget %v\n", id, budget)
	// Sequential calls sharing one deadline: the budget is for the WHOLE
	// request, not per call.
	if err := slowOperation(ctx, "db query", 40*time.Millisecond); err != nil {
		return
	}
	if err := slowOperation(ctx, "cache set", 40*time.Millisecond); err != nil {
		return
	}
	if err := slowOperation(ctx, "render", 40*time.Millisecond); err != nil {
		return
	}
	fmt.Printf("request %s: OK\n", id)
}

func main() {
	// Background() is the root of every context tree.
	root := context.Background()

	// --- 1) Timeout: enough budget → everything completes. -------------
	handleRequest(root, "A", 200*time.Millisecond)

	// --- 2) Timeout: tight budget → later stages get cut off. ----------
	// db query (40ms) + cache set (40ms) exceed the 70ms budget, so the
	// cache call is aborted with context.DeadlineExceeded.
	handleRequest(root, "B", 70*time.Millisecond)

	// --- 3) Manual cancellation with WithCancel. ------------------------
	// A goroutine watches for an "event" (here: a timer) and cancels.
	// Real-world versions cancel on user disconnect, shutdown signal, or
	// the first error from a sibling goroutine (see errgroup in the README).
	ctx, cancel := context.WithCancel(root)
	go func() {
		time.Sleep(30 * time.Millisecond)
		fmt.Println("controller: cancelling now!")
		cancel()
	}()
	ctx = context.WithValue(ctx, requestIDKey, "C")
	err := slowOperation(ctx, "long job", time.Second)
	// errors.Is works on context errors like any other error value.
	fmt.Printf("cancelled? %v\n", errors.Is(err, context.Canceled))

	// --- 4) The tree in action: cancelling a PARENT kills the child, ----
	//        but a child's cancel never affects the parent.
	parent, cancelParent := context.WithCancel(root)
	child, cancelChild := context.WithCancel(parent)
	defer cancelChild() // good hygiene even if we never use it

	cancelParent() // cancel the parent...
	<-child.Done() // ...and the child is instantly done too
	fmt.Printf("child sees parent's cancellation: %v\n", child.Err())
}
