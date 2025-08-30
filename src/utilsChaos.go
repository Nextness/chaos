package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unicode"
)

const PAD_WIDTH = 2

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
	pc, filename, line, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(pc).Name()
	if !ok {
		panic(fmt.Sprintf("%s[%s:%d] Assertion failed - %s\n", caller, filepath.Base(filename), line, reason))
	}
}

func pp(anything any) string {
	s, _ := json.MarshalIndent(anything, "", "\t")
	return string(s)
}

func chaosDebug(anything ...any) {
	for id, thing := range anything {
		fmt.Fprintf(os.Stdout, "DEBUG %d: "+pp(thing)+"\n", id)
	}
}

func todo[V any](args ...any) V {
	panic("TODO: Not implemented yet")
}

func cast[V any](value any) (V, bool) {
	result, ok := value.(V)
	return result, ok
}

func castAssert[V any](value any) V {
	result, ok := cast[V](value)
	pc, filename, line, _ := runtime.Caller(1)
	caller := runtime.FuncForPC(pc).Name()
	if !ok {
		panic(
			fmt.Sprintf(
				"%s[%s:%d] Assertion failed - failed to cast %T into %T\n",
				caller,
				filepath.Base(filename),
				line,
				value,
				new(V),
			),
		)
	}
	return result
}
