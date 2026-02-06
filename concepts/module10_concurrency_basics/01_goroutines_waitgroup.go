// Module 10, Example 1 — Goroutines, the `go` keyword, why main must wait,
// and sync.WaitGroup.
//
// Run with: go run 01_goroutines_waitgroup.go
package main

import (
	"fmt"
	"sync"
	"time"
)

// A goroutine is a function running CONCURRENTLY with the rest of the
// program, scheduled by the Go runtime (not the OS). It starts with a tiny
// stack (a few KB, grows on demand), so launching thousands is normal.
//
//   G = goroutine, M = OS thread, P = logical processor (≈ CPU core)
//
//   G G G G G G G G     <- many cheap goroutines (yours)
//    \ | | /   \ | /
//    ┌─▼─▼─┐   ┌─▼─┐
//    │ P0  │   │P1 │    <- runtime multiplexes G's onto P's...
//    └──┬──┘   └─┬─┘
//    ┌──▼──┐   ┌─▼─┐
//    │ M0  │   │M1 │    <- ...which run on a few real OS threads
//    └─────┘   └───┘
//
// This "M:N scheduling" is why goroutines are cheap: no kernel involvement
// to switch between them.

func say(who string, n int) {
	for i := 1; i <= n; i++ {
		fmt.Printf("  [%s] message %d\n", who, i)
		time.Sleep(10 * time.Millisecond) // yield: lets others interleave
	}
}

func main() {
	// ---- Sequential baseline -------------------------------------------------
	fmt.Println("== sequential ==")
	say("alpha", 2)
	say("beta", 2) // runs only after alpha completely finishes

	// ---- The `go` keyword ---------------------------------------------------
	// Prefixing a call with `go` starts it as a goroutine and returns
	// IMMEDIATELY — main does not wait for it.
	fmt.Println("\n== concurrent, but main doesn't wait (bug!) ==")
	go say("gamma", 3)
	fmt.Println("  main: I did NOT wait for gamma")

	// If main returned right now, the program would exit and gamma would be
	// killed mid-flight — likely printing NOTHING. Sleeping "long enough"
	// is a race, not a fix:
	time.Sleep(50 * time.Millisecond) // BAD synchronization — demo only

	// ---- The right tool: sync.WaitGroup ----------------------------------------
	// A WaitGroup is a counter:
	//   Add(1)  -> +1, BEFORE starting the goroutine
	//   Done()  -> -1, when the goroutine finishes (always via defer)
	//   Wait()  -> block until the counter is 0
	fmt.Println("\n== concurrent with WaitGroup ==")

	var wg sync.WaitGroup
	workers := []string{"red", "green", "blue"}

	for _, name := range workers {
		wg.Add(1) // count BEFORE `go` — never inside the goroutine
		// (inside, Wait() could run before Add() — a race)

		go func() {
			defer wg.Done() // defer: Done runs even if the work panics

			// Since Go 1.22 each loop iteration has a FRESH `name`
			// variable, so capturing it in the closure is safe. (Pre-1.22
			// all iterations shared one variable — a famous bug source.)
			say(name, 2)
		}()
	}

	fmt.Println("  main: waiting for all workers...")
	wg.Wait() // blocks here until every Done() has been called
	fmt.Println("  main: all workers finished")

	// Note the interleaved output above: the runtime switched between the
	// three goroutines however it liked. Order across goroutines is NOT
	// guaranteed — only "all finished before Wait returned" is.

	// ---- Just to show they're cheap ------------------------------------------------
	fmt.Println("\n== launching 10,000 goroutines ==")
	var wg2 sync.WaitGroup
	start := time.Now()

	for i := 0; i < 10_000; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			// trivial work
		}()
	}
	wg2.Wait()
	fmt.Printf("  10,000 goroutines started and finished in %v\n", time.Since(start))
	// Try that with OS threads and get back to us.
}
