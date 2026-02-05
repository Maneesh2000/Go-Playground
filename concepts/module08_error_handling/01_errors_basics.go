// Module 08, Example 1 — Errors are values: the error interface,
// errors.New, fmt.Errorf, and the early-return style.
//
// Run with: go run 01_errors_basics.go
package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Go convention: a function that can fail returns (result, error), with
// error LAST. nil error means success.
func divide(a, b float64) (float64, error) {
	if b == 0 {
		// errors.New: the simplest error — a fixed message.
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}

// fmt.Errorf builds a formatted error message. Convention for messages:
// lowercase, no trailing punctuation (they get composed into longer chains:
// "load config: parse port: invalid syntax").
func findUser(id int) (string, error) {
	users := map[int]string{1: "ada", 2: "grace"}
	name, ok := users[id]
	if !ok {
		return "", fmt.Errorf("user %d not found", id)
	}
	return name, nil
}

// A multi-step function showing the idiomatic SHAPE of Go error handling:
// each step is "try; if err != nil, return early". The happy path stays at
// the left margin, failures exit immediately. No try/catch, no exceptions —
// every failure path is explicit and visible.
func parseConfigLine(line string) (key string, value int, err error) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("malformed line %q: missing '='", line)
	}

	key = strings.TrimSpace(parts[0])
	if key == "" {
		return "", 0, fmt.Errorf("malformed line %q: empty key", line)
	}

	value, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		// Add context so the caller knows WHERE Atoi failed.
		// (Next example upgrades %v to %w for proper wrapping.)
		return "", 0, fmt.Errorf("line %q: value is not a number: %v", line, err)
	}

	return key, value, nil // success: err is nil
}

func main() {
	// ---- The basic check-and-handle pattern --------------------------------
	result, err := divide(10, 3)
	if err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Printf("10 / 3 = %.4f\n", result)
	}

	_, err = divide(1, 0)
	if err != nil {
		// An error is just a value with an Error() string method;
		// fmt prints that string.
		fmt.Println("error:", err)
	}

	// ---- Errors are VALUES: store them, collect them, compare them ---------
	// Because errors are plain values, you can do ordinary things with them,
	// like gathering all failures instead of stopping at the first:
	var problems []error
	for _, id := range []int{1, 42, 2, 99} {
		name, err := findUser(id)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		fmt.Println("found user:", name)
	}
	fmt.Printf("%d lookups failed:\n", len(problems))
	for _, p := range problems {
		fmt.Println("  -", p)
	}

	// errors.Join combines several errors into one (Go 1.20+):
	if combined := errors.Join(problems...); combined != nil {
		fmt.Printf("joined error:\n%v\n", combined)
	}

	// ---- Early-return style in action ---------------------------------------
	lines := []string{
		"port = 8080",
		"timeout = fast", // bad value
		"= 12",           // bad key
		"retries = 3",
	}
	fmt.Println("\nparsing config lines:")
	for _, line := range lines {
		key, value, err := parseConfigLine(line)
		if err != nil {
			fmt.Println("  skip:", err)
			continue // handle and move on — our choice as the caller
		}
		fmt.Printf("  ok: %s -> %d\n", key, value)
	}

	// The caller DECIDES what handling means: retry, log, substitute a
	// default, return upward... The language forces nothing except that
	// ignoring an error is visible (you'd write `_` explicitly).
}
