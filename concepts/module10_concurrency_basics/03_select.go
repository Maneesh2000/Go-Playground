// Module 10, Example 3 — select: waiting on multiple channels, the default
// case (non-blocking operations), and timeouts with time.After.
//
// select is to channels what switch is to values: it blocks until ONE of
// its cases can proceed, then runs that case. If several are ready at once,
// it picks one at RANDOM (so no channel gets starved).
//
// Run with: go run 03_select.go
package main

import (
	"fmt"
	"time"
)

func main() {
	// ---- Basic multiplexing: first channel to deliver wins ---------------------
	fmt.Println("== multiplexing two workers ==")

	fast := make(chan string)
	slow := make(chan string)

	go func() {
		time.Sleep(30 * time.Millisecond)
		fast <- "fast worker done"
	}()
	go func() {
		time.Sleep(120 * time.Millisecond)
		slow <- "slow worker done"
	}()

	// We want BOTH results, arriving in WHATEVER order — select in a loop:
	for received := 0; received < 2; received++ {
		select {
		case msg := <-fast:
			fmt.Println("  got:", msg)
		case msg := <-slow:
			fmt.Println("  got:", msg)
		}
	}

	// ---- default: making channel ops NON-blocking --------------------------------
	// A select with `default` never blocks: if no case is ready, default
	// runs immediately. This is how you "poll" a channel.
	fmt.Println("\n== default: non-blocking receive ==")

	updates := make(chan string, 1)

	checkForUpdate := func() {
		select {
		case u := <-updates:
			fmt.Println("  update:", u)
		default:
			fmt.Println("  no update available, moving on (didn't block!)")
		}
	}

	checkForUpdate()             // nothing there yet
	updates <- "v2.0 downloaded" // now there is
	checkForUpdate()             // consumes it
	checkForUpdate()             // empty again

	// Same trick for non-blocking SEND (drop instead of block when full):
	tryLog := func(line string) {
		select {
		case updates <- line:
			fmt.Println("  queued:", line)
		default:
			fmt.Println("  buffer full, DROPPED:", line)
		}
	}
	tryLog("event A") // fills the 1-slot buffer
	tryLog("event B") // buffer full -> dropped, no blocking

	// ---- Timeouts with time.After ---------------------------------------------------
	// time.After(d) returns a channel that delivers one value after d.
	// Racing "the answer" against "the clock" is the classic timeout:
	fmt.Println("\n== timeout with time.After ==")

	result := make(chan string)
	go func() {
		time.Sleep(200 * time.Millisecond) // pretend this is a slow API call
		result <- "API response"
	}()

	select {
	case r := <-result:
		fmt.Println("  got:", r)
	case <-time.After(100 * time.Millisecond):
		// The clock won: give up on waiting.
		fmt.Println("  timed out after 100ms — the API was too slow")
	}

	// Try flipping the sleeps above to see the other branch win.
	//
	// NOTE: in long-lived production loops prefer time.NewTimer / context
	// deadlines — a time.After inside a hot loop creates a new timer each
	// iteration that lingers until it fires. Fine for one-shot waits and
	// learning; know the caveat.

	// ---- A heartbeat loop: select as an event loop -------------------------------------
	fmt.Println("\n== mini event loop: work + heartbeat + shutdown ==")

	work := make(chan int)
	done := make(chan struct{}) // struct{} = "pure signal", no data
	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()

	go func() { // producer: some work, then a shutdown signal
		for i := 1; i <= 3; i++ {
			time.Sleep(25 * time.Millisecond)
			work <- i * 100
		}
		close(done) // closing broadcasts: EVERY receiver unblocks
	}()

	for running := true; running; {
		select {
		case job := <-work:
			fmt.Println("  processing job", job)
		case <-tick.C:
			fmt.Println("  ...heartbeat (still alive)")
		case <-done:
			fmt.Println("  shutdown signal received")
			running = false
		}
	}
	fmt.Println("  loop exited cleanly")
}
