// Module 10, Example 2 — Channels: unbuffered vs buffered, send/receive,
// close, range, and the comma-ok idiom.
//
// A channel is a typed pipe between goroutines. Motto:
// "Don't communicate by sharing memory; share memory by communicating."
//
// Run with: go run 02_channels.go
package main

import (
	"fmt"
	"time"
)

func main() {
	// ---- Unbuffered channel: a synchronizing HANDOFF -------------------------
	// make(chan T) has NO storage. A send blocks until a receiver is ready,
	// and a receive blocks until a sender is ready. When the value moves,
	// you KNOW both goroutines were at that point — it's a rendezvous.
	//
	//   goroutine                          main
	//   ch <- "hi" ──── blocks... ──┐
	//                               │  value changes hands only when
	//                               ▼  BOTH sides are ready
	//                          msg := <-ch
	//
	fmt.Println("== unbuffered: handoff ==")
	ch := make(chan string)

	go func() {
		fmt.Println("  sender: about to send (will block until main receives)")
		ch <- "hello over the channel"
		fmt.Println("  sender: send completed — main must have received")
	}()

	time.Sleep(50 * time.Millisecond) // let the sender reach its send & block
	fmt.Println("  main: now receiving")
	msg := <-ch
	fmt.Println("  main: got:", msg)
	time.Sleep(10 * time.Millisecond) // let sender print its last line

	// ---- Buffered channel: a small queue ----------------------------------------
	// make(chan T, n) stores up to n values.
	//   send    blocks only when the buffer is FULL
	//   receive blocks only when the buffer is EMPTY
	// Buffering decouples sender/receiver timing; it does NOT remove blocking.
	fmt.Println("\n== buffered: queue of capacity 3 ==")
	buf := make(chan int, 3)

	buf <- 1 // all three fit in the buffer:
	buf <- 2 // no receiver needed (yet), nothing blocks
	buf <- 3
	fmt.Printf("  after 3 sends: len=%d cap=%d\n", len(buf), cap(buf))
	// A 4th send here would BLOCK (buffer full, nobody receiving):
	//   buf <- 4 // would deadlock this program

	fmt.Println("  received:", <-buf, <-buf, <-buf) // FIFO order

	// ---- close, and why receivers care ------------------------------------------
	// close(ch) is the SENDER saying "no more values will ever come".
	// Rules:
	//   * only the sender closes (receiver closing = panic risk)
	//   * sending on a closed channel PANICS
	//   * receiving from a closed channel yields zero values immediately
	//   * closing is optional — only needed when receivers must know the end
	fmt.Println("\n== close + comma-ok ==")
	nums := make(chan int, 2)
	nums <- 10
	nums <- 20
	close(nums)

	// comma-ok on receive: ok=true  -> real value
	//                      ok=false -> channel closed AND drained
	v, ok := <-nums
	fmt.Printf("  recv: v=%d ok=%v (buffered value survives close)\n", v, ok)
	v, ok = <-nums
	fmt.Printf("  recv: v=%d ok=%v\n", v, ok)
	v, ok = <-nums
	fmt.Printf("  recv: v=%d ok=%v (closed+empty: zero value, ok=false)\n", v, ok)

	// ---- range over a channel ------------------------------------------------------
	// `for v := range ch` receives until the channel is CLOSED — the
	// idiomatic way to consume a stream. Without a close, range would block
	// forever after the last value (deadlock — see example 5).
	fmt.Println("\n== range over a channel ==")
	squares := make(chan int)

	go func() {
		defer close(squares) // sender closes when done — enables range to end
		for i := 1; i <= 5; i++ {
			squares <- i * i
		}
	}()

	for sq := range squares { // ends cleanly when squares is closed
		fmt.Println("  square:", sq)
	}
	fmt.Println("  range ended: channel was closed")

	// ---- Directional channel types (documentation + safety) --------------------------
	// A function can declare it only SENDS (chan<- T) or only RECEIVES
	// (<-chan T). The compiler then forbids misuse. See produce():
	results := make(chan string, 3)
	go produce(results)
	for r := range results {
		fmt.Println("  produced:", r)
	}
}

// produce takes a SEND-ONLY view of the channel: it cannot receive from it,
// and callers can see at a glance which direction data flows.
func produce(out chan<- string) {
	defer close(out)
	for _, item := range []string{"one", "two", "three"} {
		out <- item
		// <-out  // compile error: cannot receive from send-only channel
	}
}
