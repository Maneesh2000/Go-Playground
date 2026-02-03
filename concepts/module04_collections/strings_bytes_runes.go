// strings_bytes_runes.go — strings are immutable BYTE sequences (UTF-8),
// runes are decoded code points. Indexing gives bytes; range gives runes.
//
// Run it with:   go run strings_bytes_runes.go
package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func main() {
	// 'é' is one character but TWO bytes in UTF-8 (0xC3 0xA9);
	// '世' and '界' are THREE bytes each.
	s := "héllo, 世界"

	// ---- len() counts BYTES, not characters -------------------------------
	fmt.Printf("s = %q\n", s)
	fmt.Println("len(s)              =", len(s), " <- BYTES")
	fmt.Println("utf8.RuneCountInString(s) =", utf8.RuneCountInString(s), "<- characters (runes)")

	// ---- Indexing gives raw bytes -------------------------------------------
	// s[1] is the FIRST byte of 'é', not 'é' itself:
	fmt.Printf("s[0] = %d (%c)   s[1] = %d (0x%x — half of 'é'!)\n",
		s[0], s[0], s[1], s[1])

	// ---- The WRONG way to iterate: byte by byte ------------------------------
	// Multibyte characters shatter into garbage bytes:
	fmt.Print("byte loop (broken for non-ASCII): ")
	for i := 0; i < len(s); i++ {
		fmt.Printf("%c", s[i]) // prints Ã© mojibake for é, etc.
	}
	fmt.Println()

	// ---- The RIGHT way: range decodes UTF-8 -----------------------------------
	// range over a string yields (byteIndex, rune). Watch the indexes
	// jump by 2 or 3 where characters are multibyte:
	fmt.Println("range loop (correct):")
	for i, r := range s {
		fmt.Printf("  byte offset %2d: %c  (U+%04X, %d bytes)\n",
			i, r, r, utf8.RuneLen(r))
	}

	// ---- Strings are IMMUTABLE ----------------------------------------------
	// s[0] = 'H'   // COMPILE ERROR: cannot assign to s[0]
	// To "modify", convert to a mutable copy, edit, convert back:
	b := []byte(s) // copy of the bytes — fine for ASCII-only edits
	b[0] = 'H'
	fmt.Println("\nvia []byte:", string(b))

	// For character-level work use []rune — one element per code point,
	// so indexing is by character, not byte:
	r := []rune(s)
	r[7] = '地' // replace 世 (rune index 7, NOT byte index!)
	fmt.Println("via []rune:", string(r))

	// Correct string reversal must reverse RUNES, not bytes:
	fmt.Println("reversed:  ", reverse(s))

	// ---- Building strings efficiently -------------------------------------------
	// += on strings copies the whole string every time (immutability!).
	// strings.Builder appends into a growing buffer instead:
	var sb strings.Builder
	for i := range 3 {
		fmt.Fprintf(&sb, "part%d;", i) // Builder is an io.Writer
	}
	fmt.Println("built:", sb.String())

	// ---- A few everyday helpers from the strings package -------------------------
	fmt.Println("\nstrings package sampler:")
	fmt.Println("  Contains:", strings.Contains(s, "世"))
	fmt.Println("  ToUpper: ", strings.ToUpper("héllo")) // Unicode-aware: HÉLLO
	fmt.Println("  Split:   ", strings.Split("a,b,c", ","))
	fmt.Println("  Join:    ", strings.Join([]string{"a", "b", "c"}, "-"))
	fmt.Println("  TrimSpace:", strings.TrimSpace("  padded  ")+"|")
}

// reverse returns s with its CHARACTERS in reverse order. Converting to
// []rune first is what keeps multibyte characters intact.
func reverse(s string) string {
	r := []rune(s)
	// two-pointer swap from both ends toward the middle:
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
