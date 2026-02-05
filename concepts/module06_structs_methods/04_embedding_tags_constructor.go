// Module 06, Example 4 — Embedding (composition over inheritance),
// field/method promotion, struct tags (JSON preview), and the NewXxx
// constructor pattern.
//
// Run with: go run 04_embedding_tags_constructor.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ---- Embedding ---------------------------------------------------------------

// A plain struct with a method.
type Animal struct {
	Name string
}

func (a Animal) Describe() string {
	return "I am " + a.Name
}

func (a Animal) Speak() string {
	return "..." // generic animals are quiet
}

// Dog EMBEDS Animal: the field has a type but no name. Everything Animal has
// (fields and methods) is "promoted" — reachable directly on a Dog.
// This is COMPOSITION: a Dog *has an* Animal inside it; Go just adds sugar
// so it also *feels like* a Dog *is an* Animal.
type Dog struct {
	Animal // embedded (anonymous) field
	Breed  string
}

// Dog defines its own Speak. This SHADOWS the promoted Animal.Speak — the
// outer type's method wins. But note: this is NOT overriding. If some Animal
// method internally called a.Speak(), it would still run Animal.Speak,
// because inside Animal methods the receiver is the Animal, never the Dog.
// There is no virtual dispatch in Go.
func (d Dog) Speak() string {
	return "Woof!"
}

// ---- Struct tags (JSON preview) ------------------------------------------------

// Tags are raw strings attached to fields. They do nothing by themselves;
// libraries read them via reflection. encoding/json uses the `json:"..."`
// tag to choose key names and options.
type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	// omitempty: leave the key out of the JSON if the field is zero-valued.
	Discount float64 `json:"discount,omitempty"`
	// "-" means: never include this field in JSON.
	internalNote string `json:"-"`
}

// ---- Constructor pattern --------------------------------------------------------

// Go has no constructors. Convention: a function called NewXxx (or just New
// if the package is named after the type) that validates inputs, sets
// defaults, and returns a ready-to-use value — usually a pointer, often with
// an error.
func NewProduct(id int, name string, price float64) (*Product, error) {
	if name == "" {
		return nil, errors.New("product name must not be empty")
	}
	if price < 0 {
		return nil, fmt.Errorf("price must be >= 0, got %.2f", price)
	}
	return &Product{
		ID:           id,
		Name:         name,
		Price:        price,
		internalNote: "created via constructor",
	}, nil
}

func main() {
	// ---- Promotion in action ---------------------------------------------
	d := Dog{
		Animal: Animal{Name: "Rex"}, // embedded field is named after its type
		Breed:  "Beagle",
	}

	fmt.Println(d.Describe())    // promoted method from Animal
	fmt.Println("Name:", d.Name) // promoted FIELD from Animal
	fmt.Println("Breed:", d.Breed)

	// Shadowing: Dog's own Speak wins over the promoted one...
	fmt.Println("Dog speaks:   ", d.Speak()) // Woof!
	// ...but the inner one is still reachable by naming the embedded type:
	fmt.Println("Animal speaks:", d.Animal.Speak()) // ...

	// ---- Struct tags + JSON -------------------------------------------------
	p, err := NewProduct(1, "Mechanical Keyboard", 129.99)
	if err != nil {
		fmt.Println("could not create product:", err)
		return
	}

	// Marshal = Go value -> JSON bytes. Watch how the tags shape the output:
	// lowercase keys, no "discount" (zero + omitempty), no internalNote ("-",
	// and it's unexported anyway — json can only see exported fields).
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		fmt.Println("marshal failed:", err)
		return
	}
	fmt.Println("as JSON:")
	fmt.Println(string(data))

	// Unmarshal = JSON bytes -> Go value, matching keys via the same tags.
	var back Product
	if err := json.Unmarshal([]byte(`{"id":2,"name":"Mouse","price":25}`), &back); err != nil {
		fmt.Println("unmarshal failed:", err)
		return
	}
	fmt.Printf("from JSON: %+v\n", back)

	// ---- Constructor rejecting bad input ------------------------------------
	if _, err := NewProduct(3, "", 10); err != nil {
		fmt.Println("constructor said no:", err)
	}
}
