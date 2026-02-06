// Module 09, Example 2 — Generic functions: Map, Filter, Reduce.
//
// These three little functions are the classic showcase for generics:
// they capture LOOP SHAPES ("transform each", "keep some", "boil down to
// one value") independent of the element types involved.
//
// Run with: go run 02_map_filter_reduce.go
package main

import (
	"fmt"
	"strings"
)

// Map transforms a []T into a []U by applying f to each element.
// Two type parameters: input element type T, output element type U.
// `any` is the loosest constraint — fine here, because the only things we
// do with T and U are pass them to f and store them.
func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs)) // preallocate: we know the final length
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

// Filter keeps the elements for which keep(x) is true.
// One type parameter — output type equals input type.
func Filter[T any](xs []T, keep func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

// Reduce folds a slice into a single value: start from `init`, then
// repeatedly combine the accumulator with the next element.
//
//	Reduce([1,2,3], 0, +)  =>  ((0+1)+2)+3  =>  6
//
// Accumulator type A is independent of element type T — that's what lets
// us reduce []string into an int, for example.
func Reduce[T, A any](xs []T, init A, combine func(A, T) A) A {
	acc := init
	for _, x := range xs {
		acc = combine(acc, x)
	}
	return acc
}

type Person struct {
	Name string
	Age  int
}

func main() {
	nums := []int{1, 2, 3, 4, 5, 6}

	// ---- Map: same type in and out --------------------------------------------
	doubled := Map(nums, func(x int) int { return x * 2 })
	fmt.Println("doubled:      ", doubled)

	// ---- Map: CHANGING the type (int -> string) — T and U differ ---------------
	labels := Map(nums, func(x int) string { return fmt.Sprintf("n%02d", x) })
	fmt.Println("labels:       ", labels)

	// Note the call sites: no [int, string] anywhere. Type inference reads
	// the argument types and fills the brackets in for us.

	// ---- Filter -------------------------------------------------------------------
	evens := Filter(nums, func(x int) bool { return x%2 == 0 })
	fmt.Println("evens:        ", evens)

	// ---- Reduce: sum and product -----------------------------------------------
	sum := Reduce(nums, 0, func(acc, x int) int { return acc + x })
	product := Reduce(nums, 1, func(acc, x int) int { return acc * x })
	fmt.Println("sum, product: ", sum, product)

	// ---- Reduce with DIFFERENT accumulator type: []string -> int -----------------
	words := []string{"go", "is", "surprisingly", "fun"}
	totalLen := Reduce(words, 0, func(acc int, w string) int { return acc + len(w) })
	fmt.Println("total letters:", totalLen)

	// ---- Composing them: a mini data pipeline over structs -------------------------
	people := []Person{
		{"Ada", 36}, {"Grace", 46}, {"Alan", 41}, {"Linus", 21}, {"Ken", 17},
	}

	// adults' names, uppercased, joined:
	adults := Filter(people, func(p Person) bool { return p.Age >= 18 })
	names := Map(adults, func(p Person) string { return strings.ToUpper(p.Name) })
	sentence := Reduce(names, "", func(acc, n string) string {
		if acc == "" {
			return n
		}
		return acc + ", " + n
	})
	fmt.Println("adults:       ", sentence)

	// average age via Reduce:
	totalAge := Reduce(people, 0, func(acc int, p Person) int { return acc + p.Age })
	fmt.Printf("average age:   %.1f\n", float64(totalAge)/float64(len(people)))

	// Reality check: in everyday Go, a plain for-loop is often MORE readable
	// than chained Map/Filter/Reduce — Go favours explicit loops. Use these
	// when they clarify, not to prove you can. (And check package `slices`
	// first: ContainsFunc, IndexFunc, DeleteFunc... may already do the job.)
}
