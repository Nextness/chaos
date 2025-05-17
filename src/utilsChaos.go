package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

func isUppercase(b byte) bool {
	s := rune(b)
	return unicode.IsUpper(s) && unicode.IsLetter(s)
}

func isNum(b byte) bool {
	s := rune(b)
	return unicode.IsNumber(s)
}

func isAlpha(b byte) bool {
	s := rune(b)
	return unicode.IsLetter(s)
}

func isAlphanum(b byte) bool {
	s := rune(b)
	return unicode.IsNumber(s) || unicode.IsLetter(s)
}

func isInside(b byte, listChar []byte) bool {
	result := false
	for _, item := range listChar {
		if result = (b == item); result {
			break
		}
	}
	return result
}

func assert(ok bool, reason string) {
	if !ok {
		panic(fmt.Sprintf("Assertion failed - %s\n", reason))
	}
}

func makePadOffset(padSize int, offset int) string {
	repeat := padSize + offset
	return strings.Repeat(" ", repeat)
}

func makePad(padSize int) string {
	return strings.Repeat(" ", padSize)
}

func chaosDebug(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "DEBUG: "+format+"\n", args...)
}

func todo[V any](args ...any) V {
	panic("TODO: Not implemented yet")
}
