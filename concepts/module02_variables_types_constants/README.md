# Module 02 — Variables, Types, and Constants

## What you'll learn

- Declaring variables: `var`, short declaration `:=`, and zero values
- Go's basic types: sized ints/uints, floats, bool, string, byte, rune, complex
- Why Go has **no implicit conversions** — and how explicit conversion works
- Constants and `iota`, including typed vs untyped constants
- A `fmt.Printf` verbs cheat-sheet you'll use forever
- How `int`, `string`, and `rune` relate to bytes in memory

## Declaring variables

Go gives you two main ways to declare a variable:

```go
var name string = "Ada"   // full form: var name type = value
var name = "Ada"          // type inferred from the value
var count int             // no value → gets the ZERO VALUE (0 here)

name := "Ada"             // short form: declare + assign, type inferred
```

Rules of thumb:

- `:=` only works **inside functions** (not at package level) and only for
  *new* variables (at least one on the left must be new).
- Use `var` when you want the zero value, an explicit type, or a
  package-level variable. Use `:=` for everything else — it's what you'll
  see in most Go code.
- Unused **local** variables are a compile error. Assign to the blank
  identifier `_` to deliberately discard a value.

## Zero values — no uninitialized variables, ever

Every type has a well-defined zero value. Declaring without initializing is
safe:

| Type                        | Zero value |
|-----------------------------|------------|
| numeric (int, float, ...)   | `0`        |
| bool                        | `false`    |
| string                      | `""`       |
| pointers, slices, maps, funcs, interfaces, channels | `nil` |

This is a real design feature: types are often built so their zero value is
immediately useful ("make the zero value useful" is a Go proverb).

## The basic types

```go
// Signed integers — the number is the bit width:
int8  int16  int32  int64
// Unsigned:
uint8 uint16 uint32 uint64  uintptr
// Machine-dependent (64-bit on modern machines) — YOUR DEFAULT CHOICE:
int   uint

// Floating point (there is no "float" — you must pick):
float32 float64          // default to float64; it's what literals become

// Complex numbers (rare, but built in):
complex64 complex128

bool                     // true / false — no truthy/falsy values!
string                   // immutable sequence of BYTES (usually UTF-8 text)

byte  // alias for uint8  — "this is raw data / one byte"
rune  // alias for int32  — "this is one Unicode code point"
```

Notes:

- Use plain `int` unless you have a specific reason (binary formats,
  huge arrays, interop). Don't micro-optimize with `int8` etc.
- Integer overflow wraps around silently for sized types — know your ranges.
- `byte` and `rune` are aliases, not distinct types: they exist to signal
  *intent* (bytes vs characters).

## No implicit conversions — Go makes you say it

Coming from C, Java, or Python this is the biggest surprise:

```go
var i int = 65
var f float64 = i        // COMPILE ERROR — no implicit conversion
var f float64 = float64(i)  // OK: explicit conversion T(value)

var a int32 = 1
var b int64 = 2
c := a + b               // COMPILE ERROR — even int32 + int64 won't mix
c := int64(a) + b        // OK
```

Every numeric conversion is spelled out as `T(v)`. This eliminates a whole
class of subtle bugs (silent truncation, sign surprises) at the cost of a few
keystrokes. Conversions can lose information (`int8(300)` wraps; `int(3.9)`
truncates toward zero) — the compiler trusts you once you've written it
explicitly.

`string(bytes)` and `[]byte(s)` convert between strings and byte slices
(copying the data). Careful: `string(65)` is `"A"` (code point → string),
**not** `"65"` — use `strconv.Itoa` for number→text.

## Constants and iota

Constants are created with `const` and must be computable at **compile time**:

```go
const Pi = 3.14159
const Greeting = "hello"
```

### Untyped vs typed constants

An untyped constant (`const x = 3`) is a *pure value* with arbitrary
precision. It doesn't commit to a type until it's used, and then it adapts:

```go
const big = 1 << 40         // fine as a constant, even on 32-bit systems
var f float64 = 3           // untyped constant 3 becomes float64 — OK!
var i int = 3               // the same 3 becomes int — OK!

const typed int = 3         // TYPED constant: now it's an int, period
var g float64 = typed       // COMPILE ERROR — typed constants don't bend
```

This is why numeric literals feel flexible even though variables are strict:
literals are untyped constants.

### iota — auto-incrementing enumerator

Inside a `const` block, `iota` counts declaration lines starting at 0. It's
Go's answer to enums:

```go
type Weekday int

const (
    Sunday Weekday = iota  // 0
    Monday                 // 1 (the expression repeats implicitly)
    Tuesday                // 2
    // ...
)

const (
    _  = iota             // skip 0
    KB = 1 << (10 * iota) // 1 << 10 = 1024
    MB                    // 1 << 20
    GB                    // 1 << 30
)
```

## How int, string, and rune relate to memory

```
var n int = 5              one machine word (8 bytes on 64-bit):
                           +---------------------------------+
                       n:  | 00 00 00 00 00 00 00 05         |
                           +---------------------------------+

s := "héllo"               a string is a 2-word HEADER pointing at
                           immutable bytes (UTF-8 encoded):

        s (header)                 underlying bytes (read-only)
   +----------------+         +----+----+----+----+----+----+
   | ptr  ──────────┼───────> | 68 | c3 | a9 | 6c | 6c | 6f |
   | len = 6        |         +----+----+----+----+----+----+
   +----------------+           'h' [ 'é' = 2B ] 'l'  'l'  'o'

   len(s) == 6  ← BYTES, not characters!
   'é' occupies two bytes (0xC3 0xA9) in UTF-8.

r := 'é'                   a rune is just an int32 holding the Unicode
                           code point U+00E9 = 233:
                           +-------------------+
                       r:  | 00 00 00 e9       |  (4 bytes)
                           +-------------------+
```

So: `string` = bytes on the wire (UTF-8), `rune` = one decoded character
(code point), `byte` = one raw byte. Module 04 shows how to iterate strings
correctly with this in mind.

## fmt.Printf cheat-sheet

| Verb  | Meaning                              | Example output          |
|-------|--------------------------------------|-------------------------|
| `%v`  | default format of any value          | `{Ada 36}`              |
| `%+v` | like %v, adds struct field names     | `{Name:Ada Age:36}`     |
| `%#v` | Go-syntax representation             | `main.User{Name:"Ada"}` |
| `%T`  | the TYPE of the value                | `main.User`, `int`      |
| `%d`  | integer, base 10                     | `42`                    |
| `%b` / `%o` / `%x` / `%X` | base 2 / 8 / 16      | `101010`, `2a`          |
| `%c`  | character for a code point           | `A`                     |
| `%q`  | quoted string/char, escaped          | `"hi\n"`, `'A'`         |
| `%f`  | float, decimal (`%.2f` = 2 places)   | `3.141593`, `3.14`      |
| `%e` / `%g` | scientific / shortest float    | `3.14e+00`              |
| `%t`  | boolean                              | `true`                  |
| `%s`  | string (or anything with String())   | `hello`                 |
| `%p`  | pointer address                      | `0xc000014078`          |
| `%%`  | a literal percent sign               | `%`                     |

Width/precision: `%6d` (pad to 6), `%-6d` (left-align), `%08.3f` (zero-pad).
Tip: `go vet` catches mismatched verbs and arguments — run it.

## Run the examples

```
go run variables_and_zero_values.go
go run types_and_conversions.go
go run constants_and_iota.go
go run printf_cheatsheet.go
```

## Key takeaways

- `var` for zero values and package level; `:=` for everyday declarations
  inside functions.
- Every type has a zero value; there is no "uninitialized" in Go.
- Pick `int` and `float64` by default; `byte`/`rune` are aliases that signal
  intent.
- **No implicit numeric conversions** — write `T(v)` explicitly, always.
- Untyped constants are flexible, typed constants are strict; `iota` builds
  enums.
- `len(string)` counts bytes; a rune is a code point (int32).

## Exercises

1. Declare an `int8` with value 127 and add 1 to it (via a variable, since
   the compiler rejects constant overflow). Print the result and explain it.
2. Build a `const` block using `iota` for file permission bits: `Execute = 1`,
   `Write = 2`, `Read = 4`. Print an octal (`%o`) representation of
   `Read|Write`.
3. Given `price := 19.999`, print it as `$20.00` using a single `Printf`
   call. Then print the *type* of `price` without naming it yourself.
