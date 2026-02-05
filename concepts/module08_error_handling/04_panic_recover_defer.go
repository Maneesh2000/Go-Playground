// Module 08, Example 4 — defer for cleanup, panic for bugs, recover at
// boundaries. And when NOT to use panic/recover.
//
// Run with: go run 04_panic_recover_defer.go
package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	// ---- defer basics ---------------------------------------------------------
	// defer schedules a call to run when the surrounding FUNCTION returns —
	// on every exit path: normal return, early error return, even a panic.
	fmt.Println("== defer basics ==")
	deferDemo()

	// ---- defer for real cleanup -------------------------------------------------
	fmt.Println("\n== defer for file cleanup ==")
	if err := writeTempFile(); err != nil {
		fmt.Println("error:", err)
	}

	// ---- panic: reserved for bugs -------------------------------------------------
	// panic unwinds the stack, running deferred calls, then crashes the
	// program with a stack trace. It is for PROGRAMMER ERRORS — impossible
	// states, broken invariants — not for expected failures.
	//
	//   WRONG: panic("file not found")        <- expected failure: return error
	//   OK:    panic("negative length after validation") <- can't-happen bug
	//
	// The runtime itself panics on bugs: nil dereference, index out of
	// range, division by zero.

	// ---- recover: stopping a panic at a boundary -----------------------------------
	// recover() only does something when called INSIDE a deferred function
	// during a panic. It returns the panic value and resumes normal flow.
	// Legit use: a server keeping one bad request from killing the process.
	fmt.Println("\n== recover at a boundary ==")

	jobs := []int{3, 0, 5} // job "0" will trigger a divide-by-zero panic
	for _, j := range jobs {
		runJobSafely(j)
	}
	fmt.Println("main is still alive after a job panicked — that's the point")

	// ---- converting a panic into an error (library boundary pattern) -----------------
	fmt.Println("\n== panic converted to error ==")
	if err := safeDivide(10, 0); err != nil {
		fmt.Println("got a normal error back:", err)
	}

	// Final warnings:
	//  * Do NOT use panic/recover as try/catch control flow. Errors-as-values
	//    is the Go way; recover is a last-resort safety net.
	//  * A library should never let panics escape its public API.
	//  * recover can't catch everything (e.g. it cannot stop a deadlock or
	//    a panic in another goroutine — each goroutine needs its own).
}

func deferDemo() {
	fmt.Println("step 1")

	// Deferred calls run LIFO: last deferred, first executed.
	defer fmt.Println("deferred A (deferred first, runs LAST)")
	defer fmt.Println("deferred B (deferred last, runs FIRST)")

	// Arguments are evaluated NOW, at defer time — not when the call runs:
	x := 1
	defer fmt.Println("deferred C sees x =", x) // captures x = 1
	x = 99

	fmt.Println("step 2 (x is now", x, ")")
	// function returns here -> C, then B, then A
}

// writeTempFile shows the canonical pattern: acquire, check error,
// IMMEDIATELY defer the release. Whatever happens below — early returns,
// panics — the file gets closed and removed.
func writeTempFile() error {
	f, err := os.CreateTemp("", "module08-*.txt")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	// Two cleanups, deferred right after acquisition. LIFO order means
	// Close runs before Remove — exactly what we need.
	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := f.WriteString("hello from defer-land\n"); err != nil {
		return fmt.Errorf("write: %w", err) // cleanup STILL happens
	}

	fmt.Println("wrote and will auto-clean:", f.Name())
	return nil
}

// runJobSafely is the "boundary" pattern: one panicking job must not take
// down the whole worker loop.
func runJobSafely(divisor int) {
	// The deferred closure runs even if doRiskyJob panics; recover()
	// captures the panic value and cancels the crash.
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("  recovered from panic: %v\n", r)
		}
	}()

	doRiskyJob(divisor)
}

func doRiskyJob(divisor int) {
	// Integer division by zero panics at runtime — simulating a latent bug.
	fmt.Printf("  job: 100/%d = %d\n", divisor, 100/divisor)
}

// safeDivide shows the one respectable recover pattern in libraries:
// translate an internal panic into an ordinary error at the API boundary,
// so callers never see panics.
func safeDivide(a, b int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Named return value lets the deferred func SET the error.
			err = fmt.Errorf("safeDivide: recovered: %v", r)
		}
	}()

	fmt.Println("result:", a/b) // panics if b == 0
	return nil
}

// Keep errors package imported for exercise experimentation.
var _ = errors.New
