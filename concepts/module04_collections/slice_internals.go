// slice_internals.go — pointer/len/cap, append growth, the aliasing gotcha,
// copy, and the full slice expression s[a:b:c].
//
// Run it with:   go run slice_internals.go
package main

import "fmt"

// Small helper: print a slice with its len and cap so we can watch them.
func show(name string, s []int) {
	fmt.Printf("%8s = %-18v len=%d cap=%d\n", name, s, len(s), cap(s))
}

func main() {
	// A slice header is three words:
	//     +---------+
	//     | ptr     |──> first element it can see in the underlying array
	//     | len     |    elements you may index: s[0] .. s[len-1]
	//     | cap     |    room from ptr to the END of the underlying array
	//     +---------+

	// ---- Watching append grow a slice ------------------------------------
	fmt.Println("-- append growth --")
	var s []int // nil slice: len=0 cap=0
	for i := range 9 {
		s = append(s, i)
		// Whenever len exceeds cap, append allocates a LARGER array
		// (small slices roughly double: 0→1→2→4→8→16...) and copies.
		show(fmt.Sprintf("after %d", i), s)
	}

	// ---- Slicing shares memory ---------------------------------------------
	fmt.Println("\n-- slicing = new header, same array --")
	base := []int{0, 1, 2, 3, 4, 5}
	mid := base[1:4] // elements 1,2,3 — len=3, cap=5 (to array's end)
	show("base", base)
	show("mid", mid)

	mid[0] = 111 // writes base[1]! Same memory.
	fmt.Println("after mid[0] = 111:")
	show("base", base)

	// ---- Gotcha 1: append WITHOUT reallocation clobbers the neighbor ---------
	fmt.Println("\n-- aliasing gotcha: in-place append --")
	// mid has len=3 cap=5 → append has spare room, so it writes into the
	// SHARED array... right on top of base[4]:
	fmt.Println("before: base[4] =", base[4])
	mid = append(mid, 222)
	fmt.Println("after mid = append(mid, 222):")
	show("base", base) // base[4] silently became 222!
	show("mid", mid)

	// ---- Gotcha 2: append WITH reallocation silently stops sharing ------------
	fmt.Println("\n-- aliasing gotcha: reallocating append --")
	mid = append(mid, 7, 8, 9) // exceeds cap → new bigger array, data copied
	mid[0] = -1                // now writes only mid's PRIVATE array
	show("base", base)         // unaffected this time
	show("mid", mid)
	// Same code, two behaviors, depending on spare capacity — this is why
	// careless slice sharing causes heisenbugs. Control it explicitly:

	// ---- Defense 1: copy makes an independent slice ----------------------------
	fmt.Println("\n-- copy --")
	src := []int{1, 2, 3}
	dst := make([]int, len(src)) // destination must have length already
	n := copy(dst, src)          // copies min(len(dst), len(src)) elements
	dst[0] = 42
	fmt.Println("copied", n, "elements; src:", src, "dst:", dst, "(independent)")

	// ---- Defense 2: full slice expression s[a:b:c] ------------------------------
	// Third index caps capacity: len = b-a, cap = c-a. With no spare
	// capacity, any future append MUST reallocate — sharing is cut off.
	fmt.Println("\n-- full slice expression s[a:b:c] --")
	base2 := []int{0, 1, 2, 3, 4, 5}
	safe := base2[1:4:4] // len=3, cap=3: capacity clamped at index 4
	show("safe", safe)
	safe = append(safe, 999) // cap exceeded → copies away; base2 untouched
	fmt.Println("after append to safe:")
	show("base2", base2)
	show("safe", safe)
}
