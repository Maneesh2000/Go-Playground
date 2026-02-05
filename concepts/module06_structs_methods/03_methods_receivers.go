// Module 06, Example 3 — Methods: value receivers vs pointer receivers,
// and method sets.
//
// A method is just a function with a "receiver" — the value it's attached
// to. The receiver appears in its own parameter list before the name.
//
// Run with: go run 03_methods_receivers.go
package main

import (
	"fmt"
	"strings"
)

type Counter struct {
	name  string
	count int
}

// ---- Value receiver --------------------------------------------------------
// (c Counter) — the method gets a COPY of the counter. Good for methods that
// only READ. Any mutation here would be lost when the method returns.
func (c Counter) Describe() string {
	return fmt.Sprintf("%s = %d", c.name, c.count)
}

// This looks like it increments, but it increments a COPY. Classic bug —
// kept here on purpose so you can see it fail below.
func (c Counter) IncrementBroken() {
	c.count++ // mutates the copy; caller never sees this
}

// ---- Pointer receiver ------------------------------------------------------
// (c *Counter) — the method gets the ADDRESS of the counter, so it can
// modify the original. Required for any method that mutates state.
func (c *Counter) Increment() {
	c.count++ // c.count is shorthand for (*c).count
}

// Rules for choosing (memorize these):
//  1. Method mutates the receiver?           -> pointer receiver.
//  2. Receiver is a large struct?            -> pointer receiver (avoid copies).
//  3. ANY method on the type needs a pointer -> make ALL of them pointers
//     (consistency: don't mix unless you have a strong reason).
//  4. Small immutable value type (a point, a duration, money)? -> value receiver.

// ---- Methods on non-struct types --------------------------------------------
// You can define methods on ANY named type declared in your package,
// not just structs:
type ShoutString string

func (s ShoutString) Shout() string {
	return strings.ToUpper(string(s)) + "!!!"
}

func main() {
	// ---- The broken value-receiver mutation --------------------------------
	c := Counter{name: "clicks"}
	c.IncrementBroken()
	c.IncrementBroken()
	fmt.Println("after 2 IncrementBroken calls:", c.Describe()) // clicks = 0 (!)

	// ---- The pointer-receiver version works --------------------------------
	c.Increment()
	c.Increment()
	fmt.Println("after 2 Increment calls:     ", c.Describe()) // clicks = 2

	// Note: we called c.Increment() on a VALUE variable, yet it worked.
	// The compiler rewrote it to (&c).Increment() because c is addressable.
	// This convenience is why the difference feels invisible... until
	// interfaces get involved (see method sets below).

	// ---- Method values and method expressions -------------------------------
	// Methods are first-class: you can store a bound method in a variable.
	inc := c.Increment // "method value": receiver c is captured
	inc()
	inc()
	fmt.Println("after calling stored method:  ", c.Describe()) // clicks = 4

	// ---- Methods on a non-struct type ---------------------------------------
	s := ShoutString("go is fun")
	fmt.Println(s.Shout())

	// ---- Method sets ---------------------------------------------------------
	// The METHOD SET decides which interfaces a type satisfies:
	//   * value  Counter : only value-receiver methods   {Describe, IncrementBroken}
	//   * pointer *Counter: value AND pointer methods    {Describe, IncrementBroken, Increment}
	//
	// Demonstration with a tiny interface:
	type Incrementer interface{ Increment() }

	var i Incrementer
	i = &c // OK: *Counter has Increment in its method set
	i.Increment()
	fmt.Println("via interface:                ", c.Describe()) // clicks = 5

	// This would NOT compile, because Counter (the value) lacks Increment:
	//   i = c // error: Counter does not implement Incrementer
	//         //        (method Increment has pointer receiver)
	//
	// The auto-&(c) trick only works for direct calls, not for interface
	// assignment — the interface would have no address to take.
	_ = i
}
