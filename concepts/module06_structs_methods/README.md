# Module 06 — Structs, Pointers & Methods

## What you'll learn

- How to define structs and create them with literals
- Comparing structs and using anonymous structs
- Pointers in Go: `&` and `*` (and why there's no pointer arithmetic)
- Methods, and the difference between value receivers and pointer receivers
- Method sets — which methods a value vs a pointer "has"
- Struct embedding: Go's take on composition over inheritance
- Struct tags (a preview of JSON encoding)
- The constructor pattern (`NewXxx`)

## Structs: grouping related data

A **struct** is a typed collection of fields. If you come from other languages,
think of it as a class *without* inheritance and *without* methods declared
inside the body — data only. Behaviour is attached separately (see methods below).

```go
type User struct {
    Name string
    Age  int
}
```

You create struct values with **literals**:

```go
u1 := User{Name: "Ada", Age: 36}   // field names (preferred: order-independent, clear)
u2 := User{"Grace", 46}            // positional (fragile — avoid outside tiny structs)
u3 := User{}                       // zero value: Name is "", Age is 0
```

There is no `null` surprise here: a struct variable is never "missing", it
always exists with **zero-valued fields** until you fill them in.

### Comparing structs

Structs are comparable with `==` **if all their fields are comparable**
(numbers, strings, bools, other comparable structs...). Structs containing
slices, maps, or functions are *not* comparable and `==` won't compile.

### Anonymous structs

For quick one-off groupings (table-driven tests use this a lot) you can define
a struct type inline, without naming it:

```go
point := struct{ X, Y int }{X: 3, Y: 4}
```

## Pointers: `&` and `*`

Go passes **everything by value** — when you pass a struct to a function, the
function gets a *copy*. A **pointer** lets you share the original instead.

- `&x` — "address of x" — creates a pointer to `x`
- `*p` — "value at p" — follows (dereferences) the pointer

```
   variable x (a User struct)          pointer p := &x
   ┌─────────────────────────┐         ┌──────────────┐
   │ Name: "Ada"             │ ◄────── │ 0xc000010040 │
   │ Age:  36                │         └──────────────┘
   └─────────────────────────┘          p stores x's address.
        lives at 0xc000010040           *p reads/writes x itself.
```

Unlike C, Go has **no pointer arithmetic** (`p++` is a compile error) and
pointers are garbage-collected, so they are safe. A pointer's zero value is
`nil`; dereferencing `nil` panics.

Handy shortcut: Go automatically dereferences struct pointers for field
access — `p.Name` works, you never write `(*p).Name`.

## Methods and receivers

A **method** is a function with a *receiver* — the value it is attached to:

```go
func (u User) Greet() string        { return "Hi, " + u.Name }  // value receiver
func (u *User) Rename(n string)     { u.Name = n }              // pointer receiver
```

### Value vs pointer receiver — the rules

| Choose a **pointer receiver** when...            | Choose a **value receiver** when...       |
|--------------------------------------------------|-------------------------------------------|
| the method must **modify** the receiver          | the type is small and immutable (e.g. `time.Time`-like) |
| the struct is **large** (avoid copying)          | the type is a map/func/chan or small slice |
| **any other method** already uses a pointer      | you want copies to be safe to use concurrently |

The most important rule: **be consistent**. If one method needs a pointer
receiver, give *all* methods on that type pointer receivers.

### Method sets (why this matters)

- A **value** of type `T` has the methods with value receivers.
- A **pointer** `*T` has methods with value *and* pointer receivers.

The compiler papers over this for direct calls (`u.Rename(...)` becomes
`(&u).Rename(...)` automatically when `u` is addressable), but it matters for
**interfaces**: if a method set requires `*T`, a plain `T` value does not
satisfy the interface. You'll meet this again in Module 07.

## Embedding: composition over inheritance

Go has no inheritance. Instead you **embed** a type inside a struct (a field
with no name), and its fields and methods get **promoted**:

```go
type Animal struct{ Name string }
func (a Animal) Describe() string { return "I am " + a.Name }

type Dog struct {
    Animal        // embedded — no field name
    Breed string
}

d := Dog{Animal{Name: "Rex"}, "Beagle"}
d.Describe()      // promoted method — looks inherited, but it's composition
d.Name            // promoted field
```

```
        Dog
   ┌───────────────────┐
   │ Animal (embedded) │──► Name, Describe() promoted up to Dog
   │ Breed string      │
   └───────────────────┘
```

This is *not* inheritance: inside `Describe`, the receiver is the `Animal`,
not the `Dog` — there is no method overriding or virtual dispatch.

## Struct tags (JSON preview)

Tags are string metadata on fields, read via reflection by packages like
`encoding/json`:

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age,omitempty"`
}
```

`json.Marshal` uses the tags to pick the JSON key names. A full JSON module
comes later — here we just show the mechanics.

## Constructor pattern: `NewXxx`

Go has no constructors. The convention is a plain function named `New` (if the
package has one main type) or `NewTypeName`, which validates input and returns
a ready-to-use value (usually a pointer):

```go
func NewUser(name string, age int) (*User, error) {
    if name == "" {
        return nil, errors.New("name must not be empty")
    }
    return &User{Name: name, Age: age}, nil
}
```

## Run the examples

```sh
go run 01_structs_basics.go
go run 02_pointers.go
go run 03_methods_receivers.go
go run 04_embedding_tags_constructor.go
```

## Key takeaways

- Structs group data; behaviour is attached with methods, defined separately.
- Everything is passed by value; pointers (`&`, `*`) let you share and mutate.
- Pointer receiver: mutation, big structs, consistency. Value receiver: small immutable data.
- Method sets: `*T` has all methods, `T` only the value-receiver ones — this bites via interfaces.
- Embedding promotes fields/methods — composition, not inheritance.
- Use `NewXxx` constructor functions for validation and required setup.

## Exercises

1. Define a `Rectangle` struct with `Width` and `Height` fields. Add a value-receiver
   method `Area() float64` and a pointer-receiver method `Scale(factor float64)`
   that multiplies both fields. Print the area before and after scaling.
2. Create a `BankAccount` struct with an unexported `balance` field and a
   `NewBankAccount(initial float64) (*BankAccount, error)` constructor that rejects
   negative initial balances. Add `Deposit` and `Balance` methods with appropriate receivers.
3. Build an `Employee` struct that embeds a `Person` struct (with `Name`) and adds
   a `Company` field. Call a method promoted from `Person` on an `Employee` value,
   then give `Employee` its own method with the same name and observe which one wins.
