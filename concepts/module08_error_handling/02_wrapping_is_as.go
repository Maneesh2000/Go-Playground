// Module 08, Example 2 — Error wrapping with %w, and inspecting chains
// with errors.Is and errors.As.
//
// The idea: as an error travels up the call stack, each layer adds context
// ("what was I doing when this happened?") WITHOUT destroying the original
// error, so callers at the top can still detect the root cause.
//
// Run with: go run 02_wrapping_is_as.go
package main

import (
	"errors"
	"fmt"
)

// A sentinel error: a package-level, exported error value that callers can
// test for with errors.Is. (Standard library examples: io.EOF, sql.ErrNoRows,
// fs.ErrNotExist.)
var ErrNotFound = errors.New("user not found")

// A custom error TYPE that carries data (compare with errors.As below).
type QueryError struct {
	Query string
	Took  int // milliseconds
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query %q failed after %dms", e.Query, e.Took)
}

// ---- Three layers, each wrapping with %w --------------------------------------

// Layer 1 (bottom): the "database".
func dbLookup(id int) error {
	if id == 42 {
		return nil // found
	}
	if id < 0 {
		// A rich, typed error carrying details:
		return &QueryError{Query: fmt.Sprintf("SELECT * WHERE id=%d", id), Took: 31}
	}
	return ErrNotFound // the sentinel
}

// Layer 2 (middle): wraps with context. %w is the magic verb — it stores
// the original error inside the new one, forming a chain.
func getUser(id int) error {
	if err := dbLookup(id); err != nil {
		return fmt.Errorf("get user %d: %w", id, err)
	}
	return nil
}

// Layer 3 (top): wraps again.
func loadProfile(id int) error {
	if err := getUser(id); err != nil {
		return fmt.Errorf("load profile: %w", err)
	}
	return nil
}

func main() {
	// ---- The chain that %w builds -------------------------------------------
	//
	//   err := loadProfile(7)
	//
	//   ┌───────────────────────────┐
	//   │ "load profile: ..."       │   outermost (most context)
	//   └────────────┬──────────────┘
	//                │ wraps (%w)
	//   ┌────────────▼──────────────┐
	//   │ "get user 7: ..."         │
	//   └────────────┬──────────────┘
	//                │ wraps (%w)
	//   ┌────────────▼──────────────┐
	//   │ ErrNotFound               │   root cause
	//   └───────────────────────────┘
	//
	// Printing shows the whole story; errors.Is/As walk the chain.

	err := loadProfile(7)
	fmt.Println("printed error:", err)
	// -> load profile: get user 7: user not found

	// ---- errors.Is: "is this sentinel anywhere in the chain?" ----------------
	// A plain == would fail (err is the outer wrapper, not ErrNotFound).
	fmt.Println("err == ErrNotFound            ?", err == ErrNotFound)          // false!
	fmt.Println("errors.Is(err, ErrNotFound)   ?", errors.Is(err, ErrNotFound)) // true

	if errors.Is(err, ErrNotFound) {
		fmt.Println("=> caller can react: show a 404 page, not a 500")
	}

	// ---- errors.As: "is there a *QueryError in the chain? give it to me" ------
	err = loadProfile(-1)
	fmt.Println("\nprinted error:", err)

	var qe *QueryError // a pointer of the target type
	if errors.As(err, &qe) {
		// errors.As found it AND filled qe — now we can read its fields.
		fmt.Printf("=> extracted QueryError: query=%q took=%dms\n", qe.Query, qe.Took)
	}

	// ---- errors.Unwrap peels ONE layer (rarely needed directly) ---------------
	fmt.Println("\nunwrapping manually:")
	for e := err; e != nil; e = errors.Unwrap(e) {
		fmt.Printf("  layer: %v\n", e)
	}

	// ---- The %v trap ------------------------------------------------------------
	// %v formats the error into TEXT but does NOT wrap. The chain is broken
	// and errors.Is can no longer see the sentinel:
	broken := fmt.Errorf("load profile: %v", ErrNotFound) // note: %v, not %w
	fmt.Println("\nwith %v instead of %w:")
	fmt.Println("  errors.Is finds sentinel?", errors.Is(broken, ErrNotFound)) // false
	fmt.Println("  => rule: when passing an error upward, wrap with %w")

	// When NOT to wrap: if the lower-level error is an implementation detail
	// you don't want callers to depend on, use %v deliberately to hide it.
	// Wrapping makes the wrapped error part of your API.
}
