# Module 08 — Error Handling

## What you'll learn

- Why Go treats **errors as values** (no exceptions, no try/catch)
- The `error` interface and creating errors with `errors.New` / `fmt.Errorf`
- Wrapping errors with `%w`, and inspecting them with `errors.Is` / `errors.As`
- Sentinel errors vs custom error types — when to use which
- Idiomatic style: check-and-return-early
- `panic` and `recover` — what they're for and when **not** to use them
- `defer` for cleanup that runs even on error paths

## Errors are values

Go has no exceptions. A function that can fail returns an error as its
**last return value**, and the caller checks it like any other value:

```go
f, err := os.Open("config.json")
if err != nil {
    return fmt.Errorf("loading config: %w", err) // handle or pass up
}
```

This looks repetitive at first, but it means every failure path is **visible
in the code** — nothing jumps over your function invisibly, and error
handling is ordinary programming (you can store errors, compare them, wrap
them, put them in slices...).

### The `error` interface

`error` is just a tiny built-in interface:

```go
type error interface {
    Error() string
}
```

Anything with an `Error() string` method is an error. That's it.

### Creating errors

```go
errors.New("connection refused")               // fixed message
fmt.Errorf("user %d not found", id)            // formatted message
fmt.Errorf("fetch user: %w", err)              // formatted AND wraps err
```

## Idiomatic style: early return

Handle the failure first and `return`; keep the happy path at minimal
indentation ("the happy path hugs the left margin"):

```go
func process(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("read %s: %w", path, err)
    }
    // happy path continues, un-indented
    ...
    return nil
}
```

No exceptions means: no invisible control flow, no wondering what might throw.

## Error wrapping

When you pass an error upward, **add context** and wrap it with `%w` so the
original remains inspectable:

```
   errors.Is(err, ErrNotFound) walks this chain:

   ┌────────────────────────────────────────────────┐
   │ "load profile: query user 42: user not found"  │  <- what gets printed
   └────────────────────────────────────────────────┘
   ┌──────────────────────┐
   │ "load profile: ..."  │  fmt.Errorf("load profile: %w", err)
   └─────────┬────────────┘
             │ wraps
   ┌─────────▼────────────┐
   │ "query user 42: ..." │  fmt.Errorf("query user %d: %w", id, err)
   └─────────┬────────────┘
             │ wraps
   ┌─────────▼────────────┐
   │ ErrNotFound          │  the sentinel at the root
   └──────────────────────┘
```

- `errors.Is(err, target)` — "is `target` anywhere in the chain?" (identity check; use for sentinels)
- `errors.As(err, &customErr)` — "is there a value of this **type** in the chain? If so, extract it."
- `%w` wraps; `%v` does **not** (it flattens the error into text and breaks `Is`/`As`).

## Sentinel errors vs custom error types

**Sentinel**: an exported, fixed error value callers compare against with `errors.Is`.

```go
var ErrNotFound = errors.New("not found")   // like io.EOF, sql.ErrNoRows
```

Use when callers only need to know **which** thing happened.

**Custom type**: a struct implementing `Error()`, extracted with `errors.As`.

```go
type ValidationError struct { Field, Reason string }
func (e *ValidationError) Error() string { ... }
```

Use when callers need **data** about the failure (which field? what limit?).

## panic and recover

`panic` crashes the program, unwinding the stack and running deferred calls
on the way. It is for **unrecoverable programmer bugs** (index out of range,
nil dereference, "impossible" state) — **not** for ordinary failures like
"file missing" or "bad user input". Those are errors.

`recover` (only useful inside a `defer`) stops an in-flight panic. Legitimate
uses are rare: mainly at a boundary that must survive misbehaving code, e.g. an
HTTP server keeping one crashed handler from killing the whole process — and
`net/http` already does that for you.

**Rules:** don't use panic as control flow; don't use recover to fake
try/catch; a library should return errors, not panic across its API.

## defer for cleanup

`defer` schedules a call to run when the function returns — **however** it
returns (normal, early error return, or panic). That makes it the tool for
cleanup: close files, unlock mutexes, roll back transactions.

```go
f, err := os.Open(path)
if err != nil {
    return err
}
defer f.Close() // runs on EVERY exit path below this line
```

Deferred calls run **LIFO** (last deferred, first run), and their arguments
are evaluated at `defer` time, not at run time.

## Run the examples

```sh
go run 01_errors_basics.go
go run 02_wrapping_is_as.go
go run 03_sentinel_vs_custom.go
go run 04_panic_recover_defer.go
```

## Key takeaways

- Errors are ordinary values; the last return value by convention.
- Check errors immediately; return early; keep the happy path left-aligned.
- Add context with `fmt.Errorf("doing x: %w", err)` — always `%w` to keep the chain.
- `errors.Is` for sentinel values, `errors.As` for typed errors with data.
- `panic` = bugs; `error` = expected failures. Don't build APIs on panic/recover.
- `defer` guarantees cleanup on every exit path; deferred calls run LIFO.

## Exercises

1. Write `parseAge(s string) (int, error)` that wraps the `strconv.Atoi` error
   with context, and rejects ages outside 0–150 with a custom `RangeError` type
   carrying the offending value. In `main`, use `errors.As` to print the value
   from a `RangeError`.
2. Create a sentinel `ErrEmptyCart` in a mini checkout function. Call it through
   two layers, each adding `%w` context, then prove `errors.Is(err, ErrEmptyCart)`
   still finds it — and show it stops working if one layer uses `%v` instead.
3. Write a function that opens two files (create them first with `os.CreateTemp`)
   and uses `defer` so both are closed no matter which step fails. Print messages
   in the deferred calls to observe LIFO ordering.
