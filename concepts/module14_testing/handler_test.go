// handler_test.go — testing HTTP handlers with net/http/httptest and a fake.
//
// Two levels shown here:
//  1. httptest.NewRecorder — call ServeHTTP directly, no network at all.
//  2. httptest.NewServer   — a real listener on 127.0.0.1 for client tests.
package testingdemo

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeGreeter is a hand-rolled test double. Because Greeter has ONE method,
// a struct with a function field replaces any mocking framework: each test
// scripts exactly the behavior it needs.
type fakeGreeter struct {
	greetFunc func(name string) (string, error)
}

func (f fakeGreeter) Greet(name string) (string, error) { return f.greetFunc(name) }

// ---------------------------------------------------------------------------
// Level 1: httptest.NewRecorder — the fast path. NewRequest builds an
// *http.Request without a client; NewRecorder is an in-memory
// http.ResponseWriter that records status, headers, and body.
// ---------------------------------------------------------------------------
func TestGreetHandler(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		greet      func(string) (string, error) // the fake's script
		wantStatus int
		wantBody   string // substring we expect in the body
	}{
		{
			name:       "happy path",
			url:        "/greet/Ada",
			greet:      func(n string) (string, error) { return "Hello, " + n + "!", nil },
			wantStatus: http.StatusOK,
			wantBody:   "Hello, Ada!",
		},
		{
			name:       "greeter says name missing",
			url:        "/greet/x", // route needs a value; the FAKE reports the error
			greet:      func(string) (string, error) { return "", ErrNoName },
			wantStatus: http.StatusBadRequest,
			wantBody:   "name required",
		},
		{
			name:       "greeter blows up → 500, internals not leaked",
			url:        "/greet/Ada",
			greet:      func(string) (string, error) { return "", errors.New("db on fire") },
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal error", // note: NOT "db on fire"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGreetHandler(fakeGreeter{greetFunc: tt.greet})

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req) // no sockets, no goroutines — a plain call

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", body, tt.wantBody)
			}
		})
	}
}

// The wrong METHOD should be rejected by 1.22 pattern routing itself.
func TestGreetHandlerMethodNotAllowed(t *testing.T) {
	handler := NewGreetHandler(EnglishGreeter{})

	req := httptest.NewRequest(http.MethodPost, "/greet/Ada", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// ---------------------------------------------------------------------------
// Level 2: httptest.NewServer — spins up a REAL server on a random localhost
// port. Use this when the thing under test is an HTTP *client*, or when you
// need real transport behavior (redirects, timeouts, TLS via NewTLSServer).
// ---------------------------------------------------------------------------
func TestGreetOverRealHTTP(t *testing.T) {
	srv := httptest.NewServer(NewGreetHandler(EnglishGreeter{}))
	defer srv.Close() // frees the port and shuts down cleanly

	resp, err := http.Get(srv.URL + "/greet/Gopher") // srv.URL = http://127.0.0.1:<port>
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got GreetResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("decoding %q: %v", body, err)
	}
	if want := "Hello, Gopher!"; got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
}
