// Module 10, Example 4 — A simple pipeline: generator -> worker -> printer.
//
// A pipeline chains goroutines with channels, like an assembly line:
//
//	┌───────────┐  nums   ┌──────────┐  results  ┌──────────┐
//	│ generator │ ──chan─►│  worker  │ ──chan───►│ printer  │
//	│ (produce) │         │(transform)│          │ (consume)│
//	└───────────┘         └──────────┘           └──────────┘
//
// Rules that make pipelines work:
//  1. Each stage OWNS its output channel: it sends on it and closes it
//     when done (usually via defer).
//  2. Each stage ranges over its input, so it ends when upstream closes.
//  3. Closes cascade: generator closes nums -> worker's range ends ->
//     worker closes results -> printer's range ends. Clean shutdown, no
//     leaked goroutines.
//
// Run with: go run 04_pipeline.go
package main

import (
	"fmt"
	"sync"
	"time"
)

// generate is the SOURCE stage: it produces numbers into a channel it
// creates, owns, and closes. Returning a receive-only channel (<-chan)
// stops callers from sending on or closing our channel.
func generate(count int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out) // rule 1: producer closes its output when done
		for i := 1; i <= count; i++ {
			out <- i
		}
	}()
	return out
}

// square is a MIDDLE stage: consumes ints, produces their squares.
// Input is receive-only, output (to the caller) is receive-only too —
// direction annotations document the flow and the compiler enforces them.
func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in { // rule 2: ends when upstream closes
			time.Sleep(20 * time.Millisecond) // simulate real work
			out <- n * n
		}
	}()
	return out
}

func main() {
	// ---- The basic 3-stage pipeline -------------------------------------------
	fmt.Println("== generator -> worker -> printer ==")

	nums := generate(5)     // stage 1: source
	results := square(nums) // stage 2: transform

	// stage 3: the SINK runs right here in main — which neatly solves
	// "main must wait": main blocks until results is closed, which only
	// happens after every upstream stage finished. No WaitGroup needed
	// for a straight line.
	for r := range results {
		fmt.Println("  result:", r)
	}
	fmt.Println("  pipeline drained — all stages exited")

	// ---- Fan-out / fan-in: multiple workers on one input -------------------------
	// One generator, THREE workers reading the SAME channel (each value is
	// received by exactly ONE of them — channels distribute, they don't
	// broadcast), and one merged output.
	fmt.Println("\n== fan-out to 3 workers, fan-in to one channel ==")

	in := generate(9)
	merged := make(chan string)

	// Fan-in bookkeeping: we may close `merged` only after ALL workers are
	// done sending — close too early and workers panic ("send on closed
	// channel"). A WaitGroup counts the workers; a small extra goroutine
	// waits and then closes. This is THE standard fan-in pattern.
	var wg sync.WaitGroup
	for w := 1; w <= 3; w++ {
		wg.Add(1)
		go func() { // one worker
			defer wg.Done()
			for n := range in { // workers compete for values — automatic load balancing
				time.Sleep(30 * time.Millisecond) // simulate work
				merged <- fmt.Sprintf("worker %d computed %d^2 = %d", w, n, n*n)
			}
		}()
	}

	go func() {
		wg.Wait()     // all workers finished (their range on `in` ended)...
		close(merged) // ...NOW closing the merged output is safe
	}()

	for line := range merged {
		fmt.Println("  " + line)
	}
	fmt.Println("  fan-in complete")

	// Notice in the output that the 9 jobs were split among the 3 workers
	// in whatever order the scheduler chose — but every job ran exactly
	// once, and total wall time was roughly 1/3 of the serial version.
}
