// arrays_vs_slices.go — arrays have VALUE semantics (copies everywhere);
// slices are lightweight headers that SHARE underlying data.
//
// Run it with:   go run arrays_vs_slices.go
package main

import "fmt"

func main() {
	// ---- Arrays: fixed length, part of the type -------------------------
	var a [4]int // exactly four ints, zero-valued
	fmt.Println("zeroed array:", a)

	b := [3]string{"x", "y", "z"} // array literal
	c := [...]int{1, 2, 3}        // ... = compiler counts; type is [3]int
	fmt.Println("b:", b, " c:", c, " len(c):", len(c))

	// The LENGTH IS PART OF THE TYPE: [3]int and [4]int are different,
	// incompatible types. This line would not compile:
	//   var d [4]int = c   // ERROR: cannot use c ([3]int) as [4]int

	// ---- Arrays copy on assignment ---------------------------------------
	d := c // copies ALL elements — d is fully independent
	d[0] = 99
	fmt.Println("after d[0]=99:  c =", c, " d =", d) // c unchanged

	// Same when passing to a function — the callee gets a copy:
	doubleArray(c)
	fmt.Println("after doubleArray(c): c =", c, "(unchanged — copy semantics)")

	// ---- Slices: flexible length, shared data ------------------------------
	s := []int{10, 20, 30} // NOTE: no length in the brackets → it's a slice
	fmt.Println("\nslice s:", s, " len:", len(s), " cap:", cap(s))

	// Assigning a slice copies only the small header (ptr/len/cap).
	// Both t and s now point at the SAME underlying array:
	t := s
	t[0] = 999
	fmt.Println("after t[0]=999:  s =", s, " t =", t, " <- both changed!")

	// Passing a slice to a function shares data the same way:
	doubleSlice(s)
	fmt.Println("after doubleSlice(s): s =", s, "(changed — shared data)")

	// ---- Making slices -------------------------------------------------------
	// make(type, len) or make(type, len, cap) pre-allocates:
	pre := make([]int, 3)     // [0 0 0]        len=3 cap=3
	room := make([]int, 0, 8) // []             len=0 cap=8 (room to append cheaply)
	fmt.Println("make'd:", pre, "len/cap:", len(pre), cap(pre),
		"| empty with capacity:", room, "len/cap:", len(room), cap(room))

	// A nil slice (declared, never initialized) is safe to read len/cap
	// of and to append to — idiomatic Go often starts with one:
	var nilSlice []int
	fmt.Println("nil slice:", nilSlice, "len:", len(nilSlice), "is nil:", nilSlice == nil)
	nilSlice = append(nilSlice, 1) // append handles nil fine
	fmt.Println("append to nil slice works:", nilSlice)

	// ---- Slicing an array gives you a slice over it ---------------------------
	arr := [5]int{1, 2, 3, 4, 5}
	window := arr[1:4] // slice viewing arr's elements 1..3 — no copy
	window[0] = 200    // writes through to the array
	fmt.Println("\narr after window[0]=200:", arr, " window:", window)
}

// Arrays are passed BY VALUE: this doubles a private copy; caller unaffected.
func doubleArray(x [3]int) {
	for i := range x {
		x[i] *= 2
	}
}

// Slices pass a header pointing at the caller's data: mutations are shared.
func doubleSlice(x []int) {
	for i := range x {
		x[i] *= 2
	}
}
