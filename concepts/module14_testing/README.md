# Module 14 — Testing

## What you'll learn

- How Go's built-in testing works: `_test.go` files, `go test`, `t.Errorf` vs `t.Fatalf`
- **Table-driven tests** — the canonical Go testing pattern
- Subtests with `t.Run`, helpers with `t.Helper`, fixtures with `t.TempDir`
- Benchmarks (`go test -bench`) — `b.N` today, `b.Loop` since Go 1.24
- **Fuzzing** (`go test -fuzz`) with a real bug-finding example
- Coverage: `-cover` and `-coverprofile` + the HTML report
- Testing through interfaces and fakes; testing HTTP handlers with `net/http/httptest`
- Running the race detector in tests (`go test -race`)

## ⚠️ This module runs differently

Every other module's examples are standalone `go run file.go` programs. **Test
files cannot be run with `go run`** — `go test` compiles the package together
with its `_test.go` files and runs the generated test binary. So this module is
a tiny real Go module (see `go.mod`, `module example.com/testingdemo`) with a
normal library package and its tests. You run it with `go test`, not `go run`
(see the Run section below).

## The basics

A test lives in a file ending in `_test.go`, in the same package, and is a
function `TestXxx(t *testing.T)`:

```go
// wordcount_test.go
func TestWordCount(t *testing.T) {
    got := WordCount("one two two")
    if got["two"] != 2 {
        t.Errorf("WordCount: got %d for %q, want 2", got["two"], "two")
    }
}
```

- `t.Errorf` — report a failure and **keep going** (see all failures at once).
- `t.Fatalf` — report and **stop this test now** (use when continuing is
  pointless, e.g. a setup step failed).
- No assertion library needed: `if got != want { t.Errorf(...) }` is idiomatic.
  The convention is to print **got before want**.

## Table-driven tests

The canonical pattern: a slice of cases, one loop, `t.Run` per case. Adding a
case is one line; each case gets its own name, pass/fail status, and can be run
alone with `go test -run 'TestSlugify/spaces'`.

```go
func TestSlugify(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  string
    }{
        {"simple", "Hello", "hello"},
        {"spaces", "Hello World", "hello-world"},
        {"punctuation", "Go: 100% fun!", "go-100-fun"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Slugify(tt.input); got != tt.want {
                t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

## Helpers and fixtures

- Mark shared assertion funcs with `t.Helper()` — failures are then reported at
  the **caller's** line, not inside the helper.
- `t.TempDir()` returns a fresh directory, automatically deleted when the test
  finishes — perfect for file-based fixtures. No cleanup code needed.
- `t.Cleanup(func(){...})` registers teardown that runs even if the test fails.

## Benchmarks

```go
func BenchmarkSlugify(b *testing.B) {
    for i := 0; i < b.N; i++ {   // the framework grows b.N until timing is stable
        Slugify("Hello, Benchmark World!")
    }
}
```

Run with `go test -bench=. -benchmem`. Since **Go 1.24** the preferred form is
`for b.Loop() { ... }` — it prevents the compiler from optimizing the
benchmarked call away and keeps setup out of the timing. We use `b.N` in the
code so it compiles on Go 1.22+; switch to `b.Loop` once you're on 1.24+.

## Fuzzing

Fuzzing feeds your function randomly-mutated inputs, hunting for panics and
property violations:

```go
func FuzzReverse(f *testing.F) {
    f.Add("hello")                    // seed corpus
    f.Fuzz(func(t *testing.T, s string) {
        if Reverse(Reverse(s)) != s { // a property that must ALWAYS hold
            t.Errorf("double reverse of %q changed it", s)
        }
    })
}
```

`go test -fuzz=FuzzReverse -fuzztime=10s` mutates inputs until it finds a
failure (crashing inputs are saved to `testdata/fuzz/` and replayed by plain
`go test` forever after). Our example hides a real bug: a byte-wise `Reverse`
corrupts multi-byte UTF-8 — the fuzzer finds it in milliseconds.

## Coverage

```bash
go test -cover                       # percentage per package
go test -coverprofile=cover.out      # write a profile...
go tool cover -html=cover.out        # ...and browse untested lines in red
```

Treat coverage as a flashlight, not a target: 100% coverage proves the code
*ran*, not that the assertions were meaningful.

## Fakes and httptest

Design for testability by **accepting interfaces**. Our handler depends on a
tiny `Greeter` interface; production wires in the real implementation, tests
wire in a fake — no network, no globals:

```go
type Greeter interface { Greet(name string) (string, error) }
```

`net/http/httptest` tests handlers without opening sockets:

```go
req := httptest.NewRequest("GET", "/greet/Ada", nil)
rec := httptest.NewRecorder()          // an in-memory http.ResponseWriter
handler.ServeHTTP(rec, req)
// assert on rec.Code and rec.Body
```

`httptest.NewServer(handler)` goes one step further: a real server on a random
localhost port — ideal for testing HTTP *clients*.

## The race detector in tests

```bash
go test -race ./...
```

Any test that exercises concurrent code should run under `-race` in CI. The
test suite here includes a concurrent `SafeCounter` test that is only proven
correct because `-race` says so.

## Run the examples

```bash
cd module14_testing        # NOTE: go test, not go run (see the warning above)

go test ./...                        # run everything
go test -v ./...                     # verbose: see every subtest
go test -run 'TestSlugify/spaces' .  # one specific subtest
go test -race ./...                  # with the race detector
go test -cover ./...                 # coverage summary
go test -bench=. -benchmem .         # benchmarks
go test -fuzz=FuzzReverse -fuzztime=10s .   # fuzz (finds the seeded bug's cousins)
```

## Key takeaways

- Tests are plain Go in `_test.go` files; `got != want → t.Errorf` beats any
  assertion DSL.
- Default to table-driven tests with named subtests; `-run name/subname`
  re-runs one case.
- `t.Helper`, `t.TempDir`, `t.Cleanup` remove almost all fixture boilerplate.
- Benchmarks: `b.N` loop (or `b.Loop` on Go 1.24+); always add `-benchmem`.
- Fuzz properties ("round-trips", "never panics"), not examples.
- Accept interfaces so tests can inject fakes; use `httptest` for handlers and
  clients; run `-race` in CI, always.

## Exercises

1. Add a `MostCommon(text string) (word string, n int)` function to
   `wordcount.go` and write a table-driven test covering: empty input, a tie,
   and mixed case. Aim for 100% coverage of the new function (`-coverprofile`).
2. Write `BenchmarkWordCount` for inputs of 10, 1,000 and 100,000 words (use
   `strings.Repeat` and subtests via `b.Run`). Which part scales worst?
3. Add a `POST /greet` route to the handler that reads `{"name": "..."}` JSON,
   and test it with `httptest.NewRecorder`, including the malformed-JSON case
   (expect 400).
