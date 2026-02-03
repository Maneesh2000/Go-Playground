// labels_and_goto.go — escaping nested loops with labeled break/continue,
// plus a look at goto (which you should read about once and then avoid).
//
// Run it with:   go run labels_and_goto.go
package main

import "fmt"

func main() {
	// ---- The problem: break only exits the INNERMOST loop -----------------
	fmt.Println("-- plain break (only exits inner loop) --")
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if j == 1 {
				break // leaves the j-loop only; the i-loop keeps going
			}
			fmt.Println("i,j =", i, j)
		}
	}

	// ---- labeled break: exit BOTH loops at once -----------------------------
	// A label is a name followed by a colon, placed on the loop statement.
	fmt.Println("-- labeled break --")
search:
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			if i*j == 6 {
				fmt.Println("found i*j == 6 at", i, j)
				break search // jumps out of the loop LABELED search
			}
		}
	}
	fmt.Println("after the labeled break")

	// ---- labeled continue: next iteration of the OUTER loop -----------------
	// Useful for "skip this whole row" logic in grid scans.
	fmt.Println("-- labeled continue --")
rows:
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if j > i {
				continue rows // abandon this row, start the next i
			}
			fmt.Println("kept i,j =", i, j)
		}
	}

	// ---- break inside switch: label to the rescue ----------------------------
	// `break` inside a switch breaks the SWITCH, not the loop around it.
	// A label lets you break the loop from within a case.
	fmt.Println("-- break out of a loop from inside a switch --")
loop:
	for i := 0; ; i++ { // note: condition omitted = infinite
		switch {
		case i > 3:
			break loop // without the label, this would only exit the switch
		default:
			fmt.Println("i =", i)
		}
	}

	// ---- goto: legal, restricted, discouraged ----------------------------------
	// goto jumps to a label in the SAME function. The compiler forbids
	// jumping over variable declarations or into a block, which prevents
	// the worst spaghetti. Idiomatic Go replaces goto with loops, labeled
	// break/continue, early returns, and defer — you may see goto in
	// generated code or the runtime's hot paths, but don't reach for it.
	fmt.Println("-- goto (for completeness only) --")
	attempts := 0
retry:
	attempts++
	if attempts < 3 {
		fmt.Println("attempt", attempts, "-> pretending it failed, retrying")
		goto retry // jump back up — a plain for-loop would be clearer!
	}
	fmt.Println("succeeded on attempt", attempts)
}
