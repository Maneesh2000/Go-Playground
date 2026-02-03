// recursion.go — functions calling themselves: factorial, fibonacci
// (naive vs memoized with a closure), and walking a nested structure.
//
// Run it with:   go run recursion.go
package main

import "fmt"

// ---- The shape of every recursive function --------------------------------
//  1. BASE CASE first: the input small enough to answer directly.
//  2. RECURSIVE CASE: call yourself on a SMALLER input and combine.
//
// Miss the base case (or fail to shrink the input) = infinite recursion,
// which crashes with a stack overflow.
func factorial(n int) int {
	if n <= 1 {
		return 1 // base case
	}
	return n * factorial(n-1) // recursive case: n-1 is strictly smaller
}

// Naive fibonacci — beautiful and TERRIBLE: it recomputes the same values
// exponentially many times (fib(40) is ~1.6 billion calls).
func fib(n int) int {
	if n < 2 {
		return n
	}
	return fib(n-1) + fib(n-2)
}

// Fix: memoization. The cache map persists across recursive calls because
// the closure captures it (Module 05's closures meet recursion!).
func makeMemoFib() func(int) int {
	cache := map[int]int{}
	var f func(int) int // declare first so the closure can call ITSELF
	f = func(n int) int {
		if n < 2 {
			return n
		}
		if v, ok := cache[n]; ok { // comma-ok from Module 04
			return v
		}
		v := f(n-1) + f(n-2)
		cache[n] = v
		return v
	}
	return f
}

// Recursion shines on NESTED data. Sum a tree of values:
type node struct {
	value    int
	children []node
}

func treeSum(n node) int {
	total := n.value
	for _, child := range n.children {
		total += treeSum(child) // recurse into each subtree
	}
	return total
}

func main() {
	fmt.Println("factorial(5) =", factorial(5))
	fmt.Println("factorial(10) =", factorial(10))

	fmt.Println("naive fib(10) =", fib(10))

	memoFib := makeMemoFib()
	fmt.Println("memoized fib(50) =", memoFib(50)) // instant; naive would take ages

	// A small tree:        1
	//                     / \
	//                    2   3
	//                   / \
	//                  4   5
	tree := node{
		value: 1,
		children: []node{
			{value: 2, children: []node{{value: 4}, {value: 5}}},
			{value: 3},
		},
	}
	fmt.Println("treeSum =", treeSum(tree)) // 1+2+3+4+5 = 15

	// ---- Practical notes -----------------------------------------------------
	// * Go has NO tail-call optimization: every recursive call adds a
	//   stack frame. Goroutine stacks grow as needed (from ~8KB up to a
	//   ~1GB default limit), so deep recursion eventually panics:
	//   "goroutine stack exceeds limit".
	// * Rule of thumb: recursion for naturally nested data (trees, JSON,
	//   directories, parsers); plain loops for unbounded linear work.
}
