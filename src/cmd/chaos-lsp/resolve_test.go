package main

import (
	"strings"
	"testing"

	"chaos_new/compiler"
)

// buildResolverFor tokenizes and parses source in tolerant mode and builds a
// resolver for it.
func buildResolverFor(t *testing.T, source string) *resolver {
	t.Helper()
	sm := &compiler.SourceManager{}
	fileID := sm.Register("test.chaos", []byte(source))
	sf := sm.Lookup(fileID)
	tokens, _ := compiler.Tokenize(sf.Source, fileID)
	result := compiler.ParseProgramTolerant(tokens)
	r := &resolver{sf: sf, uri: "file:///test.chaos", program: result.Program, tokens: tokens}
	r.buildScopes()
	r.collectOccurrences()
	return r
}

// offsetOf returns the byte offset of the first occurrence of substr.
func offsetOf(t *testing.T, source, substr string) int {
	t.Helper()
	idx := strings.Index(source, substr)
	if idx < 0 {
		t.Fatalf("substring %q not found in %q", substr, source)
	}
	return idx
}

const resolveSource = "add :: proc (a: S64, b: S64) -> S64 {\n\treturn a + b;\n}\nmain :: proc {\n\tx := 42;\n\ty := add(x, 1);\n\treturn;\n}"

func TestDefinitionResolvesLocalAndProc(t *testing.T) {
	r := buildResolverFor(t, resolveSource)

	// Definition on the local reference `x` in add(x, 1) resolves to x := 42.
	xRef := offsetOf(t, resolveSource, "add(x") + 4
	sym := r.definitionAt(xRef)
	if sym == nil || sym.name != "x" || sym.varDecl == nil {
		t.Fatalf("definition of x ref = %+v, want local VarDecl x", sym)
	}

	// Definition on the call `add` resolves to the top-level proc.
	addRef := offsetOf(t, resolveSource, "add(x")
	sym = r.definitionAt(addRef)
	if sym == nil || sym.name != "add" || sym.proc == nil {
		t.Fatalf("definition of add ref = %+v, want top-level proc add", sym)
	}

	// Definition on the param reference `a` resolves to the param.
	aRef := offsetOf(t, resolveSource, "a + b")
	sym = r.definitionAt(aRef)
	if sym == nil || sym.name != "a" || sym.param == nil {
		t.Fatalf("definition of a ref = %+v, want param a", sym)
	}

	// Definition on the decl name `add` resolves to itself.
	addDecl := offsetOf(t, resolveSource, "add :: proc")
	sym = r.definitionAt(addDecl)
	if sym == nil || sym.name != "add" || sym.proc == nil {
		t.Fatalf("definition of add decl = %+v, want itself", sym)
	}
}

func TestReferencesListsAllOccurrences(t *testing.T) {
	r := buildResolverFor(t, resolveSource)
	xRef := offsetOf(t, resolveSource, "add(x") + 4
	occs := r.referencesAt(xRef)
	// x occurrences: the decl `x := 42;` and the reference in add(x, 1).
	if len(occs) != 2 {
		t.Fatalf("got %d references for x, want 2", len(occs))
	}
	for _, occ := range occs {
		if occ.name != "x" {
			t.Errorf("occurrence name = %q, want x", occ.name)
		}
	}
}

func TestHoverReturnsSignature(t *testing.T) {
	r := buildResolverFor(t, resolveSource)

	addRef := offsetOf(t, resolveSource, "add(x")
	content := r.hoverAt(addRef)
	if !strings.Contains(content, "add(a: S64, b: S64) -> S64") {
		t.Errorf("hover = %q, want signature", content)
	}

	xDecl := offsetOf(t, resolveSource, "x := 42")
	content = r.hoverAt(xDecl)
	if !strings.Contains(content, "x : inferred") {
		t.Errorf("hover = %q, want x : inferred", content)
	}

	lit := offsetOf(t, resolveSource, "42")
	content = r.hoverAt(lit)
	if !strings.Contains(content, "42") {
		t.Errorf("hover = %q, want literal 42", content)
	}
}

func TestCompletionNamesInScope(t *testing.T) {
	r := buildResolverFor(t, resolveSource)
	pos := offsetOf(t, resolveSource, "return;") + len("return;")
	items := r.completionAt(pos)
	names := make(map[string]bool)
	for _, item := range items {
		names[item.Label] = true
	}
	for _, want := range []string{"add", "x", "y"} {
		if !names[want] {
			t.Errorf("completion missing %q", want)
		}
	}
	for _, not := range []string{"a", "b"} {
		if names[not] {
			t.Errorf("completion should not include %q", not)
		}
	}
}

func TestCompletionExcludesNamesAfterCursor(t *testing.T) {
	source := "main :: proc {\n\tx := 42;\n\ty := 1;\n}"
	r := buildResolverFor(t, source)
	pos := offsetOf(t, source, "x := 42;") + len("x := 42;")
	items := r.completionAt(pos)
	names := make(map[string]bool)
	for _, item := range items {
		names[item.Label] = true
	}
	if !names["x"] {
		t.Errorf("completion missing x (declared before cursor)")
	}
	if names["y"] {
		t.Errorf("completion should exclude y (declared after cursor)")
	}
}
