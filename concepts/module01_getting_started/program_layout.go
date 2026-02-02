// program_layout.go — how a Go program is structured:
// package → file → top-level declarations (constants, variables, types,
// functions), and what "exported" means.
//
// Run it with:   go run program_layout.go
package main

import (
	"fmt"
	"strings"
)

// ---- Top-level declarations ------------------------------------------------
// Anything declared at the top level of a file is visible to EVERY file in
// the same package, in any order. Go does not need forward declarations:
// main() below happily calls describe(), which is defined after it.

// A top-level constant. By convention Go uses MixedCaps, not SCREAMING_CASE.
const tutorialName = "Go Concepts"

// A top-level ("package-level") variable. Unlike local variables, an unused
// package-level variable is allowed — but unused LOCAL variables are a
// compile error.
var moduleNumber = 1

// Language rule worth memorizing:
//   Uppercase first letter  -> EXPORTED  (visible when other packages import this one)
//   lowercase first letter  -> unexported (private to this package)
// That single rule replaces public/private/protected keywords entirely.

// Exported (if this were a library, importers could call it):
func Describe() string {
	return describe() // exported functions often wrap unexported helpers
}

// unexported helper — only code inside this package can call it.
func describe() string {
	// strings.Repeat builds "=====..." — small taste of the stdlib.
	line := strings.Repeat("=", 40)
	return line + "\n" +
		fmt.Sprintf("%s — module %d\n", tutorialName, moduleNumber) +
		line
}

// init() is a special optional function: it runs automatically BEFORE main,
// once per file that declares it. Use it sparingly (e.g. validating package
// state); overuse makes program startup hard to follow.
func init() {
	fmt.Println("[init] runs before main — package is being set up")
}

func main() {
	fmt.Println(Describe())

	fmt.Println(`Layout of a Go program:

  module  (go.mod)          the project; unit of versioning/dependencies
   └─ package               one directory = one package
       └─ files (*.go)      all files in the dir declare the same package
           └─ declarations  const / var / type / func — order irrelevant

  package main + func main() = an executable program.`)

	// Note the backquoted string above: a "raw string literal" keeps
	// newlines and needs no escaping — handy for multi-line text.
}
