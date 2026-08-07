package compiler

import (
	"strings"
	"testing"
)

func TestDumpTokens(t *testing.T) {
	tokens, diags := Tokenize([]byte("x :: 42;"), 0)
	if diags.HasErrors() {
		t.Fatalf("unexpected tokenize errors: %v", diags)
	}
	out := DumpTokens(tokens)
	for _, want := range []string{"identifier", ":", "integer literal(42)", "span=[5,7)"} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpTokens output missing %q:\n%s", want, out)
		}
	}
}

func TestDumpAST(t *testing.T) {
	tokens, _ := Tokenize([]byte("main :: proc (n: S64) -> S64 {\n    return n;\n}"), 0)
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected parse errors: %v", result.Diags)
	}
	out := DumpAST(result.Program)
	for _, want := range []string{
		"Program",
		"ProcDecl main",
		"Param n: Ident(S64)",
		"Results: Ident(S64)",
		"ReturnStmt Ident(n)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpAST output missing %q:\n%s", want, out)
		}
	}
}

func TestDumpParseResult(t *testing.T) {
	tokens, _ := Tokenize([]byte("x := 1;"), 0)
	result := ParseProgram(tokens)
	out := DumpParseResult(result)
	if !strings.Contains(out, "Diagnostics:") || !strings.Contains(out, "(none)") {
		t.Errorf("DumpParseResult missing diagnostics section:\n%s", out)
	}
	if !strings.Contains(out, "VarDecl x = Int(1)") {
		t.Errorf("DumpParseResult missing VarDecl:\n%s", out)
	}
}
