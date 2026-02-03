// slices_maps_stdlib.go — the generic `slices` and `maps` packages
// (standard library since Go 1.21) replace piles of hand-written loops.
//
// Run it with:   go run slices_maps_stdlib.go
package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

func main() {
	nums := []int{5, 2, 8, 1, 9, 2}

	// ---- Searching -------------------------------------------------------
	fmt.Println("nums:", nums)
	fmt.Println("Contains 8:", slices.Contains(nums, 8))
	fmt.Println("Index of 2:", slices.Index(nums, 2)) // first occurrence, -1 if absent

	// ---- Min / Max --------------------------------------------------------
	fmt.Println("Min:", slices.Min(nums), " Max:", slices.Max(nums))

	// ---- Sorting ------------------------------------------------------------
	// Sort mutates in place; sort a Clone if you need the original intact.
	sorted := slices.Clone(nums) // proper independent copy in one call
	slices.Sort(sorted)
	fmt.Println("sorted clone:", sorted, " original untouched:", nums)

	// SortFunc with a custom comparison: negative = a first, positive = b
	// first, zero = equal. Sort words by length here:
	words := []string{"kiwi", "fig", "banana", "date"}
	slices.SortFunc(words, func(a, b string) int {
		return len(a) - len(b)
	})
	fmt.Println("by length:", words)

	// BinarySearch requires a sorted slice; returns (index, found):
	if i, found := slices.BinarySearch(sorted, 8); found {
		fmt.Println("binary search: 8 is at sorted index", i)
	}

	// ---- Comparing & transforming ---------------------------------------------
	fmt.Println("Equal:", slices.Equal([]int{1, 2}, []int{1, 2})) // element-wise ==
	rev := slices.Clone(sorted)
	slices.Reverse(rev) // in-place reversal
	fmt.Println("reversed:", rev)
	// Compact removes ADJACENT duplicates (pair with Sort for full dedupe):
	fmt.Println("deduped:", slices.Compact(sorted))

	// ---- The maps package --------------------------------------------------------
	inventory := map[string]int{"apples": 5, "pears": 2, "plums": 7}

	// maps.Clone: independent copy of the table (keys/values shallow-copied).
	backup := maps.Clone(inventory)
	inventory["apples"] = 0
	fmt.Println("\nbackup unaffected by edit:", backup)
	fmt.Println("maps.Equal(inventory, backup):", maps.Equal(inventory, backup))

	// maps.Keys / maps.Values return ITERATORS (Go 1.23 range-over-func).
	// You can range over them directly...
	total := 0
	for v := range maps.Values(inventory) {
		total += v
	}
	fmt.Println("total items:", total)

	// ...or materialize them. slices.Sorted(collect + sort) is THE idiom
	// for iterating a map deterministically:
	fmt.Println("keys in stable order:")
	for _, k := range slices.Sorted(maps.Keys(inventory)) {
		fmt.Printf("  %s = %d\n", k, inventory[k])
	}

	// ---- Before vs after: why these packages exist --------------------------------
	// The old way to ask "does this slice contain x?":
	found := false
	for _, n := range nums {
		if n == 8 {
			found = true
			break
		}
	}
	// The new way: slices.Contains(nums, 8). Same result, one line,
	// impossible to get subtly wrong.
	fmt.Println("\nmanual loop agrees with slices.Contains:",
		found == slices.Contains(nums, 8))

	_ = strings.ToUpper // (strings imported for the exercise below)
	// Exercise seed: try slices.SortFunc(words, ...) to sort
	// case-insensitively using strings.ToLower in the comparison.
}
