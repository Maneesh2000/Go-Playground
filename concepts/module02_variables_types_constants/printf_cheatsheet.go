// printf_cheatsheet.go — a runnable fmt.Printf reference. Run it, read the
// output next to the code, and keep it around as a lookup table.
//
// Run it with:   go run printf_cheatsheet.go
package main

import "fmt"

// A tiny struct so we can show %v vs %+v vs %#v on something structured.
type user struct {
	Name string
	Age  int
}

func main() {
	u := user{Name: "Ada", Age: 36}
	n := 42
	pi := 3.14159265
	s := "hi\tthere"

	// ---- The universal verbs -------------------------------------------
	fmt.Printf("%%v   default:            %v\n", u)  // {Ada 36}
	fmt.Printf("%%+v  with field names:   %+v\n", u) // {Name:Ada Age:36}
	fmt.Printf("%%#v  Go syntax:          %#v\n", u) // main.user{Name:"Ada", Age:36}
	fmt.Printf("%%T   type of the value:  %T\n", u)  // main.user
	fmt.Printf("%%T   works on anything:  %T %T\n", n, pi)

	fmt.Println()

	// ---- Integers --------------------------------------------------------
	fmt.Printf("%%d  decimal:       %d\n", n)
	fmt.Printf("%%b  binary:        %b\n", n)
	fmt.Printf("%%o  octal:         %o\n", n)
	fmt.Printf("%%x  hex (lower):   %x\n", 255)
	fmt.Printf("%%X  hex (upper):   %X\n", 255)
	fmt.Printf("%%c  as character:  %c\n", 19990) // code point 19990 = 丗
	fmt.Printf("%%U  Unicode form:  %U\n", '世')

	fmt.Println()

	// ---- Floats -----------------------------------------------------------
	fmt.Printf("%%f    fixed point:     %f\n", pi)   // 3.141593 (6 places default)
	fmt.Printf("%%.2f  two decimals:    %.2f\n", pi) // 3.14
	fmt.Printf("%%e    scientific:      %e\n", pi)
	fmt.Printf("%%g    compact/auto:    %g\n", pi)

	fmt.Println()

	// ---- Strings and booleans ---------------------------------------------
	fmt.Printf("%%s  plain string:   %s\n", s) // the \t prints as a real tab
	fmt.Printf("%%q  quoted+escaped: %q\n", s) // "hi\tthere" — great for debugging!
	fmt.Printf("%%t  boolean:        %t\n", true)
	fmt.Printf("%%p  pointer:        %p\n", &n) // address of n (varies per run)

	fmt.Println()

	// ---- Width, alignment, padding -----------------------------------------
	// %6d   : right-align in 6 columns      %-6d : left-align
	// %06d  : pad with zeros                %8.2f: width 8, 2 decimals
	fmt.Printf("|%6d|%-6d|%06d|\n", 42, 42, 42)
	fmt.Printf("|%8.2f|\n", pi)
	// Argument index: %[1]d reuses argument 1
	fmt.Printf("%[1]d in hex is %[1]x\n", 255)
	// A literal percent sign is %%:
	fmt.Printf("progress: 100%%\n")

	fmt.Println()

	// ---- The Printf family --------------------------------------------------
	// Printf  -> writes to stdout
	// Sprintf -> RETURNS the string instead of printing (very common)
	// Errorf  -> creates an error value (module on errors)
	msg := fmt.Sprintf("user %s is %d", u.Name, u.Age)
	fmt.Println("Sprintf gave us:", msg)

	// Mismatched verb/argument? go vet catches it, and at runtime Go
	// prints a loud placeholder instead of corrupting memory:
	fmt.Printf("oops: %d\n", "not a number") // -> %!d(string=not a number)
}
