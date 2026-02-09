// Module 13, example 4: stdin/stdout/stderr as io streams — building a
// pipe-friendly CLI tool and detecting whether stdin is a pipe or a terminal.
//
// Run it BOTH ways and compare:
//
//	go run 04_stdin_streams.go                     # interactive: no pipe
//	echo "hello pipe" | go run 04_stdin_streams.go # pipe mode: processes stdin
//	printf 'a\nbb\nccc\n' | go run 04_stdin_streams.go | cat -n   # plays well in pipelines
//
// The Unix contract this program honors:
//   - DATA        → stdout   (so downstream tools can consume it)
//   - DIAGNOSTICS → stderr   (so they don't pollute the data stream)
//   - exit code   → 0 ok, non-zero on failure
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	// ------------------ detect: pipe or terminal? -----------------------
	// Stat on stdin tells us what KIND of file descriptor 0 is.
	// A terminal is a "character device"; a pipe or redirected file is not.
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stat stdin:", err) // diagnostics → stderr
		os.Exit(2)
	}
	isPipe := fi.Mode()&os.ModeCharDevice == 0

	// This status line is a DIAGNOSTIC, so it goes to stderr. Try the
	// `| cat -n` pipeline above: stdout stays clean data.
	if isPipe {
		fmt.Fprintln(os.Stderr, "(stdin is a pipe/redirect — processing it)")
	} else {
		fmt.Fprintln(os.Stderr, "(stdin is a terminal — using demo input; pipe something in to change this)")
	}

	// ------------------- choose the input stream ------------------------
	// Because everything is io.Reader, "read from the pipe" and "read from
	// a built-in demo string" are the SAME code path. Tools like grep/wc
	// do exactly this with "file args or stdin".
	var input = bufio.NewScanner(os.Stdin)
	if !isPipe {
		demo := "The quick brown fox\njumps over\nthe lazy dog\n"
		input = bufio.NewScanner(strings.NewReader(demo))
	}

	// ---------------------- process line by line ------------------------
	// Our "tool": number lines and report per-line byte/word counts —
	// data to stdout via a buffered writer (flush before exit!).
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush() // unflushed buffers are a classic "my output vanished" bug

	lines, words, bytes := 0, 0, 0
	for input.Scan() {
		line := input.Text()
		lines++
		w := len(strings.Fields(line))
		words += w
		bytes += len(line) + 1 // +1 for the newline Scanner stripped
		fmt.Fprintf(out, "%4d | %-40q  words=%d\n", lines, line, w)
	}
	if err := input.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read error:", err)
		os.Exit(2)
	}

	// Summary is data too — a downstream `tail -1` might want it.
	fmt.Fprintf(out, "total: %d lines, %d words, %d bytes\n", lines, words, bytes)

	// Exit code: 0 = success. (We'd os.Exit(1) for "nothing processed" in
	// a grep-like tool — see 05_minigrep.go.)
}
