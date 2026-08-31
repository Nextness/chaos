package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func parseProgramAtPath(t *testing.T, path, source string) *Program {
	t.Helper()
	tokens, tokenDiags := Tokenize([]byte(source), 0)
	if tokenDiags.HasErrors() {
		t.Fatalf("tokenize: %v", tokenDiags)
	}
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		t.Fatalf("parse: %v", result.Diags)
	}
	result.Program.Sources = map[FileID]SourceFile{
		0: {ID: 0, Path: path, Source: []byte(source), LineOffsets: BuildLineOffsets([]byte(source))},
	}
	return result.Program
}

func writeModule(t *testing.T, path, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir module directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}
}

func TestResolveImportsDistinguishesRepeatedBasenames(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, filepath.Join(dir, "left", "root.chaos"), "#import «shared»;\nleft_value :: proc -> S64 { return left_shared(); }")
	writeModule(t, filepath.Join(dir, "left", "shared.chaos"), "left_shared :: proc -> S64 { return 1; }")
	writeModule(t, filepath.Join(dir, "right", "root.chaos"), "#import «shared»;\nright_value :: proc -> S64 { return right_shared(); }")
	writeModule(t, filepath.Join(dir, "right", "shared.chaos"), "right_shared :: proc -> S64 { return 2; }")

	source := "#import «left/root»;\n#import «right/root»;\n#entry main :: proc -> S64 { return left_value() + right_value(); }"
	program := parseProgramAtPath(t, filepath.Join(dir, "main.chaos"), source)
	resolved, diags := ResolveImports(program, filepath.Join(dir, "missing-stdlib"))
	if diags.HasErrors() {
		t.Fatalf("resolve imports: %v", diags)
	}
	seen := make(map[string]bool)
	for _, decl := range resolved.Decls {
		if proc, ok := decl.(*ProcDecl); ok {
			seen[proc.Name] = true
		}
	}
	for _, name := range []string{"left_shared", "right_shared", "left_value", "right_value"} {
		if !seen[name] {
			t.Errorf("resolved declarations omit %s", name)
		}
	}
}

func TestResolveImportsDetectsDirectCycle(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, filepath.Join(dir, "cycle.chaos"), "#import «cycle»;")
	source := "#import «cycle»;\n#entry main :: proc -> S64 { return 0; }"
	program := parseProgramAtPath(t, filepath.Join(dir, "main.chaos"), source)
	_, diags := ResolveImports(program, filepath.Join(dir, "missing-stdlib"))
	if !hasError(diags, "cyclic import") {
		t.Fatalf("expected cyclic import diagnostic, got %v", diags)
	}
}

func TestNamespacedImportReceivesDeclarationValidation(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, filepath.Join(dir, "bad.chaos"), "Problems :: error { DUP; DUP; }\nBroken :: enum { FIRST; }\nNode :: struct { next: Node; }\nvalue: S64 = «wrong»;\nduplicate :: proc { }\nduplicate :: proc { }")
	source := "bad :: #import «bad»;\n#entry main :: proc -> S64 { return 0; }"
	program := parseProgramAtPath(t, filepath.Join(dir, "main.chaos"), source)
	previous := StdlibDir
	StdlibDir = dir
	t.Cleanup(func() { StdlibDir = previous })
	_, diags := AnalyzeProgram(program)
	for _, message := range []string{"duplicate error member", "first enum member must declare", "has infinite size", "cannot assign String to S64", "shadows an existing name"} {
		if !hasError(diags, message) {
			t.Errorf("expected %q diagnostic, got %v", message, diags)
		}
	}
}

func TestNamespacedImportGlobalLowersThroughModuleProcedure(t *testing.T) {
	dir := t.TempDir()
	writeModule(t, filepath.Join(dir, "math.chaos"), "base := 40;\nanswer :: proc -> S64 { return base + 2; }")
	source := "math :: #import «math»;\nlocal := 1;\n#entry main :: proc -> S64 { return math.answer() + local; }"
	program := parseProgramAtPath(t, filepath.Join(dir, "main.chaos"), source)
	previous := StdlibDir
	StdlibDir = dir
	t.Cleanup(func() { StdlibDir = previous })
	analysis, diags := AnalyzeProgram(program)
	if diags.HasErrors() {
		t.Fatalf("AnalyzeProgram diagnostics: %v", diags)
	}
	hir, diags := LowerAnalyzedProgram(program, analysis)
	if diags.HasErrors() {
		t.Fatalf("LowerAnalyzedProgram diagnostics: %v", diags)
	}
	globalCounts := make(map[string]int)
	for _, global := range hir.Globals {
		globalCounts[global.Name]++
	}
	if globalCounts["base"] != 1 || globalCounts["local"] != 1 || len(hir.Globals) != 2 {
		t.Fatalf("lowered globals = %+v, want one module and one root global", globalCounts)
	}
	mir, diags := LowerToMIR(hir)
	if diags.HasErrors() {
		t.Fatalf("LowerToMIR diagnostics: %v", diags)
	}
	if diags := VerifyMIR(mir); diags.HasErrors() {
		t.Fatalf("VerifyMIR diagnostics: %v", diags)
	}
}
