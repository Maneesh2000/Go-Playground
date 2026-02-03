// variadic.go — functions that take any number of arguments (...T),
// and spreading a slice into one with s...
//
// Run it with:   go run variadic.go
package main

import (
	"fmt"
	"strings"
)

// The ... before the type makes the FINAL parameter variadic.
// Inside the function, nums is an ordinary []int.
func sum(nums ...int) int {
	fmt.Printf("  (sum received %d args as %T)\n", len(nums), nums)
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

// Only the LAST parameter may be variadic; regular parameters come first.
func join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

// A variadic of type ...any accepts anything — this is how fmt.Println is
// declared: func Println(a ...any) (n int, err error)
func describeAll(values ...any) {
	for _, v := range values {
		fmt.Printf("  %v is a %T\n", v, v)
	}
}

func main() {
	// ---- Calling with individual arguments -------------------------------
	fmt.Println("sum(1, 2, 3):")
	fmt.Println("  =", sum(1, 2, 3))

	// Zero arguments is legal: nums is an empty (non-nil) slice.
	fmt.Println("sum():")
	fmt.Println("  =", sum())

	// ---- Spreading a slice with ... ----------------------------------------
	// If your values are already in a slice, s... passes the slice
	// directly as the variadic parameter (no copying, no loop needed):
	scores := []int{90, 85, 77}
	fmt.Println("sum(scores...):")
	fmt.Println("  =", sum(scores...))

	// You CANNOT mix the two forms: sum(1, scores...) doesn't compile.
	// Either list values, or spread exactly one slice.

	// GOTCHA: because the callee receives the SAME slice, it could mutate
	// your data. Most functions don't, but it's shared memory (Module 04!).

	// ---- Regular + variadic parameters ----------------------------------------
	fmt.Println(`join("-", "a", "b", "c") =`, join("-", "a", "b", "c"))
	words := []string{"go", "is", "fun"}
	fmt.Println(`join(" ", words...)      =`, join(" ", words...))

	// ---- ...any --------------------------------------------------------------
	fmt.Println("describeAll on mixed types:")
	describeAll(42, "hello", 3.14, true)

	// ---- append is the variadic you already use --------------------------------
	// append(s, elems ...T) — so both of these work:
	base := []int{1, 2}
	base = append(base, 3, 4)           // individual elements
	base = append(base, []int{5, 6}...) // spread another slice = concat idiom
	fmt.Println("appended:", base)
}
