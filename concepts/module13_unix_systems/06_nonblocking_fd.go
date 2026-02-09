// Module 13, example 6: blocking vs non-blocking file descriptors — the raw
// mechanics that every event loop (and Go's netpoller) is built on.
//
//	go run 06_nonblocking_fd.go
//
// Works on macOS and Linux (any Unix). What it shows, step by step:
//
//  1. A pipe's read end in NON-BLOCKING mode: read() with no data returns
//     EAGAIN *immediately* instead of parking the thread. EAGAIN is not an
//     error in the usual sense — it's the kernel saying "not ready, try
//     later", and it is exactly the signal that readiness APIs
//     (select/poll/epoll/kqueue) were invented to replace with a callback.
//  2. After a write, the same read succeeds — the fd became "readable".
//  3. The same pipe back in BLOCKING mode: read() parks the calling thread
//     until a writer shows up. That is fine for one fd, catastrophic for
//     ten thousand (one parked OS thread each).
//  4. Bonus: *os.File on a pipe is pollable, so SetReadDeadline works —
//     the runtime's poller forcibly unblocks the read with
//     os.ErrDeadlineExceeded. Files join the same netpoller story as sockets.
package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func main() {
	// ------------------ 1. a pipe, read end non-blocking -----------------
	// syscall.Pipe gives us two RAW fds (plain ints, no Go wrapper), so the
	// Go runtime is not involved at all — we're talking straight to the
	// kernel, the way a C program or an event-loop library would.
	fds := make([]int, 2)
	if err := syscall.Pipe(fds); err != nil {
		fmt.Fprintln(os.Stderr, "pipe:", err)
		os.Exit(1)
	}
	rfd, wfd := fds[0], fds[1] // read end, write end
	defer syscall.Close(rfd)
	defer syscall.Close(wfd)

	// Flip the read end into non-blocking mode. Under the hood this is
	// fcntl(rfd, F_SETFL, O_NONBLOCK) — one bit on the open file.
	if err := syscall.SetNonblock(rfd, true); err != nil {
		fmt.Fprintln(os.Stderr, "set nonblock:", err)
		os.Exit(1)
	}

	buf := make([]byte, 64)

	// No data has been written yet. A BLOCKING read would park us here
	// forever. A NON-BLOCKING read returns at once with EAGAIN ("try
	// again") — on every modern Unix EWOULDBLOCK is the same value.
	_, err := syscall.Read(rfd, buf)
	if errors.Is(err, syscall.EAGAIN) {
		fmt.Println("1) non-blocking read, no data  ->", err, "(EAGAIN — kernel says: not ready)")
		fmt.Println("   this immediate 'not ready' is the signal readiness APIs are built on:")
		fmt.Println("   instead of retrying in a hot loop (busy-polling, 100% CPU), you hand the")
		fmt.Println("   fd to epoll/kqueue and get woken exactly when it becomes readable.")
	} else {
		fmt.Fprintln(os.Stderr, "unexpected result:", err)
		os.Exit(1)
	}

	// ------------------ 2. write, then the read succeeds ----------------
	msg := []byte("data arrived")
	if _, err := syscall.Write(wfd, msg); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	n, err := syscall.Read(rfd, buf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	fmt.Printf("2) after a write the fd is readable -> read %d bytes: %q\n", n, buf[:n])

	// ------------------ 3. contrast: blocking mode -----------------------
	// Back to blocking mode. Now read() PARKS the calling OS thread until
	// data shows up. We prove it: a helper goroutine writes after 100ms,
	// and the read below sits parked in the kernel for those 100ms.
	if err := syscall.SetNonblock(rfd, false); err != nil {
		fmt.Fprintln(os.Stderr, "set blocking:", err)
		os.Exit(1)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		syscall.Write(wfd, []byte("wake up"))
	}()
	start := time.Now()
	n, err = syscall.Read(rfd, buf) // <- thread parked in the kernel here
	if err != nil {
		fmt.Fprintln(os.Stderr, "blocking read:", err)
		os.Exit(1)
	}
	fmt.Printf("3) blocking read parked the thread for ~%v, then got %q\n",
		time.Since(start).Round(time.Millisecond), buf[:n])
	fmt.Println("   one parked thread per fd is why '1 thread per connection' doesn't scale —")
	fmt.Println("   10k idle connections = 10k idle threads. Readiness APIs fix exactly this.")

	// ------------- 4. bonus: os.File pipes are pollable too --------------
	// os.Pipe returns *os.File wrappers whose fds the Go runtime has
	// already registered with its poller (kqueue here on macOS, epoll on
	// Linux). That's why SetReadDeadline works on pipes: when the deadline
	// fires, the poller forcibly UNBLOCKS the goroutine stuck in Read and
	// makes it return os.ErrDeadlineExceeded. Same mechanism as net.Conn.
	// (Regular disk files are always "ready" and don't support deadlines.)
	pr, pw, err := os.Pipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "os.Pipe:", err)
		os.Exit(1)
	}
	defer pr.Close()
	defer pw.Close()

	pr.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	start = time.Now()
	_, err = pr.Read(buf) // no writer: parked goroutine, NOT a parked thread
	if errors.Is(err, os.ErrDeadlineExceeded) {
		fmt.Printf("4) os.File deadline unblocked a Read after ~%v -> %v\n",
			time.Since(start).Round(time.Millisecond), err)
		fmt.Println("   deadlines ARE the poller: a timer fires, the poller readies the goroutine.")
	} else {
		fmt.Fprintln(os.Stderr, "expected deadline error, got:", err)
		os.Exit(1)
	}
}
