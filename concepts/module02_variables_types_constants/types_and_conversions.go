// types_and_conversions.go — Go's basic types and the golden rule:
// there are NO implicit conversions. Every type change is written T(v).
//
// Run it with:   go run types_and_conversions.go
package main

import (
	"fmt"
	"math"
	"strconv"
)

func main() {
	// ---- Integers ------------------------------------------------------
	// Sized types state their bit width; plain `int` is the size of a
	// machine word (64 bits on modern hardware) and is the default choice.
	var small int8 = 127        // range -128..127
	var big int64 = 1 << 40     // 2^40 — needs a wide type
	var machine int = 12345     // use this one unless you have a reason not to
	var unsigned uint16 = 65535 // 0..65535; unsigned types can't be negative

	fmt.Printf("int8=%d  int64=%d  int=%d  uint16=%d\n",
		small, big, machine, unsigned)

	// Overflow on sized integers WRAPS AROUND silently:
	small++                                        // 127 + 1 ...
	fmt.Println("int8 overflow: 127 + 1 =", small) // ... = -128 (!)

	// ---- Floats --------------------------------------------------------
	// There's no plain "float": choose float32 or float64. Literals like
	// 3.14 are untyped constants that default to float64 — so should you.
	var f64 float64 = 3.141592653589793
	var f32 float32 = 3.141592653589793 // silently loses precision when stored
	fmt.Println("float64:", f64)
	fmt.Println("float32:", f32, " <- fewer significant digits")

	// Floats are binary approximations — classic result. (We must use
	// variables here: 0.1+0.2 written as constants is computed EXACTLY at
	// compile time by Go's arbitrary-precision constant arithmetic!)
	tenth, fifth := 0.1, 0.2
	fmt.Println("0.1 + 0.2 =", tenth+fifth, "(not exactly 0.3!)")

	// ---- bool, byte, rune, complex --------------------------------------
	var ok bool = true // no truthy/falsy: `if 1 {}` does not compile
	var b byte = 'A'   // byte = alias for uint8; 'A' is the byte value 65
	var r rune = '世'   // rune = alias for int32; a Unicode code point
	var c complex128 = complex(3, 4)

	// %c prints the character a number represents, %U its Unicode form.
	fmt.Printf("bool=%t  byte=%d(%c)  rune=%d(%c %U)\n", ok, b, b, r, r, r)
	fmt.Println("complex:", c, " |c| =", math.Sqrt(real(c)*real(c)+imag(c)*imag(c)))

	// ---- NO implicit conversions ----------------------------------------
	var i int = 65
	var f float64 = float64(i) // required! `var f float64 = i` will NOT compile

	var a32 int32 = 10
	var a64 int64 = 20
	// sum := a32 + a64          // COMPILE ERROR: mismatched types int32 and int64
	sum := int64(a32) + a64 // fix: convert explicitly
	fmt.Println("float64(i) =", f, "  int64(a32)+a64 =", sum)

	// Conversions may LOSE data — the compiler allows it because you asked.
	// (Note: we convert VARIABLES here. Converting a constant that doesn't
	// fit, like int8(300) written literally, is caught at compile time —
	// constants are checked, runtime values just wrap/truncate.)
	almostFour := 3.99
	threeHundred := 300
	fmt.Println("int(3.99)      =", int(almostFour), " (truncates toward zero)")
	fmt.Println("int8(300)      =", int8(threeHundred), " (wraps: 300-256 = 44)")
	fmt.Println("uint8(300)     =", uint8(threeHundred), " (unsigned wraps the same way)")

	// ---- Strings vs numbers ---------------------------------------------
	// string(65) converts a CODE POINT to text: "A", NOT "65".
	fmt.Println(`string(rune(65)) =`, string(rune(65)))
	// To convert numbers to/from text, use the strconv package:
	s := strconv.Itoa(65)        // int -> "65"
	n, err := strconv.Atoi("42") // "42" -> int; can fail, so returns an error too
	fmt.Println("strconv.Itoa(65) =", s, " Atoi(\"42\") =", n, " err =", err)

	// string <-> []byte conversions copy the underlying data:
	bs := []byte("héllo") // the UTF-8 bytes of the string
	fmt.Println("[]byte(\"héllo\") =", bs, "-> back:", string(bs))
}
