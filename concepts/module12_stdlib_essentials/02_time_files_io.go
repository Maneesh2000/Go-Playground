// Module 12, example 2: time (reference date, durations, timers/tickers)
// and files/streams with os + io + bufio.
//
// Run with: go run 02_time_files_io.go
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	// ================================ time ==============================
	fmt.Println("--- time.Time & Duration ---")

	now := time.Now()
	fmt.Println("now:", now.Format(time.RFC3339))

	// THE REFERENCE DATE: Mon Jan 2 15:04:05 MST 2006.
	// You don't write "YYYY-MM-DD"; you write how the reference date
	// would look in your desired layout. 2006=year 01=month 02=day
	// 15=hour(24h) 04=minute 05=second.
	fmt.Println("custom :", now.Format("2006-01-02 15:04:05"))
	fmt.Println("kitchen:", now.Format(time.Kitchen)) // "3:04PM"
	fmt.Println("verbose:", now.Format("Monday, January 2, 2006"))

	// Parsing uses the same layout trick.
	t, err := time.Parse("02/01/2006", "04/07/2026")
	fmt.Println("parsed:", t.Format(time.DateOnly), "err:", err)

	// Durations are typed nanosecond counts — arithmetic just works.
	d := 90 * time.Minute
	fmt.Printf("duration: %v = %.1f hours\n", d, d.Hours())
	fmt.Println("deadline:", now.Add(d).Format(time.TimeOnly))
	fmt.Println("since epoch-ish date:", time.Since(t) < 0, "(t is in the future)")

	// Comparisons: Before / After / Equal (don't use == on time.Time).
	fmt.Println("now.Before(now.Add(d)):", now.Before(now.Add(d)))

	// --- timers fire once; tickers fire repeatedly ---
	fmt.Println("--- timer & ticker ---")

	// time.After: a channel that delivers one value after the duration.
	start := time.Now()
	<-time.After(50 * time.Millisecond)
	fmt.Printf("timer fired after %v\n", time.Since(start).Round(time.Millisecond))

	// Ticker: fires every interval. ALWAYS Stop() it, or it leaks.
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for i := 1; i <= 3; i++ {
		<-ticker.C
		fmt.Printf("tick %d at %v\n", i, time.Since(start).Round(time.Millisecond))
	}

	// ========================= os + io + bufio ==========================
	fmt.Println("--- files & readers ---")

	// Work in a temp dir so this example is self-cleaning.
	dir, err := os.MkdirTemp("", "module12-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "notes.txt")

	// Write a file the simple way (creates/truncates, mode 0644).
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}

	// Read it back — TWO styles:

	// Style 1: slurp the whole file (fine for small files).
	data, _ := os.ReadFile(path)
	fmt.Printf("ReadFile got %d bytes\n", len(data))

	// Style 2: stream it line-by-line with bufio.Scanner. This is the
	// composition philosophy in action — the reader CHAIN:
	//
	//   os.File ──► bufio.Scanner ──► your loop
	//   (disk)      (buffers reads,    (sees clean lines,
	//                splits lines)      no trailing \n)
	f, _ := os.Open(path)
	defer f.Close()
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		fmt.Printf("  line %d: %q\n", lineNo, sc.Text())
	}
	if err := sc.Err(); err != nil { // check AFTER the loop
		fmt.Fprintln(os.Stderr, "scan:", err)
	}

	// Because countWords takes io.Reader, it works on a file, a string,
	// stdin, an HTTP body... the caller decides, the function doesn't care.
	fmt.Println("--- io.Reader is an interface ---")
	f2, _ := os.Open(path)
	defer f2.Close()
	fmt.Println("words in file:  ", countWords(f2))
	fmt.Println("words in string:", countWords(strings.NewReader("one two three four")))

	// io.Copy streams bytes from any Reader to any Writer — here: file → stdout.
	// io.MultiWriter tees a write to several writers at once.
	f3, _ := os.Open(path)
	defer f3.Close()
	var backup strings.Builder
	tee := io.MultiWriter(os.Stdout, &backup) // stdout AND an in-memory copy
	n, _ := io.Copy(tee, f3)
	fmt.Printf("io.Copy moved %d bytes (backup holds %d too)\n", n, backup.Len())
}

// countWords counts whitespace-separated words from ANY source of bytes.
// Accepting io.Reader (not *os.File, not string) is what makes it reusable
// and trivially testable.
func countWords(r io.Reader) int {
	sc := bufio.NewScanner(r)
	sc.Split(bufio.ScanWords) // Scanner can split by lines, words, runes, bytes
	count := 0
	for sc.Scan() {
		count++
	}
	return count
}
