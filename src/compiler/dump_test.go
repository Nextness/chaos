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

func TestDumpASTCoversImplementedContracts(t *testing.T) {
	source := "#import «math»;\nns :: #import «values»;\nBox <T: S64 | String> :: struct { value: T; count: S64 = 1; }\npair :: proc -> (S64, Bool) { return 1, true; }\nmain :: proc {\n    left, right := pair();\n    pending: *S64 = ...;\n    fixed := [2]S64.{1, 2};\n    dynamic := [dyn]S64.{1};\n    runtime := []S64.{1};\n    #deallocate pending;\n}"
	tokens, _ := Tokenize([]byte(source), 0)
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		t.Fatalf("parse errors: %v", result.Diags)
	}
	out := DumpAST(result.Program)
	for _, want := range []string{
		"ImportDecl module=\"math\"",
		"ImportDecl module=\"values\" namespace=ns",
		"StructDecl Box <T: Ident(S64) | Ident(String)>",
		"Field count: Ident(S64) = Int(1)",
		"Results: Ident(S64), Ident(Bool)",
		"ReturnStmt Int(1) Bool(true)",
		"MultiVarDecl left, right",
		"VarDecl pending: Pointer(*Ident(S64)) = ...",
		"ArrayType(fixed size=Int(2) elem=Ident(S64))",
		"ArrayType(dynamic elem=Ident(S64))",
		"ArrayType(runtime elem=Ident(S64))",
		"DeallocateStmt Ident(pending)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpAST output missing %q:\n%s", want, out)
		}
	}
	for _, fallback := range []string{"Decl *compiler.", "Stmt *compiler.", "Expr(*compiler."} {
		if strings.Contains(out, fallback) {
			t.Errorf("DumpAST used fallback %q:\n%s", fallback, out)
		}
	}
}
