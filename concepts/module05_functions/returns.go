// returns.go — multiple return values, the (value, error) convention,
// named returns, and modifying a named result in a deferred closure.
//
// Run it with:   go run returns.go
package main

import (
	"errors"
	"fmt"
)

// ---- Multiple return values --------------------------------------------
// Just list the types in parentheses. No tuples, no out-params.
func divmod(a, b int) (int, int) {
	return a / b, a % b
}

// The single most important pattern in Go: the LAST return value is an
// error. nil error = success.
func safeDivide(a, b float64) (float64, error) {
	if b == 0 {
		// errors.New creates a simple error value.
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}

// ---- Named return values --------------------------------------------------
// x and y are declared by the signature itself: real variables, zero-valued
// when the function starts. A bare `return` returns their current values.
func split(sum int) (x, y int) {
	x = sum * 4 / 9
	y = sum - x
	return // "naked return" — returns x, y implicitly
	// Fine in a 4-line function. In a 50-line function a naked return
	// forces readers to reconstruct the values in their head — avoid.
}

// Better everyday use of names: DOCUMENTATION. The names say what each
// result means, but we still return explicitly:
func minMax(nums []int) (min, max int) {
	min, max = nums[0], nums[0]
	for _, n := range nums[1:] {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	return min, max // explicit — reader never has to guess
}

// ---- The legit superpower of named returns: defer can modify them ----------
// The deferred closure runs AFTER `return` sets err, but BEFORE the caller
// receives it — so it can wrap/replace the result on the way out.
func fetchConfig(path string) (err error) {
	defer func() {
		if err != nil {
			// %w wraps the original error so callers can still inspect it.
			err = fmt.Errorf("fetchConfig(%q): %w", path, err)
		}
	}()

	if path == "" {
		return errors.New("empty path") // gets wrapped by the defer above
	}
	return nil // success passes through the defer untouched
}

func main() {
	// Grab both values...
	q, r := divmod(17, 5)
	fmt.Println("17 / 5 =", q, "remainder", r)

	// ...or discard one with the blank identifier. You cannot just ignore
	// a value you've assigned — unused variables don't compile — so _ is
	// the explicit "I don't care".
	q2, _ := divmod(100, 7)
	fmt.Println("100 / 7 =", q2)

	// The (value, error) dance — you'll write this thousands of times:
	if result, err := safeDivide(10, 3); err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Printf("10 / 3 = %.4f\n", result)
	}
	if _, err := safeDivide(1, 0); err != nil {
		fmt.Println("as expected:", err)
	}

	x, y := split(17)
	fmt.Println("split(17) ->", x, y)

	lo, hi := minMax([]int{7, 2, 9, 4})
	fmt.Println("minMax ->", lo, hi)

	// The deferred wrapper in action:
	fmt.Println("fetchConfig(\"\") ->", fetchConfig(""))
	fmt.Println("fetchConfig(\"app.yaml\") ->", fetchConfig("app.yaml"))
}
