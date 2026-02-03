# Module 04 — Collections: Arrays, Slices, Maps, and Strings

## What you'll learn

- Arrays vs slices: value semantics vs a small "header" over shared data
- Slice internals: pointer / length / capacity, with memory diagrams
- How `append` grows a slice, and the sharing/aliasing gotcha
- `copy` and full slice expressions `s[a:b:c]`
- Maps: create, access, the comma-ok idiom, delete, random iteration order
- Strings as immutable byte sequences; bytes vs runes; iterating UTF-8
  correctly
- The modern `slices` and `maps` standard-library packages

## Arrays: fixed size, value semantics

```go
var a [4]int            // exactly 4 ints, zeroed: [0 0 0 0]
b := [3]string{"x", "y", "z"}
c := [...]int{1, 2, 3}  // ... = let the compiler count (type is [3]int)
```

- The length is **part of the type**: `[3]int` and `[4]int` are different,
  incompatible types.
- Arrays are **values**. Assigning or passing one **copies all elements**:

```go
d := c      // full copy
d[0] = 99   // c is untouched
```

Arrays are the building block, but day-to-day Go code uses slices.

## Slices: a header over an array

A slice is a small struct — three words — describing a window onto an
underlying array:

```
s := []int{10, 20, 30}

     slice header (the "value" you pass around)
     +-----------+
  s: | ptr   ────┼─────┐
     | len = 3   |     │        underlying array (the actual data)
     | cap = 3   |     └──> +----+----+----+
     +-----------+          | 10 | 20 | 30 |
                            +----+----+----+
```

- `len(s)` — how many elements you can index right now.
- `cap(s)` — how far the underlying array extends from the slice's start.
- Copying a slice (assignment, function argument) copies only the **header**
  — both headers point at the **same array**. Cheap, but it means writes are
  visible through every alias.

### Slicing expressions

```go
s := []int{0, 1, 2, 3, 4, 5}
s[1:4]   // [1 2 3]      len=3, cap=5 (from index 1 to the array's end)
s[:2]    // [0 1]
s[3:]    // [3 4 5]
s[:]     // whole slice
```

Slicing never copies data — it makes a new header into the same array:

```
s:            +----+----+----+----+----+----+
              |  0 |  1 |  2 |  3 |  4 |  5 |
              +----+----+----+----+----+----+
                     ^
t := s[1:4]          |
     +-----------+   |
  t: | ptr   ────┼───┘        t[0] is s[1] — SAME memory!
     | len = 3   |
     | cap = 5   |   (capacity runs to the end of the array)
     +-----------+
```

### append and growth

`append(s, v)` returns a (possibly new) slice — **always assign the result**:

```go
s = append(s, 42)
```

- If `len < cap`, append writes into the existing array in place.
- If the array is full, append allocates a **bigger array** (roughly
  doubling for small slices, ~1.25x for large ones), copies everything, and
  returns a header pointing at the new array. The old array — and anyone
  still slicing it — is left behind.

### The aliasing gotcha

Because slices share arrays, an append that *doesn't* reallocate can
overwrite a sibling slice's data — and one that *does* reallocate silently
stops sharing. Both behaviors are demonstrated in `slice_internals.go`.
Defenses:

- `copy(dst, src)` to make a genuinely independent slice
  (copies `min(len(dst), len(src))` elements).
- The **full slice expression** `s[a:b:c]` sets `cap` to `c-a`, so a later
  append is *forced* to reallocate instead of scribbling on shared memory:

```go
part := s[0:2:2]        // len 2, cap 2 — append will copy away safely
```

## Maps: hash tables

```go
ages := map[string]int{"ada": 36}   // literal
m := make(map[string]int)           // empty, ready to use
var nilMap map[string]int           // nil map: read-only-ish — WRITES PANIC
```

- `m[k] = v` inserts/updates; `m[k]` reads (missing key → **zero value**,
  no error).
- Because missing keys return zero values, use **comma-ok** to distinguish
  "absent" from "present with zero value":

```go
age, ok := ages["grace"]
if !ok { fmt.Println("no such person") }
```

- `delete(m, k)` removes a key (no-op if absent). `len(m)` counts entries.
- **Iteration order is deliberately randomized** — Go's runtime varies it
  per run so you can't accidentally depend on it. Need order? Collect keys,
  sort them (`slices.Sorted(maps.Keys(m))`), then iterate.
- Maps are reference-like: passing a map copies a handle to the same table,
  so callees see your writes.

## Strings, bytes, and runes

A string is an **immutable** sequence of bytes, almost always UTF-8 text:

- `s[i]` gives a **byte** (`uint8`), *not* a character.
- `len(s)` counts **bytes**. `"héllo"` has len 6 — 'é' is 2 bytes.
- `for i, r := range s` decodes UTF-8: `r` is a **rune** (code point) and
  `i` the byte offset where it starts. This is the correct way to iterate
  characters.
- Counting characters: `utf8.RuneCountInString(s)`.
- Strings are immutable — to edit, convert: `b := []byte(s)` or
  `r := []rune(s)`, modify, convert back (each conversion copies).
- Building strings in a loop? `+=` re-copies every time; use
  `strings.Builder`.

```
"héllo"  bytes:   68 | c3 a9 | 6c | 6c | 6f      len(s) == 6
                  'h'   'é'    'l'   'l'  'o'
                        └─ one rune, two bytes ─ range gives you 5 runes
```

## The `slices` and `maps` packages (Go 1.21+)

Generic helpers so you stop hand-rolling loops:

```go
import ("slices"; "maps")

slices.Contains(s, x)      slices.Index(s, x)
slices.Sort(s)             slices.SortFunc(s, cmp)
slices.Max(s), slices.Min(s)
slices.Reverse(s)          slices.Equal(a, b)
slices.Clone(s)            // proper independent copy in one call

maps.Keys(m), maps.Values(m)   // iterators (Go 1.23) — wrap in slices.Collect / slices.Sorted
maps.Clone(m)              maps.Equal(a, b)
```

## Run the examples

```
go run arrays_vs_slices.go
go run slice_internals.go
go run maps_basics.go
go run strings_bytes_runes.go
go run slices_maps_stdlib.go
```

## Key takeaways

- Arrays copy their elements; slices copy a 3-word header pointing at shared
  data.
- A slice = pointer + len + cap. `append` may or may not reallocate — always
  use `s = append(s, ...)` and beware aliasing.
- `copy`, `slices.Clone`, and `s[a:b:c]` are your tools for controlling
  sharing.
- Map reads of missing keys return zero values — comma-ok tells you the
  truth; iteration order is random on purpose.
- `len(string)` is bytes; `range` over a string yields runes; convert to
  `[]rune` for random access to characters.
- Reach for the `slices` and `maps` packages before writing manual loops.

## Exercises

1. Write `func head(s []int) []int` that returns the first 2 elements with
   capacity clamped via `s[0:2:2]`. Demonstrate that appending to the result
   does NOT disturb the original slice — then show the disturbance when you
   use plain `s[0:2]`.
2. Count word frequencies in the sentence "the quick brown fox jumps over
   the lazy dog the fox" using a map (hint: `strings.Fields`), then print
   the words in alphabetical order with their counts.
3. Write a function that reverses a string *correctly* for non-ASCII input
   (hint: `[]rune`). Test it on "héllo, 世界". What goes wrong if you reverse
   bytes instead?
