// Module 09, Example 3 — A generic TYPE: Stack[T], a type-safe LIFO
// container.
//
// Before generics, a reusable stack stored interface{} — so a "stack of
// ints" would happily accept a string, and every Pop needed a type
// assertion. Stack[T] fixes both: the compiler enforces the element type.
//
// Run with: go run 03_generic_stack.go
package main

import "fmt"

// Stack is a generic type: the type parameter appears on the TYPE.
// `T any` = elements can be anything; we never compare or add them,
// so the loosest constraint is correct.
type Stack[T any] struct {
	items []T
}

// Methods on a generic type repeat the parameter in the receiver:
// (s *Stack[T]). Note: methods may only use the type's own parameters —
// a method cannot introduce a new one (e.g. no `func (s *Stack[T]) Weird[U any](...)`).

// Push adds an item on top. Pointer receiver: it mutates the slice.
func (s *Stack[T]) Push(v T) {
	s.items = append(s.items, v)
}

// Pop removes and returns the top item. The comma-ok result handles the
// empty case WITHOUT panicking and without a magic sentinel value —
// idiomatic Go for "might not be there".
func (s *Stack[T]) Pop() (T, bool) {
	if len(s.items) == 0 {
		var zero T // the zero value of T, whatever T is
		return zero, false
	}
	top := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return top, true
}

// Peek returns the top item without removing it.
func (s *Stack[T]) Peek() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

func (s *Stack[T]) Len() int { return len(s.items) }

type task struct {
	id   int
	name string
}

func main() {
	// ---- A stack of ints -------------------------------------------------------
	// Instantiating a generic type literal needs explicit [int]: an empty
	// literal gives inference nothing to work with.
	var ints Stack[int]
	ints.Push(10)
	ints.Push(20)
	ints.Push(30)

	fmt.Println("len:", ints.Len())
	if top, ok := ints.Peek(); ok {
		fmt.Println("peek:", top)
	}

	// LIFO: last in, first out.
	for {
		v, ok := ints.Pop()
		if !ok {
			break // stack empty
		}
		fmt.Println("pop:", v)
	}

	// Popping empty: no panic, just ok=false and the zero value.
	v, ok := ints.Pop()
	fmt.Printf("pop on empty: value=%d ok=%v\n\n", v, ok)

	// ---- The SAME code, now holding strings --------------------------------------
	var undo Stack[string]
	undo.Push("typed 'hello'")
	undo.Push("deleted a line")
	undo.Push("pasted a block")

	fmt.Println("undo history (most recent first):")
	for {
		action, ok := undo.Pop()
		if !ok {
			break
		}
		fmt.Println("  undo:", action)
	}

	// ---- And structs — anything works ----------------------------------------------
	var tasks Stack[task]
	tasks.Push(task{1, "write module"})
	tasks.Push(task{2, "review module"})
	if t, ok := tasks.Pop(); ok {
		fmt.Printf("\npopped struct: %+v\n", t)
	}

	// ---- The whole point: COMPILE-TIME safety --------------------------------------
	// This line will not build — a Stack[int] only takes ints:
	//   ints.Push("surprise!")
	//   // error: cannot use "surprise!" (untyped string constant) as int value
	//
	// With the old interface{} stack this bug would compile fine and only
	// explode later, at the type assertion after Pop. Generics move the
	// failure to the earliest, cheapest place: the compiler.
}
