# Module 05 — Functions

## What you'll learn

- Multiple return values and the `(value, error)` convention
- Named return values — what they do, and why to use them sparingly
- Variadic functions (`...T`) and spreading a slice with `s...`
- Functions as first-class values; function types and simple callbacks
- Closures: the counter example, and loop-variable capture (including the
  Go 1.22 per-iteration change)
- Recursion
- `defer` + closures: seeing final values, and modifying named results

## Multiple return values

Go functions return any number of values — no wrapper objects, no output
parameters:

```go
func divmod(a, b int) (int, int) {
    return a / b, a % b
}

q, r := divmod(17, 5)      // q=3, r=2
q, _ := divmod(17, 5)      // discard what you don't need with _
```

The most important use is Go's error convention — the *last* return value is
an `error`:

```go
func parse(s string) (int, error) { ... }

n, err := parse("42")
if err != nil {
    // handle it — this if is the heartbeat of Go code
}
```

You must do *something* with every returned value (use it or `_` it) —
silently dropping results doesn't compile when you assign, so errors are
hard to lose by accident.

## Named return values

You can name the results in the signature. The names become real variables,
zero-valued at entry; a bare `return` returns their current values:

```go
func split(sum int) (x, y int) {   // x, y declared here, start at 0
    x = sum * 4 / 9
    y = sum - x
    return                          // "naked return": returns x, y
}
```

**Use sparingly.** Good uses: documenting what each result *means*
(`(lat, long float64)`), and modifying a result in a deferred function (see
below). Bad use: naked returns in long functions — the reader has to scroll
up and mentally track the current values of the result variables. Rule of
thumb: name results when it helps the *reader*; almost always still write
explicit `return x, y`.

## Variadic functions

`...T` makes the final parameter accept any number of arguments; inside the
function it is simply a `[]T`:

```go
func sum(nums ...int) int {
    total := 0
    for _, n := range nums { total += n }   // nums is a []int
    return total
}

sum()            // 0 args — nums is an empty slice
sum(1, 2, 3)     // 3 args
s := []int{4, 5, 6}
sum(s...)        // spread an existing slice with ...
```

`fmt.Println(a ...any)` and `append(s, elems...)` are the variadics you use
every day.

## Functions are values

A function is a value like any other: it has a type, can be stored in a
variable, passed as an argument, and returned from another function:

```go
var op func(int, int) int      // a function TYPE: takes 2 ints, returns int
op = func(a, b int) int { return a + b }   // anonymous function (lambda)
fmt.Println(op(2, 3))          // 5

type Callback func(item string)            // named function types read better
func each(items []string, cb Callback) {
    for _, it := range items { cb(it) }
}
```

This is how Go does callbacks, strategies, and higher-order helpers
(`slices.SortFunc`, `http.HandlerFunc`, ...). No special syntax — functions
are just values.

## Closures

An anonymous function that references variables from its enclosing scope
*captures* them — it keeps a live reference, not a snapshot:

```go
func makeCounter() func() int {
    count := 0                 // lives on, owned by the closure
    return func() int {
        count++                // each call updates the SAME count
        return count
    }
}

next := makeCounter()
next() // 1
next() // 2 — count survives between calls
```

```
   makeCounter() returns ──> +-----------------+
                             |  closure        |
                             |  code: count++  |
                             |  env:  count ───┼──> 2   (heap-allocated;
                             +-----------------+         outlives makeCounter)
```

Each *call* to `makeCounter` creates a fresh, independent `count`.

### Loop-variable capture — and the Go 1.22 change

The historic gotcha: before Go 1.22, a `for` loop had **one** loop variable
reused across iterations, so every closure created in the loop captured *the
same variable* and saw its final value:

```go
for i := 0; i < 3; i++ {
    fns = append(fns, func() { fmt.Print(i) })
}
// Go ≤1.21: prints 3 3 3   (all closures share one i, now equal to 3)
// Go ≥1.22: prints 0 1 2   (each iteration gets its OWN i)
```

Go 1.22 changed the language: each iteration now has a fresh per-iteration
variable, so closures behave the way people always expected. You'll still
see `i := i` copies in older codebases — that was the pre-1.22 fix. Note the
new semantics apply to *iterations of a loop*; capturing a variable declared
*outside* the loop still shares it (sometimes that's exactly what you want —
that's the counter pattern).

## Recursion

Functions can call themselves; Go has no special syntax and **no tail-call
optimization**, so extremely deep recursion can exhaust the (growable)
goroutine stack — prefer loops for unbounded depth:

```go
func factorial(n int) int {
    if n <= 1 { return 1 }     // base case — always first!
    return n * factorial(n-1)  // recursive case, on a smaller problem
}
```

## defer + closures

Module 03 showed that deferred *arguments* are evaluated immediately. Defer
a **closure** instead and it reads variables when it *runs* (at return):

```go
i := 0
defer fmt.Println(i)             // prints 0 — argument captured now
defer func() { fmt.Println(i) }() // prints 99 — closure reads i at return
i = 99
```

Combined with **named returns**, a deferred closure can even *change* the
function's result on the way out — the standard trick for wrapping errors:

```go
func work() (err error) {
    defer func() {
        if err != nil { err = fmt.Errorf("work failed: %w", err) }
    }()
    ...
}
```

## Run the examples

```
go run returns.go
go run variadic.go
go run function_values.go
go run closures.go
go run recursion.go
```

## Key takeaways

- Multiple returns are the norm; the trailing `error` is the Go error
  convention.
- Named returns document results and enable defer-time modification — but
  prefer explicit `return` values; keep naked returns out of long functions.
- `...T` gathers args into a slice; `slice...` spreads one back out.
- Functions are first-class values — callbacks are just parameters with
  function types.
- Closures capture variables by reference; since Go 1.22 each loop iteration
  has its own variable, retiring the `i := i` idiom.
- A deferred closure sees (and, with named returns, can modify) final
  values; deferred *arguments* are frozen at the `defer` line.

## Exercises

1. Write `minMax(nums ...int) (min, max int, err error)` that returns an
   error when called with no arguments. Call it three ways: with literal
   arguments, by spreading a slice, and with no arguments.
2. Write `makeAccumulator(start float64) func(float64) float64` that returns
   a closure adding each deposit to a running balance. Create two
   independent accumulators and show they don't interfere.
3. Write `apply(nums []int, f func(int) int) []int` and use it with three
   different anonymous functions: double, square, negate. Then write a
   function `timed(f func()) ` that runs `f` and prints how long it took
   using `defer` and `time.Since`.
