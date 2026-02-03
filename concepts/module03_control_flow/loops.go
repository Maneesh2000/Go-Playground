// loops.go — for is Go's ONLY loop keyword. Here are all its forms,
// including range over an integer (added in Go 1.22).
//
// Run it with:   go run loops.go
package main

import "fmt"

func main() {
	// ---- Form 1: the classic three-part loop ----------------------------
	// for <init>; <condition>; <post> { }
	// No parentheses, braces required.
	fmt.Print("three-part: ")
	for i := 0; i < 5; i++ {
		fmt.Print(i, " ")
	}
	fmt.Println() // i is scoped to the loop; it doesn't exist here

	// ---- Form 2: condition only — Go's "while" ---------------------------
	// Drop init and post and you have a while loop. There is no `while`
	// keyword in Go; this is it.
	fmt.Print("while-style: ")
	n := 100
	for n > 1 {
		fmt.Print(n, " ")
		n /= 2 // halve until we reach 1
	}
	fmt.Println()

	// ---- Form 3: infinite loop --------------------------------------------
	// `for { }` loops forever; you leave with break (or return).
	fmt.Print("infinite+break: ")
	count := 0
	for {
		count++
		if count == 3 {
			break // the only exit
		}
	}
	fmt.Println("broke out at count =", count)

	// continue skips to the next iteration:
	fmt.Print("odd numbers: ")
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			continue // even? skip the rest of the body
		}
		fmt.Print(i, " ")
	}
	fmt.Println()

	// ---- Form 4: range over collections ------------------------------------
	// range yields (index, element) for slices and arrays:
	fruits := []string{"apple", "banana", "cherry"}
	for i, fruit := range fruits {
		fmt.Printf("  fruits[%d] = %s\n", i, fruit)
	}

	// Only need the index? Drop the second variable.
	// Only need the value? Discard the index with _ :
	total := 0
	for _, v := range []int{10, 20, 30} {
		total += v
	}
	fmt.Println("sum via range:", total)

	// range over a string decodes UTF-8: you get (byte index, rune).
	// Note the indexes jump 0 -> 2: 'é' is TWO bytes. (More in Module 04.)
	for i, r := range "héllo" {
		fmt.Printf("  byte %d: %c\n", i, r)
	}

	// ---- Form 5: range over an int (Go 1.22+) --------------------------------
	// `range n` counts 0, 1, ..., n-1. Sugar for the three-part counter loop.
	fmt.Print("range over int: ")
	for i := range 5 {
		fmt.Print(i, " ")
	}
	fmt.Println()

	// Don't need the counter at all? Just repeat:
	for range 3 {
		fmt.Println("  knock")
	}

	// ---- Go 1.22 semantics note ----------------------------------------------
	// Since Go 1.22, every iteration gets a FRESH copy of the loop
	// variable. This mostly matters when closures capture it — the classic
	// gotcha and its history are demonstrated in Module 05 (closures.go).
}
