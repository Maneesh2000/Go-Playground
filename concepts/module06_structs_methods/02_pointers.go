// Module 06, Example 2 — Pointers: & (address-of) and * (dereference).
//
// Go passes EVERYTHING by value. A pointer is how you share one piece of
// data instead of copying it. Go pointers are safe: no pointer arithmetic,
// and memory is garbage-collected.
//
// Run with: go run 02_pointers.go
package main

import "fmt"

type User struct {
	Name string
	Age  int
}

// birthdayByValue receives a COPY of the struct. Changing the copy does
// nothing to the caller's variable.
func birthdayByValue(u User) {
	u.Age++ // mutates only the local copy
}

// birthdayByPointer receives the ADDRESS of the caller's struct, so it can
// modify the original.
func birthdayByPointer(u *User) {
	// Shortcut: Go auto-dereferences struct pointers for field access.
	// u.Age is shorthand for (*u).Age — you never write the long form.
	u.Age++
}

func main() {
	// ---- The basics: & takes an address, * follows it ---------------------
	x := 42
	p := &x // p is of type *int ("pointer to int") and holds x's address

	fmt.Println("x  =", x)  // 42
	fmt.Println("p  =", p)  // something like 0x14000110018 (an address)
	fmt.Println("*p =", *p) // 42 — "the value AT the address p"

	// Memory picture:
	//
	//      x (an int)                    p (a *int)
	//   ┌──────────────┐             ┌────────────────┐
	//   │      42      │ ◄────────── │ 0x14000110018  │
	//   └──────────────┘             └────────────────┘
	//    at 0x14000110018             p stores x's address;
	//                                 *p reads/writes x itself.

	// Writing through the pointer changes x:
	*p = 100
	fmt.Println("after *p = 100, x =", x) // 100 — same memory!

	// There is NO pointer arithmetic in Go. This is a compile error:
	//   p++            // invalid operation
	//   p = p + 1      // invalid operation
	// Pointers are references, not numbers you can walk through memory with.

	// ---- The zero value of a pointer is nil -------------------------------
	var q *int // declared but pointing at nothing
	fmt.Println("q =", q, "| q == nil ?", q == nil)
	// Dereferencing nil panics at runtime — uncomment to see:
	//   fmt.Println(*q) // panic: runtime error: invalid memory address or
	//                   //        nil pointer dereference

	// ---- Why pointers matter: value vs pointer semantics ------------------
	ada := User{Name: "Ada", Age: 36}

	birthdayByValue(ada)
	fmt.Println("after birthdayByValue: ", ada.Age) // still 36 — copy mutated

	birthdayByPointer(&ada)                          // pass her address
	fmt.Println("after birthdayByPointer:", ada.Age) // 37 — original mutated

	// ---- new() and &T{} — two ways to get a pointer to a fresh value ------
	u1 := new(User)            // *User pointing at a zero-valued User
	u2 := &User{Name: "Grace"} // pointer to a composite literal (idiomatic)
	u1.Name = "Alan"
	fmt.Println("u1:", *u1, "| u2:", *u2)

	// ---- Pointers are copied by value too ---------------------------------
	// Copying a pointer copies the ADDRESS, so both copies point at the
	// same underlying value:
	alias := &ada
	other := alias // two pointers, one User
	other.Name = "Countess Lovelace"
	fmt.Println("ada.Name via original variable:", ada.Name)

	// Rule of thumb:
	//   * Use values for small, immutable data (cheap to copy, no sharing).
	//   * Use pointers when a function must MODIFY its argument, or when
	//     the struct is large and copying would be wasteful.
}
