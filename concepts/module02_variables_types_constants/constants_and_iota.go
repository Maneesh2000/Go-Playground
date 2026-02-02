// constants_and_iota.go — const, typed vs untyped constants, and iota.
//
// Run it with:   go run constants_and_iota.go
package main

import "fmt"

// ---- Plain constants --------------------------------------------------
// Constants are fixed at COMPILE time. They can be numbers, strings, or
// booleans — never slices, maps, or anything computed at runtime.
const Pi = 3.14159
const Greeting = "hello, constants"

// ---- Untyped constants have arbitrary precision ------------------------
// This value doesn't fit in any int on a 32-bit machine, but as an untyped
// constant it's fine — it only needs to fit when USED somewhere.
const huge = 1 << 62

// ---- iota: the enum generator ------------------------------------------
// Inside a const block, iota is 0 on the first line and increments by one
// per line. When a line has no expression, the previous expression repeats.
type Weekday int

const (
	Sunday    Weekday = iota // iota = 0
	Monday                   // iota = 1 (implicitly: Monday Weekday = iota)
	Tuesday                  // 2
	Wednesday                // 3
	Thursday                 // 4
	Friday                   // 5
	Saturday                 // 6
)

// iota can be part of any expression. Classic trick: powers of 1024.
const (
	_  = iota             // iota = 0: assign to _ to skip it
	KB = 1 << (10 * iota) // 1 << 10 = 1024
	MB                    // 1 << 20
	GB                    // 1 << 30
	TB                    // 1 << 40
)

// Bit flags with iota — each constant gets its own bit:
type Permission uint8

const (
	Execute Permission = 1 << iota // 1 (binary 001)
	Write                          // 2 (binary 010)
	Read                           // 4 (binary 100)
)

func main() {
	fmt.Println(Greeting, "— Pi =", Pi)

	// ---- Untyped constants adapt to context --------------------------
	// The SAME untyped constant 3 can initialize different types:
	var i int = 3
	var f float64 = 3
	var c complex128 = 3
	fmt.Println("untyped 3 as int/float64/complex128:", i, f, c)

	// Untyped constant arithmetic happens at full precision at compile
	// time; only the final result must fit the destination:
	var third float64 = huge / 3 // computed exactly, then converted
	fmt.Println("huge/3 as float64:", third)

	// ---- Typed constants are strict -----------------------------------
	const typed int = 3
	// var g float64 = typed     // COMPILE ERROR: int is not float64
	var g float64 = float64(typed) // must convert, like any int variable
	fmt.Println("typed constant needed a conversion:", g)

	// ---- Enums in action ----------------------------------------------
	today := Wednesday
	fmt.Println("today's Weekday value:", today) // prints 3 (it's just an int)
	// (Give enums a String() method to print names — covered with methods.)

	fmt.Println("KB, MB, GB:", KB, MB, GB)

	// ---- Bit flags -----------------------------------------------------
	perms := Read | Write // combine flags with bitwise OR
	fmt.Printf("perms = %03b (Read|Write)\n", perms)
	fmt.Println("can read?   ", perms&Read != 0) // AND isolates one bit
	fmt.Println("can execute?", perms&Execute != 0)
}
