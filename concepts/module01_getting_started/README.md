# Module 01 — Getting Started with Go

## What you'll learn

- What Go is and why it exists (compiled, garbage-collected, built for concurrency)
- How to install Go and verify your setup
- The core Go toolchain: `go run`, `go build`, `go fmt`, `go vet`, `go test`, `go mod`
- Anatomy of a "Hello, World" program: `package`, `import`, `func main`
- Go modules: `go.mod`, `go mod init`, `go mod tidy`
- How a Go program is laid out: package → files → functions
- A one-paragraph history lesson: GOPATH vs modules

## What is Go, and why?

Go (often called Golang) is a programming language created at Google in 2009 by
Robert Griesemer, Rob Pike, and Ken Thompson. It was designed to make building
large, reliable server software *simple and fast*. Three properties define it:

1. **Compiled.** Go source code compiles ahead of time to a single, statically
   linked native binary. There is no interpreter and no virtual machine to
   install on the target machine — you copy one file and run it. Compilation
   is famously fast, so the edit-compile-run loop feels like a scripting
   language.

2. **Garbage-collected.** You allocate memory freely (`new`, `make`, composite
   literals) and the runtime reclaims it automatically. No `malloc`/`free`, no
   ownership annotations. Go's GC is optimized for low pause times, which
   matters for servers.

3. **Built for concurrency.** Goroutines (extremely cheap threads managed by
   the Go runtime) and channels (typed pipes between goroutines) are language
   primitives, not a library bolted on later. Spinning up 100,000 concurrent
   tasks is normal in Go. (Modules 08+ cover this in depth.)

Other deliberate design choices: a small language spec you can read in an
afternoon, one standard code format (`gofmt` ends style debates), fast builds,
and a rich standard library (HTTP servers, JSON, crypto, testing — all built in).

## Installing Go

Download the installer for your OS from <https://go.dev/dl/> or use a package
manager:

```
# macOS (Homebrew)
brew install go

# Linux (example: download & unpack the official tarball)
tar -C /usr/local -xzf go1.22.x.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

Verify:

```
go version        # e.g. "go version go1.22.4 darwin/arm64"
go env GOOS GOARCH  # your target OS and CPU architecture
```

## The Go toolchain — one `go` command to rule them all

Everything is a subcommand of `go`:

| Command        | What it does                                                        |
|----------------|---------------------------------------------------------------------|
| `go run x.go`  | Compile **and** run in one step (binary is temporary). Great for learning. |
| `go build`     | Compile the current package into a binary in the current directory. |
| `go fmt`       | Rewrite source files into the one true Go style. Non-negotiable, and wonderful. |
| `go vet`       | Static analysis: catches suspicious code (wrong Printf verbs, unreachable code, ...). |
| `go test`      | Run tests in `_test.go` files (Module on testing covers this).      |
| `go mod init`  | Start a new module (creates `go.mod`).                              |
| `go mod tidy`  | Add missing / remove unused dependencies in `go.mod`.               |
| `go doc fmt.Println` | Show documentation for any symbol from the terminal.          |

## The compile/run pipeline

```
+-------------+     go build      +-----------------+
| .go source  | ----------------> |  Go compiler    |
| files       |   (parse, type-   |  + linker       |
| (package)   |    check, compile)|                 |
+-------------+                   +--------+--------+
                                           |
                                           v
                                  +-----------------+     ./myprog
                                  | static native   | --------------> runs directly,
                                  | binary (single  |                 no VM, no deps
                                  | file, includes  |                 to install
                                  | the Go runtime) |
                                  +-----------------+

  go run  =  go build (to a temp dir)  +  execute  +  clean up
```

The binary embeds the Go runtime (scheduler, garbage collector), which is why
even "Hello, World" is a couple of megabytes — and why deployment is trivial.

## Hello, World — line by line

```go
package main            // every .go file belongs to a package; "main" = executable

import "fmt"            // bring in the standard library's formatting package

func main() {           // execution starts at main.main — exactly one per program
    fmt.Println("Hello, World!")
}
```

- `package main` tells the compiler "build an executable, not a library".
- `import "fmt"` makes the `fmt` package available. Unused imports are a
  **compile error** in Go — the compiler keeps your code tidy by force.
- `func main()` takes no arguments and returns nothing. Command-line args live
  in `os.Args`; exit codes are set with `os.Exit(n)`.

## Go modules — `go.mod`, `go mod init`, `go mod tidy`

A **module** is a versioned collection of packages — the unit of dependency
management. You create one per project:

```
mkdir myproject && cd myproject
go mod init example.com/myproject     # creates go.mod
```

`go.mod` records your module's name, the Go version, and your dependencies:

```
module example.com/myproject

go 1.22

require github.com/some/dependency v1.2.3
```

When you import a third-party package in code, run `go mod tidy`: it downloads
what you use, records exact versions in `go.mod` (and hashes in `go.sum`), and
removes anything you stopped using. Builds are reproducible by default.

> Note: for the single-file examples in this tutorial, `go run file.go` works
> without a module — modules matter as soon as a project has multiple packages
> or third-party dependencies.

## How a Go program is laid out

```
module (go.mod)                      one repo/project, versioned
 └── package                         one directory = one package
      └── files (*.go)               all files in a dir share the package name
           └── functions, types,     top-level declarations; order between
               constants, variables  files doesn't matter
```

- A **package** is a directory of `.go` files that all start with the same
  `package` clause. The compiler treats them as one unit — a function in
  `a.go` can call one in `b.go` with no import.
- Names starting with an **Uppercase letter are exported** (visible to other
  packages); lowercase names are private to the package. That's the entire
  visibility system — no `public`/`private` keywords.
- An executable program is simply a module containing a `package main` with a
  `func main()`.

### GOPATH vs modules (the one-paragraph history)

Before Go 1.11 (2018), all Go code had to live inside a single workspace
directory tree called `GOPATH` (e.g. `~/go/src/github.com/you/project`), and
dependencies were fetched un-versioned from their repos' latest commit — which
made reproducible builds painful and spawned third-party vendoring tools.
Modules replaced all of that: your project can live anywhere on disk, `go.mod`
pins exact dependency versions, and `GOPATH` today survives only as the default
location for the download cache (`~/go/pkg/mod`) and installed binaries
(`~/go/bin`). If you see tutorials telling you to put code in `$GOPATH/src`,
they are outdated — just use modules.

## Run the examples

From this directory:

```
go run hello.go
go run toolchain_tour.go
go run program_layout.go
```

## Key takeaways

- Go compiles to a single static binary: fast builds, trivial deployment.
- The `go` command is the whole toolchain: `run`, `build`, `fmt`, `vet`,
  `test`, `mod`.
- Every file declares a `package`; `package main` + `func main()` = executable.
- Uppercase = exported, lowercase = package-private.
- Modules (`go.mod`) are how dependencies are declared and versioned; GOPATH is
  history.
- Unused imports and unused local variables are compile errors — Go enforces
  hygiene.

## Exercises

1. Create a new directory, run `go mod init example.com/greet`, and write a
   program that prints a greeting followed by the current time (hint:
   `time.Now()` from the `time` package). Build it with `go build` and run the
   produced binary directly.
2. Take `hello.go`, deliberately mis-indent it and add an unused import, then
   run `go fmt hello.go` and `go vet hello.go`. Observe what each tool fixes or
   reports.
3. Modify `hello.go` to print each element of `os.Args` on its own line, then
   run it with `go run hello.go one two three`. What is `os.Args[0]`?
