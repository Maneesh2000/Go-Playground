// Module 17, Example 3 — Deadlines and the error taxonomy of net code:
//   - SetReadDeadline making a blocked Read time out
//   - detecting timeouts with errors.As + net.Error.Timeout()
//   - io.EOF when the peer closes
//   - net.ErrClosed when Accept is unblocked by closing the listener
//
// Every long-lived network program eventually needs all four.
//
// Run with: go run 03_deadlines_and_errors.go
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

func main() {
	log.SetFlags(0)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	fmt.Println("listening on", ln.Addr())

	// A deliberately RUDE server: it accepts connections and then says
	// nothing for a while — perfect for demonstrating client-side
	// deadlines. It closes each conn after 600ms (so demo 2 can observe EOF).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				// Demo 3 below: closing the listener makes Accept return
				// net.ErrClosed — the standard shutdown signal.
				fmt.Println("\n-- demo 3: listener closed, Accept unblocked --")
				fmt.Println("accept error:", err)
				fmt.Println("errors.Is(err, net.ErrClosed):", errors.Is(err, net.ErrClosed))
				return
			}
			go func(c net.Conn) {
				time.Sleep(600 * time.Millisecond) // stay silent, then hang up
				c.Close()
			}(conn)
		}
	}()

	// ---- Demo 1: SetReadDeadline times out a blocked Read ------------------
	fmt.Println("\n-- demo 1: read deadline fires on a silent peer --")
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		log.Fatalf("dial: %v", err)
	}

	// Deadlines are ABSOLUTE instants (time.Time), not durations. This
	// arms a timer for 200ms from now; if Read is still blocked when it
	// passes, Read returns a timeout error. SetDeadline(t) would set the
	// read AND write deadlines at once; SetWriteDeadline covers Write
	// (useful when the peer stops draining and our send buffer fills).
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

	buf := make([]byte, 64)
	start := time.Now()
	_, err = conn.Read(buf) // server sends nothing -> this must time out
	fmt.Printf("Read returned after %v with err: %v\n", time.Since(start).Round(time.Millisecond), err)

	// The right way to ASK "was that a timeout?": pull a net.Error out of
	// the (possibly wrapped) error chain and call its Timeout() method.
	// Don't string-match error text.
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		fmt.Println("classified: TIMEOUT — the connection is still usable; we could retry")
	}

	// A timed-out connection is NOT closed. Clear the deadline (zero time
	// = no deadline) and keep using it:
	conn.SetReadDeadline(time.Time{})

	// ---- Demo 2: io.EOF when the peer closes -------------------------------
	fmt.Println("\n-- demo 2: peer closes -> Read returns io.EOF --")
	// The server closes this conn ~600ms after accepting; with no deadline
	// set, our Read now blocks until that close arrives as EOF.
	_, err = conn.Read(buf)
	fmt.Println("Read err:", err)
	fmt.Println("errors.Is(err, io.EOF):", errors.Is(err, io.EOF),
		"(EOF = orderly close by the peer, not a failure)")
	conn.Close()

	// ---- Demo 3: closing the listener unblocks Accept -----------------------
	// The accept-loop goroutine is currently BLOCKED inside ln.Accept().
	// Listener.Close() is the graceful-shutdown lever: it makes that Accept
	// return net.ErrClosed immediately. (Writing to an already-closed conn
	// yields the same net.ErrClosed — historically seen as the string
	// "use of closed network connection" — which is why shutdown code
	// checks errors.Is(err, net.ErrClosed) instead of treating it as a bug.)
	ln.Close()
	wg.Wait()

	fmt.Println("\ndone")
}
