// function_values.go — functions are first-class values: store them in
// variables, pass them as arguments (callbacks), return them, keep them
// in maps. No special syntax required.
//
// Run it with:   go run function_values.go
package main

import (
	"fmt"
	"slices"
	"strings"
)

// A NAMED FUNCTION TYPE. Anywhere this type appears you can pass any
// function with the matching signature. Naming the type documents intent.
type MathOp func(a, b int) int

// A function that TAKES a function — the classic callback shape.
func compute(a, b int, op MathOp) int {
	return op(a, b) // call the parameter like any function
}

// A function that RETURNS a function (a factory).
func multiplierOf(factor int) func(int) int {
	return func(n int) int { return n * factor }
}

// Ordinary named functions are also values of their signature's type:
func add(a, b int) int { return a + b }
func mul(a, b int) int { return a * b }

func main() {
	// ---- Storing functions in variables ------------------------------------
	// The type of `op` is func(int, int) int:
	var op MathOp = add // a named function used as a value — no parentheses!
	fmt.Println("op(2, 3) with add:", op(2, 3))

	op = mul // reassign to a different function of the same type
	fmt.Println("op(2, 3) with mul:", op(2, 3))

	// Anonymous function (function literal) assigned inline:
	sub := func(a, b int) int { return a - b }
	fmt.Println("anonymous sub(2, 3):", sub(2, 3))

	// ---- Passing functions: callbacks -----------------------------------------
	fmt.Println("compute with add:", compute(10, 4, add))
	fmt.Println("compute with sub:", compute(10, 4, sub))
	// Or define the callback right at the call site:
	fmt.Println("compute with max:", compute(10, 4, func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}))

	// ---- Returning functions -----------------------------------------------------
	double := multiplierOf(2)
	triple := multiplierOf(3)
	fmt.Println("double(7):", double(7), " triple(7):", triple(7))

	// ---- Functions in data structures ----------------------------------------------
	// A map from operator name to implementation — a tiny dispatch table.
	// This pattern replaces long switch statements in interpreters,
	// command routers, etc.
	ops := map[string]MathOp{
		"+": add,
		"*": mul,
		"-": sub,
	}
	for _, name := range []string{"+", "*", "-"} {
		fmt.Printf("6 %s 2 = %d\n", name, ops[name](6, 2))
	}

	// ---- Callbacks in the standard library --------------------------------------------
	// You've already used this style: slices.SortFunc takes a comparison
	// function. Sort names case-insensitively:
	names := []string{"charlie", "Alice", "bob"}
	slices.SortFunc(names, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	fmt.Println("case-insensitive sort:", names)

	// A home-made higher-order helper, map/filter style:
	evens := filter([]int{1, 2, 3, 4, 5, 6}, func(n int) bool { return n%2 == 0 })
	fmt.Println("filtered evens:", evens)
}

// filter keeps the elements for which keep(element) returns true.
// The predicate parameter makes one loop reusable for any condition.
func filter(nums []int, keep func(int) bool) []int {
	var out []int
	for _, n := range nums {
		if keep(n) {
			out = append(out, n)
		}
	}
	return out
}
