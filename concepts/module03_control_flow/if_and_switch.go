// if_and_switch.go — Go's if (with init statement) and the three faces of
// switch: expression switch, multi-value cases, and the type-less switch.
//
// Run it with:   go run if_and_switch.go
package main

import (
	"fmt"
	"strconv"
)

func main() {
	// ---- Basic if/else --------------------------------------------------
	// No parentheses around the condition; braces are REQUIRED.
	x := 7
	if x > 10 {
		fmt.Println(x, "is big")
	} else if x > 5 {
		fmt.Println(x, "is medium")
	} else {
		fmt.Println(x, "is small")
	}

	// The condition must be a real bool — `if x { }` where x is an int
	// does not compile. Go has no truthy/falsy values.

	// ---- if with an init statement --------------------------------------
	// Form: if <statement>; <condition> { }
	// Variables declared in the statement exist ONLY inside the if/else.
	// This is THE idiomatic pattern for calls that return (value, error):
	for _, input := range []string{"42", "banana"} {
		if n, err := strconv.Atoi(input); err == nil {
			fmt.Printf("%q parsed to %d\n", input, n)
		} else {
			fmt.Printf("%q is not a number (%v)\n", input, err)
		}
		// n and err do NOT exist here — no scope pollution.
	}

	// ---- Expression switch ------------------------------------------------
	day := "sat"
	switch day {
	case "sat", "sun": // a case can match several values
		fmt.Println(day, "-> weekend")
	case "fri":
		fmt.Println(day, "-> almost there")
	default:
		fmt.Println(day, "-> weekday")
	}
	// NOTE: no `break` needed! Go cases do not fall through by default —
	// the #1 C/Java switch bug simply cannot happen here.

	// switch also supports an init statement, just like if:
	switch hour := 15; { // note: init statement, then NO expression (see below)
	case hour < 12:
		fmt.Println("good morning")
	case hour < 18:
		fmt.Println("good afternoon")
	default:
		fmt.Println("good evening")
	}

	// ---- Type-less switch (switch true) ------------------------------------
	// Omitting the expression means "switch true": each case is a boolean
	// condition, evaluated top to bottom. Cleaner than a long else-if chain.
	score := 83
	var grade string
	switch {
	case score >= 90:
		grade = "A"
	case score >= 80:
		grade = "B" // first true case wins; the rest are skipped
	case score >= 70:
		grade = "C"
	default:
		grade = "F"
	}
	fmt.Println("score", score, "-> grade", grade)

	// ---- fallthrough: explicit, rare, and slightly dangerous ----------------
	// fallthrough transfers control to the NEXT case's body
	// UNCONDITIONALLY — the next case's condition is NOT checked!
	switch n := 1; n {
	case 1:
		fmt.Println("case 1 ran")
		fallthrough // deliberately continue into case 2's body...
	case 2:
		fmt.Println("case 2 ran too (even though n != 2!)")
	case 3:
		fmt.Println("case 3 (not reached: no fallthrough above)")
	}
}
