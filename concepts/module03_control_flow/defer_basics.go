// defer_basics.go — defer schedules a call to run when the surrounding
// FUNCTION returns. Two rules to burn in:
//  1. multiple defers run in LIFO (stack) order
//  2. deferred ARGUMENTS are evaluated immediately, at the defer line
//
// Run it with:   go run defer_basics.go
package main

import "fmt"

func main() {
	lifoOrder()
	fmt.Println()
	argumentsEvaluatedNow()
	fmt.Println()
	deferInLoopGotcha()
	fmt.Println()
	realisticCleanup()
}

// ---- Rule 1: LIFO order -----------------------------------------------
func lifoOrder() {
	fmt.Println("-- LIFO order --")
	defer fmt.Println("deferred FIRST  -> runs LAST")
	defer fmt.Println("deferred SECOND -> runs middle")
	defer fmt.Println("deferred THIRD  -> runs FIRST")
	fmt.Println("function body runs before any defer")
	// Think of defers as pushed onto a stack, popped at return:
	//   push A, push B, push C  ->  run C, B, A
	// Why LIFO? Cleanup should reverse setup: acquire A then B => release B then A.
}

// ---- Rule 2 (THE classic gotcha): arguments evaluate NOW ----------------
func argumentsEvaluatedNow() {
	fmt.Println("-- arguments are evaluated at the defer line --")

	i := 0
	// fmt.Println's argument `i` is evaluated RIGHT HERE, capturing 0.
	// Only the CALL is postponed.
	defer fmt.Println("deferred saw i =", i)

	i = 99
	fmt.Println("current i =", i)
	// Output order: "current i = 99" then "deferred saw i = 0"  <- not 99!

	// The fix, when you WANT the final value: defer a closure with no
	// arguments. The closure reads i when it RUNS (at return):
	defer func() {
		fmt.Println("closure defer sees final i =", i) // 99
	}()
	// (Closures are Module 05; this preview shows why they pair with defer.)
}

// ---- Gotcha: defer in a loop runs at FUNCTION exit, not per iteration ----
func deferInLoopGotcha() {
	fmt.Println("-- defer inside a loop --")
	for i := range 3 {
		fmt.Println("iteration", i)
		// This does NOT run at the end of each iteration! Each defer is
		// stacked up and all of them run when deferInLoopGotcha returns.
		// Real-world version of this bug: opening files in a loop with
		// `defer f.Close()` — no file closes until the function ends,
		// and you can run out of file descriptors.
		defer fmt.Println("  deferred from iteration", i)
	}
	fmt.Println("loop finished; now the function returns...")
	// Note the LIFO order in the output: iteration 2, 1, 0.
	// Fix in real code: move the loop body (open+defer+use) into its own
	// small function, so each iteration's defer runs when THAT returns.
}

// ---- What defer is actually FOR: guaranteed cleanup ----------------------
func realisticCleanup() {
	fmt.Println("-- realistic cleanup pattern --")
	// Pretend acquire/release pattern (stand-in for os.Open + f.Close,
	// mutex Lock/Unlock, db transactions, timers...):
	fmt.Println("acquire resource")
	defer fmt.Println("release resource (runs even on early return or panic)")

	// Imagine many exit paths below — early returns, error branches.
	// Every one of them releases the resource, because defer is attached
	// to the function, not to a code path.
	if len("abc") == 3 {
		fmt.Println("...doing work, then returning early")
		return // release still happens!
	}
	fmt.Println("unreached")
}
