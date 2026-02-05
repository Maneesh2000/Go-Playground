// Module 06, Example 1 — Struct basics: definition, literals, comparison,
// anonymous structs.
//
// Run with: go run 01_structs_basics.go
package main

import "fmt"

// A struct is a typed collection of fields. This is how you model "a thing
// with properties" in Go. There are no classes — just structs plus methods
// (methods come in example 3).
type User struct {
	Name string
	Age  int
	// Fields starting with a lowercase letter would be unexported
	// (private to the package). Uppercase = exported (public).
}

// Structs can nest other structs.
type Address struct {
	City    string
	Country string
}

type Customer struct {
	User    User    // a named field of struct type
	Address Address // structs compose naturally
}

func main() {
	// ---- Struct literals -------------------------------------------------

	// Preferred: named fields. Order doesn't matter, and you can omit fields
	// (omitted fields get their zero value).
	ada := User{Name: "Ada", Age: 36}
	fmt.Println("ada:", ada) // {Ada 36}

	// Positional literal: values must be in field order and ALL fields must
	// be present. Fragile — if the struct changes, this breaks. Avoid except
	// for tiny obvious structs like image.Point{1, 2}.
	grace := User{"Grace", 46}
	fmt.Println("grace:", grace)

	// Zero-value struct: every field is its own zero value ("" and 0 here).
	// This is valid and usable immediately — no constructor call required.
	var nobody User
	fmt.Printf("nobody: %+v\n", nobody) // %+v prints field names too

	// %#v prints Go-syntax representation — great for debugging.
	fmt.Printf("as Go syntax: %#v\n", ada)

	// ---- Accessing and updating fields -----------------------------------
	ada.Age = 37 // dot notation reads and writes fields
	fmt.Println("ada after birthday:", ada.Name, "is", ada.Age)

	// ---- Structs are values (copied on assignment) -----------------------
	copyOfAda := ada // this COPIES all fields
	copyOfAda.Name = "Ada II"
	fmt.Println("original:", ada.Name, "| copy:", copyOfAda.Name)
	// original is untouched — the copy is independent.

	// ---- Struct comparison ------------------------------------------------
	// Structs are comparable with == when all fields are comparable.
	// Two structs are equal when ALL corresponding fields are equal.
	a := User{Name: "Ada", Age: 37}
	fmt.Println("ada == a ?", ada == a) // true — same field values

	// NOTE: a struct containing a slice, map, or function field is NOT
	// comparable; using == on it is a compile error, e.g.:
	//   type Bad struct{ Tags []string }
	//   Bad{} == Bad{}   // compile error: invalid operation

	// ---- Nested struct literals -------------------------------------------
	c := Customer{
		User:    User{Name: "Linus", Age: 55},
		Address: Address{City: "Helsinki", Country: "Finland"},
	}
	// Chained access walks into nested structs:
	fmt.Println(c.User.Name, "lives in", c.Address.City)

	// ---- Anonymous structs -------------------------------------------------
	// Define a struct type inline, use it once, no name needed.
	// Very common in table-driven tests and quick JSON payloads.
	point := struct {
		X, Y int
	}{X: 3, Y: 4}
	fmt.Println("anonymous struct point:", point)

	// A slice of anonymous structs — the classic test-table shape:
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"go", 2},
		{"", 0},
	}
	for _, tc := range tests {
		got := len(tc.input)
		fmt.Printf("len(%q) = %d (want %d) ok=%v\n",
			tc.input, got, tc.want, got == tc.want)
	}
}
