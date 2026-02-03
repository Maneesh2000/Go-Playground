# Module 03 — Control Flow

## What you'll learn

- `if` / `else`, including the handy *init statement* form
- `for` — Go's **only** loop, in all its forms, including `range` over an
  int (new in Go 1.22)
- `switch`: expression switches, type-less (true) switches, `fallthrough`
- `defer`: LIFO order, and the classic gotcha of immediately-evaluated
  arguments
- Labels with `break`/`continue` for escaping nested loops
- `goto` — what it is, and why you'll (almost) never use it

## if — no parentheses, braces required

```go
if x > 10 {
    fmt.Println("big")
} else if x > 5 {
    fmt.Println("medium")
} else {
    fmt.Println("small")
}
```

- No parentheses around the condition; braces are **mandatory** (no
  single-statement bodies — this kills the classic dangling-else bug).
- The condition must be a real `bool`. `if 1 { ... }` does not compile;
  there is no truthiness in Go.

### The init statement

`if` can run a small statement before the condition. The variable it declares
is scoped to the `if`/`else` chain only:

```go
if v, err := strconv.Atoi(s); err == nil {
    fmt.Println("number:", v)   // v and err exist only here...
} else {
    fmt.Println("not a number") // ...and here
}
// v, err are GONE here — tight scoping, fewer stray variables
```

This pattern is everywhere in Go, especially with functions that return
`(value, error)` or `(value, ok)`.

## for — the only loop in Go

There is no `while` and no `do-while`. `for` covers everything:

```go
// 1) Classic three-part loop:
for i := 0; i < 5; i++ { ... }

// 2) Condition-only — this IS Go's "while":
for n > 1 { n /= 2 }

// 3) Infinite — loop forever until break/return:
for { ... }

// 4) range — iterate over collections (and more):
for i, v := range slice { ... }   // index, value
for k, v := range m { ... }       // map key, value
for i, r := range "héllo" { ... } // byte index, rune (decodes UTF-8!)

// 5) range over an int (Go 1.22+): 0, 1, ..., n-1
for i := range 5 { fmt.Println(i) }
for range 3 { fmt.Println("hi") } // don't even need the variable
```

Flow of the three-part form:

```
        init (once)
          |
          v
   +--> condition --false--> exit loop
   |      |
   |    true
   |      v
   |    body
   |      |
   |    post (i++)
   +------+
```

## switch — more flexible than you're used to

```go
switch day {
case "sat", "sun":          // multiple values per case
    fmt.Println("weekend")
default:
    fmt.Println("weekday")
}
```

Key differences from C-family switches:

- **No automatic fallthrough.** Each case breaks by itself. If you truly want
  to fall into the next case, write the `fallthrough` keyword explicitly (it
  runs the next case's body *unconditionally*, ignoring its condition).
- Cases can list several values, and can be **any expression**, not just
  constants.
- Like `if`, a switch can have an init statement:
  `switch x := f(); x { ... }`.

### The type-less switch (switch true)

Omit the expression and each case becomes a boolean condition — a cleaner
`if/else if` chain:

```go
switch {
case score >= 90:
    grade = "A"
case score >= 80:
    grade = "B"
default:
    grade = "F"
}
```

(There is also a *type switch* — `switch v := x.(type)` — covered with
interfaces in a later module.)

## defer — "run this when the function returns"

`defer` schedules a function call to run when the surrounding function
returns, however it returns (normal return, early return, even a panic).
It's Go's resource-cleanup mechanism:

```go
f, err := os.Open(path)
if err != nil { return err }
defer f.Close()            // close happens automatically at return
// ... use f freely, no cleanup bookkeeping below ...
```

Two rules you MUST internalize (both demonstrated in `defer_basics.go`):

1. **LIFO order.** Multiple defers run last-in, first-out — like unwinding a
   stack:

   ```
   defer A   ┐ registered 1st        runs 3rd ┐
   defer B   │ registered 2nd        runs 2nd │  (stack)
   defer C   ┘ registered 3rd        runs 1st ┘
   ```

2. **Arguments are evaluated IMMEDIATELY**, at the `defer` line — only the
   *call* is delayed:

   ```go
   i := 0
   defer fmt.Println("deferred i =", i) // captures 0, right now
   i = 99
   // at return, prints: deferred i = 0   (not 99!)
   ```

   To see the *final* value, defer a closure that reads the variable when it
   runs: `defer func() { fmt.Println(i) }()` — see Module 05.

Classic real-world gotcha: `defer` in a loop doesn't run per iteration — all
the defers pile up until the *function* exits (e.g. opening many files in a
loop and deferring `Close` can exhaust file descriptors).

## Labels: break / continue for nested loops

`break` and `continue` affect the innermost loop. To escape an outer loop,
label it:

```go
outer:
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if i*j > 2 {
            break outer      // leaves BOTH loops
        }
    }
}
```

`continue outer` similarly jumps to the next iteration of the labeled loop.
`break` also breaks out of `switch` and `select`; a label lets you break the
surrounding loop from inside a switch.

## goto — exists, discouraged

Go has `goto label` for jumps within a function. It cannot jump over variable
declarations or into blocks. You will essentially never need it: loops,
labeled break/continue, early returns, and defer cover real-world control
flow more clearly. You may spot it in generated code or hyper-optimized
stdlib internals — read it there, don't write it yourself.

## Run the examples

```
go run if_and_switch.go
go run loops.go
go run defer_basics.go
go run labels_and_goto.go
```

## Key takeaways

- Conditions are bare booleans, braces are mandatory, and the `if` init
  statement keeps error-handling variables tightly scoped.
- `for` is the only loop: three-part, while-style, infinite, `range` — and
  since Go 1.22, `range` over an int.
- `switch` doesn't fall through unless you say `fallthrough`; an
  expression-less `switch` replaces long `if/else` chains.
- `defer` runs LIFO at function return; its **arguments are evaluated at the
  defer statement**, not at run time.
- Labeled `break`/`continue` handle nested loops; `goto` is legal but leave
  it alone.

## Exercises

1. FizzBuzz with a twist: print 1–30 using `for i := range 30` (mind the
   off-by-one!) and a type-less `switch` instead of if/else.
2. Write a function that "opens" three resources (just print "open N") and
   defers a matching "close N" for each. Predict the output order before
   running — then verify.
3. Using nested loops and a labeled `break`, find the first pair `(i, j)`
   with `i < j < 20` such that `i*j == 91`, and stop searching immediately.
