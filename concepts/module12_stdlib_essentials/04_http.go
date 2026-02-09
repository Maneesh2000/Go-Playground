// Module 12, example 4: net/http — a real server with Go 1.22 routing +
// middleware, and a client with a timeout that calls it.
//
// Run with: go run 04_http.go
//
// The program starts a server on 127.0.0.1:8090, exercises it with a
// client, prints the responses, then shuts the server down — so it's
// fully self-contained and exits on its own.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Note struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// notes is our "database". A real service would guard it with a mutex
// (handlers run concurrently!) — kept simple here because our demo client
// makes one request at a time.
var notes = map[string]Note{
	"1": {ID: "1", Text: "learn net/http"},
}

// ---------------------------------------------------------------------------
// Middleware in ~10 lines: a middleware is just func(http.Handler) http.Handler.
// It runs code before/after calling the wrapped handler. Chain as many as
// you like: logging(auth(mux)).
// ---------------------------------------------------------------------------
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r) // call the real handler
		log.Printf("%-6s %-12s %v", r.Method, r.URL.Path, time.Since(start).Round(time.Microsecond))
	})
}

func newServer() *http.Server {
	mux := http.NewServeMux()

	// Go 1.22 patterns: "METHOD /path/{wildcard}". The mux now routes by
	// method AND extracts path parameters — no third-party router needed.

	// GET a single note; {id} is available via r.PathValue.
	mux.HandleFunc("GET /notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		note, ok := notes[id]
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(note) // encode straight onto the wire
	})

	// POST creates a note from a JSON body.
	mux.HandleFunc("POST /notes", func(w http.ResponseWriter, r *http.Request) {
		var n Note
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
			return
		}
		n.ID = fmt.Sprint(len(notes) + 1)
		notes[n.ID] = n
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated) // set status BEFORE writing the body
		json.NewEncoder(w).Encode(n)
	})

	// "GET /{$}" matches ONLY the root path. A plain "/" pattern would be a
	// catch-all for everything unmatched — a common surprise.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "notes API: GET /notes/{id}, POST /notes")
	})

	return &http.Server{
		Addr:    "127.0.0.1:8090",
		Handler: logging(mux), // wrap the whole mux in middleware
		// Production servers should also set ReadTimeout/WriteTimeout.
	}
}

func main() {
	log.SetFlags(0) // tidy demo output

	// Start the server in a goroutine so main can act as the client.
	srv := newServer()
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	time.Sleep(100 * time.Millisecond) // crude "wait until listening" for demo purposes

	// -------------------------- the client ------------------------------
	// NEVER use the zero-config client for real work: http.Get has NO
	// timeout, so a hung server hangs your program forever.
	client := &http.Client{Timeout: 5 * time.Second}
	base := "http://127.0.0.1:8090"

	// GET
	resp, err := client.Get(base + "/notes/1")
	if err != nil {
		log.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close() // ALWAYS close the body, or you leak connections
	fmt.Printf("GET /notes/1     → %d %s", resp.StatusCode, body)

	// GET a missing note → 404
	resp, err = client.Get(base + "/notes/99")
	if err != nil {
		log.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("GET /notes/99    → %d %s", resp.StatusCode, body)

	// POST with a JSON body. http.Post takes (url, contentType, bodyReader).
	// strings.NewReader/bytes.NewReader turn data into the io.Reader it wants.
	resp, err = client.Post(base+"/notes", "application/json",
		jsonBody(Note{Text: "buy milk"}))
	if err != nil {
		log.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("POST /notes      → %d %s", resp.StatusCode, body)

	// Wrong method on a routed path → 405 Method Not Allowed, for free.
	req, _ := http.NewRequest(http.MethodDelete, base+"/notes/1", nil)
	resp, err = client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body) // drain unread bodies so conns are reused
	resp.Body.Close()
	fmt.Printf("DELETE /notes/1  → %d (mux enforces methods)\n", resp.StatusCode)

	// ------------------------ graceful shutdown -------------------------
	// Shutdown stops accepting new connections and waits (up to the ctx
	// deadline) for in-flight requests to finish. Module 13 builds on this.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown:", err)
	}
	fmt.Println("server stopped cleanly")
}

// jsonBody marshals v and returns it as an io.Reader for a request body.
func jsonBody(v any) io.Reader {
	data, err := json.Marshal(v)
	if err != nil {
		log.Fatal(err) // fine for a demo; return the error in real code
	}
	return bytes.NewReader(data)
}
