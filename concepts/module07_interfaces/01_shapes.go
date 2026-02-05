// Module 07, Example 1 — Interfaces: implicit satisfaction, small
// interfaces, and fmt.Stringer, via a Shape area calculator.
//
// Run with: go run 01_shapes.go
package main

import (
	"fmt"
	"math"
)

// Shape is an INTERFACE: it lists behaviour (method signatures) and nothing
// else. Any type that has these two methods IS a Shape — automatically.
// There is no "implements Shape" anywhere in this file.
type Shape interface {
	Area() float64
	Perimeter() float64
}

// ---- Circle satisfies Shape ------------------------------------------------

type Circle struct {
	Radius float64
}

func (c Circle) Area() float64      { return math.Pi * c.Radius * c.Radius }
func (c Circle) Perimeter() float64 { return 2 * math.Pi * c.Radius }

// String makes Circle satisfy fmt.Stringer — the standard library's most
// famous one-method interface:
//
//	type Stringer interface { String() string }
//
// fmt.Println / fmt.Printf("%v") automatically call String() when a type
// has it. You control how your type prints, without touching fmt.
func (c Circle) String() string {
	return fmt.Sprintf("Circle(r=%.1f)", c.Radius)
}

// ---- Rectangle satisfies Shape ----------------------------------------------

type Rectangle struct {
	Width, Height float64
}

func (r Rectangle) Area() float64      { return r.Width * r.Height }
func (r Rectangle) Perimeter() float64 { return 2 * (r.Width + r.Height) }

func (r Rectangle) String() string {
	return fmt.Sprintf("Rectangle(%.1f x %.1f)", r.Width, r.Height)
}

// ---- Code written against the interface --------------------------------------

// describe knows NOTHING about circles or rectangles. It works with any
// Shape — including ones written next year in another package. This is the
// point of interfaces: the function depends on behaviour, not on concrete
// types.
func describe(s Shape) {
	// %v uses String() because our shapes are Stringers.
	fmt.Printf("%-22v area=%8.2f perimeter=%8.2f\n", s, s.Area(), s.Perimeter())
}

func totalArea(shapes []Shape) float64 {
	sum := 0.0
	for _, s := range shapes {
		sum += s.Area()
	}
	return sum
}

func main() {
	// A Circle value can be assigned to a Shape variable because Circle has
	// both methods. The compiler checks this — no runtime surprises.
	var s Shape = Circle{Radius: 2}
	fmt.Println("a shape:", s)

	// A slice of the INTERFACE type can mix different concrete types:
	shapes := []Shape{
		Circle{Radius: 1},
		Rectangle{Width: 3, Height: 4},
		Circle{Radius: 2.5},
		Rectangle{Width: 10, Height: 0.5},
	}

	fmt.Println("\nAll shapes:")
	for _, s := range shapes {
		describe(s) // dynamic dispatch: the right Area/Perimeter runs
	}

	fmt.Printf("\nTotal area: %.2f\n", totalArea(shapes))

	// Compile-time guarantee trick (common idiom): this line fails to
	// compile if Circle ever stops satisfying Shape. The blank identifier
	// discards the value; we only want the type check.
	var _ Shape = Circle{}
	var _ fmt.Stringer = Rectangle{}

	// Small-interface philosophy:
	// Shape has 2 methods and already fits every 2D figure imaginable.
	// fmt.Stringer has 1 method and fits EVERYTHING that can describe
	// itself. Small interfaces = many implementations = reusable code.
}
