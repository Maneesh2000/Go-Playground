// Module 09, Example 4 — The standard library's generic packages: cmp and
// slices (Go 1.21+). Before writing your own generic helper, check here —
// it probably exists, tested and optimized.
//
// Run with: go run 04_cmp_slices_stdlib.go
package main

import (
	"cmp"
	"fmt"
	"slices"
)

type Player struct {
	Name  string
	Score int
}

func main() {
	nums := []int{42, 7, 19, 3, 42, 88}
	names := []string{"grace", "ada", "linus", "ken"}

	// ---- slices: generic helpers for ANY element type -------------------------
	// Each of these is a generic function; inference hides the brackets.

	// Contains / Index need `comparable` elements (== must work):
	fmt.Println("contains 19?   ", slices.Contains(nums, 19))
	fmt.Println("index of 42:   ", slices.Index(nums, 42)) // first match

	// Min / Max / Sort need cmp.Ordered elements (< must work):
	fmt.Println("min, max:      ", slices.Min(nums), slices.Max(nums))

	slices.Sort(nums) // in-place, ascending
	fmt.Println("sorted:        ", nums)

	// Binary search only makes sense on sorted data:
	pos, found := slices.BinarySearch(nums, 19)
	fmt.Println("bsearch 19:    ", pos, found)

	// Works identically for strings — that's the generic payoff:
	slices.Sort(names)
	fmt.Println("sorted names:  ", names)

	// Reverse, Compact (dedup ADJACENT duplicates — sort first!):
	nums = slices.Compact(nums)
	fmt.Println("deduped:       ", nums)
	slices.Reverse(nums)
	fmt.Println("reversed:      ", nums)

	// ---- cmp: tiny package, big helper ---------------------------------------------
	// cmp.Compare(a, b) returns -1, 0, or +1 for any ordered type.
	fmt.Println("\ncmp.Compare(3,7):    ", cmp.Compare(3, 7))
	fmt.Println("cmp.Compare(\"b\",\"a\"):", cmp.Compare("b", "a"))

	// cmp.Ordered is the CONSTRAINT the sorting world is built on. Use it
	// in your own generics:
	fmt.Println("clamp 15 to [0,10]:  ", clamp(15, 0, 10))
	fmt.Println("clamp 3.7 to [0,1]:  ", clamp(3.7, 0.0, 1.0))

	// ---- Sorting structs: SortFunc + cmp.Compare -------------------------------------
	players := []Player{
		{"ada", 310}, {"ken", 250}, {"grace", 310}, {"linus", 275},
	}

	// SortFunc takes a comparison func returning negative/zero/positive.
	// cmp.Compare is the natural building block:
	slices.SortFunc(players, func(a, b Player) int {
		return cmp.Compare(b.Score, a.Score) // b first = descending
	})
	fmt.Println("\nby score (desc):", players)

	// Multi-key sort: cmp.Or picks the first non-zero comparison —
	// "by score desc, then by name asc":
	slices.SortFunc(players, func(a, b Player) int {
		return cmp.Or(
			cmp.Compare(b.Score, a.Score),
			cmp.Compare(a.Name, b.Name),
		)
	})
	fmt.Println("score,name:     ", players)

	// Func-flavoured helpers exist for most operations:
	rich, ok := findFirst(players, func(p Player) bool { return p.Score > 300 })
	fmt.Println("first >300:     ", rich, ok)

	// (findFirst above is just IndexFunc dressed up — see its body.)

	// Moral: the standard library already wrote the generic code you were
	// about to write. Reach for slices/maps/cmp first; write your own
	// generic helpers only for what they don't cover.
}

// clamp shows using the STANDARD constraint cmp.Ordered in your own code:
// any type supporting < gets clamping for free.
func clamp[T cmp.Ordered](v, lo, hi T) T {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// findFirst wraps slices.IndexFunc into a comma-ok shape.
func findFirst[T any](xs []T, pred func(T) bool) (T, bool) {
	if i := slices.IndexFunc(xs, pred); i >= 0 {
		return xs[i], true
	}
	var zero T
	return zero, false
}
