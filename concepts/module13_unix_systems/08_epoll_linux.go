//go:build linux

// Module 13, example 8: a real, minimal epoll echo server — the raw Linux
// syscalls that Go's netpoller wraps for you. LINUX ONLY (epoll does not
// exist on macOS; the Darwin equivalent is kqueue). On this Mac you can
// still verify it compiles, and run it in a container:
//
//	GOOS=linux GOARCH=amd64 go build 08_epoll_linux.go   # cross-compile check
//	docker run --rm -v $PWD:/src -w /src golang:1.26 go run 08_epoll_linux.go
//
// One thread, one loop, many connections — the classic event-driven shape:
//
//	epoll_create1 -> one "interest list" fd
//	epoll_ctl     -> ADD/MOD/DEL fds you care about
//	epoll_wait    -> sleep until one or more fds are READY, get just those
//
// Everything is non-blocking: accept and read return EAGAIN instead of
// parking, so the single thread is only ever parked inside epoll_wait.
// We use LEVEL-TRIGGERED mode (the default): epoll_wait keeps reporting an
// fd as long as unread data remains — forgiving, you may leave bytes behind.
// EDGE-TRIGGERED (EPOLLET) fires only on the *transition* to readable, so
// you must drain until EAGAIN every time or lose wakeups forever.
//
// The program self-tests: it connects to itself, sends a message, checks the
// echo, then shuts down cleanly — it terminates on its own.
package main

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"syscall"
)

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	// ------------- a listening socket, by hand, non-blocking -------------
	// This is what net.Listen does under the hood (minus portability).
	lfd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		fatalf("socket: %v", err)
	}
	defer syscall.Close(lfd)
	// Port 0 = kernel picks a free port; 127.0.0.1 = loopback only.
	addr := syscall.SockaddrInet4{Port: 0, Addr: [4]byte{127, 0, 0, 1}}
	if err := syscall.Bind(lfd, &addr); err != nil {
		fatalf("bind: %v", err)
	}
	if err := syscall.Listen(lfd, 128); err != nil {
		fatalf("listen: %v", err)
	}
	// THE key step: without this, accept() would park our only thread.
	if err := syscall.SetNonblock(lfd, true); err != nil {
		fatalf("nonblock: %v", err)
	}
	sa, err := syscall.Getsockname(lfd) // what port did we get?
	if err != nil {
		fatalf("getsockname: %v", err)
	}
	port := sa.(*syscall.SockaddrInet4).Port
	fmt.Printf("epoll echo server on 127.0.0.1:%d (single thread)\n", port)

	// --------------------------- epoll setup -----------------------------
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		fatalf("epoll_create1: %v", err)
	}
	defer syscall.Close(epfd)
	// Register the LISTENER: "wake me when it's readable" — for a listening
	// socket, readable means a connection is waiting to be accepted.
	ev := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(lfd)}
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, lfd, &ev); err != nil {
		fatalf("epoll_ctl add listener: %v", err)
	}

	// ------------------------ self-test client ---------------------------
	// A normal net.Conn client in a goroutine (the SERVER stays one
	// thread). It exercises the whole path, then flips `done`.
	var done atomic.Bool
	go func() {
		defer done.Store(true)
		c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			fatalf("self-test dial: %v", err)
		}
		defer c.Close()
		msg := "hello epoll"
		fmt.Fprint(c, msg)
		buf := make([]byte, 64)
		n, err := c.Read(buf)
		if err != nil {
			fatalf("self-test read: %v", err)
		}
		if got := string(buf[:n]); got != msg {
			fatalf("self-test: sent %q, echoed %q", msg, got)
		}
		fmt.Printf("self-test client: sent and received %q — echo works\n", msg)
	}()

	// --------------------------- event loop ------------------------------
	events := make([]syscall.EpollEvent, 64)
	buf := make([]byte, 4096)
	for !done.Load() {
		// Park in the kernel until an fd is ready, at most 100ms (the
		// timeout lets us notice `done`; a real server would wait forever
		// or integrate a shutdown fd/signalfd into the same epoll set).
		n, err := syscall.EpollWait(epfd, events, 100)
		if err == syscall.EINTR {
			continue // interrupted by a signal — harmless, retry
		}
		if err != nil {
			fatalf("epoll_wait: %v", err)
		}
		// We are handed ONLY the ready fds — O(ready), not O(all conns).
		// This is epoll's win over select/poll's O(n) rescans.
		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			if fd == lfd {
				// Listener ready: accept until EAGAIN (there may be
				// several queued connections behind one wakeup).
				for {
					cfd, _, err := syscall.Accept(lfd)
					if err == syscall.EAGAIN {
						break // queue drained
					}
					if err != nil {
						fatalf("accept: %v", err)
					}
					// New conns must be non-blocking too, then join
					// the interest list — same dance as the listener.
					syscall.SetNonblock(cfd, true)
					cev := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(cfd)}
					if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, cfd, &cev); err != nil {
						fatalf("epoll_ctl add conn: %v", err)
					}
					fmt.Printf("event loop: accepted fd %d, registered EPOLLIN\n", cfd)
				}
				continue
			}
			// A connection is readable: drain it, echo everything back.
			for {
				nr, err := syscall.Read(fd, buf)
				if err == syscall.EAGAIN {
					break // nothing left right now — epoll will re-arm us
				}
				if err != nil || nr == 0 { // error or EOF: deregister, close
					syscall.EpollCtl(epfd, syscall.EPOLL_CTL_DEL, fd, nil)
					syscall.Close(fd)
					fmt.Printf("event loop: fd %d closed\n", fd)
					break
				}
				// Echo. (Demo shortcut: we assume the socket buffer can
				// take it. A robust server handles a short/EAGAIN write
				// by registering EPOLLOUT — see the exercises.)
				if _, err := syscall.Write(fd, buf[:nr]); err != nil {
					fatalf("write: %v", err)
				}
				fmt.Printf("event loop: echoed %d bytes on fd %d\n", nr, fd)
			}
		}
	}
	fmt.Println("self-test complete, shutting down — bye")
}
