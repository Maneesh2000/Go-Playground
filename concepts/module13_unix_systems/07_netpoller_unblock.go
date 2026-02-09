// Module 13, example 7: the Go netpoller in action — 1000 goroutines
// "blocked" on network reads, zero raw syscalls, and how they get unblocked.
//
//	go run 07_netpoller_unblock.go
//
// Works on macOS and Linux. The point: when a goroutine calls conn.Read and
// no data is ready, NO OS THREAD BLOCKS. The runtime has already put the
// socket in non-blocking mode and registered it with the platform readiness
// API (epoll on Linux, kqueue here on macOS). The read hits EAGAIN, the
// runtime PARKS THE GOROUTINE (gopark) — a cheap in-memory operation — and
// the thread moves on to run other goroutines. When the kernel reports the
// fd readable, the netpoller readies exactly that goroutine again.
//
// So this program has ~1000 goroutines sitting in Read simultaneously while
// the process uses only a handful of OS threads. You write simple
// blocking-STYLE code; Go gives you event-driven scalability underneath.
//
// Three phases:
//  1. park:    1000 goroutines each Read from their own TCP conn (no data)
//  2. wake:    write to 3 chosen conns -> exactly those 3 goroutines unblock
//  3. deadline: SetReadDeadline on the remaining 997 -> the poller's timer
//     forcibly unblocks ALL of them at once with a timeout error.
//     "Deadlines are the poller forcibly unblocking you."
package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

const numConns = 1000

// result is what each reader goroutine reports when its Read unblocks.
type result struct {
	id  int
	msg string // data read, if any
	err error  // timeout error, if the deadline unblocked us
}

func main() {
	fmt.Printf("goroutines at start: %d, GOMAXPROCS: %d\n",
		runtime.NumGoroutine(), runtime.GOMAXPROCS(0))

	// A real TCP listener on loopback — port 0 lets the kernel pick a port.
	// (net.Pipe would also block/unblock goroutines, but it's in-memory and
	// synchronized with channels; real sockets go through the netpoller.)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
	defer ln.Close()

	// Accept loop: keep the server side of every connection so we can write
	// into chosen ones later. Each Accept also parks on the poller!
	server := make([]net.Conn, numConns)
	acceptDone := make(chan error, 1)
	go func() {
		for i := 0; i < numConns; i++ {
			c, err := ln.Accept()
			if err != nil {
				acceptDone <- err
				return
			}
			server[i] = c
		}
		acceptDone <- nil
	}()

	// ---------------- phase 1: park 1000 goroutines in Read --------------
	client := make([]net.Conn, numConns)
	results := make(chan result, numConns)
	var dialed sync.WaitGroup
	for i := 0; i < numConns; i++ {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			fmt.Fprintln(os.Stderr, "dial:", err)
			os.Exit(1)
		}
		client[i] = c
		dialed.Add(1)
		go func(id int, c net.Conn) {
			dialed.Done()
			buf := make([]byte, 128)
			// This LOOKS like blocking code. Under the hood: fd is
			// non-blocking, read returns EAGAIN, runtime.gopark suspends
			// this goroutine, and the netpoller owns the wakeup.
			n, err := c.Read(buf) // <- goroutine (not thread!) parks here
			if err != nil && !errors.Is(err, io.EOF) {
				results <- result{id: id, err: err}
				return
			}
			results <- result{id: id, msg: strings.TrimSpace(string(buf[:n]))}
		}(i, c)
	}
	if err := <-acceptDone; err != nil {
		fmt.Fprintln(os.Stderr, "accept:", err)
		os.Exit(1)
	}
	dialed.Wait()
	time.Sleep(100 * time.Millisecond) // let every reader reach Read and park

	fmt.Printf("\nphase 1: %d goroutines parked in Read (runtime.NumGoroutine() = %d)\n",
		numConns, runtime.NumGoroutine())
	fmt.Println("   yet the OS thread count stays tiny — parked goroutines cost ~a few KB of")
	fmt.Println("   memory each, not a thread. (There's no portable stdlib way to count")
	fmt.Println("   threads; on Linux see /proc/self/status below, on macOS try")
	fmt.Println("   `ps -M <pid>` in another terminal.)")
	if runtime.GOOS == "linux" {
		// Only meaningful on Linux: the Threads: line of /proc/self/status.
		if data, err := os.ReadFile("/proc/self/status"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "Threads:") {
					fmt.Println("   /proc/self/status ->", line)
				}
			}
		}
	}

	// ---------------- phase 2: wake exactly three of them ----------------
	// Writing into a conn makes its peer's fd readable; the kernel tells
	// epoll/kqueue; the netpoller readies exactly the goroutine parked on
	// that fd. Nobody else wakes up — no thundering herd, no O(n) scan.
	fmt.Println("\nphase 2: writing to conns 7, 42 and 999 — watch those exact readers wake")
	for _, id := range []int{7, 42, 999} {
		fmt.Fprintf(server[id], "wakeup for reader %d\n", id)
	}
	for i := 0; i < 3; i++ {
		r := <-results
		fmt.Printf("   reader %-3d unblocked, got: %q\n", r.id, r.msg)
	}

	// ------------- phase 3: deadlines unblock everyone else --------------
	// The remaining 997 goroutines are still parked. SetReadDeadline arms a
	// timer INSIDE the same poller; when it fires, the poller readies the
	// goroutine even though the fd never became readable, and Read returns
	// a timeout error. This is the only way a blocked Read can be forced to
	// return without data, EOF, or closing the conn.
	fmt.Println("\nphase 3: SetReadDeadline(now) on all remaining conns — forced unblock")
	deadline := time.Now() // already passed: unblock immediately
	for i, c := range client {
		if i == 7 || i == 42 || i == 999 {
			continue // these already finished
		}
		c.SetReadDeadline(deadline)
	}
	timeouts := 0
	for i := 0; i < numConns-3; i++ {
		r := <-results
		var ne net.Error
		// Both checks pass: the concrete error is an *net.OpError wrapping
		// os.ErrDeadlineExceeded, and it satisfies net.Error with
		// Timeout() == true.
		if errors.As(r.err, &ne) && ne.Timeout() && errors.Is(r.err, os.ErrDeadlineExceeded) {
			timeouts++
		} else {
			fmt.Fprintf(os.Stderr, "reader %d: unexpected result %v %q\n", r.id, r.err, r.msg)
		}
	}
	fmt.Printf("   %d readers returned a timeout error (os.ErrDeadlineExceeded)\n", timeouts)
	fmt.Println("   deadlines are the poller forcibly unblocking you.")

	for i := range client {
		client[i].Close()
		if server[i] != nil {
			server[i].Close()
		}
	}
	time.Sleep(50 * time.Millisecond) // let reader goroutines exit
	fmt.Printf("\ngoroutines at end: %d — everyone was unblocked, nothing leaked\n",
		runtime.NumGoroutine())
}
