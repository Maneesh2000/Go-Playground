// Tests for wordcount.go — demonstrates the whole Module 14 toolbox:
// table-driven tests, subtests, t.Helper, t.TempDir, benchmarks, fuzzing,
// and a race-detector-oriented concurrency test.
//
// Run me with `go test` (NOT `go run`):
//
//	go test -v ./...
//	go test -bench=. -benchmem .
//	go test -fuzz=FuzzReverse -fuzztime=10s .
//	go test -race ./...
package testingdemo

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
//  1. A basic test. Signature: func TestXxx(t *testing.T).
//     t.Errorf = "record failure, continue"; t.Fatalf = "record and stop".
//
// ---------------------------------------------------------------------------
func TestWordCountBasic(t *testing.T) {
	got := WordCount("the cat and the hat")

	if got["the"] != 2 {
		t.Errorf(`WordCount: count for "the" = %d, want 2`, got["the"])
	}
	if got["cat"] != 1 {
		t.Errorf(`WordCount: count for "cat" = %d, want 1`, got["cat"])
	}
	if len(got) != 4 { // the, cat, and, hat
		t.Errorf("WordCount: %d distinct words, want 4 (map: %v)", len(got), got)
	}
}

// ---------------------------------------------------------------------------
//  2. THE canonical pattern: table-driven tests with named subtests.
//     Adding a case = adding one struct literal. Run one case alone with:
//     go test -run 'TestSlugify/punctuation_collapses'
//
// ---------------------------------------------------------------------------
func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string // subtest name — shows up in -v output and -run filters
		input string
		want  string
	}{
		{"simple lowercase", "hello", "hello"},
		{"uppercase folded", "Hello", "hello"},
		{"spaces become dashes", "Hello World", "hello-world"},
		{"punctuation collapses", "Go: 100% fun!", "go-100-fun"},
		{"leading junk trimmed", "  --Go--  ", "go"},
		{"unicode letters kept", "Café Über", "café-über"},
		{"empty input", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				// Idiomatic failure message: call, got, want.
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
//  3. t.Helper: shared assertions that blame the CALLER's line on failure.
//     Without t.Helper(), every failure would point here, uselessly.
//
// ---------------------------------------------------------------------------
func assertCount(t *testing.T, counts map[string]int, word string, want int) {
	t.Helper()
	if got := counts[word]; got != want {
		t.Errorf("count[%q] = %d, want %d", word, got, want)
	}
}

func TestWordCountCaseFolding(t *testing.T) {
	counts := WordCount("Go go GO gopher")
	assertCount(t, counts, "go", 3) // failures reported at THESE lines
	assertCount(t, counts, "gopher", 1)
}

// ---------------------------------------------------------------------------
//  4. Fixtures with t.TempDir: a real file on disk, zero cleanup code —
//     the directory is deleted automatically when the test ends.
//
// ---------------------------------------------------------------------------
func TestCountLinesInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "three.txt")

	// Setup failure → Fatalf: no point asserting on a file we couldn't write.
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	got, err := CountLinesInFile(path)
	if err != nil {
		t.Fatalf("CountLinesInFile(%q) returned error: %v", path, err)
	}
	if got != 3 {
		t.Errorf("CountLinesInFile(%q) = %d, want 3", path, got)
	}

	// The error path deserves a test too.
	if _, err := CountLinesInFile(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("CountLinesInFile on a missing file: expected an error, got nil")
	}
}

// ---------------------------------------------------------------------------
//  5. Concurrency test — only meaningful under `go test -race`, which
//     instruments memory accesses. Make -race part of your CI invocation.
//
// ---------------------------------------------------------------------------
func TestSafeCounterConcurrent(t *testing.T) {
	var c SafeCounter
	var wg sync.WaitGroup
	const goroutines, perG = 100, 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()

	if got, want := c.Value(), goroutines*perG; got != want {
		t.Errorf("SafeCounter.Value() = %d, want %d", got, want)
	}
}

// ---------------------------------------------------------------------------
//  6. Benchmarks: func BenchmarkXxx(b *testing.B). The framework raises b.N
//     until the timing is statistically stable.
//     Go 1.24+ prefers `for b.Loop() { ... }` (immune to dead-code
//     elimination, excludes setup); we use b.N so this compiles on 1.22.
//     Run: go test -bench=. -benchmem .
//
// ---------------------------------------------------------------------------
func BenchmarkSlugify(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Slugify("Go: 100% Fun for Systems Programming!")
	}
}

func BenchmarkWordCount(b *testing.B) {
	text := strings.Repeat("the quick brown fox jumps over the lazy dog ", 100)
	b.ResetTimer() // don't charge the setup above to the benchmark
	for i := 0; i < b.N; i++ {
		WordCount(text)
	}
}

// ---------------------------------------------------------------------------
//  7. Fuzzing: the runtime mutates inputs looking for panics and property
//     violations. We assert two PROPERTIES of Reverse:
//     (a) Reverse(Reverse(s)) == s        (round-trip)
//     (b) valid UTF-8 in → valid UTF-8 out
//     Try replacing Reverse with a byte-wise loop — `go test -fuzz=FuzzReverse`
//     finds a counterexample (some multi-byte string) within seconds, and
//     saves it under testdata/fuzz/ so plain `go test` replays it forever.
//
// ---------------------------------------------------------------------------
func FuzzReverse(f *testing.F) {
	// Seed corpus: known-interesting starting points for the mutator.
	for _, seed := range []string{"", "a", "hello", "héllo", "世界", "!12345"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, s string) {
		rev := Reverse(s)
		doubleRev := Reverse(rev)

		if s != doubleRev {
			t.Errorf("Reverse(Reverse(%q)) = %q, want the original", s, doubleRev)
		}
		if utf8.ValidString(s) && !utf8.ValidString(rev) {
			t.Errorf("Reverse(%q) produced invalid UTF-8: %q", s, rev)
		}
	})
}
