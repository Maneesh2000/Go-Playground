// Module 17, Example 5 — The client side of net: Dial, DialTimeout, and
// net.Dialer with DialContext, plus what a connect timeout looks like.
//
// Three ways to open a TCP connection, from quick-and-dirty to
// production-grade:
//
//	net.Dial(net, addr)                  // simple, NO time limit
//	net.DialTimeout(net, addr, d)        // same + overall time limit
//	(&net.Dialer{...}).DialContext(ctx)  // timeout + cancellation + knobs
//
// Run with: go run 05_tcp_client_dialer.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	log.SetFlags(0)

	// A tiny local greeter server to dial against (ephemeral port again).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				fmt.Fprintf(c, "hello from %s\n", c.LocalAddr())
				c.Close()
			}(conn)
		}
	}()

	buf := make([]byte, 128)

	// ---- 1. net.Dial: the simplest client ----------------------------------
	// Blocks until the TCP handshake completes or the OS gives up — which
	// can take MINUTES against an unresponsive host. Fine for scripts;
	// risky in servers, where you always want a bound on waiting.
	fmt.Println("-- 1. net.Dial --")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("Dial: %v", err)
	}
	n, _ := conn.Read(buf)
	fmt.Printf("Dial ok: %s -> %s, got %q\n", conn.LocalAddr(), conn.RemoteAddr(), buf[:n])
	conn.Close()

	// ---- 2. net.DialTimeout: Dial with an upper bound ----------------------
	// One extra argument: the maximum time for the WHOLE dial (including
	// DNS resolution if the address has a hostname).
	fmt.Println("\n-- 2. net.DialTimeout --")
	conn, err = net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		log.Fatalf("DialTimeout: %v", err)
	}
	n, _ = conn.Read(buf)
	fmt.Printf("DialTimeout ok, got %q\n", buf[:n])
	conn.Close()

	// ---- 3. net.Dialer + DialContext: the production tool ------------------
	// A Dialer is a reusable bundle of connection policy:
	//   Timeout   — connect time limit (like DialTimeout)
	//   KeepAlive — period for TCP keep-alive probes on the new conn
	//               (Go 1.23+ adds the finer-grained KeepAliveConfig)
	//   LocalAddr, Control, Resolver... for advanced setups
	// DialContext additionally honors ctx: if the context is canceled or
	// its deadline passes mid-dial, the dial aborts. This is how you tie
	// a connection attempt to an HTTP request's or job's lifetime.
	fmt.Println("\n-- 3. net.Dialer.DialContext --")
	dialer := &net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	conn, err = dialer.DialContext(ctx, "tcp", addr)
	cancel()
	if err != nil {
		log.Fatalf("DialContext: %v", err)
	}
	n, _ = conn.Read(buf)
	fmt.Printf("DialContext ok, got %q\n", buf[:n])
	conn.Close()

	// ---- 4. What failure looks like: a context deadline killing a dial -----
	// A context whose deadline has effectively already passed makes the
	// dial fail immediately and deterministically — the same mechanism
	// that aborts a slow dial to a distant host, without waiting minutes.
	fmt.Println("\n-- 4. dial aborted by context deadline --")
	shortCtx, cancel2 := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel2()
	start := time.Now()
	_, err = dialer.DialContext(shortCtx, "tcp", addr)
	fmt.Printf("failed after %v: %v\n", time.Since(start).Round(time.Millisecond), err)
	fmt.Println("errors.Is(err, context.DeadlineExceeded):", errors.Is(err, context.DeadlineExceeded))

	// Dial errors also satisfy net.Error, so the Timeout() test from
	// example 3 classifies them uniformly:
	var nerr net.Error
	if errors.As(err, &nerr) {
		fmt.Println("net.Error.Timeout():", nerr.Timeout())
	}

	// ---- 5. An unreachable address with a short timeout --------------------
	// 203.0.113.0/24 (TEST-NET-3) is reserved for documentation and never
	// routed, so connecting can't succeed. With a 500ms limit we fail
	// fast; the exact error text varies by OS/network ("i/o timeout",
	// "no route to host", ...), which is exactly why code should test
	// nerr.Timeout() / errors.Is rather than match strings.
	fmt.Println("\n-- 5. unreachable address, 500ms budget --")
	start = time.Now()
	_, err = net.DialTimeout("tcp", "203.0.113.1:81", 500*time.Millisecond)
	fmt.Printf("failed after %v: %v\n", time.Since(start).Round(time.Millisecond), err)
	if errors.As(err, &nerr) {
		fmt.Println("net.Error.Timeout():", nerr.Timeout())
	}

	fmt.Println("\ndone")
}
