// Module 11, example 4: fan-out / fan-in, and a pipeline that cancels cleanly.
//
// Run with: go run 04_fanout_fanin_pipeline.go
//
// Pipeline: each stage is a goroutine; channels are the conveyor belts.
//
//	generate ──► square ──► square ──► fan-in ──► consumer
//	              (fan-out: 3 square stages share one input)
//
// The consumer stops early, and context cancellation unwinds every stage —
// no goroutine leaks. Leaked goroutines are THE classic pipeline bug: a
// stage blocked forever on a send nobody will receive.
package main

import (
	"context"
	"fmt"
	"sync"
)

// generate emits the integers 1..n. Every send also selects on ctx.Done(),
// so the stage exits promptly if downstream loses interest.
func generate(ctx context.Context, n int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out) // sender closes — receivers just range
		for i := 1; i <= n; i++ {
			select {
			case out <- i:
			case <-ctx.Done():
				fmt.Println("  [generate] cancelled, shutting down")
				return
			}
		}
	}()
	return out
}

// square is one worker stage: read, compute, forward. We'll FAN OUT by
// running several of these against the same input channel — the runtime
// delivers each value to exactly one of them.
func square(ctx context.Context, id int, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			select {
			case out <- v * v:
			case <-ctx.Done():
				fmt.Printf("  [square %d] cancelled, shutting down\n", id)
				return
			}
		}
	}()
	return out
}

// merge FANS IN: it forwards values from many channels onto one. The
// WaitGroup answers the key question — "when may we close the merged
// channel?" — answer: only after ALL inputs are exhausted.
func merge(ctx context.Context, ins ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup

	forward := func(c <-chan int) {
		defer wg.Done()
		for v := range c {
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(len(ins))
	for _, c := range ins {
		go forward(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func main() {
	// cancel() is our "consumer walked away" button.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build the pipeline: 1 generator → 3 squarers (fan-out) → merge (fan-in).
	nums := generate(ctx, 100) // could produce up to 100 values...
	s1 := square(ctx, 1, nums)
	s2 := square(ctx, 2, nums) // all three read from the SAME channel
	s3 := square(ctx, 3, nums)
	squares := merge(ctx, s1, s2, s3)

	// ...but the consumer only wants 5. After that we cancel, and every
	// stage's ctx.Done() case fires, letting it return and close its output.
	fmt.Println("consumer takes 5 values, then cancels:")
	got := 0
	for v := range squares {
		fmt.Printf("  got %d\n", v)
		got++
		if got == 5 {
			cancel() // broadcast "stop" to generate + all squares + merge
			break
		}
	}

	// Drain anything already in flight so merge's forwarders can finish.
	// (Values that were mid-send when we cancelled.)
	for range squares {
	}

	fmt.Println("pipeline shut down cleanly — no goroutines left blocked")

	// Note the order of results above: it is NOT sorted. Fan-in merges
	// whatever arrives first. If order matters, tag values with an index
	// and reorder at the end — or don't fan out at all.
}
