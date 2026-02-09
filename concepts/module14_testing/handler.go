// handler.go — an HTTP handler designed for testability.
//
// The trick: the handler depends on a small INTERFACE (Greeter), not a
// concrete implementation. Production injects the real thing; tests inject
// a fake. This is "accept interfaces" put to work (more in Module 15).
package testingdemo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Greeter is the handler's one dependency. Small interfaces (1–2 methods)
// are cheap to fake — no mocking framework required.
type Greeter interface {
	Greet(name string) (string, error)
}

// ErrNoName is a sentinel the handler maps to 400 Bad Request.
var ErrNoName = errors.New("no name given")

// EnglishGreeter is the "real" production implementation.
type EnglishGreeter struct{}

func (EnglishGreeter) Greet(name string) (string, error) {
	if name == "" {
		return "", ErrNoName
	}
	return "Hello, " + name + "!", nil
}

// GreetResponse is the JSON shape we return.
type GreetResponse struct {
	Message string `json:"message"`
}

// NewGreetHandler wires routes to the given Greeter and returns an
// http.Handler. Returning the interface type keeps callers (main and tests)
// decoupled from ServeMux specifics.
func NewGreetHandler(g Greeter) http.Handler {
	mux := http.NewServeMux()

	// Go 1.22 pattern routing: method + path wildcard (see Module 12).
	mux.HandleFunc("GET /greet/{name}", func(w http.ResponseWriter, r *http.Request) {
		msg, err := g.Greet(r.PathValue("name"))
		switch {
		case errors.Is(err, ErrNoName):
			http.Error(w, "name required", http.StatusBadRequest)
			return
		case err != nil:
			// Log the real error server-side; don't leak internals to clients.
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(GreetResponse{Message: msg}); err != nil {
			// Headers are already sent; nothing useful left to do but note it.
			fmt.Println("encode:", err)
		}
	})

	return mux
}
