// Module 13, example 5: a mini `grep` clone — ties the module together:
// flag parsing, regexp, files vs stdin, io streams, stderr vs stdout,
// and grep-style exit codes.
//
// Usage:
//
//	go run 05_minigrep.go [-n] [-v] PATTERN [FILE ...]
//
// Try:
//
//	go run 05_minigrep.go func 05_minigrep.go            # search a file
//	go run 05_minigrep.go -n 'os\.' 05_minigrep.go       # line numbers, regex
//	echo -e "cat\ndog\ncow" | go run 05_minigrep.go 'c.t' # search stdin
//	go run 05_minigrep.go -v -n cat 05_minigrep.go       # invert match
//
// Exit codes follow grep tradition: 0 = matched, 1 = no matches, 2 = error.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
)

func main() {
	// ----------------------------- flags --------------------------------
	showLineNums := flag.Bool("n", false, "prefix matches with line numbers")
	invert := flag.Bool("v", false, "select NON-matching lines")
	flag.Usage = func() { // custom help text (shown by -h or bad usage)
		fmt.Fprintf(os.Stderr, "usage: %s [-n] [-v] PATTERN [FILE ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	// flag.Args() = positional args after the flags: PATTERN, then files.
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(2)
	}
	pattern, files := args[0], args[1:]

	// Compile (not MustCompile): the pattern comes from the USER, so a bad
	// pattern is a runtime condition, not a programmer bug.
	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, "minigrep: bad pattern:", err)
		os.Exit(2)
	}

	// --------------------- files or stdin? ------------------------------
	// No file args → read stdin, like real grep. That's what makes
	// `something | minigrep pat` work.
	matched := false
	hadError := false

	if len(files) == 0 {
		matched = grep(re, os.Stdin, "", *showLineNums, *invert)
	}
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			// Report the error and CONTINUE with other files (grep behavior),
			// but remember it for the exit code.
			fmt.Fprintln(os.Stderr, "minigrep:", err)
			hadError = true
			continue
		}
		// Only prefix output with the filename when searching >1 file.
		label := ""
		if len(files) > 1 {
			label = name
		}
		if grep(re, f, label, *showLineNums, *invert) {
			matched = true
		}
		f.Close()
	}

	// ------------------------- exit code --------------------------------
	switch {
	case hadError:
		os.Exit(2)
	case !matched:
		os.Exit(1) // silent "no matches" — scripts test this with `if minigrep ...`
	}
	// implicit os.Exit(0)
}

// grep scans one stream and prints matching lines. Taking io.Reader (not
// *os.File) means files and stdin flow through identical code — and makes
// this function trivially testable with strings.NewReader.
func grep(re *regexp.Regexp, r io.Reader, label string, nums, invert bool) bool {
	sc := bufio.NewScanner(r)
	// Default Scanner token limit is 64KB/line; bump it so long lines
	// (minified JS, logs) don't kill us.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	matched := false
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if re.MatchString(line) != invert { // XOR with -v
			matched = true
			switch {
			case label != "" && nums:
				fmt.Printf("%s:%d:%s\n", label, lineNo, line)
			case label != "":
				fmt.Printf("%s:%s\n", label, line)
			case nums:
				fmt.Printf("%d:%s\n", lineNo, line)
			default:
				fmt.Println(line)
			}
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "minigrep: read:", err)
	}
	return matched
}
