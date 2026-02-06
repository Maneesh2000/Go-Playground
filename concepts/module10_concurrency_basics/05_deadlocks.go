// Module 10, Example 5 — Common deadlocks and how to READ the panic.
//
// A deadlock = every goroutine is blocked waiting for something that can
// never happen. The Go runtime detects the "ALL goroutines asleep" case and
// crashes with:
//
//	fatal error: all goroutines are asleep - deadlock!
//
//	goroutine 1 [chan send]:
//	main.main()
//	        /path/to/05_deadlocks.go:NN +0x2c
//
// How to read it:
//   - `goroutine 1` — which goroutine (1 is always main)
//   - `[chan send]` — WHAT it's blocked on: chan send, chan receive,
//     semacquire (WaitGroup/mutex), select, ...
//   - the file:line — WHERE it's stuck. Go there; ask "who was supposed
//     to unblock this, and why didn't they?"
//
// This file demonstrates each classic deadlock SAFELY (in ways the program
// survives), and each section shows the one-line change that would make it
// fatal — uncomment to experience the real crash.
//
// Run with: go run 05_deadlocks.go
package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	// =========================================================================
	// DEADLOCK #1: send on an unbuffered channel with NO receiver
	// =========================================================================
	// An unbuffered send blocks until someone receives. If nobody ever
	// will, main sleeps forever -> runtime kills the program:
	//
	//   ch := make(chan int)
	//   ch <- 1                       // fatal: goroutine 1 [chan send]
	//   fmt.Println(<-ch)             // never reached
	//
	// Fix A: have a receiver running FIRST (another goroutine).
	// Fix B: give the channel a buffer if you just need to drop one value off.
	fmt.Println("== #1: send with no receiver ==")

	ch := make(chan int)
	go func() { ch <- 1 }() // fix A: sender in its own goroutine
	fmt.Println("  received:", <-ch, "(sender ran concurrently — no deadlock)")

	ch2 := make(chan int, 1) // fix B: buffer of 1
	ch2 <- 42                // doesn't block: value parks in the buffer
	fmt.Println("  received:", <-ch2, "(buffered — no deadlock)")

	// =========================================================================
	// DEADLOCK #2: receive that no one will ever satisfy
	// =========================================================================
	//   ch := make(chan int)
	//   <-ch                          // fatal: goroutine 1 [chan receive]
	//
	// Typical real-world version: you started the producer goroutine —
	// but it exited early (error path!) without sending.
	// Fix: producers should ALWAYS close their channel (defer close), and
	// consumers should use comma-ok/range so a close unblocks them.
	fmt.Println("\n== #2: receive with no sender ==")

	ch3 := make(chan int)
	go func() {
		defer close(ch3) // even on the "error" path below, close happens
		if tooHard := true; tooHard {
			return // oops, producer bails out early — but the close saves us
		}
		ch3 <- 99 // never runs
	}()

	if v, ok := <-ch3; !ok {
		fmt.Println("  channel closed with no value (v =", v, ", ok =", ok, ") — unblocked, no deadlock")
	}

	// =========================================================================
	// DEADLOCK #3: range over a channel that is never closed
	// =========================================================================
	//   ch := make(chan int)
	//   go func() { ch <- 1; ch <- 2 }()   // sends but NEVER closes
	//   for v := range ch { ... }          // gets 1, 2... then blocks forever
	//                                      // fatal: goroutine 1 [chan receive]
	//
	// Fix: the sender closes when done. range then ends cleanly.
	fmt.Println("\n== #3: range needs a close ==")

	ch4 := make(chan int)
	go func() {
		defer close(ch4) // THE fix — delete this line for the fatal version
		ch4 <- 1
		ch4 <- 2
	}()
	for v := range ch4 {
		fmt.Println("  got:", v)
	}
	fmt.Println("  range exited because sender closed")

	// =========================================================================
	// DEADLOCK #4: WaitGroup that can never reach zero
	// =========================================================================
	//   var wg sync.WaitGroup
	//   wg.Add(2)                     // says "wait for TWO"...
	//   go func() { defer wg.Done(); work() }()   // ...but only ONE exists
	//   wg.Wait()                     // fatal: goroutine 1 [semacquire]
	//
	// Note the different marker: [semacquire], not [chan ...] — that's how
	// WaitGroup/mutex blockage shows up in the trace.
	// Fix: Add must match the goroutines actually started (Add(1) right
	// before each `go`, Done via defer inside).
	fmt.Println("\n== #4: WaitGroup counted correctly ==")

	var wg sync.WaitGroup
	for i := 1; i <= 2; i++ {
		wg.Add(1) // one Add per goroutine, immediately before `go`
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			fmt.Println("  worker", i, "done")
		}()
	}
	wg.Wait()
	fmt.Println("  Wait returned — counts matched")

	// =========================================================================
	// NOT-QUITE-DEADLOCK: a single stuck goroutine (worse in a way!)
	// =========================================================================
	// The runtime only crashes when ALL goroutines are asleep. If main is
	// alive but a background goroutine is stuck forever, you get NO panic —
	// just a silent, permanent goroutine LEAK. That's why every send/receive
	// needs a story for "how does this end?" (close, timeout, or context).
	leaky := make(chan int)
	go func() { leaky <- 1 }() // stuck forever: nobody will receive
	time.Sleep(20 * time.Millisecond)
	fmt.Println("\n== bonus ==")
	fmt.Println("  a goroutine is leaked right now and nothing crashed —")
	fmt.Println("  deadlock panics only fire when EVERY goroutine is stuck")

	fmt.Println("\nAll demos survived. Now uncomment a fatal line and read your first deadlock trace!")

	// Try it — paste at the end of main and run:
	//   done := make(chan bool)
	//   <-done // fatal error: all goroutines are asleep - deadlock!
}
