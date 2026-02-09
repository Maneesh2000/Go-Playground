// Module 12, example 1: fmt verbs, strings, strconv, unicode.
//
// Run with: go run 01_fmt_strings_strconv.go
package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type User struct {
	Name string
	Age  int
}

func main() {
	// ------------------------------- fmt --------------------------------
	u := User{Name: "Ada", Age: 36}

	fmt.Println("--- fmt verbs ---")
	fmt.Printf("%%v  : %v\n", u)                                     // default: {Ada 36}
	fmt.Printf("%%+v : %+v\n", u)                                    // with field names: {Name:Ada Age:36}
	fmt.Printf("%%#v : %#v\n", u)                                    // Go syntax: main.User{Name:"Ada", Age:36}
	fmt.Printf("%%T  : %T\n", u)                                     // the type: main.User
	fmt.Printf("%%q  : %q\n", "a \"quoted\" string")                 // quoted + escaped
	fmt.Printf("%%d %%o %%x %%b: %d %o %x %b\n", 255, 255, 255, 255) // bases
	fmt.Printf("%%8.3f : %8.3f\n", 3.14159)                          // width 8, 3 decimals: "   3.142"
	fmt.Printf("%%-8s| : %-8s|\n", "left")                           // minus = left-align in width
	fmt.Printf("%%p  : %p\n", &u)                                    // pointer address

	// Sprintf builds a string; Fprintf writes to any io.Writer.
	label := fmt.Sprintf("%s (%d)", u.Name, u.Age)
	fmt.Println("Sprintf gave us:", label)

	// ----------------------------- strings ------------------------------
	fmt.Println("--- strings ---")
	s := "  Go makes systems programming fun  "

	fmt.Printf("TrimSpace:  %q\n", strings.TrimSpace(s))
	fmt.Println("Contains 'systems':", strings.Contains(s, "systems"))
	fmt.Println("Fields:", strings.Fields(s)) // split on any whitespace
	fmt.Println("ReplaceAll:", strings.ReplaceAll("a-b-c", "-", "+"))
	fmt.Println("Split:", strings.Split("2026-07-04", "-"))
	fmt.Println("Join:", strings.Join([]string{"usr", "local", "bin"}, "/"))
	fmt.Println("HasPrefix:", strings.HasPrefix("module12", "module"))
	fmt.Println("Repeat:", strings.Repeat("=", 20))

	// Building a string in a loop? Use strings.Builder, not +=.
	// += copies the whole string every iteration (O(n²)); Builder appends
	// into a growable buffer (O(n)).
	var b strings.Builder
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&b, "%d,", i) // Builder is an io.Writer — verbs work!
	}
	fmt.Println("Builder:", strings.TrimSuffix(b.String(), ","))

	// ----------------------------- strconv ------------------------------
	fmt.Println("--- strconv ---")

	// String → int. Conversion can fail, so we get (value, error).
	n, err := strconv.Atoi("42")
	fmt.Printf("Atoi(\"42\") = %d, err = %v\n", n, err)

	_, err = strconv.Atoi("forty-two") // this one fails
	fmt.Println("Atoi(\"forty-two\") err:", err)

	// Int → string. NOTE: string(65) is "A" (a rune conversion), NOT "65"!
	// That's one of Go's classic beginner traps — use Itoa.
	fmt.Println("Itoa(65):", strconv.Itoa(65), " but string(65) would be:", string(rune(65)))

	f, _ := strconv.ParseFloat("3.14", 64)
	ok, _ := strconv.ParseBool("true")
	fmt.Println("ParseFloat:", f, " ParseBool:", ok)
	fmt.Println("FormatInt base 2:", strconv.FormatInt(42, 2)) // "101010"
	fmt.Println("Quote:", strconv.Quote(`tab	here`))           // escapes for you

	// ----------------------------- unicode ------------------------------
	fmt.Println("--- unicode & runes ---")
	word := "Héllo, 世界"

	// len() counts BYTES; ranging yields RUNES (code points) + byte offsets.
	fmt.Printf("len(%q) = %d bytes, %d runes\n",
		word, len(word), len([]rune(word)))
	for i, r := range word {
		fmt.Printf("  byte offset %2d: %q (upper=%v, letter=%v)\n",
			i, r, unicode.IsUpper(r), unicode.IsLetter(r))
	}
}
