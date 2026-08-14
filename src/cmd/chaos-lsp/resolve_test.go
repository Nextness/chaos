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
	r := &resolver{sf: sf, uri: "file:///test.chaos", program: result.Program, tokens: tokens, errorMembers: make(map[string]map[string]*symbol)}
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

func TestReferencesRespectShadowedDeclarations(t *testing.T) {
	source := "main :: proc {\n    x := 1;\n    {\n        x := 2;\n        inner := x;\n    }\n    outer := x;\n}"
	r := buildResolverFor(t, source)

	innerRef := offsetOf(t, source, "inner := x") + len("inner := ")
	inner := r.referencesAt(innerRef)
	if len(inner) != 2 {
		t.Fatalf("inner x references = %d, want declaration and inner use", len(inner))
	}
	if sym := r.definitionAt(innerRef); sym == nil || sym.span.Start != offsetOf(t, source, "x := 2") {
		t.Fatalf("inner x definition = %+v, want inner declaration", sym)
	}

	outerRef := offsetOf(t, source, "outer := x") + len("outer := ")
	outer := r.referencesAt(outerRef)
	if len(outer) != 2 {
		t.Fatalf("outer x references = %d, want declaration and outer use", len(outer))
	}
	if sym := r.definitionAt(outerRef); sym == nil || sym.span.Start != offsetOf(t, source, "x := 1") {
		t.Fatalf("outer x definition = %+v, want outer declaration", sym)
	}
}

func TestDefinitionResolvesForwardGlobal(t *testing.T) {
	source := "main :: proc {\n    later();\n}\nlater :: proc { }"
	r := buildResolverFor(t, source)
	ref := offsetOf(t, source, "later();")
	sym := r.definitionAt(ref)
	if sym == nil || sym.proc == nil || sym.span.Start != offsetOf(t, source, "later :: proc") {
		t.Fatalf("forward definition = %+v, want later procedure", sym)
	}
}

func TestLoopAndUnlessBindingsResolve(t *testing.T) {
	source := "Some_Error :: error { BAD; }\nf :: proc -> (S64 <> Some_Error) { return 1; }\nmain :: proc {\n    arr := []S64.{1};\n    for idx, elem: arr {\n        value := idx + elem;\n    }\n    result := f() unless catch err { exit 1; }\n    final := result;\n}"
	r := buildResolverFor(t, source)

	for _, tt := range []struct {
		decl string
		ref  string
		add  int
	}{
		{decl: "idx, elem", ref: "idx + elem"},
		{decl: "elem: arr", ref: "idx + elem", add: len("idx + ")},
		{decl: "result := f", ref: "result;"},
	} {
		ref := offsetOf(t, source, tt.ref) + tt.add
		sym := r.definitionAt(ref)
		if sym == nil || sym.span.Start != offsetOf(t, source, tt.decl) {
			t.Errorf("definition at %q = %+v, want declaration %q", tt.ref, sym, tt.decl)
		}
	}

	bodyPos := offsetOf(t, source, "value := idx") + len("value := idx")
	items := r.completionAt(bodyPos)
	names := make(map[string]bool)
	for _, item := range items {
		names[item.Label] = true
	}
	for _, want := range []string{"idx", "elem"} {
		if !names[want] {
			t.Errorf("loop-body completion missing %q", want)
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

func TestHoverReturnsVoidErrorSignature(t *testing.T) {
	source := "Some_Error :: error { GENERIC; }\nf :: proc -> (Void <> Some_Error) { return; }\nmain :: proc { f() unless catch { return; } }"
	r := buildResolverFor(t, source)
	ref := offsetOf(t, source, "f() unless")
	content := r.hoverAt(ref)
	if !strings.Contains(content, "f() -> (Void <> Some_Error)") {
		t.Errorf("hover = %q, want full Void error-return signature", content)
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

func TestStructDefinitionHoverAndCompletion(t *testing.T) {
	source := "Something_New :: struct {\n\tfield1: String;\n\tfield2: U64;\n}\nmain :: proc {\n\tx := 1;\n}"
	r := buildResolverFor(t, source)

	// Definition on the struct name resolves to itself.
	structDecl := offsetOf(t, source, "Something_New :: struct")
	sym := r.definitionAt(structDecl)
	if sym == nil || sym.name != "Something_New" || sym.structDecl == nil {
		t.Fatalf("definition of struct decl = %+v, want struct symbol", sym)
	}

	// Hover on the struct name shows the struct signature.
	content := r.hoverAt(structDecl)
	if !strings.Contains(content, "Something_New :: struct") || !strings.Contains(content, "field1: String") {
		t.Errorf("hover = %q, want struct signature", content)
	}

	// The struct name is available for completion inside the proc body.
	pos := offsetOf(t, source, "x := 1;") + len("x := 1;")
	items := r.completionAt(pos)
	names := make(map[string]bool)
	for _, item := range items {
		names[item.Label] = true
	}
	if !names["Something_New"] {
		t.Errorf("completion missing struct name Something_New")
	}
}

func TestStructFieldDefinitionAndReferences(t *testing.T) {
	source := "Something_New :: struct {\n\tfield1: String;\n}"
	r := buildResolverFor(t, source)
	fieldRef := offsetOf(t, source, "field1")
	sym := r.definitionAt(fieldRef)
	if sym == nil || sym.name != "field1" {
		t.Fatalf("definition of field = %+v, want field1", sym)
	}
	occs := r.referencesAt(fieldRef)
	if len(occs) != 1 {
		t.Fatalf("references for field1 = %d, want 1", len(occs))
	}
}

func TestDefinitionResolvesInsideStructLiteral(t *testing.T) {
	// Identifiers used as struct literal field values must be collected as
	// occurrences so definition/references work on them.
	source := "Something_New :: struct {\n\tfield1: String;\n}\nmain :: proc {\n\tx := 42;\n\ty := Something_New.{field1=x};\n}"
	r := buildResolverFor(t, source)

	xRef := offsetOf(t, source, "field1=x") + len("field1=")
	sym := r.definitionAt(xRef)
	if sym == nil || sym.name != "x" || sym.varDecl == nil {
		t.Fatalf("definition of x ref inside struct literal = %+v, want local VarDecl x", sym)
	}

	// The struct literal type name resolves to the struct declaration.
	typeRef := offsetOf(t, source, "Something_New.{")
	sym = r.definitionAt(typeRef)
	if sym == nil || sym.name != "Something_New" || sym.structDecl == nil {
		t.Fatalf("definition of struct literal type = %+v, want struct symbol", sym)
	}
}

const errorSource = "Hash_Table_Error :: error {\n\tGENERIC;\n\tOUT_OF_MEMORY;\n\tNOT_FOUND;\n}\nmain :: proc {\n\terr: Hash_Table_Error = .OUT_OF_MEMORY!;\n\tif err == Hash_Table_Error.NOT_FOUND! { }\n}"

func TestErrorDefinitionHoverAndCompletion(t *testing.T) {
	r := buildResolverFor(t, errorSource)

	// Definition on the error type name resolves to itself.
	errDecl := offsetOf(t, errorSource, "Hash_Table_Error :: error")
	sym := r.definitionAt(errDecl)
	if sym == nil || sym.name != "Hash_Table_Error" || sym.errorDecl == nil {
		t.Fatalf("definition of error decl = %+v, want error type symbol", sym)
	}

	// Definition on the member in 'Hash_Table_Error.NOT_FOUND' resolves to
	// the member declaration.
	memberRef := offsetOf(t, errorSource, "Hash_Table_Error.NOT_FOUND") + len("Hash_Table_Error.")
	sym = r.definitionAt(memberRef)
	if sym == nil || sym.name != "NOT_FOUND" || sym.errorMember == nil {
		t.Fatalf("definition of member ref = %+v, want error member symbol", sym)
	}

	// Definition on the bare '.OUT_OF_MEMORY' resolves to the member
	// declaration (unique member name).
	bareRef := offsetOf(t, errorSource, ".OUT_OF_MEMORY") + 1
	sym = r.definitionAt(bareRef)
	if sym == nil || sym.name != "OUT_OF_MEMORY" || sym.errorMember == nil {
		t.Fatalf("definition of bare member ref = %+v, want error member symbol", sym)
	}

	// Hover on the error type shows the error signature.
	content := r.hoverAt(errDecl)
	if !strings.Contains(content, "Hash_Table_Error :: error") || !strings.Contains(content, "GENERIC") {
		t.Errorf("hover = %q, want error signature", content)
	}

	// Hover on a member shows 'Type.MEMBER : Type'.
	content = r.hoverAt(memberRef)
	if !strings.Contains(content, "Hash_Table_Error.NOT_FOUND : Hash_Table_Error") {
		t.Errorf("hover = %q, want member type info", content)
	}

	// Completion after 'Hash_Table_Error.' returns the members.
	pos := offsetOf(t, errorSource, "Hash_Table_Error.NOT_FOUND") + len("Hash_Table_Error.")
	items := r.completionAt(pos)
	names := make(map[string]bool)
	for _, item := range items {
		names[item.Label] = true
	}
	for _, want := range []string{"GENERIC", "OUT_OF_MEMORY", "NOT_FOUND"} {
		if !names[want] {
			t.Errorf("member completion missing %q", want)
		}
	}
}

func TestErrorReferences(t *testing.T) {
	r := buildResolverFor(t, errorSource)
	// References on OUT_OF_MEMORY: the declaration and the bare
	// '.OUT_OF_MEMORY' usage.
	ref := offsetOf(t, errorSource, "OUT_OF_MEMORY;\n\tNOT_FOUND")
	occs := r.referencesAt(ref)
	if len(occs) != 2 {
		t.Fatalf("references for OUT_OF_MEMORY = %d, want 2", len(occs))
	}
	for _, occ := range occs {
		if occ.name != "OUT_OF_MEMORY" {
			t.Errorf("occurrence name = %q, want OUT_OF_MEMORY", occ.name)
		}
	}
	// References on NOT_FOUND: the declaration and the explicit
	// 'Hash_Table_Error.NOT_FOUND' usage.
	ref = offsetOf(t, errorSource, "NOT_FOUND;\n}")
	occs = r.referencesAt(ref)
	if len(occs) != 2 {
		t.Fatalf("references for NOT_FOUND = %d, want 2", len(occs))
	}
}

func TestCatchBindingDefinitionAndReferences(t *testing.T) {
	source := "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc -> S64 {\n    r := f(-1) unless catch err {\n        if err == .GENERIC! {\n            return 1;\n        }\n        return 2;\n    }\n    return r;\n}"
	r := buildResolverFor(t, source)
	// Definition on the catch binding resolves to itself.
	bindRef := offsetOf(t, source, "catch err") + len("catch ")
	sym := r.definitionAt(bindRef)
	if sym == nil || sym.name != "err" {
		t.Fatalf("definition of catch binding = %+v, want err", sym)
	}
	// Definition on the use of err inside the body resolves to the binding.
	useRef := offsetOf(t, source, "err == .GENERIC!")
	sym = r.definitionAt(useRef)
	if sym == nil || sym.name != "err" {
		t.Fatalf("definition of err use = %+v, want err", sym)
	}
	// References on err: the binding and the use.
	occs := r.referencesAt(bindRef)
	if len(occs) != 2 {
		t.Fatalf("references for err = %d, want 2", len(occs))
	}
}
