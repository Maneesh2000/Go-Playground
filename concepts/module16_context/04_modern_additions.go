// Module 16, Example 4 — The Go 1.20/1.21 additions:
// context.WithCancelCause + context.Cause (WHY was I cancelled?),
// context.WithoutCancel (detach from the parent's cancellation),
// and context.AfterFunc (run a callback when a context ends).
//
// Run with: go run 04_modern_additions.go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errQuotaExceeded = errors.New("user exceeded API quota")

func main() {
	// ---- WithCancelCause + Cause (Go 1.20) -----------------------------------
	// Plain cancel() is mute: every callee just sees context.Canceled and has
	// no idea WHY. WithCancelCause gives you a cancel(err) that records a
	// reason, retrievable anywhere in the subtree via context.Cause(ctx).
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errQuotaExceeded) // attach the reason (nil would mean plain Canceled)

	fmt.Println("ctx.Err()  =", ctx.Err())          // context canceled  (unchanged!)
	fmt.Println("Cause(ctx) =", context.Cause(ctx)) // user exceeded API quota
	// Existing code keying off ctx.Err()/errors.Is(err, context.Canceled)
	// keeps working; code that wants the real reason asks Cause. And it's a
	// normal error, so errors.Is works too:
	fmt.Println("quota?     =", errors.Is(context.Cause(ctx), errQuotaExceeded))

	// For deadline contexts Cause defaults to DeadlineExceeded (there is also
	// WithDeadlineCause/WithTimeoutCause to customize even that):
	tctx, tcancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer tcancel()
	<-tctx.Done()
	fmt.Println("timeout cause =", context.Cause(tctx)) // context deadline exceeded

	// ---- WithoutCancel (Go 1.21) ----------------------------------------------
	// Problem: the request is cancelled, but SOME follow-up work must still
	// finish — writing an audit log, emitting metrics, rolling back. If that
	// work uses the request ctx, it dies with the request.
	//
	// WithoutCancel(parent) returns a context that KEEPS the parent's values
	// (request ID! trace ID!) but is immune to the parent's cancellation and
	// deadline. Before 1.21 people hand-rolled this ("detach" contexts).
	type key struct{}
	reqCtx, reqCancel := context.WithCancelCause(context.Background())
	reqCtx2 := context.WithValue(reqCtx, key{}, "req-42")

	auditCtx := context.WithoutCancel(reqCtx2) // detached, values intact

	reqCancel(errors.New("client disconnected"))           // the request dies...
	fmt.Println("\nrequest ctx err:", reqCtx2.Err())       // context canceled
	fmt.Println("audit ctx err:  ", auditCtx.Err())        // <nil> — still alive
	fmt.Println("audit ctx value:", auditCtx.Value(key{})) // req-42 — inherited
	// (auditCtx.Done() returns nil: it can never be cancelled from above.
	//  Give the audit work its OWN timeout so it can't run forever:)
	auditCtx2, auditCancel := context.WithTimeout(auditCtx, 100*time.Millisecond)
	defer auditCancel()
	fmt.Println("audit write ok: ", auditCtx2.Err() == nil)

	// ---- AfterFunc (Go 1.21) ----------------------------------------------------
	// AfterFunc(ctx, f) runs f in its own goroutine as soon as ctx ends
	// (cancelled OR deadline). It replaces the hand-written
	//     go func() { <-ctx.Done(); f() }()
	// pattern — and unlike that pattern, it can be UNREGISTERED via the
	// returned stop function, so nothing lingers if you finish first.
	fmt.Println("\nAfterFunc demo:")
	cleanupDone := make(chan struct{})
	actx, acancel := context.WithCancel(context.Background())

	stop := context.AfterFunc(actx, func() {
		fmt.Println("  cleanup: releasing resources (ctx ended)")
		close(cleanupDone)
	})
	_ = stop // if the work finished normally we'd call stop() to unregister

	acancel()     // ends the context → the callback fires asynchronously
	<-cleanupDone // wait so the program doesn't exit before we see it

	// The stop() path: register, finish early, unregister — callback never runs.
	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()
	stop2 := context.AfterFunc(bctx, func() {
		fmt.Println("  this should never print")
	})
	if stop2() { // true = we won the race; the callback will NOT run
		fmt.Println("  second callback unregistered before it could fire")
	}
	bcancel()
	time.Sleep(20 * time.Millisecond) // give a (non-)callback a chance to prove itself

	// Classic real-world use: interrupt something that does NOT take a ctx
	// (e.g. unblock a net.Conn read by setting its deadline, or a
	// sync.Cond.Broadcast) the moment the context is cancelled.
	fmt.Println("done")
}
