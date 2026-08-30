package compiler

import (
	"os"
	"testing"
)

// TestMain points the import resolver at the repository's stdlib directory.
// The compiler package tests run from src/compiler/, so the stdlib lives two
// levels up.
func TestMain(m *testing.M) {
	StdlibDir = "../../chaos-stdlib"
	os.Exit(m.Run())
}
