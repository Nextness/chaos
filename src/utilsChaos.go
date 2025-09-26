package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"unicode"
)

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

func assert[T any](ok bool, reason string) T {
	if ok {
		return *new(T)
	}
	// TODO: Improve colors/style for asserts
	progCounter := make([]uintptr, 20)                  // Stacktrace size set to 20 for now
	invocationsCount := runtime.Callers(2, progCounter) // Skipping Callers, Caller of Callers
	frames := runtime.CallersFrames(progCounter[:invocationsCount])
	iter := 0
	printOnce := true
	fmt.Print("\033[1;31m[ERROR] Assertion failed - stopping execution\033[0m\n")
	for true {
		frame, more := frames.Next()
		if iter == 0 || iter == 1 || iter == 2 || iter == 3 || iter == 4 || iter == invocationsCount-2 {
			fmt.Fprintf(os.Stderr, "  %d. %s:%d %s\n", iter, filepath.Base(frame.File), frame.Line, frame.Function)
		} else if printOnce {
			fmt.Fprintf(os.Stderr, "  ...\n")
			printOnce = false
		}
		iter++
		if !more {
			break
		}
	}
	if reason == "" {
		reason = "Reason not provided"
	}
	fmt.Printf("  \033[4;37mREASON:\033[0m %s\n", reason)
	panic("")
}

func chaosDebug(anything ...any) {
	pp := func(anything any) string {
		s, _ := json.MarshalIndent(anything, "", "\t")
		return string(s)
	}
	fmt.Printf("DEBUG:\n")
	for _, thing := range anything {
		fmt.Printf("%s\n", pp(thing))
	}
}

func todo[V any](args ...any) V {
	os.Exit(0)
	return *new(V)
}

func cast[V any](value any) (V, bool) {
	result, ok := value.(V)
	return result, ok
}

func castAssert[V any](value any) V {
	if result, ok := cast[V](value); ok {
		return result
	}
	return assert[V](false, fmt.Sprintf("Could not cast %T into %T", value, *new(V)))
}

func almostEqual(a, b float64) bool {
	const float64EqualityThreshold = 1e-9
	return math.Abs(a-b) <= float64EqualityThreshold
}
