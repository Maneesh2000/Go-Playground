// toolchain_tour.go — a program that talks about the toolchain that built it.
//
// Run it with:   go run toolchain_tour.go
//
// It uses the "runtime" package to show information about the Go toolchain
// and the machine it is running on, and demonstrates multiple imports.
package main

// When importing more than one package, use a parenthesized import block.
// go fmt will sort these alphabetically for you — you never argue about
// import order in Go, the tool decides.
import (
	"fmt"     // formatted printing
	"os"      // operating-system helpers: Args, Exit, environment, files
	"runtime" // information about the Go runtime and build target
)

func main() {
	// runtime.Version reports the Go toolchain version that compiled
	// this binary — the version is baked in at compile time.
	fmt.Println("Compiled with:", runtime.Version())

	// GOOS and GOARCH are the target operating system and CPU
	// architecture. Cross-compiling in Go is just:
	//   GOOS=linux GOARCH=amd64 go build
	// ...and you get a Linux binary from your Mac or Windows machine.
	fmt.Println("Target OS/arch:", runtime.GOOS+"/"+runtime.GOARCH)

	// NumCPU reports how many logical CPUs are available. The Go
	// scheduler spreads goroutines across all of them by default.
	fmt.Println("Logical CPUs available:", runtime.NumCPU())

	// NumGoroutine shows how many goroutines are alive right now.
	// Even "single-threaded" programs have at least one: main itself.
	// (The runtime may run a few housekeeping goroutines too.)
	fmt.Println("Goroutines running:", runtime.NumGoroutine())

	// os.Args holds the command-line arguments. Args[0] is the path of
	// the executable itself (a temp path under "go run"!), and the rest
	// are the arguments the user passed.
	fmt.Println("Program invoked as:", os.Args[0])
	if len(os.Args) > 1 {
		fmt.Println("Extra arguments:", os.Args[1:])
	} else {
		fmt.Println("Extra arguments: (none — try: go run toolchain_tour.go a b c)")
	}

	// Toolchain cheat-sheet, printed so it's in front of you:
	fmt.Println()
	fmt.Println("Toolchain reminders:")
	fmt.Println("  go run  file.go   compile + run in one step")
	fmt.Println("  go build          compile to a binary in this directory")
	fmt.Println("  go fmt  ./...     format all code (./... = this dir and below)")
	fmt.Println("  go vet  ./...     static analysis for suspicious code")
	fmt.Println("  go test ./...     run tests")
	fmt.Println("  go mod tidy       sync go.mod with your imports")
}
