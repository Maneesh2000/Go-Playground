// closures.go — closures capture variables (by reference, not by value!),
// the counter pattern, loop-variable capture and Go 1.22's fix, and the
// defer+closure combination.
//
// Run it with:   go run closures.go
package main

import "fmt"

// ---- The counter: a closure with private state ---------------------------
// count is a LOCAL variable of makeCounter, but the returned function
// references it — so the Go compiler moves it to the heap and it lives on
// after makeCounter returns. Only the closure can reach it: encapsulation
// without a struct.
func makeCounter() func() int {
	count := 0
	return func() int {
		count++ // captures count BY REFERENCE — updates the original
		return count
	}
}

func main() {
	// ---- Counters: independent state per closure --------------------------
	next := makeCounter()
	fmt.Println("next():", next(), next(), next()) // 1 2 3 — state persists!

	other := makeCounter()                                     // a SECOND call = a brand-new count variable
	fmt.Println("other():", other(), " next() again:", next()) // 1 and 4

	// ---- Capture is by reference, not snapshot -------------------------------
	x := 10
	show := func() { fmt.Println("closure sees x =", x) }
	show() // 10
	x = 20 // change AFTER creating the closure...
	show() // ...and the closure sees 20. It holds x itself, not a copy.

	// The closure can also WRITE the captured variable:
	bump := func() { x += 5 }
	bump()
	fmt.Println("after bump, main's x =", x) // 25 — shared both ways

	// ---- Loop-variable capture: the historic gotcha ----------------------------
	// Build three closures inside a loop, run them after the loop ends:
	var fns []func()
	for i := range 3 {
		fns = append(fns, func() { fmt.Print(i, " ") })
	}
	fmt.Print("loop closures print: ")
	for _, f := range fns {
		f()
	}
	fmt.Println()
	// Go 1.22+ : prints 0 1 2 — each iteration has its OWN i.
	// Go <=1.21: printed 3 3 3 (!) — ONE i was shared by all iterations,
	//            and by run time it had reached its final value.
	// The old workaround was a shadow copy inside the loop:  i := i
	// You'll still meet that line in older codebases; it's now redundant.

	// BUT: the 1.22 change is per-ITERATION. A variable declared OUTSIDE
	// the loop is still one shared variable — closures see its final value:
	var fns2 []func()
	j := 0 // declared outside...
	for ; j < 3; j++ {
		fns2 = append(fns2, func() { fmt.Print(j, " ") })
	}
	fmt.Print("shared-variable closures print: ")
	for _, f := range fns2 {
		f()
	}
	fmt.Println("<- all 3: one shared j (this is the counter pattern, on purpose)")

	// ---- defer + closures ----------------------------------------------------------
	deferAndClosures()
}

func deferAndClosures() {
	fmt.Println("\n-- defer + closures --")
	i := 0

	// Deferred call with an ARGUMENT: i is evaluated NOW (Module 03):
	defer fmt.Println("defer with argument saw i =", i) // will print 0

	// Deferred CLOSURE with no arguments: reads i when it RUNS, at return:
	defer func() {
		fmt.Println("deferred closure sees final i =", i) // will print 99
	}()

	i = 99
	fmt.Println("function body set i to", i)
	// Output order (defers are LIFO): closure first, then the argument one.
}
