// maps_basics.go — Go's built-in hash table: create, read, comma-ok,
// delete, iterate (in random order!), and common patterns.
//
// Run it with:   go run maps_basics.go
package main

import (
	"fmt"
	"strings"
)

func main() {
	// ---- Creating maps ----------------------------------------------------
	// Literal:
	ages := map[string]int{
		"ada":   36,
		"linus": 28,
	}
	// make() for an empty, ready-to-use map:
	scores := make(map[string]int)

	// GOTCHA: a declared-but-uninitialized map is nil. Reading a nil map
	// is fine (zero values), but WRITING to one PANICS at runtime:
	var nilMap map[string]int
	fmt.Println("read from nil map:", nilMap["anything"]) // 0, no panic
	// nilMap["x"] = 1  // would panic: assignment to entry in nil map

	// ---- Insert / update / read ---------------------------------------------
	scores["alice"] = 90 // insert
	scores["alice"] = 95 // update (same syntax)
	scores["bob"] = 82
	fmt.Println("scores:", scores, " len:", len(scores))

	// Reading a MISSING key does not error — it returns the value type's
	// zero value. Convenient, but ambiguous:
	fmt.Println("carol's score (absent):", scores["carol"]) // 0... or is it?

	// ---- The comma-ok idiom ----------------------------------------------------
	// The two-value form tells you whether the key was actually present:
	if score, ok := scores["carol"]; ok {
		fmt.Println("carol:", score)
	} else {
		fmt.Println("carol is not in the map (ok was false)")
	}
	// This matters whenever the zero value is meaningful — a score of 0
	// and "no score recorded" are different facts!

	// ---- delete -------------------------------------------------------------------
	delete(scores, "bob")      // remove a key
	delete(scores, "not-here") // deleting a missing key is a harmless no-op
	fmt.Println("after delete:", scores)

	// ---- Iteration order is RANDOMIZED ----------------------------------------------
	// The runtime deliberately varies map iteration order between runs so
	// no one can depend on it. Run this program twice — the order of the
	// lines below will likely differ:
	inventory := map[string]int{"apples": 5, "pears": 2, "plums": 7, "figs": 1}
	fmt.Println("\nmap iteration (order varies per run!):")
	for item, count := range inventory {
		fmt.Printf("  %s: %d\n", item, count)
	}
	// Need deterministic order? Sort the keys first —
	// see slices_maps_stdlib.go for the one-liner.

	// ---- Maps are reference-like -------------------------------------------------------
	// Passing a map to a function passes a handle to the SAME table:
	addPerson(ages)
	fmt.Println("\nages after addPerson:", ages, "<- callee's write is visible")

	// ---- A classic map pattern: counting ------------------------------------------------
	sentence := "the quick fox jumps over the lazy dog the fox"
	freq := make(map[string]int)
	for _, word := range strings.Fields(sentence) { // Fields splits on spaces
		freq[word]++ // missing key reads as 0, then we store 1 — no setup needed
	}
	fmt.Println("\nword frequencies:", freq)

	// ---- Map as a set --------------------------------------------------------------------
	// Go has no set type; map[T]bool (or map[T]struct{} to save bytes) is the idiom.
	seen := map[string]bool{}
	for _, w := range strings.Fields(sentence) {
		seen[w] = true
	}
	fmt.Println("unique words:", len(seen), "| contains 'fox'?", seen["fox"])
}

// Maps passed to functions share the underlying table with the caller.
func addPerson(m map[string]int) {
	m["grace"] = 47
}
