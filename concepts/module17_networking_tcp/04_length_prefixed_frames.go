// Module 17, Example 4 — Length-prefixed framing: the binary answer to
// "TCP has no message boundaries".
//
// Newline framing (example 2) works for text, but breaks the moment a
// payload may CONTAIN the delimiter (binary data, JSON with newlines...).
// The robust alternative: prefix every message with its length, in a
// fixed-size header the receiver can always read first.
//
//	wire format:  [ 4-byte big-endian uint32 = N ][ N bytes of payload ]
//
// This is how many real protocols frame messages (gRPC/HTTP2 frames,
// Kafka, Postgres wire protocol, ...).
//
// Run with: go run 04_length_prefixed_frames.go
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// maxFrameSize guards against a corrupt/hostile length header making us
// allocate gigabytes. ALWAYS bound lengths you read off the network.
const maxFrameSize = 1 << 20 // 1 MiB

// writeFrame sends one framed message: header first, then payload.
func writeFrame(conn net.Conn, payload []byte) error {
	// Encode the length as a fixed 4-byte header. Big-endian ("network
	// byte order") is the convention on the wire; encoding/binary does
	// the byte shuffling for us.
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))

	// Two Writes are fine — TCP will glue them back into one stream.
	// (Real code often uses net.Buffers or a single append to save a
	// syscall, but correctness-wise this is complete.)
	if _, err := conn.Write(header[:]); err != nil {
		return fmt.Errorf("write frame header: %w", err)
	}
	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write frame payload: %w", err)
	}
	return nil
}

// readFrame receives one framed message: read EXACTLY 4 header bytes,
// decode the length, then read EXACTLY that many payload bytes.
func readFrame(conn net.Conn) ([]byte, error) {
	// conn.Read alone may return FEWER bytes than asked (stream
	// semantics!), so "read exactly k bytes" is io.ReadFull's job:
	// it loops over Read until buf is full or an error occurs.
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		// io.EOF here = peer closed BETWEEN frames: a clean end-of-stream.
		// io.ErrUnexpectedEOF = closed MID-frame: a truncated message.
		return nil, err
	}

	length := binary.BigEndian.Uint32(header[:])
	if length > maxFrameSize {
		return nil, fmt.Errorf("frame of %d bytes exceeds limit %d", length, maxFrameSize)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, fmt.Errorf("read frame payload: %w", err)
	}
	return payload, nil
}

func main() {
	log.SetFlags(0)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	fmt.Println("frame server listening on", ln.Addr())

	// Server: read frames until EOF, replying to each with an
	// uppercase-tagged frame — proving both sides agree on the framing.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for i := 1; ; i++ {
			payload, err := readFrame(conn)
			if err != nil {
				if errors.Is(err, io.EOF) {
					fmt.Println("server: clean EOF, client finished")
				} else {
					log.Printf("server: readFrame: %v", err)
				}
				return
			}
			fmt.Printf("server: frame %d: %d bytes: %q\n", i, len(payload), payload)
			reply := fmt.Sprintf("ACK %d: %s", i, payload)
			if err := writeFrame(conn, []byte(reply)); err != nil {
				log.Printf("server: writeFrame: %v", err)
				return
			}
		}
	}()

	// Client: send a few frames of very different sizes — including one
	// with embedded newlines, which would have broken newline framing.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		log.Fatalf("dial: %v", err)
	}

	messages := [][]byte{
		[]byte("hi"),
		[]byte("a message\nwith newlines\ninside it"), // fine: length says where it ends
		[]byte("the third and final frame"),
	}
	for _, msg := range messages {
		if err := writeFrame(conn, msg); err != nil {
			log.Fatalf("client: %v", err)
		}
		reply, err := readFrame(conn)
		if err != nil {
			log.Fatalf("client: %v", err)
		}
		fmt.Printf("client: got reply: %q\n", reply)
	}

	conn.Close() // -> server's next readFrame sees io.EOF between frames
	wg.Wait()
	fmt.Println("done")
}
