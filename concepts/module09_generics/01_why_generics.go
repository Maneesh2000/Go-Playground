// Module 09, Example 1 — Why generics: the problem, the old workarounds,
// and the generic solution. Plus constraint syntax including ~unions.
//
// Run with: go run 01_why_generics.go
package main

import "fmt"

// =============================================================================
// THE PROBLEM: "same logic, different types"
// =============================================================================

// ---- Old workaround #1: copy-paste ------------------------------------------
// Identical bodies. Every bug fixed twice. Every new numeric type = another copy.

func sumInts(xs []int) int {
	var total int
	for _, x := range xs {
		total += x
	}
	return total
}

func sumFloats(xs []float64) float64 {
	var total float64
	for _, x := range xs {
		total += x
	}
	return total
}

// ---- Old workaround #2: interface{} (a.k.a. any) -------------------------------
// One function... but the type system checked out. The caller can pass
// []any of anything, we need runtime type switches, mistakes surface as
// runtime failures instead of compile errors, and the result must be
// asserted back. Slow AND unsafe.

func sumAny(xs []any) (float64, error) {
	var total float64
	for i, x := range xs {
		switch v := x.(type) {
		case int:
			total += float64(v)
		case float64:
			total += v
		default:
			return 0, fmt.Errorf("element %d: unsupported type %T", i, x)
		}
	}
	return total, nil
}

// =============================================================================
// THE GENERIC SOLUTION
// =============================================================================

// A CONSTRAINT is an interface used as a type filter. This one is a UNION:
// it admits any type in the list. The tilde is important:
//
//	~int   means "int, or ANY named type whose underlying type is int"
//	       (e.g. type Celsius int)
//	 int   without ~ would mean int and NOTHING else — Celsius rejected.
//
// Constraints with unions unlock OPERATORS (+, <, ...) on T, which plain
// method-based interfaces never could.
type Number interface {
	~int | ~int64 | ~float64
}

// One implementation. [T Number] declares a type parameter T that must
// satisfy Number. The compiler generates/checks the right code per type —
// misuse is a COMPILE error, not a runtime one.
func sum[T Number](xs []T) T {
	var total T // zero value of whatever T is
	for _, x := range xs {
		total += x // legal because every type in Number supports +
	}
	return total
}

// A named type with underlying type int — the reason ~ exists:
type Celsius int

func main() {
	ints := []int{1, 2, 3, 4}
	floats := []float64{1.5, 2.5, 3.0}

	// ---- the old ways -------------------------------------------------------
	fmt.Println("copy-paste  :", sumInts(ints), sumFloats(floats))

	mixed := []any{1, 2.5, 3}
	got, err := sumAny(mixed)
	fmt.Println("interface{} :", got, err)

	// interface{} failure happens at RUNTIME:
	_, err = sumAny([]any{1, "oops"})
	fmt.Println("interface{} with bad input:", err)

	// ---- the generic way -------------------------------------------------------
	// Type INFERENCE: the compiler sees []int and figures out T=int.
	// We write sum(ints), not sum[int](ints).
	fmt.Println("generic     :", sum(ints), sum(floats))

	// Explicit type arguments are allowed but rarely needed:
	fmt.Println("explicit    :", sum[int64]([]int64{10, 20, 30}))

	// The ~ payoff — a named type with int underneath just works:
	temps := []Celsius{20, 21, 19}
	fmt.Println("~ union     :", sum(temps), "(sum of []Celsius)")

	// And misuse is caught at COMPILE time, not runtime:
	//   sum([]string{"a", "b"})
	//   // error: string does not satisfy Number (~int | ~int64 | ~float64)

	// ---- when NOT to use generics ------------------------------------------------
	// If your function only CALLS METHODS on its input, an interface is
	// simpler and just as safe:
	//
	//   func Print(s fmt.Stringer)        // good: behaviour abstraction
	//   func Print[T fmt.Stringer](s T)   // needless ceremony, same effect
	//
	// Generics earn their keep for CONTAINERS (Stack[T], next examples) and
	// ALGORITHMS over operators (+, <) that interfaces can't express.
}
