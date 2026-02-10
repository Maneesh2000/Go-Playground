# Module 15 — Idiomatic Go & Best Practices

## What you'll learn

- How real Go projects are laid out (`cmd/`, `internal/` — and the honest
  truth about `pkg/`)
- Naming conventions: short names, `MixedCaps`, `-er` interfaces, package names
- Error-handling style, "accept interfaces, return structs", zero values,
  small functions and early returns
- The tooling belt: `gofmt`/`goimports`, `go vet`, `staticcheck`,
  `golangci-lint`, `go mod tidy`, `pprof`
- Performance habits: preallocating slices, `strings.Builder`, when *not* to
  use pointers
- A checklist of the classic pitfalls (nil maps, slice aliasing, goroutine
  leaks, shadowing, the interface-nil bug…)
- Production readiness: graceful shutdown, `slog`, config via environment
- A "Go proverbs"-style one-page summary

## Project layout — the honest version

There is **no official required layout**. The only names the toolchain treats
specially are `internal/` (packages importable only from within your module —
compiler-enforced) and `cmd/` by convention. A sane, boring layout:

```
myservice/
├── go.mod
├── cmd/
│   └── myservice/        # each subfolder = one binary
│       └── main.go       # tiny: parse config, wire deps, call run()
├── internal/
│   ├── server/           # HTTP handlers, routing
│   ├── store/            # persistence
│   └── billing/          # domain logic
└── README.md
```

The honest notes:

- **Small project? Start flat.** A `main.go` and a couple of files at the root
  is fine. Grow structure when pain appears, not before.
- **`internal/` is genuinely useful**: it keeps your API surface deliberate.
  Default new packages into it.
- **`pkg/` is contested.** Some large repos use `pkg/` for "public library
  code", but it adds a meaningless path segment and the Go team doesn't
  endorse it. If a package is meant to be imported by others, the repo root
  already does that job. You'll see `pkg/` in the wild (Kubernetes); you don't
  need to copy it.
- One package = one coherent idea. Don't create `utils`/`common`/`helpers`
  dumping grounds — name packages after what they *provide* (`retry`, `slug`),
  not what they contain.

## Naming

- **MixedCaps**, never snake_case: `maxRetries`, `ServeHTTP`, `userID` (not
  `UserId`). Exported = Capitalized — that *is* the access control.
- **Short names in small scopes**: `i`, `r`, `buf`, `srv` are idiomatic when
  the scope is a few lines. The wider the scope, the longer the name.
- **Interfaces**: one-method interfaces end in `-er` — `Reader`, `Writer`,
  `Stringer`, `Greeter`.
- **Packages**: short, lowercase, singular, no underscores: `http`, `store`,
  `slug`. Callers see `store.Open`, so don't stutter: `store.Store` is fine,
  `store.StoreConnection` is not; `NewStore` in package `store` should just be
  `New` (callers write `store.New()`).
- Getters drop `Get`: `u.Name()`, not `u.GetName()`.

## Style essentials

**Errors are values; handle them once.** Wrap with context and `%w`, return
early, don't log *and* return (pick one — usually return, log at the top):

```go
f, err := os.Open(path)
if err != nil {
    return fmt.Errorf("loading config: %w", err)
}
```

**Accept interfaces, return structs.** Take the *smallest* interface that
covers what you use (`io.Reader`, not `*os.File`); return your concrete type
so callers get its full API. Define interfaces where they're *consumed*, not
next to the implementation.

**Avoid premature abstraction.** Don't write an interface with one
implementation "for flexibility" — add the interface when the second
implementation (or the test fake) actually arrives. Duplication is cheaper
than the wrong abstraction.

**Make the zero value useful.** `var mu sync.Mutex`, `var b strings.Builder`,
`var buf bytes.Buffer` all work with no constructor. Design your types the
same way where possible — it deletes `New` functions and nil checks.

**Small functions, early returns.** Handle errors and edge cases first and
`return`; keep the happy path at minimum indentation, flowing down the left
edge. If you're three `if`s deep, extract a function.

## Tooling belt

| Tool | What it does | When |
|------|--------------|------|
| `gofmt` / `go fmt` | THE formatter — non-negotiable | on save |
| `goimports` | gofmt + fixes import lines | on save (editors: gopls does this) |
| `go vet` | catches real bugs (printf args, copied locks, unreachable code) | CI + before commit |
| `staticcheck` | hundreds of deeper correctness/simplification checks | CI |
| `golangci-lint` | runs many linters (incl. staticcheck) with one config | CI for teams |
| `go mod tidy` | syncs go.mod/go.sum with actual imports | after changing imports |
| `go test -race -cover` | tests + race detector + coverage | CI, always |
| `go test -bench . -benchmem` | benchmarks with allocation counts | when perf matters |
| `net/http/pprof` + `go tool pprof` | CPU/heap profiles of a live service | when perf REALLY matters |

Workflow for performance: **benchmark → profile → fix the top item → repeat.**
Never optimize what you haven't measured; `pprof`'s flame graph usually points
somewhere surprising.

## Performance notes (the 20% that matters)

- **Preallocate slices** when you know the size: `make([]T, 0, n)` — one
  allocation instead of log(n) grow-and-copies.
- **`strings.Builder`** for building strings in loops; `+=` is O(n²).
- **Don't reach for pointers "for speed".** Small structs are often *faster*
  copied than heap-allocated behind a pointer (escape analysis, GC pressure).
  Use pointers for shared mutation or genuinely large structs — measure before
  believing either way.
- Reuse buffers (`sync.Pool`) only after a profile proves allocation is the
  bottleneck.

## Common pitfalls checklist

Run through this list when code misbehaves — all demonstrated in
`03_pitfalls.go`:

- [ ] **Nil map write** — `var m map[string]int; m["k"] = 1` panics. Maps need
  `make`; nil *slices* are fine to append to.
- [ ] **Slice aliasing** — a sub-slice shares the backing array; writing
  through one is visible through the other; `append` may or may not detach.
- [ ] **Goroutine leaks** — a goroutine blocked forever on a channel no one
  reads. Every goroutine needs a guaranteed exit path (ctx, close, buffer).
- [ ] **Shadowing** — `x, err := ...` inside an `if`/`for` creates a NEW `x`,
  the outer one stays stale. `go vet`/linters catch some cases.
- [ ] **`time.Format` reference date** — `Format("YYYY-MM-DD")` prints
  literally `YYYY-MM-DD`. The layout must use `2006-01-02 15:04:05`.
- [ ] **Interface-nil bug** — an interface holding a nil *pointer* is NOT
  `== nil` (it has a type). Return literal `nil` for errors, never a typed nil.
- [ ] (Pre-1.22 only) loop-variable capture in closures — fixed by Go 1.22's
  per-iteration loop variables, but you'll still see `i := i` in old code.

## Production readiness

- **Graceful shutdown**: `signal.NotifyContext` + `srv.Shutdown(ctx)` +
  waiting for workers (Module 13 has the full pattern & diagram).
- **Structured logging**: `slog` with a JSON handler; log values as key/value
  attrs, add `slog.With("request_id", id)` context; never `fmt.Println` in a
  service.
- **Config via environment** (12-factor): read `PORT`, `DATABASE_URL` etc.
  from env vars with defaults; validate at startup and fail fast. Flags for
  tools, env for services, files only when config is genuinely complex.
- Set timeouts on everything: `http.Server{ReadTimeout, WriteTimeout}`,
  `http.Client{Timeout}`, `context.WithTimeout` around outbound calls.

## Go proverbs — one-page summary

Adapted from (and inspired by) Rob Pike's Go Proverbs:

```
Clear is better than clever.
The zero value is your friend — make it useful.
Errors are values; handle them, don't hide them.
Don't panic. (Return errors; panic only for impossible states.)
Accept interfaces, return structs.
The bigger the interface, the weaker the abstraction.
Don't communicate by sharing memory; share memory by communicating.
Concurrency is not parallelism.
Channels orchestrate; mutexes serialize.
A little copying is better than a little dependency.
gofmt's style is no one's favorite, yet gofmt is everyone's favorite.
Documentation is for users — write doc comments for every exported name.
Measure before you optimize; profile before you believe.
Start flat; grow structure only when it hurts.
When in doubt, do less.
```

## Run the examples

```bash
cd module15_best_practices

go run 01_before_refactor.go     # works, but ignores ~every guideline
go run 02_after_refactor.go      # same behavior, idiomatic shape
go run 03_pitfalls.go            # the classic traps, demonstrated safely

# then try the tools on them:
gofmt -l .                       # list files that need formatting (should be none)
go vet ./...
```

## Key takeaways

- Layout: start flat, use `internal/` liberally, treat `pkg/` as optional
  folklore; name packages for what they provide.
- Naming: MixedCaps, short names in short scopes, `-er` interfaces, no
  stuttering (`store.New`, not `store.NewStore`).
- Style: early returns, wrap errors with `%w`, accept interfaces / return
  structs, don't abstract until the second implementation exists.
- Tooling is the culture: `gofmt` + `go vet` + `staticcheck` + `-race` in CI
  settle debates and catch bugs before review.
- Performance: preallocate, use `strings.Builder`, and let `pprof` — not
  intuition — pick what to optimize.
- Know the pitfalls checklist; most "weird Go bugs" are on it.

## Exercises

1. Take `01_before_refactor.go` and refactor it **yourself** before reading
   `02_after_refactor.go` — then diff your instincts against ours. What did
   you split differently?
2. In `03_pitfalls.go`, un-comment the nil-map write and the deadlocking
   goroutine variants, and observe both failure modes. Then fix each one two
   different ways.
3. Add `net/http/pprof` to any earlier module's HTTP server example, generate
   load with a loop of requests, and capture a 5-second CPU profile with
   `go tool pprof`. Find the hottest function.
