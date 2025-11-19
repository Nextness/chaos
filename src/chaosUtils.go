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

const debug = true

var debugCall int = 0
var triggerCustomPanic bool = false
var printWarningOnce = false

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

func getFrames(skip int) (*runtime.Frames, int) {
	// Stacktrace size set to 20 for now
	progCounter := make([]uintptr, 20)
	// Skipping Callers, Caller of Callers
	invocationsCount := runtime.Callers(skip, progCounter)
	return runtime.CallersFrames(progCounter[:invocationsCount]), invocationsCount
}

func customPanic() {
	iter := 0
	printOnce := true
	frames, invocationsCount := getFrames(2)
	fmt.Print("\033[1;31m[ERROR] Assertion failed - stopping execution\033[0m\n")
	for true {
		frame, more := frames.Next()
		if !more {
			break
		}

		if iter == 0 || iter == 1 || iter == 2 || iter == 3 || iter == 4 || iter == 5 || iter == invocationsCount-2 {
			fmt.Fprintf(os.Stderr, "  %d. %s:%d %s\n", iter, filepath.Base(frame.File), frame.Line, frame.Function)
			iter++
			continue
		}

		if iter < invocationsCount-2 && printOnce {
			fmt.Fprintf(os.Stderr, "  ...\n")
			printOnce = false
			iter++
			continue
		}
	}
}

func warning(ok bool, msg string) {
	if ok {
		return
	}
	if !printWarningOnce {
		fmt.Print("\033[1;33m[WARNING] Something weird is happening - but not an error\033[0m\n")
		printWarningOnce = true
	}
	fmt.Printf("  \033[1;33mREASON:\033[0m %s\n", msg)

}

func assert[T any](ok bool, reason string) T {
	if ok {
		return *new(T)
	}
	triggerCustomPanic = true
	customPanic()
	if reason == "" {
		reason = "Reason not provided..."
	}
	fmt.Printf("  \033[4;37mREASON\033[0m: %s\n", reason)
	panic("")
}

func panicHandler() {
	if err := recover(); err != nil && debug {
		if !triggerCustomPanic {
			customPanic()
			fmt.Printf("  \033[4;37mREASON\033[0m: %v\n", err)
		}
		os.Exit(1)
	}
}

func chaosDebug(anything ...any) {
	pp := func(anything any) string {
		s, _ := json.MarshalIndent(anything, "", "\t")
		return string(s)
	}
	fmt.Printf("DEBUG %03d:\n", debugCall)
	for _, thing := range anything {
		fmt.Printf("%s\n", pp(thing))
	}
	debugCall++
}

func todo[V any](args ...any) V {
	chaosDebug(args...)
	os.Exit(1)
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
