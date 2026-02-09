// Package testingdemo is the code-under-test for Module 14.
//
// NOTE: this module is different from the others — it's a real library
// package (not package main), because tests can only be run with `go test`,
// which needs a package + its _test.go files. See README.md.
package testingdemo

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"unicode"
)

// WordCount returns how many times each whitespace-separated, lowercased
// word appears in text. Simple and pure — the easiest kind of code to test.
func WordCount(text string) map[string]int {
	counts := make(map[string]int)
	for _, w := range strings.Fields(text) {
		counts[strings.ToLower(w)]++
	}
	return counts
}

// Slugify turns a title into a URL slug: lowercase, letters/digits kept,
// every other run of characters collapsed to a single '-'.
// "Go: 100% fun!" → "go-100-fun".
func Slugify(title string) string {
	var b strings.Builder
	lastDash := true // suppress a leading dash
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// Reverse reverses a string RUNE by rune, so multi-byte UTF-8 stays intact.
// (A byte-wise reverse — the buggy version fuzzing famously catches — would
// shred non-ASCII input; see FuzzReverse in wordcount_test.go.)
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// CountLinesInFile counts newline-separated lines in a file. It touches the
// filesystem, so its test uses the t.TempDir fixture pattern.
func CountLinesInFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

// SafeCounter is a concurrency-safe counter. Its test runs 100 goroutines
// against it — meaningful only under `go test -race`.
type SafeCounter struct {
	mu sync.Mutex
	n  int
}

func (c *SafeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *SafeCounter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}
