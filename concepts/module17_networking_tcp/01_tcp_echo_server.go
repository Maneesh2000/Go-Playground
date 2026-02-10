// Module 17, Example 1 — A minimal TCP echo server: net.Listen, the
// Accept loop, one goroutine per connection, Read/Write, and Close.
//
// The server and a demo client both live in THIS file: the server runs in
// goroutines, the client runs in main, so `go run` exercises a full
// client/server round trip and then exits on its own.
//
// Run with: go run 01_tcp_echo_server.go
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// handleConn serves ONE client. The server spawns one of these per
// accepted connection — that goroutine-per-connection model is the
// idiomatic Go server shape (goroutines are cheap; blocking Reads are fine).
func handleConn(conn net.Conn) {
	// Always close the connection when this handler returns; that sends
	// FIN to the peer and frees the file descriptor.
	defer conn.Close()

	// Conn.RemoteAddr() tells us who connected (their ip:port).
	// Conn.LocalAddr() would be OUR end of this particular connection.
	log.Printf("server: accepted connection from %s", conn.RemoteAddr())

	buf := make([]byte, 1024)
	for {
		// Conn.Read fills buf with whatever bytes have arrived — between
		// 1 and len(buf) of them. TCP is a BYTE STREAM: n bytes here may
		// be half a message or three messages glued together. Read blocks
		// until at least one byte arrives, the peer closes (io.EOF), or
		// an error/deadline occurs.
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// io.EOF = the client closed its side. A normal goodbye,
				// not a failure.
				log.Printf("server: %s disconnected", conn.RemoteAddr())
			} else {
				log.Printf("server: read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}

		// Echo the bytes back. Conn.Write blocks until ALL len(b) bytes
		// are handed to the OS or an error occurs — if err == nil you can
		// rely on n == len(b), so no manual write loop is needed in Go.
		if _, err := conn.Write(buf[:n]); err != nil {
			log.Printf("server: write error to %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
}

// runServer starts listening and runs the Accept loop until the listener
// is closed. It returns the listener (so main can shut it down) and a
// WaitGroup that finishes when the Accept loop has exited.
func runServer() (net.Listener, *sync.WaitGroup) {
	// net.Listen("tcp", "127.0.0.1:0"):
	//   network "tcp"  -> IPv4 or IPv6 TCP ("tcp4"/"tcp6" force one).
	//   address ":0"   -> port 0 asks the OS for a free EPHEMERAL port,
	//                     so this example never collides with a busy port.
	// Under the hood this performs the classic socket() + bind() + listen()
	// system calls and returns a net.Listener.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	// Listener.Addr() reports the address we ACTUALLY bound — essential
	// with port 0, since only the OS knows which port we got.
	log.Printf("server: listening on %s", ln.Addr())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// Listener.Accept blocks until a client connects, then returns
			// a fresh net.Conn dedicated to that one client. The listener
			// keeps listening; each Conn is an independent conversation.
			conn, err := ln.Accept()
			if err != nil {
				// When main calls ln.Close(), the blocked Accept returns
				// an error (net.ErrClosed). That is THE graceful-shutdown
				// signal for an accept loop.
				log.Printf("server: accept loop stopping: %v", err)
				return
			}
			go handleConn(conn) // one goroutine per client
		}
	}()
	return ln, &wg
}

func main() {
	log.SetFlags(0) // cleaner demo output

	ln, wg := runServer()

	// ---- Client side (same process, real TCP over loopback) ---------------
	// net.Dial is the client mirror of net.Listen: it performs socket() +
	// connect() and returns a net.Conn once the TCP handshake completes.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	log.Printf("client: connected %s -> %s", conn.LocalAddr(), conn.RemoteAddr())

	buf := make([]byte, 1024)
	for _, msg := range []string{"hello", "echo echo", "goodbye"} {
		if _, err := conn.Write([]byte(msg)); err != nil {
			log.Fatalf("client write: %v", err)
		}
		n, err := conn.Read(buf) // read the echo back
		if err != nil {
			log.Fatalf("client read: %v", err)
		}
		fmt.Printf("client: sent %q, got back %q\n", msg, string(buf[:n]))
	}

	// Conn.Close() sends FIN; the server's blocked Read returns io.EOF.
	conn.Close()

	// Listener.Close() unblocks the server's Accept, ending the accept loop.
	ln.Close()
	wg.Wait() // wait for the accept loop goroutine to finish
	fmt.Println("done: listener closed, server stopped cleanly")
}
