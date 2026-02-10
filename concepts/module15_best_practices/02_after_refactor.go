// Module 15, example 2: the AFTER half of the refactor pair.
//
// Run with: go run 02_after_refactor.go
//
// Same job as 01_before_refactor.go — validate users, print a report — but
// idiomatic. Numbered FIX comments answer the SMELLs from the before file.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// FIX 2+3: a real type. Fields are typed (Age is an int), typos are compile
// errors, and behavior (Validate) lives WITH the data.
type User struct {
	Name  string
	Email string
	Age   int
}

// FIX 8: errors are VALUES the caller can inspect. Sentinel errors let
// callers use errors.Is; the messages say which field and why.
var (
	errNoName   = errors.New("name is empty")
	errBadEmail = errors.New("email is malformed")
	errBadAge   = errors.New("age is out of range")
)

// Validate returns nil for a good user. FIX 6: early returns keep every
// check flat — no pyramid; the happy path is the last line.
func (u User) Validate() error {
	if u.Name == "" {
		return errNoName
	}
	if !strings.Contains(u.Email, "@") {
		return errBadEmail
	}
	if u.Age <= 0 || u.Age > 150 {
		return errBadAge
	}
	return nil
}

// FIX 4+5: one small function per job, named for what it DOES. Report takes
// input as a parameter and returns its result — no globals, trivially
// testable: call it, compare the string.
//
// FIX 7: strings.Builder instead of += — O(n) instead of O(n²).
func Report(users []User) (report string, errCount int) {
	var b strings.Builder
	valid := 0
	for _, u := range users {
		if err := u.Validate(); err != nil {
			// Handle the error ONCE: count it here; detailed reporting is
			// the caller's decision (see main), not buried printf noise.
			errCount++
			continue // early continue — no else-ladders
		}
		valid++
		fmt.Fprintf(&b, "user: %s <%s>\n", u.Name, u.Email)
	}
	fmt.Fprintf(&b, "valid users: %d, errors: %d\n", valid, errCount)
	return b.String(), errCount
}

// FIX (accept interfaces): PrintInvalid writes to ANY io.Writer — stdout in
// production, a bytes.Buffer in tests. It accepts the small interface it
// needs, nothing more.
func PrintInvalid(w io.Writer, users []User) {
	for _, u := range users {
		if err := u.Validate(); err != nil {
			// %w-style wrapping matters when RETURNING errors; here we just
			// present them, with enough context to act on.
			fmt.Fprintf(w, "skipping %q: %v\n", u.Name, err)
		}
	}
}

func main() {
	// FIX 1+9: data flows through parameters and return values — you can
	// read main top-to-bottom and see exactly what feeds what.
	// FIX (zero value / no ceremony): a plain slice literal, preallocation
	// via make would matter only for big n.
	users := []User{
		{Name: "Ada", Email: "ada@example.com", Age: 36},
		{Name: "", Email: "nobody@example.com", Age: 50},    // invalid: no name
		{Name: "Bob", Email: "bob-at-example.com", Age: 41}, // invalid: bad email
		{Name: "Grace", Email: "grace@navy.mil", Age: 85},
	}

	// Diagnostics to stderr, data to stdout (the Unix contract, Module 13).
	PrintInvalid(os.Stderr, users)

	report, _ := Report(users)
	fmt.Print(report)

	// What we did NOT do (avoid premature abstraction):
	//   - no UserRepositoryInterface with one implementation,
	//   - no config struct with one field,
	//   - no generics for a function used with one type.
	// The second use case earns the abstraction; guessing at it costs now
	// and usually guesses wrong.
}
