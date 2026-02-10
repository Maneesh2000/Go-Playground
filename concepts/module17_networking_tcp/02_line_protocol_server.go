// Module 17, Example 2 — A newline-framed text protocol over TCP: a tiny
// key/value store speaking GET/SET/QUIT, built with bufio, serving several
// concurrent clients, with a per-connection idle timeout via SetReadDeadline.
//
// TCP gives you a raw byte stream with NO message boundaries, so every
// protocol must invent its own FRAMING. The oldest trick in the book:
// one message per line, '\n' as the delimiter (SMTP, POP3, Redis, HTTP/1.1
// headers all do a variant of this).
//
// Run with: go run 02_line_protocol_server.go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// How long a client may stay silent before the server hangs up on it.
// Tiny here so the demo can SHOW the timeout firing; real servers use
// seconds or minutes.
const idleTimeout = 400 * time.Millisecond

// kvStore is the shared state; a mutex makes it safe for the many
// per-connection goroutines that touch it concurrently.
type kvStore struct {
	mu   sync.Mutex
	data map[string]string
}

func (s *kvStore) get(k string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[k]
	return v, ok
}

func (s *kvStore) set(k, v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = v
}

// serveClient speaks the line protocol with one client:
//
//	SET key value\n  -> OK\n
//	GET key\n        -> VALUE <v>\n   or  ERR no such key\n
//	QUIT\n           -> BYE\n and close
func serveClient(conn net.Conn, store *kvStore) {
	defer conn.Close()

	// bufio.Reader turns the raw stream into something we can pull LINES
	// from: ReadString('\n') keeps reading (across however many TCP
	// segments) until it sees the delimiter. bufio.Writer batches our
	// small replies into fewer syscalls; Flush pushes them out.
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// IDLE TIMEOUT: deadlines are ABSOLUTE points in time, not
		// durations — so to say "at most 400ms of silence PER read" we
		// must re-arm the deadline before every read. If the deadline
		// passes while Read is blocked, Read fails with a timeout error.
		conn.SetReadDeadline(time.Now().Add(idleTimeout))

		line, err := reader.ReadString('\n')
		if err != nil {
			// Was it our idle timeout? net.Error's Timeout() method says so.
			var nerr net.Error
			if errors.As(err, &nerr) && nerr.Timeout() {
				log.Printf("server: %s idle for %v, closing", conn.RemoteAddr(), idleTimeout)
				fmt.Fprint(writer, "ERR idle timeout, bye\n")
				writer.Flush()
				return
			}
			// Otherwise: io.EOF (client hung up) or a real error.
			log.Printf("server: %s read ended: %v", conn.RemoteAddr(), err)
			return
		}

		// Parse "VERB arg1 arg2..." from the line (minus the '\n').
		parts := strings.SplitN(strings.TrimRight(line, "\r\n"), " ", 3)
		switch strings.ToUpper(parts[0]) {
		case "SET":
			if len(parts) != 3 {
				fmt.Fprint(writer, "ERR usage: SET key value\n")
				break
			}
			store.set(parts[1], parts[2])
			fmt.Fprint(writer, "OK\n")
		case "GET":
			if len(parts) != 2 {
				fmt.Fprint(writer, "ERR usage: GET key\n")
				break
			}
			if v, ok := store.get(parts[1]); ok {
				fmt.Fprintf(writer, "VALUE %s\n", v)
			} else {
				fmt.Fprint(writer, "ERR no such key\n")
			}
		case "QUIT":
			fmt.Fprint(writer, "BYE\n")
			writer.Flush()
			return
		default:
			fmt.Fprintf(writer, "ERR unknown command %q\n", parts[0])
		}
		writer.Flush() // actually send the buffered reply
	}
}

// client dials the server, sends each command, and prints each reply.
// The same newline framing is used on the client side: one command out,
// read exactly one line back.
func client(name, addr string, commands []string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("%s dial: %v", name, err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for _, cmd := range commands {
		fmt.Fprintf(conn, "%s\n", cmd) // Conn is an io.Writer — Fprintf works
		reply, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("%s: read: %v", name, err)
			return
		}
		fmt.Printf("%s> %-16s -> %s", name, cmd, reply)
	}
}

func main() {
	log.SetFlags(0)

	store := &kvStore{data: make(map[string]string)}

	ln, err := net.Listen("tcp", "127.0.0.1:0") // ephemeral port, no collisions
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	log.Printf("server: KV store listening on %s", addr)

	// Accept loop: every client gets its own serveClient goroutine, so
	// the two concurrent clients below are served simultaneously.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed -> shut down
			}
			go serveClient(conn, store)
		}
	}()

	// ---- Two clients talking to the server CONCURRENTLY -------------------
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		client("alice", addr, []string{
			"SET lang go",
			"SET editor vim",
			"GET lang",
			"QUIT",
		})
	}()
	go func() {
		defer wg.Done()
		client("bob", addr, []string{
			"GET missing",
			"SET fruit mango",
			"GET fruit",
			"BOGUS",
			"QUIT",
		})
	}()
	wg.Wait()

	// ---- Demonstrate the idle timeout firing -------------------------------
	// This client connects, sends one command, then goes silent — longer
	// than idleTimeout — so the server's re-armed read deadline expires
	// and it hangs up on us.
	fmt.Println("\n-- idle client: sending nothing and waiting for the server to hang up --")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("idle dial: %v", err)
	}
	fmt.Fprint(conn, "GET lang\n")
	r := bufio.NewReader(conn)
	first, _ := r.ReadString('\n')
	fmt.Printf("idle> GET lang        -> %s", first)

	// ...now we just sit here. After idleTimeout the server sends its
	// timeout message and closes; our next read sees that, then EOF.
	msg, err := r.ReadString('\n')
	fmt.Printf("idle> (silent...)     -> %s(err=%v)\n", msg, err)
	conn.Close()

	ln.Close()
	fmt.Println("done")
}
