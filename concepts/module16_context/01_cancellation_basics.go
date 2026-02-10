// Module 16, Example 1 — Why context exists, the Context interface,
// root contexts (Background/TODO), and manual cancellation with WithCancel.
//
// Run with: go run 01_cancellation_basics.go
package main

import (
	"context"
	"fmt"
	"time"
)

// THE PROBLEM context solves
// --------------------------
// A request comes in, you spin up goroutines and call into libraries. Then
// the user hangs up / the deadline passes / a sibling task fails. How do you
// tell *everything downstream* "stop, your work is no longer wanted"?
//
// Before context, every library invented its own stop channel. context.Context
// is the standard answer: ONE value, passed explicitly down the call chain,
// that carries (a) a cancellation signal, (b) an optional deadline, and
// (c) request-scoped values. Deriving a child from a parent builds a TREE —
// cancelling a parent cancels its whole subtree, never the other way around.
//
// THE INTERFACE (all of it — context is tiny)
// -------------------------------------------
//	type Context interface {
//	    Done() <-chan struct{}                  // closed when ctx is cancelled/expired.
//	                                            // YOU (the callee) receive from it in selects.
//	    Err() error                             // nil while alive; context.Canceled after
//	                                            // cancel(); context.DeadlineExceeded after
//	                                            // a timeout/deadline fires. Never un-sets.
//	    Deadline() (deadline time.Time, ok bool)// "when must I be done?" — callees may read
//	                                            // it to budget work (ok=false: no deadline).
//	    Value(key any) any                      // request-scoped data lookup (Example 3).
//	}
//
// You almost never implement this interface yourself; you DERIVE contexts
// with the With* constructors and CONSUME them via Done()/Err().

// worker simulates a background job that must stop promptly when asked.
// The pattern: do a unit of work, then check ctx — in a select, so we react
// to cancellation even while "waiting" between units.
func worker(ctx context.Context, name string, done chan<- struct{}) {
	defer close(done) // let main know we actually exited (no goroutine leak)
	for i := 1; ; i++ {
		select {
		case <-ctx.Done():
			// ctx.Err() tells us WHY we were stopped.
			fmt.Printf("  [%s] stopping after %d units: %v\n", name, i-1, ctx.Err())
			return
		case <-time.After(50 * time.Millisecond): // pretend this is one unit of work
			fmt.Printf("  [%s] finished unit %d\n", name, i)
		}
	}
}

func main() {
	// ---- Root contexts: Background vs TODO ---------------------------------
	// Both are empty roots: never cancelled, no deadline, no values.
	//
	//   context.Background() — THE root. Use in main, init, tests, and at the
	//                          top of a request's lifetime.
	//   context.TODO()       — a placeholder meaning "I haven't plumbed a real
	//                          ctx here yet". Functionally identical, but it
	//                          reads as a TODO for humans and static analysis.
	//
	// Never pass nil as a Context — use TODO() if unsure.
	root := context.Background()

	// ---- WithCancel: manual cancellation ------------------------------------
	// WithCancel derives a child context plus a cancel function. Calling
	// cancel() closes ctx.Done() for this context AND every context derived
	// from it.
	ctx, cancel := context.WithCancel(root)
	// ALWAYS defer cancel(), even if you also call it explicitly later.
	// Until cancel runs, the parent keeps a reference to this child —
	// forgetting it leaks the context (and any timer) until the parent dies.
	defer cancel()

	fmt.Println("starting worker; will cancel after ~180ms")
	workerDone := make(chan struct{})
	go worker(ctx, "worker-A", workerDone)

	time.Sleep(180 * time.Millisecond)
	cancel()     // broadcast "stop" — safe to call more than once
	<-workerDone // wait until the worker has really exited

	// After cancellation the context reports why:
	fmt.Println("ctx.Err() =", ctx.Err()) // context.Canceled

	// ---- The context TREE: parent cancel → whole subtree cancels ------------
	//
	//        Background
	//            │
	//         parent ── cancelParent()   <- we cancel HERE
	//         ┌──┴───┐
	//      child1  child2 (with its own 1h timeout, still pending)
	//
	// Cancelling `parent` closes Done() on parent, child1 AND child2.
	// child2's far-away deadline doesn't save it: a context ends at whichever
	// comes FIRST — its own cancel/deadline or any ancestor's.
	fmt.Println("\nbuilding a tree: parent -> child1, child2")
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	child1, cancel1 := context.WithCancel(parent)
	defer cancel1()
	child2, cancel2 := context.WithTimeout(parent, time.Hour) // deadline far away
	defer cancel2()

	cancelParent() // one call, whole subtree goes down

	// Done() channels of BOTH children are now closed; receives don't block:
	<-child1.Done()
	<-child2.Done()
	fmt.Println("child1.Err() =", child1.Err()) // context.Canceled (inherited)
	fmt.Println("child2.Err() =", child2.Err()) // context.Canceled — NOT DeadlineExceeded

	// Note the direction: cancelling child1 would NOT have touched parent or
	// child2. Cancellation only flows down the tree.

	// ---- Idioms recap (the compiler won't enforce these — reviewers will) ---
	// 1. ctx is the FIRST parameter, named ctx:  func Fetch(ctx context.Context, id int)
	// 2. Pass ctx explicitly; don't store it in a struct. (Rare exceptions:
	//    types representing an in-flight request, like http.Request.)
	// 3. Never pass nil; use context.TODO() while migrating old code.
	// 4. cancel functions: call them, and `defer cancel()` right after With*.
	fmt.Println("\ndone")
}
