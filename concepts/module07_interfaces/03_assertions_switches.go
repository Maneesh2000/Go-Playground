// Module 07, Example 3 — The empty interface (any), type assertions,
// type switches, and composing interfaces.
//
// Run with: go run 03_assertions_switches.go
package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

func main() {
	// ---- any / interface{} ---------------------------------------------------
	// `any` is a built-in alias for interface{} — the interface with ZERO
	// methods. Every type has "at least zero methods", so every type
	// satisfies it. It means "could be anything".
	var v any

	v = 42
	fmt.Printf("v = %v (%T)\n", v, v)
	v = "hello"
	fmt.Printf("v = %v (%T)\n", v, v)
	v = []float64{1.5, 2.5}
	fmt.Printf("v = %v (%T)\n\n", v, v)

	// The price: the compiler no longer knows what's inside, so you cannot
	// do anything type-specific with v directly:
	//   v + 1            // compile error
	//   len(v)           // compile error
	// You must first recover the concrete type.

	// ---- Type assertion --------------------------------------------------------
	v = "gopher"

	// Form 1: single-value. PANICS if the dynamic type is wrong.
	s := v.(string)
	fmt.Println("asserted string:", s)

	// Form 2: comma-ok. Never panics; ok tells you if it matched.
	// This is the form you should reach for by default.
	if n, ok := v.(int); ok {
		fmt.Println("it was an int:", n)
	} else {
		fmt.Println("not an int — ok was false, n is zero value:", n)
	}

	// Uncomment to watch the panic from a failed single-value assertion:
	//   bad := v.(int) // panic: interface conversion: interface {} is
	//                  //        string, not int

	// ---- Type switch ------------------------------------------------------------
	// A switch on the DYNAMIC TYPE. In each case, the variable `x` already
	// has that case's concrete type — no extra assertion needed.
	values := []any{42, "hello", 3.14, true, []int{1, 2, 3}, nil}
	for _, item := range values {
		describeAny(item)
	}
	fmt.Println()

	// ---- Composing interfaces -----------------------------------------------------
	// Big interfaces are built by EMBEDDING small ones. From the standard
	// library (package io):
	//
	//   type Reader interface { Read(p []byte) (n int, err error) }
	//   type Writer interface { Write(p []byte) (n int, err error) }
	//   type ReadWriter interface {  // = Reader AND Writer
	//       Reader
	//       Writer
	//   }
	//
	// bytes.Buffer has both Read and Write, so *bytes.Buffer is an
	// io.Reader, an io.Writer, AND an io.ReadWriter — implicitly.
	var rw io.ReadWriter = &bytes.Buffer{}

	// Write into it via the Writer half...
	fmt.Fprintf(rw, "written through io.Writer at %s", "runtime")

	// ...read back via the Reader half. io.ReadAll accepts any io.Reader.
	data, err := io.ReadAll(rw)
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	fmt.Printf("read back: %q\n", data)

	// The payoff of small composed interfaces: strings.NewReader, files,
	// network sockets, gzip streams, HTTP bodies — ALL are io.Readers, so
	// one function signature handles them all:
	fmt.Println(countBytes(strings.NewReader("any reader works here")))
}

// describeAny uses a type switch to handle each concrete type differently.
func describeAny(v any) {
	switch x := v.(type) {
	case nil:
		// a nil interface matches `case nil`
		fmt.Println("nil          -> nothing here")
	case int:
		fmt.Printf("int          -> %d (doubled: %d)\n", x, x*2)
	case string:
		fmt.Printf("string       -> %q (len %d)\n", x, len(x))
	case float64:
		fmt.Printf("float64      -> %.2f\n", x)
	case bool:
		fmt.Printf("bool         -> %v\n", x)
	case []int:
		fmt.Printf("[]int        -> %v (sum of %d elems)\n", x, len(x))
	default:
		fmt.Printf("%-12T -> no special handling\n", x)
	}
}

// countBytes works with ANY source of bytes because it asks only for the
// tiny io.Reader interface. "Accept interfaces, return concrete types."
func countBytes(r io.Reader) string {
	data, err := io.ReadAll(r)
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("read %d bytes", len(data))
}
