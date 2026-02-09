// Module 12, example 5: regexp, sort/slices/cmp, log/slog, and flag.
//
// Run with:
//
//	go run 05_regexp_sort_slog_flag.go
//	go run 05_regexp_sort_slog_flag.go -name=Gopher -verbose
//	go run 05_regexp_sort_slog_flag.go -h        # auto-generated help!
package main

import (
	"cmp"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
)

// Compile regexes ONCE, at package level. MustCompile panics on a bad
// pattern — appropriate for patterns hard-coded by you (a bad one is a bug,
// not a runtime condition). Go uses RE2: no backreferences, but guaranteed
// linear-time matching — no "catastrophic backtracking" outages.
var emailRe = regexp.MustCompile(`([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)

type Person struct {
	Name string
	Age  int
}

func main() {
	// ------------------------------ flag --------------------------------
	// Define flags, then Parse, then use. Each definition returns a POINTER.
	name := flag.String("name", "world", "who to greet")
	verbose := flag.Bool("verbose", false, "enable debug logging")
	repeat := flag.Int("repeat", 1, "how many greetings")
	flag.Parse() // MUST be called before reading flag values

	// ------------------------------ slog --------------------------------
	// Structured logging: message + key/value pairs. Machines can index
	// "user"="Gopher"; they can't index "Hello Gopher said the log".
	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	// Text handler for humans; swap for slog.NewJSONHandler in services.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Debug("flags parsed", "name", *name, "repeat", *repeat) // hidden unless -verbose
	slog.Info("starting", "greeting_count", *repeat)

	for i := 0; i < *repeat; i++ {
		fmt.Printf("Hello, %s!\n", *name)
	}

	// ----------------------------- regexp -------------------------------
	fmt.Println("--- regexp ---")
	text := "contact ada@example.com or ops@amura.ai for help"

	fmt.Println("match?          ", emailRe.MatchString(text))
	fmt.Println("first match:    ", emailRe.FindString(text))
	fmt.Println("all matches:    ", emailRe.FindAllString(text, -1)) // -1 = no limit

	// Capture groups: index 0 is the whole match, 1+ are the (...) groups.
	if m := emailRe.FindStringSubmatch(text); m != nil {
		fmt.Printf("user=%q domain=%q\n", m[1], m[2])
	}

	// Replacement, with $1/$2 referring to capture groups.
	fmt.Println("redacted:       ", emailRe.ReplaceAllString(text, "$1@[redacted]"))

	// ------------------------ sort / slices / cmp -----------------------
	fmt.Println("--- slices & cmp ---")

	nums := []int{42, 7, 19, 3, 88}
	slices.Sort(nums) // generic, fast, replaces sort.Ints
	fmt.Println("sorted:", nums)
	fmt.Println("contains 19?", slices.Contains(nums, 19))
	idx, found := slices.BinarySearch(nums, 42) // requires sorted input
	fmt.Printf("binary search 42: index=%d found=%v\n", idx, found)
	fmt.Println("max:", slices.Max(nums), "min:", slices.Min(nums))

	// Sorting structs: SortFunc + a comparator returning <0, 0, >0.
	// cmp.Compare writes the comparator for you.
	people := []Person{{"Grace", 85}, {"Ada", 36}, {"Linus", 55}, {"Ken", 36}}
	slices.SortFunc(people, func(a, b Person) int {
		// Multi-key sort: by age, then name — cmp.Or picks the first non-zero.
		return cmp.Or(
			cmp.Compare(a.Age, b.Age),
			cmp.Compare(a.Name, b.Name),
		)
	})
	fmt.Println("by age then name:", people)

	// SortStableFunc preserves the input order of "equal" items — matters
	// when sorting an already-meaningfully-ordered list by one more key.
	slices.SortStableFunc(people, func(a, b Person) int {
		return cmp.Compare(b.Age, a.Age) // descending: swap the operands
	})
	fmt.Println("by age descending:", people)

	slog.Info("done") // structured sign-off
}
