// Module 07, Example 2 — Interface values under the hood, and the classic
// "nil interface vs interface holding a nil pointer" bug.
//
// Run with: go run 02_interface_values_nil.go
package main

import "fmt"

// A tiny interface for the demo.
type Greeter interface {
	Greet() string
}

type English struct{ Name string }

func (e *English) Greet() string { return "Hello, " + e.Name }

type Spanish struct{ Name string }

func (s *Spanish) Greet() string { return "Hola, " + s.Name }

func main() {
	// ---- An interface value is a (type, value) PAIR ------------------------
	//
	//    var g Greeter = &English{Name: "Ada"}
	//
	//    g
	//    ┌──────────────────────┐
	//    │ type : *main.English │  <- which concrete type is inside
	//    ├──────────────────────┤
	//    │ value: &{Ada}        │  <- the concrete data
	//    └──────────────────────┘
	//
	// Calling g.Greet() looks up Greet on the TYPE slot, passes the VALUE
	// slot as receiver. That's dynamic dispatch.

	var g Greeter = &English{Name: "Ada"}
	fmt.Println(g.Greet())
	fmt.Printf("dynamic type: %T, dynamic value: %v\n", g, g)

	// Reassigning changes BOTH slots:
	g = &Spanish{Name: "Ada"}
	fmt.Println(g.Greet())
	fmt.Printf("dynamic type: %T, dynamic value: %v\n\n", g, g)

	// ---- A truly nil interface ----------------------------------------------
	//
	//    var n Greeter          n
	//    ┌──────────────┐
	//    │ type : nil   │       both slots empty
	//    │ value: nil   │       => n == nil is TRUE
	//    └──────────────┘
	var n Greeter
	fmt.Println("nil interface        == nil ?", n == nil) // true
	// Calling a method on it panics — there is no type to dispatch on:
	//   n.Greet() // panic: nil pointer dereference

	// ---- THE CLASSIC BUG: interface holding a nil pointer -------------------
	//
	//    var p *English = nil   // a nil POINTER (fine, typed)
	//    var g2 Greeter = p     // stored in an interface...
	//
	//    g2
	//    ┌──────────────────────┐
	//    │ type : *main.English │  <- type slot is SET (not nil!)
	//    ├──────────────────────┤
	//    │ value: nil           │
	//    └──────────────────────┘
	//    => g2 == nil is FALSE, because the pair is not (nil, nil).
	var p *English // nil pointer
	var g2 Greeter = p

	fmt.Println("interface w/ nil ptr == nil ?", g2 == nil) // FALSE — surprise!
	fmt.Printf("its dynamic type is still: %T\n\n", g2)

	// ---- Where this actually bites: error returns ---------------------------
	// getError returns the *MyError pointer as the error interface.
	err := getErrorBroken(false)
	// We asked for "no error" (false), yet:
	if err != nil {
		fmt.Println("BUG: err != nil even though there was no error!")
		fmt.Printf("     because err = (type=%T, value=%v)\n", err, err)
	}

	// The fix: return the interface type and a LITERAL nil on success.
	err = getErrorFixed(false)
	if err == nil {
		fmt.Println("FIXED: err == nil as expected")
	}
}

// MyError is a custom error type (more on errors in Module 08).
type MyError struct{ msg string }

func (e *MyError) Error() string { return e.msg }

// BROKEN: declares a typed pointer, returns it via the error interface.
// When e is nil, the returned interface still carries type *MyError,
// so callers' `err != nil` check is TRUE. Ouch.
func getErrorBroken(fail bool) error {
	var e *MyError // nil pointer of concrete type
	if fail {
		e = &MyError{msg: "boom"}
	}
	return e // WRONG: wraps nil pointer in a non-nil interface
}

// FIXED: success path returns literal nil, so both interface slots are nil.
func getErrorFixed(fail bool) error {
	if fail {
		return &MyError{msg: "boom"}
	}
	return nil // a real nil interface
}
