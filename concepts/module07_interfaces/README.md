# Module 07 — Interfaces

## What you'll learn

- What an interface type is, and how Go's **implicit** satisfaction works (no `implements` keyword)
- The "small interfaces" philosophy (`io.Reader`, `io.Writer`, `fmt.Stringer`)
- What an interface value looks like under the hood (type + value pair)
- The classic bug: a nil interface vs an interface holding a nil pointer
- The empty interface / `any`, type assertions, and type switches
- Composing interfaces (`io.ReadWriter`)
- Practical patterns: a Shape calculator and a plug-in style Notifier

## What is an interface?

An **interface** defines behaviour as a set of method signatures — *what* a
thing can do, not *what it is*:

```go
type Shape interface {
    Area() float64
    Perimeter() float64
}
```

Any type that has those methods **automatically** satisfies the interface.
There is no `implements` declaration:

```go
type Circle struct{ Radius float64 }
func (c Circle) Area() float64      { return math.Pi * c.Radius * c.Radius }
func (c Circle) Perimeter() float64 { return 2 * math.Pi * c.Radius }

var s Shape = Circle{Radius: 2} // just works — Circle has the methods
```

### Why implicit satisfaction is powerful

- **Decoupling**: a package can define an interface for what it *needs*, and
  any existing type from anywhere — even one written years before, in a
  package that has never heard of your interface — satisfies it if the
  methods match.
- You can make **someone else's type** satisfy **your** interface without
  touching their code.
- It encourages defining interfaces **where they are used** (consumer side),
  not where types are defined.

### Small interfaces philosophy

Idiomatic Go interfaces are tiny — often a single method:

```go
type Reader interface { Read(p []byte) (n int, err error) }   // io.Reader
type Writer interface { Write(p []byte) (n int, err error) }  // io.Writer
type Stringer interface { String() string }                   // fmt.Stringer
```

> "The bigger the interface, the weaker the abstraction." — Rob Pike

A one-method interface is easy to satisfy, so it fits *many* types, so code
written against it is maximally reusable. Half the standard library plugs
together through `io.Reader`/`io.Writer`.

`fmt.Stringer` is special: if your type has a `String() string` method,
`fmt.Println` & friends automatically use it to print your value.

## Interface values under the hood

An interface value is a two-word pair: the **dynamic type** and the
**dynamic value**:

```
   var s Shape = Circle{Radius: 2}

   s (interface value)
   ┌─────────────────────┐
   │ type : main.Circle  │   which concrete type is inside?
   ├─────────────────────┤
   │ value: {Radius: 2}  │   the concrete data (or a pointer to it)
   └─────────────────────┘
```

Method calls on `s` dispatch dynamically: Go looks at the *type* slot to find
the right `Area` method, then calls it with the *value* slot.

### The nil trap (classic bug)

An interface is `== nil` **only when BOTH slots are nil**:

```
   var s Shape            // truly nil interface        s == nil  -> true
   ┌──────────────┐
   │ type : nil   │
   │ value: nil   │
   └──────────────┘

   var c *Circle          // nil *pointer*
   var s Shape = c        // interface holding a nil pointer
   ┌────────────────────┐
   │ type : *main.Circle│  <- type slot is SET
   │ value: nil         │
   └────────────────────┘                               s == nil  -> FALSE!
```

This bites hardest with errors: returning a typed nil pointer as an `error`
makes `err != nil` true even though the pointer is nil. Rule of thumb:
**return the interface type, and return a literal `nil` on success.**

## `any`, type assertions, type switches

`any` is an alias for `interface{}` — the empty interface, satisfied by every
type. It means "I know nothing about this value", so to *use* it you must get
the concrete type back out:

```go
var v any = "hello"

s := v.(string)        // type assertion — panics if wrong type
s, ok := v.(string)    // comma-ok form — safe, ok reports success

switch x := v.(type) { // type switch — branch per concrete type
case string:  fmt.Println("string:", x)
case int:     fmt.Println("int:", x)
default:      fmt.Println("something else")
}
```

Use `any` sparingly — it throws away compile-time type safety. Generics
(Module 09) often replace old `interface{}` code.

## Composing interfaces

Interfaces embed other interfaces to build bigger ones from small parts:

```go
type ReadWriter interface {
    Reader
    Writer
}
```

That's exactly how `io.ReadWriter`, `io.ReadCloser`, etc. are defined.

## Run the examples

```sh
go run 01_shapes.go
go run 02_interface_values_nil.go
go run 03_assertions_switches.go
go run 04_notifier_plugin.go
```

## Key takeaways

- Interfaces describe behaviour; satisfaction is implicit and automatic.
- Keep interfaces small; accept interfaces, return concrete types.
- An interface value = (dynamic type, dynamic value). It's nil only if *both* are nil.
- Never store a typed nil pointer in an interface you'll compare against nil.
- `any` accepts everything; type assertions/switches recover the concrete type.
- Compose big interfaces from small ones by embedding.

## Exercises

1. Add a `Triangle` type (three side lengths) to the shape calculator with
   `Area()` (use Heron's formula: `√(s(s-a)(s-b)(s-c))` where `s` is half the
   perimeter) and `Perimeter()`, plus a `String()` method. Confirm it works with
   the existing `describe` function unchanged.
2. Write a function `Sum(values []any) (int, error)` that adds up all `int` and
   `int64` elements using a type switch, and returns an error naming the first
   unsupported type it meets.
3. Reproduce the nil-interface bug: write `func find(id int) *Record` that returns
   nil, assign its result to a variable of interface type, and show that `!= nil`
   is true. Then fix the calling code.
