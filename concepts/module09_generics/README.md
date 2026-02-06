# Module 09 — Generics

## What you'll learn

- The problem generics solve (pre-1.18 copy-paste and `interface{}` pain)
- Type parameter syntax: `func F[T any](...)`
- Constraints: `any`, `comparable`, custom constraint interfaces, unions with `~`
- Generic functions: `Map`, `Filter`, `Reduce`
- Generic types: a type-safe `Stack[T]`
- Type inference — why you rarely write the brackets at call sites
- The standard library's generic packages: `cmp` and `slices`
- When **not** to use generics

## The problem

Before Go 1.18 you had two bad options for "the same logic over different types":

**Option 1 — copy-paste:**

```go
func SumInts(xs []int) int          { ... }
func SumFloats(xs []float64) float64 { ... } // identical body, different type
```

**Option 2 — `interface{}`:** one function, but you lose type safety and pay
for runtime assertions:

```go
func Sum(xs []interface{}) interface{} { ... } // caller must assert, can panic
```

Generics give you **one implementation, checked at compile time**:

```go
func Sum[T int | float64](xs []T) T {
    var total T
    for _, x := range xs {
        total += x
    }
    return total
}
```

## Syntax: type parameters

Type parameters go in **square brackets** before the regular parameters:

```go
func Map[T, U any](xs []T, f func(T) U) []U
//      └────┬───┘
//   type parameter list: names + their constraints
```

- `T` and `U` are placeholders for types, decided at each call site.
- After the name, each parameter has a **constraint** saying what types are allowed.

## Constraints

A constraint is an interface used as a *type filter*:

| Constraint            | Allows                                             |
|-----------------------|----------------------------------------------------|
| `any`                 | every type (no operations available except assignment) |
| `comparable`          | types usable with `==` / `!=` (map keys, dedup)    |
| `cmp.Ordered`         | types usable with `<` (numbers, strings)           |
| custom interface      | whatever methods/types you specify                 |

**Union constraints** list allowed types with `|`. The tilde `~int` means
"`int` *or any named type whose underlying type is* `int`" (like `type Celsius int`):

```go
type Number interface {
    ~int | ~int64 | ~float64
}

func Sum[T Number](xs []T) T { ... }
```

Without `~`, `Sum` would reject `[]Celsius` even though Celsius *is* an int
underneath. Rule of thumb: almost always write unions with `~`.

## Generic types

Types can be parameterized too — the classic example is a container:

```go
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

s := Stack[string]{}   // a stack of strings — pushing an int won't compile
```

Note: methods on a generic type may **not** introduce extra type parameters;
they can only use the type's own (`T` here).

## Type inference

Go usually infers type arguments from the values you pass, so call sites look
like normal code:

```go
doubled := Map([]int{1, 2, 3}, func(x int) int { return x * 2 })
// no Map[int, int](...) needed — inferred from the arguments
```

You only spell out `[T]` when there's nothing to infer from (e.g.
`Stack[string]{}` — an empty literal gives no clues).

## The standard library uses generics: `cmp` and `slices`

Since Go 1.21, the standard library ships generic helpers you should reach
for **before** writing your own:

```go
slices.Contains(names, "ada")       // works on []T for comparable T
slices.Sort(nums)                   // any ordered T
slices.Max(nums), slices.Min(nums)
slices.Index(names, "grace")
cmp.Compare(a, b)                   // -1 / 0 / +1 for ordered types
slices.SortFunc(people, func(a, b Person) int { return cmp.Compare(a.Age, b.Age) })
```

(`maps.Keys` / `maps.Values` exist too, returning iterators as of Go 1.23.)

## When NOT to use generics

Generics answer "**same code, many types**". They do *not* replace interfaces,
which answer "**same behaviour, different implementations**".

- Method calls only? Use an interface: `func Log(s fmt.Stringer)` beats
  `func Log[T fmt.Stringer](s T)`.
- One byte-stream type? `io.Reader` already abstracts it — don't invent `[T Reader]`.
- Only ever called with one type? You don't need a type parameter.
- Reaching for reflection-level flexibility? Generics won't get you there.

Start concrete. Introduce a type parameter only when you catch yourself
copy-pasting a function to change nothing but types.

## Run the examples

```sh
go run 01_why_generics.go
go run 02_map_filter_reduce.go
go run 03_generic_stack.go
go run 04_cmp_slices_stdlib.go
```

## Key takeaways

- Generics = one implementation, many types, fully compile-time checked.
- Constraints are interfaces; unions (`~int | ~float64`) admit operator use.
- `~T` matches named types with underlying type `T` — usually what you want.
- Type inference keeps call sites clean; brackets appear mostly on type literals.
- Prefer `slices`/`maps`/`cmp` from the standard library over hand-rolled helpers.
- Interfaces for polymorphic *behaviour*; generics for type-safe *containers and algorithms*.

## Exercises

1. Write `Keys[K comparable, V any](m map[K]V) []K` returning a map's keys,
   then sort the result with `slices.Sort` (you'll need to tighten the
   constraint to `cmp.Ordered` — think about why).
2. Implement a generic `Queue[T]` (FIFO) with `Enqueue`, `Dequeue (T, bool)`,
   and `Len`. Test it with two different element types.
3. Write `MaxBy[T any](xs []T, score func(T) int) (T, bool)` that returns the
   element with the highest score. Why can't you just use `slices.Max` here?
