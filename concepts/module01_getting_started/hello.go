// hello.go — the classic first program, explained line by line.
//
// Run it with:   go run hello.go
//
// "go run" compiles this file to a temporary binary, executes it, and
// deletes the binary afterwards. Use "go build" when you want to keep
// the executable.

// Every Go file starts with a package clause. The special package name
// "main" tells the compiler: "this is an executable program, not a library".
package main

// The import block lists other packages this file uses.
// "fmt" (short for "format") is the standard library package for
// formatted input/output — think printf/println.
//
// Important Go rule: importing a package and NOT using it is a
// compile-time ERROR, not a warning. The compiler keeps imports honest.
import "fmt"

// func main is the entry point. The runtime calls exactly this function
// when the program starts. It takes no parameters and returns nothing:
//   - command-line arguments live in os.Args (see exercise 3)
//   - the exit code is 0 unless you call os.Exit(n) or the program panics
func main() {
	// fmt.Println writes its arguments to standard output, separated by
	// spaces, followed by a newline.
	fmt.Println("Hello, World!")

	// Println accepts any number of values of any type. Go figures out
	// how to print each one — here a string, a number, and a boolean.
	fmt.Println("Go says:", 42, "is the answer:", true)

	// fmt.Printf gives C-style formatted output. %s = string, %d = decimal
	// integer, \n = newline. (Module 02 has a full cheat-sheet of verbs.)
	fmt.Printf("%s was first released in %d.\n", "Go", 2012)
}
