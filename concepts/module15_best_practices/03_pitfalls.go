// Module 15, example 3: the classic Go pitfalls, demonstrated SAFELY —
// each trap is shown next to its fix, with output proving the difference.
//
// Run with: go run 03_pitfalls.go
package main

import (
	"fmt"
	"time"
)

func main() {
	pitfallNilMap()
	pitfallSliceAliasing()
	pitfallGoroutineLeak()
	pitfallShadowing()
	pitfallTimeFormat()
	pitfallInterfaceNil()
}

// ---------------------------------------------------------------------------
// PITFALL 1: writing to a nil map panics. (Reading is fine — returns zero.)
// Asymmetry to memorize: nil SLICES are usable with append; nil MAPS are not.
// ---------------------------------------------------------------------------
func pitfallNilMap() {
	fmt.Println("--- 1) nil map ---")

	var m map[string]int                                       // nil — no storage allocated
	fmt.Println("reading from nil map is fine:", m["missing"]) // 0
	// m["boom"] = 1      // ← uncomment: panic: assignment to entry in nil map

	m = make(map[string]int) // the fix: make (or a literal map[string]int{})
	m["ok"] = 1
	fmt.Println("after make, writes work:", m)

	var s []int      // nil slice…
	s = append(s, 1) // …but append handles nil fine — allocates for you
	fmt.Println("nil slice + append is fine:", s)
}

// ---------------------------------------------------------------------------
// PITFALL 2: slice aliasing. Sub-slices SHARE the backing array. Writes
// through one alias are visible through the other — and append may or may
// not break the sharing depending on spare capacity. Spooky action at a
// distance until you internalize the model.
// ---------------------------------------------------------------------------
func pitfallSliceAliasing() {
	fmt.Println("--- 2) slice aliasing ---")

	base := []int{1, 2, 3, 4, 5}
	window := base[1:3] // aliases elements 2,3 — SAME memory as base

	window[0] = 99                                             // writing through the alias...
	fmt.Println("write via sub-slice changed base too:", base) // [1 99 3 4 5]

	// append is the subtle one: window has spare capacity (cap extends to
	// the end of base), so append OVERWRITES base[3] instead of copying!
	window = append(window, 777)
	fmt.Println("append clobbered base[3]:           ", base) // [1 99 3 777 5]

	// Fix A: explicit copy when you need independence.
	independent := make([]int, 2)
	copy(independent, base[1:3])
	independent[0] = -1
	fmt.Println("copy() detaches — base untouched:   ", base)

	// Fix B: full slice expression caps capacity, forcing append to
	// reallocate instead of stomping the neighbor: base[1:3:3].
	capped := base[1:3:3]
	capped = append(capped, 42) // must reallocate — cap was exhausted
	fmt.Println("three-index slice keeps base safe:  ", base, "(capped:", capped, ")")
}

// ---------------------------------------------------------------------------
// PITFALL 3: goroutine leaks. A goroutine blocked on a channel nobody will
// ever touch stays alive FOREVER (with its whole stack and captures).
// Leaks don't crash — they accumulate until memory or fd exhaustion.
// ---------------------------------------------------------------------------
func pitfallGoroutineLeak() {
	fmt.Println("--- 3) goroutine leak ---")

	// THE LEAK (shape only — we don't actually run the blocking version):
	//
	//   ch := make(chan int)        // unbuffered
	//   go func() { ch <- expensive() }()
	//   if tooSlow { return }       // caller gives up WITHOUT receiving...
	//                               // → sender blocks on `ch <-` forever. Leaked.

	// Fix A: buffer of 1 — the send always succeeds; if nobody reads the
	// result, the goroutine still gets to exit and everything is GC'd.
	ch := make(chan int, 1)
	go func() { ch <- 42 }()
	fmt.Println("buffered send lets the goroutine exit; result:", <-ch)

	// Fix B: select on a done/ctx channel so the sender has an escape hatch.
	done := make(chan struct{})
	result := make(chan int) // unbuffered on purpose
	go func() {
		select {
		case result <- 42:
		case <-done: // caller walked away — exit instead of blocking forever
			fmt.Println("worker: caller gone, exiting cleanly (no leak)")
		}
	}()
	close(done) // simulate the caller abandoning the wait
	time.Sleep(20 * time.Millisecond)

	// Rule: NEVER start a goroutine without knowing exactly how it stops.
}

// ---------------------------------------------------------------------------
// PITFALL 4: shadowing. `:=` inside a block declares a NEW variable that
// hides the outer one. The classic victim is `err`; the sneakiest victim is
// a value you compute in a branch and lose at the closing brace.
// ---------------------------------------------------------------------------
func pitfallShadowing() {
	fmt.Println("--- 4) shadowing ---")

	conn := "not connected"
	if true {
		// BUG: `:=` creates a fresh local `conn` — the outer stays stale.
		conn := "connected!"
		_ = conn
	}
	fmt.Println("after if-block with := :", conn) // still "not connected"!

	// Fix: declare outside, assign (=, not :=) inside.
	if true {
		conn = "connected!"
	}
	fmt.Println("after plain assignment  :", conn)

	// The err flavor — `f, err := ...` inside a block shadows an outer err,
	// so a later `if err != nil` outside checks the WRONG (stale) err.
	// `go vet` and linters flag some cases; tight scopes prevent the rest.
}

// ---------------------------------------------------------------------------
// PITFALL 5: time.Format doesn't use YYYY-MM-DD. Go layouts are written as
// how the REFERENCE date (Mon Jan 2 15:04:05 MST 2006) would look.
// ---------------------------------------------------------------------------
func pitfallTimeFormat() {
	fmt.Println("--- 5) time.Format reference date ---")

	t := time.Date(2026, time.July, 4, 9, 30, 0, 0, time.UTC)

	// BUG: placeholder letters are printed LITERALLY (with bits of the
	// reference date leaking in where they happen to match).
	fmt.Println("Format(\"YYYY-MM-DD\") :", t.Format("YYYY-MM-DD")) // garbage

	// Fix: spell the layout with the reference date's numbers.
	fmt.Println("Format(\"2006-01-02\") :", t.Format("2006-01-02"))
	fmt.Println("with time            :", t.Format("2006-01-02 15:04:05"))
}

// ---------------------------------------------------------------------------
// PITFALL 6: the interface-nil bug. An interface value is (type, value).
// It is == nil ONLY when BOTH are nil. Put a nil *pointer* into an interface
// and you get (type=*MyError, value=nil) — which is NOT nil.
// ---------------------------------------------------------------------------
type myError struct{ msg string }

func (e *myError) Error() string { return e.msg }

// BUGGY: declares the concrete pointer type, returns it as `error`.
func buggyOp(fail bool) error {
	var e *myError // nil pointer
	if fail {
		e = &myError{"it failed"}
	}
	return e // even when e is nil, the interface now carries (type=*myError, value=nil)
}

// FIXED: return literal nil on success; only mention the concrete type
// on the failure path.
func fixedOp(fail bool) error {
	if fail {
		return &myError{"it failed"}
	}
	return nil // a truly nil interface: (type=nil, value=nil)
}

func pitfallInterfaceNil() {
	fmt.Println("--- 6) interface-nil bug ---")

	err := buggyOp(false) // the operation SUCCEEDED...
	fmt.Println("buggy: err == nil ?", err == nil, " (looks failed — the classic trap!)")

	err = fixedOp(false)
	fmt.Println("fixed: err == nil ?", err == nil)

	// Moral: functions returning `error` must return literal nil for
	// success — never a possibly-nil concrete pointer variable.
}
