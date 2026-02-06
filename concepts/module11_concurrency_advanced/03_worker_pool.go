// Module 11, example 3: worker pool + buffered-channel semaphore.
//
// Run with: go run 03_worker_pool.go
//
// Worker pool shape:
//
//	             jobs                        results
//	producer ──►[ ch ]──► worker 1 ──┐
//	                  ──► worker 2 ──┼──►[ ch ]──► collector
//	                  ──► worker 3 ──┘
//
// Why a pool instead of one goroutine per job? Bounded concurrency (don't
// open 10,000 DB connections) and built-in backpressure (when all workers
// are busy, sends into `jobs` block, naturally slowing the producer).
package main

import (
	"fmt"
	"sync"
	"time"
)

type job struct {
	id int
	n  int // the "payload": a number to square
}

type result struct {
	jobID  int
	square int
	worker int
}

// worker drains the jobs channel until it is closed, pushing results out.
// Many workers share ONE jobs channel — the runtime hands each job to
// exactly one receiver, so no job is processed twice.
func worker(id int, jobs <-chan job, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs { // loop ends when jobs is closed AND drained
		time.Sleep(10 * time.Millisecond) // simulate real work
		results <- result{jobID: j.id, square: j.n * j.n, worker: id}
	}
}

func main() {
	// ---------------------- Part 1: worker pool ------------------------
	const numWorkers = 3
	const numJobs = 9

	jobs := make(chan job) // unbuffered: producer feels backpressure
	results := make(chan result, numJobs)

	// Start the fixed-size pool.
	var wg sync.WaitGroup
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg)
	}

	// Produce jobs, then close(jobs) to tell workers "no more work".
	// Axiom in action: the SENDER closes; receivers (workers) just range.
	go func() {
		for i := 1; i <= numJobs; i++ {
			jobs <- job{id: i, n: i}
		}
		close(jobs)
	}()

	// Close `results` once every worker has exited. Without this, the
	// collector's `range results` below would block forever (deadlock).
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect. Order is nondeterministic — whichever worker finishes first
	// delivers first. Run the program twice and compare!
	fmt.Println("--- worker pool ---")
	for r := range results {
		fmt.Printf("job %d: %d^2 = %-3d (worker %d)\n", r.jobID, r.jobID, r.square, r.worker)
	}

	// ------------------ Part 2: semaphore pattern ----------------------
	// Sometimes you don't want a standing pool — you just want "at most N
	// of these running at once". A buffered channel IS a semaphore:
	//   - send  = acquire a slot (blocks when all N slots are taken)
	//   - recv  = release a slot
	fmt.Println("--- semaphore (max 2 concurrent downloads) ---")

	const maxConcurrent = 2
	sem := make(chan struct{}, maxConcurrent) // struct{} = zero bytes of payload

	var inFlight, peak int
	var mu sync.Mutex // protects inFlight/peak (they're shared state!)

	var dl sync.WaitGroup
	for i := 1; i <= 6; i++ {
		dl.Add(1)
		go func(id int) {
			defer dl.Done()

			sem <- struct{}{}        // acquire — goroutine parks here if 2 are busy
			defer func() { <-sem }() // release, even if the work panics

			mu.Lock()
			inFlight++
			if inFlight > peak {
				peak = inFlight
			}
			fmt.Printf("download %d started (in flight: %d)\n", id, inFlight)
			mu.Unlock()

			time.Sleep(30 * time.Millisecond) // the "download"

			mu.Lock()
			inFlight--
			mu.Unlock()
		}(i)
	}
	dl.Wait()
	fmt.Printf("peak concurrency: %d (never exceeds %d)\n", peak, maxConcurrent)
}
