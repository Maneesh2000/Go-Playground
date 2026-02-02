// variables_and_zero_values.go — every way to declare a variable in Go,
// and proof that nothing is ever "uninitialized".
//
// Run it with:   go run variables_and_zero_values.go
package main

import "fmt"

// Package-level variables must use `var` (the := short form is only legal
// inside functions). They are initialized before main runs.
var appName = "zero-values demo" // type inferred: string

// You can declare several at once in a block.
var (
	answer  int  = 42  // full form: name, type, value
	ratio        = 1.5 // inferred: float64 (float literals default to float64)
	enabled bool       // no value → zero value: false
)

func main() {
	fmt.Println("==", appName, "==")

	// ---- The three declaration styles, inside a function -------------

	var a int = 10 // 1) fully explicit
	var b = 20     // 2) type inferred from the value
	c := 30        // 3) short declaration — most common in real code
	fmt.Println("a, b, c =", a, b, c)

	// := can declare several variables at once...
	x, y := 1, 2
	fmt.Println("x, y =", x, y)

	// ...and it's legal to reuse := if AT LEAST ONE name on the left is
	// new. Here y is reassigned while z is newly declared.
	z, y := 99, 200
	fmt.Println("z, y =", z, y)

	// Unused local variables are a COMPILE ERROR. To throw a value away
	// on purpose, assign it to the blank identifier _ :
	_ = enabled // "yes compiler, I know, ignore this one"

	// ---- Zero values --------------------------------------------------
	// Declaring without initializing is safe: you get the zero value.
	var (
		zi int     // 0
		zf float64 // 0
		zb bool    // false
		zs string  // "" (empty string, NOT nil)
		zp *int    // nil (pointer with nothing to point at)
	)
	// %q quotes the string so you can SEE that it's empty, not missing.
	fmt.Printf("zero int=%d  float=%g  bool=%t  string=%q  pointer=%v\n",
		zi, zf, zb, zs, zp)

	// ---- Assignment vs declaration ------------------------------------
	// = assigns to an EXISTING variable; := creates a new one.
	count := 1 // declaration
	count = 2  // assignment (using := here would be a compile error)
	fmt.Println("count =", count)

	// Multiple assignment swaps values without a temp variable:
	x, y = y, x
	fmt.Println("after swap: x, y =", x, y)

	// ---- Shadowing gotcha ---------------------------------------------
	// A := inside a new { block } creates a NEW variable that hides
	// ("shadows") the outer one. The outer variable is untouched.
	n := 1
	{
		n := 100 // this is a DIFFERENT n, alive only in this block
		fmt.Println("inner n =", n)
	}
	fmt.Println("outer n =", n) // still 1 — shadowing bit many gophers!

	_ = answer
	_ = ratio
}
