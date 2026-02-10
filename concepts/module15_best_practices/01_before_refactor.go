// Module 15, example 1: the BEFORE half of a refactor pair.
//
// Run with: go run 01_before_refactor.go
//
// This program WORKS — it summarizes user records and prints a tiny report —
// but it ignores most Go guidelines on purpose. Read it, wince, and jot down
// what you'd change. Then open 02_after_refactor.go, which produces the same
// output idiomatically. Numbered SMELL comments mark each problem.
package main

import (
	"fmt"
	"strings"
)

// SMELL 1: package-level mutable state. Any function can poke these, nothing
// is testable in isolation, and concurrent use would be a data race.
var TheUsers []map[string]string
var Report string
var ErrorsSeen int

// SMELL 2: stringly-typed data. A user is a map[string]string, so typos in
// keys ("emial") compile fine and fail silently at runtime. No place to hang
// methods or documentation either.
func AddTheUser(n string, e string, a string) {
	u := map[string]string{}
	u["name"] = n
	u["email"] = e
	u["age"] = a // SMELL 3: an age stored as a string — conversions everywhere
	TheUsers = append(TheUsers, u)
}

// SMELL 4: one giant do-everything function. Validation, formatting,
// aggregation and I/O are welded together — you can't test the validation
// without capturing stdout, can't reuse the formatting anywhere else.
// SMELL 5 (naming): GetUserReportDataAndMakeString — verbose, "Get" prefix,
// and it doesn't even say what it returns (nothing! it writes a global).
func GetUserReportDataAndMakeString() {
	Report = "" // SMELL: accumulating output in a global via string +=
	var goodOnes int
	for i := 0; i < len(TheUsers); i++ {
		u := TheUsers[i]
		// SMELL 6: deep nesting instead of early returns/continues.
		// The happy path is buried four levels deep on the right.
		if u["name"] != "" {
			if strings.Contains(u["email"], "@") {
				if u["age"] != "" {
					// SMELL 7: string += in a loop — O(n²) copying.
					// (strings.Builder exists for exactly this.)
					Report = Report + "user: " + u["name"] + " <" + u["email"] + ">\n"
					goodOnes = goodOnes + 1
				} else {
					// SMELL 8: errors "handled" by printing and counting in a
					// global — the caller can never react to what went wrong.
					fmt.Println("bad user, no age!!", u)
					ErrorsSeen = ErrorsSeen + 1
				}
			} else {
				fmt.Println("bad user, email is wrong!!", u)
				ErrorsSeen = ErrorsSeen + 1
			}
		} else {
			fmt.Println("bad user, no name!!", u)
			ErrorsSeen = ErrorsSeen + 1
		}
	}
	Report = Report + fmt.Sprintf("valid users: %d, errors: %d\n", goodOnes, ErrorsSeen)
}

func main() {
	AddTheUser("Ada", "ada@example.com", "36")
	AddTheUser("", "nobody@example.com", "50")    // invalid: no name
	AddTheUser("Bob", "bob-at-example.com", "41") // invalid: bad email
	AddTheUser("Grace", "grace@navy.mil", "85")

	GetUserReportDataAndMakeString()

	// SMELL 9: main "just knows" that the function filled the Report global.
	// The data flow is invisible — you must read every function to trace it.
	fmt.Print(Report)
}
