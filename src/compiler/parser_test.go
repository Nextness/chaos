package compiler

import (
	"strconv"
	"strings"
	"testing"
)

// parseTestCase is a helper that tokenizes and parses a source string,
// returning the ParseResult. FileID is always 0.
func parseTestCase(t *testing.T, source string) ParseResult {
	t.Helper()
	tokens, diags := Tokenize([]byte(source), 0)
	result := ParseProgram(tokens)
	result.Diags = append(result.Diags, diags...)
	return result
}

// parseTolerantTestCase is a helper that tokenizes and parses a source string
// in tolerant mode, returning the ParseResult. FileID is always 0.
func parseTolerantTestCase(t *testing.T, source string) ParseResult {
	t.Helper()
	tokens, diags := Tokenize([]byte(source), 0)
	result := ParseProgramTolerant(tokens)
	result.Diags = append(result.Diags, diags...)
	return result
}

// parseOneDecl parses source and returns the first declaration, failing if
// there are errors or no declarations.
func parseOneDecl(t *testing.T, source string) Decl {
	t.Helper()
	result := parseTestCase(t, source)
	if result.Diags.HasErrors() {
		for _, d := range result.Diags {
			t.Logf("diagnostic: %s: %s", d.Severity, d.Message)
		}
		t.Fatalf("unexpected parse errors in %q", source)
	}
	if len(result.Program.Decls) == 0 {
		t.Fatalf("no declarations parsed from %q", source)
	}
	return result.Program.Decls[0]
}

// parseOneStmt parses source as a statement inside a procedure body and
// returns the first statement, failing if there are errors or no statements.
func parseOneStmt(t *testing.T, source string) Stmt {
	t.Helper()
	block := parseOneStmtBlock(t, source)
	if len(block.Stmts) == 0 {
		t.Fatalf("no statements parsed from %q", source)
	}
	return block.Stmts[0]
}

// parseOneStmtBlock parses source as a statement inside a procedure body
// and returns the block.
func parseOneStmtBlock(t *testing.T, source string) *BlockStmt {
	t.Helper()
	// Wrap in a dummy procedure declaration so statements are valid
	s := "dummy :: proc { " + source + " }"
	result := parseTestCase(t, s)
	if result.Diags.HasErrors() {
		for _, d := range result.Diags {
			t.Logf("diagnostic: %s: %s", d.Severity, d.Message)
		}
		t.Fatalf("unexpected parse errors in block %q", source)
	}
	if len(result.Program.Decls) == 0 {
		t.Fatalf("no declarations parsed from block %q", source)
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected ProcDecl, got %T", result.Program.Decls[0])
	}
	return proc.Body
}

// parseExpr parses source as an expression and returns it.
func parseExpr(t *testing.T, source string) Expr {
	t.Helper()
	// Wrap in a dummy proc body to parse the expression
	s := "dummy :: proc { _ := " + source + "; }"
	result := parseTestCase(t, s)
	if result.Diags.HasErrors() {
		for _, d := range result.Diags {
			t.Logf("diagnostic: %s: %s", d.Severity, d.Message)
		}
		t.Fatalf("unexpected parse errors in expression %q", source)
	}
	if len(result.Program.Decls) == 0 {
		t.Fatalf("no declarations parsed from %q", source)
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected ProcDecl, got %T", result.Program.Decls[0])
	}
	if len(proc.Body.Stmts) == 0 {
		t.Fatalf("no statements in proc body")
	}
	decl, ok := proc.Body.Stmts[0].(*VarDecl)
	if !ok {
		t.Fatalf("expected VarDecl, got %T", proc.Body.Stmts[0])
	}
	return decl.Init
}

// ──────────────────────────────────────────────
// Empty / trivial
// ──────────────────────────────────────────────

func TestParseEmpty(t *testing.T) {
	result := parseTestCase(t, "")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 0 {
		t.Fatalf("expected 0 decls, got %d", len(result.Program.Decls))
	}
}

func TestParseWhitespaceOnly(t *testing.T) {
	result := parseTestCase(t, "   \n\t  ")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
}

// ──────────────────────────────────────────────
// Variable declarations
// ──────────────────────────────────────────────

func TestParseCompileTimeVarDecl(t *testing.T) {
	decl := parseOneDecl(t, "x :: 42;")
	d, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	if d.Name != "x" {
		t.Errorf("Name = %q, want %q", d.Name, "x")
	}
	if !d.CompileTime {
		t.Error("CompileTime = false, want true")
	}
	if d.Mutable {
		t.Error("Mutable = true, want false")
	}
	if d.DeclType != nil {
		t.Errorf("DeclType = %v, want nil", d.DeclType)
	}
	if d.Init == nil {
		t.Fatal("Init = nil, want non-nil")
	}
	_, ok = d.Init.(*IntExpr)
	if !ok {
		t.Errorf("Init type = %T, want *IntExpr", d.Init)
	}
}

func TestParseInferVarDecl(t *testing.T) {
	decl := parseOneDecl(t, "x := 42;")
	d, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	if d.Name != "x" {
		t.Errorf("Name = %q, want %q", d.Name, "x")
	}
	if d.CompileTime {
		t.Error("CompileTime = true, want false")
	}
	if !d.Mutable {
		t.Error("Mutable = false, want true")
	}
}

func TestParseTypedVarDecl(t *testing.T) {
	decl := parseOneDecl(t, "x : S64 = 42;")
	d, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	if d.Name != "x" {
		t.Errorf("Name = %q, want %q", d.Name, "x")
	}
	if d.DeclType == nil {
		t.Fatal("DeclType = nil, want non-nil")
	}
	ident, ok := d.DeclType.(*IdentExpr)
	if !ok {
		t.Fatalf("DeclType type = %T, want *IdentExpr", d.DeclType)
	}
	if ident.Name != "S64" {
		t.Errorf("type name = %q, want %q", ident.Name, "S64")
	}
}

func TestParseTypedVarDeclNoInit(t *testing.T) {
	decl := parseOneDecl(t, "x : S64;")
	d, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	if d.Name != "x" {
		t.Errorf("Name = %q, want %q", d.Name, "x")
	}
	if d.Init != nil {
		t.Errorf("Init = %v, want nil", d.Init)
	}
}

func TestParseTypedCompileTimeVarDecl(t *testing.T) {
	// name : Type : value — compile-time constant with explicit type.
	decl := parseOneDecl(t, "x : S64 : 42;")
	d, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	if d.Name != "x" {
		t.Errorf("Name = %q, want %q", d.Name, "x")
	}
	if !d.CompileTime {
		t.Error("CompileTime = false, want true")
	}
	if d.Mutable {
		t.Error("Mutable = true, want false")
	}
	if d.DeclType == nil {
		t.Fatal("DeclType = nil, want non-nil")
	}
	ident, ok := d.DeclType.(*IdentExpr)
	if !ok {
		t.Fatalf("DeclType type = %T, want *IdentExpr", d.DeclType)
	}
	if ident.Name != "S64" {
		t.Errorf("type name = %q, want %q", ident.Name, "S64")
	}
	if d.Init == nil {
		t.Fatal("Init = nil, want non-nil")
	}
	_, ok = d.Init.(*IntExpr)
	if !ok {
		t.Errorf("Init type = %T, want *IntExpr", d.Init)
	}
}

func TestParseVarDeclStringInit(t *testing.T) {
	decl := parseOneDecl(t, `msg :: «hello»;`)
	d, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	str, ok := d.Init.(*StringExpr)
	if !ok {
		t.Fatalf("Init type = %T, want *StringExpr", d.Init)
	}
	if str.Value != "hello" {
		t.Errorf("string value = %q, want %q", str.Value, "hello")
	}
}

func TestParseVarDeclBoolInit(t *testing.T) {
	decl := parseOneDecl(t, "flag :: true;")
	d, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	b, ok := d.Init.(*BoolExpr)
	if !ok {
		t.Fatalf("Init type = %T, want *BoolExpr", d.Init)
	}
	if !b.Value {
		t.Error("bool value = false, want true")
	}
}

// ──────────────────────────────────────────────
// Procedure declarations
// ──────────────────────────────────────────────

func TestParseProcDeclNoParamsNoResults(t *testing.T) {
	decl := parseOneDecl(t, "main :: proc { return; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if d.Name != "main" {
		t.Errorf("Name = %q, want %q", d.Name, "main")
	}
	if len(d.Params) != 0 {
		t.Errorf("Params = %v, want empty", d.Params)
	}
	if len(d.Results) != 0 {
		t.Errorf("Results = %v, want empty", d.Results)
	}
	if d.Body == nil {
		t.Fatal("Body = nil")
	}
}

func TestParseProcDeclWithParams(t *testing.T) {
	decl := parseOneDecl(t, "add :: proc (a: S64, b: S64) -> S64 { return a + b; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if len(d.Params) != 2 {
		t.Fatalf("Params = %d, want 2", len(d.Params))
	}
	if d.Params[0].Name != "a" {
		t.Errorf("param[0].Name = %q, want %q", d.Params[0].Name, "a")
	}
	if d.Params[1].Name != "b" {
		t.Errorf("param[1].Name = %q, want %q", d.Params[1].Name, "b")
	}
	if len(d.Results) != 1 {
		t.Fatalf("Results = %d, want 1", len(d.Results))
	}
	result, ok := d.Results[0].(*IdentExpr)
	if !ok {
		t.Fatalf("Results[0] type = %T, want *IdentExpr", d.Results[0])
	}
	if result.Name != "S64" {
		t.Errorf("result type = %q, want %q", result.Name, "S64")
	}
}

func TestParseProcDeclNoParens(t *testing.T) {
	decl := parseOneDecl(t, "main :: proc -> S64 { return 0; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if len(d.Params) != 0 {
		t.Errorf("Params = %d, want 0", len(d.Params))
	}
	if len(d.Results) != 1 {
		t.Fatalf("Results = %d, want 1", len(d.Results))
	}
}

func TestParseProcDeclMultipleResults(t *testing.T) {
	decl := parseOneDecl(t, "divmod :: proc (a: S64, b: S64) -> (S64, S64) { return 0, 0; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if len(d.Results) != 2 {
		t.Fatalf("Results = %d, want 2", len(d.Results))
	}
}

// ──────────────────────────────────────────────
// Struct declarations
// ──────────────────────────────────────────────

func TestParseStructDecl(t *testing.T) {
	decl := parseOneDecl(t, "Something_New :: struct {\n\tfield1: String;\n\tfield2: U64;\n\tfield3: Bool;\n}")
	d, ok := decl.(*StructDecl)
	if !ok {
		t.Fatalf("expected *StructDecl, got %T", decl)
	}
	if d.Name != "Something_New" {
		t.Errorf("Name = %q, want %q", d.Name, "Something_New")
	}
	if len(d.Fields) != 3 {
		t.Fatalf("Fields = %d, want 3", len(d.Fields))
	}
	wantFields := []struct{ name, typ string }{
		{"field1", "String"},
		{"field2", "U64"},
		{"field3", "Bool"},
	}
	for i, wf := range wantFields {
		if d.Fields[i].Name != wf.name {
			t.Errorf("field[%d].Name = %q, want %q", i, d.Fields[i].Name, wf.name)
		}
		ident, ok := d.Fields[i].Type.(*IdentExpr)
		if !ok {
			t.Fatalf("field[%d].Type type = %T, want *IdentExpr", i, d.Fields[i].Type)
		}
		if ident.Name != wf.typ {
			t.Errorf("field[%d].Type name = %q, want %q", i, ident.Name, wf.typ)
		}
	}
}

func TestParseStructDeclNoTrailingSemicolon(t *testing.T) {
	// The struct declaration itself does not require a trailing semicolon;
	// only each field does.
	result := parseTestCase(t, "Foo :: struct { x: S64; }\nmain :: proc { return; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 2 {
		t.Fatalf("Decls = %d, want 2", len(result.Program.Decls))
	}
	if _, ok := result.Program.Decls[0].(*StructDecl); !ok {
		t.Fatalf("Decls[0] type = %T, want *StructDecl", result.Program.Decls[0])
	}
}

func TestParseStructDeclEmpty(t *testing.T) {
	decl := parseOneDecl(t, "Empty :: struct {}")
	d, ok := decl.(*StructDecl)
	if !ok {
		t.Fatalf("expected *StructDecl, got %T", decl)
	}
	if len(d.Fields) != 0 {
		t.Fatalf("Fields = %d, want 0", len(d.Fields))
	}
}

func TestParseStructDeclTrailingSemicolonTolerated(t *testing.T) {
	// A stray trailing semicolon after the closing brace is skipped.
	result := parseTestCase(t, "Foo :: struct { x: S64; };")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
}

func TestParseStructDeclMissingBrace(t *testing.T) {
	result := parseTestCase(t, "Foo :: struct x: S64;")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for missing '{' after struct, got none")
	}
}

func TestParseStructFieldMissingColon(t *testing.T) {
	result := parseTestCase(t, "Foo :: struct { x S64; }")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for missing ':' after field name, got none")
	}
}

func TestParseStructFieldMissingType(t *testing.T) {
	result := parseTestCase(t, "Foo :: struct { x: ; }")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for missing field type, got none")
	}
}

// ──────────────────────────────────────────────
// Error type declarations
// ──────────────────────────────────────────────

func TestParseErrorDecl(t *testing.T) {
	decl := parseOneDecl(t, "Hash_Table_Error :: error {\n\tGENERIC;\n\tOUT_OF_MEMORY;\n\tNOT_FOUND;\n\tOUT_OF_BOUNDS;\n}")
	d, ok := decl.(*ErrorDecl)
	if !ok {
		t.Fatalf("expected *ErrorDecl, got %T", decl)
	}
	if d.Name != "Hash_Table_Error" {
		t.Errorf("Name = %q, want %q", d.Name, "Hash_Table_Error")
	}
	if len(d.Members) != 4 {
		t.Fatalf("Members = %d, want 4", len(d.Members))
	}
	want := []string{"GENERIC", "OUT_OF_MEMORY", "NOT_FOUND", "OUT_OF_BOUNDS"}
	for i, w := range want {
		if d.Members[i].Name != w {
			t.Errorf("member[%d].Name = %q, want %q", i, d.Members[i].Name, w)
		}
	}
}

func TestParseErrorDeclExplicitAnnotation(t *testing.T) {
	// 'ident : Error : error {...}' is equivalent to 'ident :: error {...}'.
	decl := parseOneDecl(t, "Hash_Table_Error : Error : error {\n\tGENERIC;\n}")
	d, ok := decl.(*ErrorDecl)
	if !ok {
		t.Fatalf("expected *ErrorDecl, got %T", decl)
	}
	if d.Name != "Hash_Table_Error" {
		t.Errorf("Name = %q, want %q", d.Name, "Hash_Table_Error")
	}
	if len(d.Members) != 1 {
		t.Fatalf("Members = %d, want 1", len(d.Members))
	}
}

func TestParseErrorDeclEmpty(t *testing.T) {
	decl := parseOneDecl(t, "Empty :: error {}")
	d, ok := decl.(*ErrorDecl)
	if !ok {
		t.Fatalf("expected *ErrorDecl, got %T", decl)
	}
	if len(d.Members) != 0 {
		t.Fatalf("Members = %d, want 0", len(d.Members))
	}
}

func TestParseErrorDeclTrailingSemicolonTolerated(t *testing.T) {
	result := parseTestCase(t, "Foo :: error { A; };")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
}

func TestParseErrorDeclMissingBrace(t *testing.T) {
	result := parseTestCase(t, "Foo :: error A;")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for missing '{' after error, got none")
	}
}

func TestParseErrorDeclExplicitValueRejected(t *testing.T) {
	// Error values are numbered sequentially from 0; 'A = 1' is an error.
	result := parseTestCase(t, "Foo :: error { A = 1; }")
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for explicit error value, got none")
	}
	if result.Diags[0].Message != "error values cannot be explicitly assigned; they are numbered sequentially from 0" {
		t.Errorf("diagnostic message = %q, want explicit-value message", result.Diags[0].Message)
	}
}

func TestParseErrorDeclInferRejected(t *testing.T) {
	// 'ident := error {...}' is rejected: error types are compile-time only.
	result := parseTestCase(t, "Foo := error { A; }")
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for ':=' error declaration, got none")
	}
	if result.Diags[0].Message != "error types must be declared at compile time; use '::' or ': Error :'" {
		t.Errorf("diagnostic message = %q, want compile-time message", result.Diags[0].Message)
	}
	// The declaration is still parsed for recovery.
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1 (recovered ErrorDecl)", len(result.Program.Decls))
	}
	if _, ok := result.Program.Decls[0].(*ErrorDecl); !ok {
		t.Fatalf("Decls[0] type = %T, want *ErrorDecl", result.Program.Decls[0])
	}
}

func TestParseErrorDeclAssignRejected(t *testing.T) {
	// 'ident : Error = error {...}' is rejected: error types are
	// compile-time only.
	result := parseTestCase(t, "Foo : Error = error { A; }")
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for ': Error =' error declaration, got none")
	}
	if result.Diags[0].Message != "error types must be declared at compile time; use '::' or ': Error :'" {
		t.Errorf("diagnostic message = %q, want compile-time message", result.Diags[0].Message)
	}
}

func TestParseErrorDeclWrongAnnotation(t *testing.T) {
	// The annotation in 'ident : X : error {...}' must be 'Error'.
	result := parseTestCase(t, "Foo : S64 : error { A; }")
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for non-Error annotation, got none")
	}
	if result.Diags[0].Message != "error type annotation must be 'Error'" {
		t.Errorf("diagnostic message = %q, want annotation message", result.Diags[0].Message)
	}
}

func TestParseErrorMemberExpr(t *testing.T) {
	expr := parseExpr(t, "Hash_Table_Error.NOT_FOUND!")
	e, ok := expr.(*ErrorMemberExpr)
	if !ok {
		t.Fatalf("expected *ErrorMemberExpr, got %T", expr)
	}
	if e.TypeName != "Hash_Table_Error" {
		t.Errorf("TypeName = %q, want %q", e.TypeName, "Hash_Table_Error")
	}
	if e.Name != "NOT_FOUND" {
		t.Errorf("Name = %q, want %q", e.Name, "NOT_FOUND")
	}
	if !e.Bang {
		t.Error("Bang = false, want true for error literal")
	}
}

func TestParseBareErrorMemberExpr(t *testing.T) {
	expr := parseExpr(t, ".NOT_FOUND!")
	e, ok := expr.(*ErrorMemberExpr)
	if !ok {
		t.Fatalf("expected *ErrorMemberExpr, got %T", expr)
	}
	if e.TypeName != "" {
		t.Errorf("TypeName = %q, want empty for bare member", e.TypeName)
	}
	if e.Name != "NOT_FOUND" {
		t.Errorf("Name = %q, want %q", e.Name, "NOT_FOUND")
	}
	if !e.Bang {
		t.Error("Bang = false, want true for error literal")
	}
}

func TestParseErrorMemberExprNoBang(t *testing.T) {
	// A member reference without '!' parses as a field access; the type
	// checker reports the missing '!' when the base names an error type.
	expr := parseExpr(t, "Hash_Table_Error.NOT_FOUND")
	e, ok := expr.(*FieldAccessExpr)
	if !ok {
		t.Fatalf("expected *FieldAccessExpr, got %T", expr)
	}
	if ident, ok := e.Base.(*IdentExpr); !ok || ident.Name != "Hash_Table_Error" {
		t.Errorf("Base = %v, want identifier Hash_Table_Error", e.Base)
	}
	if e.Field != "NOT_FOUND" {
		t.Errorf("Field = %q, want %q", e.Field, "NOT_FOUND")
	}
}

func TestParseErrorMemberExprInComparison(t *testing.T) {
	expr := parseExpr(t, "Hash_Table_Error.NOT_FOUND! == Hash_Table_Error.GENERIC!")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpEq {
		t.Errorf("Op = %d, want %d (BinaryOpEq)", e.Op, BinaryOpEq)
	}
	if _, ok := e.Left.(*ErrorMemberExpr); !ok {
		t.Errorf("left type = %T, want *ErrorMemberExpr", e.Left)
	}
	if _, ok := e.Right.(*ErrorMemberExpr); !ok {
		t.Errorf("right type = %T, want *ErrorMemberExpr", e.Right)
	}
}

// ──────────────────────────────────────────────
// Error-returning procedure results ('<>')
// ──────────────────────────────────────────────

func TestParseProcErrorReturn(t *testing.T) {
	decl := parseOneDecl(t, "f :: proc -> String <> Some_Error { return «s»; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if len(d.Results) != 1 {
		t.Fatalf("Results = %d, want 1", len(d.Results))
	}
	if ident, ok := d.Results[0].(*IdentExpr); !ok || ident.Name != "String" {
		t.Errorf("Results[0] = %v, want String", d.Results[0])
	}
	if d.ErrorResult == nil {
		t.Fatal("ErrorResult = nil, want Some_Error")
	}
	if ident, ok := d.ErrorResult.(*IdentExpr); !ok || ident.Name != "Some_Error" {
		t.Errorf("ErrorResult = %v, want Some_Error", d.ErrorResult)
	}
}

func TestParseProcErrorReturnParens(t *testing.T) {
	decl := parseOneDecl(t, "f :: proc (input1: String) -> (String <> Some_Error) { return «s»; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if len(d.Results) != 1 {
		t.Fatalf("Results = %d, want 1", len(d.Results))
	}
	if d.ErrorResult == nil {
		t.Fatal("ErrorResult = nil, want Some_Error")
	}
	if ident, ok := d.ErrorResult.(*IdentExpr); !ok || ident.Name != "Some_Error" {
		t.Errorf("ErrorResult = %v, want Some_Error", d.ErrorResult)
	}
}

func TestParseProcErrorReturnSwapped(t *testing.T) {
	// 'Some_Error <> String' is the same as 'String <> Some_Error'; the
	// written order is preserved in the AST and normalized by the type
	// checker.
	decl := parseOneDecl(t, "f :: proc -> Some_Error <> String { return «s»; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if ident, ok := d.Results[0].(*IdentExpr); !ok || ident.Name != "Some_Error" {
		t.Errorf("Results[0] = %v, want Some_Error", d.Results[0])
	}
	if ident, ok := d.ErrorResult.(*IdentExpr); !ok || ident.Name != "String" {
		t.Errorf("ErrorResult = %v, want String", d.ErrorResult)
	}
}

func TestParseProcErrorReturnNoValue(t *testing.T) {
	// '-> Some_Error' without '<>' is a plain error-typed result.
	decl := parseOneDecl(t, "f :: proc -> Some_Error { return .GENERIC!; }")
	d, ok := decl.(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", decl)
	}
	if len(d.Results) != 1 {
		t.Fatalf("Results = %d, want 1", len(d.Results))
	}
	if d.ErrorResult != nil {
		t.Errorf("ErrorResult = %v, want nil without '<>'", d.ErrorResult)
	}
}

func TestParseProcErrorReturnMissingErrorType(t *testing.T) {
	result := parseTestCase(t, "f :: proc -> String <> { return «s»; }")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for missing error type after '<>', got none")
	}
}

func TestParseParenthesizedProcedureResults(t *testing.T) {
	result := parseTestCase(t, "f :: proc -> (S64) { return 0; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected parenthesized-result errors: %v", result.Diags)
	}
	proc := result.Program.Decls[0].(*ProcDecl)
	if len(proc.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(proc.Results))
	}
}

// ──────────────────────────────────────────────
// Assignments
// ──────────────────────────────────────────────

func TestParseAssignStmt(t *testing.T) {
	stmt := parseOneStmt(t, "x = 42;")
	a, ok := stmt.(*AssignStmt)
	if !ok {
		t.Fatalf("expected *AssignStmt, got %T", stmt)
	}
	if a.Name != "x" {
		t.Errorf("Name = %q, want %q", a.Name, "x")
	}
}

// ──────────────────────────────────────────────
// Return statements
// ──────────────────────────────────────────────

func TestParseReturnVoid(t *testing.T) {
	stmt := parseOneStmt(t, "return;")
	r, ok := stmt.(*ReturnStmt)
	if !ok {
		t.Fatalf("expected *ReturnStmt, got %T", stmt)
	}
	if r.Value != nil {
		t.Errorf("Value = %v, want nil", r.Value)
	}
}

func TestParseReturnExpr(t *testing.T) {
	stmt := parseOneStmt(t, "return 42;")
	r, ok := stmt.(*ReturnStmt)
	if !ok {
		t.Fatalf("expected *ReturnStmt, got %T", stmt)
	}
	if r.Value == nil {
		t.Fatal("Value = nil, want non-nil")
	}
}

// ──────────────────────────────────────────────
// Exit statements
// ──────────────────────────────────────────────

func TestParseExitStatus(t *testing.T) {
	stmt := parseOneStmt(t, "exit 0;")
	e, ok := stmt.(*ExitStmt)
	if !ok {
		t.Fatalf("expected *ExitStmt, got %T", stmt)
	}
	if e.Status == nil {
		t.Fatal("Status = nil")
	}
	if e.Message != nil {
		t.Errorf("Message = %v, want nil", e.Message)
	}
}

func TestParseExitStatusAndMessage(t *testing.T) {
	stmt := parseOneStmt(t, `exit 1, «error»;`)
	e, ok := stmt.(*ExitStmt)
	if !ok {
		t.Fatalf("expected *ExitStmt, got %T", stmt)
	}
	if e.Status == nil {
		t.Fatal("Status = nil")
	}
	if e.Message == nil {
		t.Fatal("Message = nil, want non-nil")
	}
}

// ──────────────────────────────────────────────
// If statements
// ──────────────────────────────────────────────

func TestParseIf(t *testing.T) {
	stmt := parseOneStmt(t, "if true { exit 0; }")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if ifs.Condition == nil {
		t.Fatal("Condition = nil")
	}
	if len(ifs.Elif) != 0 {
		t.Errorf("Elif = %v, want empty", ifs.Elif)
	}
	if ifs.ElseBody != nil {
		t.Errorf("ElseBody = %v, want nil", ifs.ElseBody)
	}
}

func TestParseIfElse(t *testing.T) {
	stmt := parseOneStmt(t, "if true { exit 0; } else { exit 1; }")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if ifs.ElseBody == nil {
		t.Fatal("ElseBody = nil, want non-nil")
	}
}

func TestParseIfElifElse(t *testing.T) {
	stmt := parseOneStmt(t, `
		if a < b { exit 0; }
		elif a > b { exit 1; }
		else { exit 2; }
	`)
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if len(ifs.Elif) != 1 {
		t.Fatalf("Elif = %d, want 1", len(ifs.Elif))
	}
	if ifs.ElseBody == nil {
		t.Fatal("ElseBody = nil, want non-nil")
	}
}

func TestParseIfMultipleElif(t *testing.T) {
	stmt := parseOneStmt(t, `
		if a == 1 { exit 1; }
		elif a == 2 { exit 2; }
		elif a == 3 { exit 3; }
		else { exit 0; }
	`)
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if len(ifs.Elif) != 2 {
		t.Fatalf("Elif = %d, want 2", len(ifs.Elif))
	}
}

func TestParseIfThenStmt(t *testing.T) {
	// Single-line if with 'then': the body is a single statement wrapped in a
	// block.
	stmt := parseOneStmt(t, "if true then exit 0;")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if len(ifs.Body.Stmts) != 1 {
		t.Fatalf("Body.Stmts = %d, want 1", len(ifs.Body.Stmts))
	}
	if _, ok := ifs.Body.Stmts[0].(*ExitStmt); !ok {
		t.Fatalf("body stmt type = %T, want *ExitStmt", ifs.Body.Stmts[0])
	}
}

func TestParseIfBareStmt(t *testing.T) {
	// 'then' is optional: a single statement directly after the condition is
	// the body.
	stmt := parseOneStmt(t, "if true exit 0;")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if len(ifs.Body.Stmts) != 1 {
		t.Fatalf("Body.Stmts = %d, want 1", len(ifs.Body.Stmts))
	}
	if _, ok := ifs.Body.Stmts[0].(*ExitStmt); !ok {
		t.Fatalf("body stmt type = %T, want *ExitStmt", ifs.Body.Stmts[0])
	}
}

func TestParseIfThenStmtNextStmtSeparate(t *testing.T) {
	// A statement after the single-line if body is not part of the if.
	block := parseOneStmtBlock(t, "if true then exit 0; x := 1;")
	if len(block.Stmts) != 2 {
		t.Fatalf("Stmts = %d, want 2", len(block.Stmts))
	}
	ifs, ok := block.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("stmt 0 type = %T, want *IfStmt", block.Stmts[0])
	}
	if len(ifs.Body.Stmts) != 1 {
		t.Fatalf("Body.Stmts = %d, want 1", len(ifs.Body.Stmts))
	}
	if _, ok := block.Stmts[1].(*VarDecl); !ok {
		t.Fatalf("stmt 1 type = %T, want *VarDecl", block.Stmts[1])
	}
}

func TestParseIfThenBlockWarning(t *testing.T) {
	// 'then' before a block is accepted but warns.
	source := "dummy :: proc { if true then { exit 0; } }"
	result := parseTestCase(t, source)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	found := false
	for _, d := range result.Diags {
		if d.Severity == SeverityWarning && d.Message == "'then' is not needed before a block" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for 'then' before a block, got %v", result.Diags)
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	ifs, ok := proc.Body.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", proc.Body.Stmts[0])
	}
	if len(ifs.Body.Stmts) != 1 {
		t.Fatalf("Body.Stmts = %d, want 1", len(ifs.Body.Stmts))
	}
}

func TestParseIfThenElifBlock(t *testing.T) {
	// A 'then' body followed by an elif with a block.
	stmt := parseOneStmt(t, "if a then return 1; elif b { return 2; }")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if len(ifs.Elif) != 1 {
		t.Fatalf("Elif = %d, want 1", len(ifs.Elif))
	}
	if len(ifs.Elif[0].Body.Stmts) != 1 {
		t.Fatalf("elif Body.Stmts = %d, want 1", len(ifs.Elif[0].Body.Stmts))
	}
}

func TestParseIfThenElifThenElse(t *testing.T) {
	// Full single-line chain: if then, elif bare, else bare.
	stmt := parseOneStmt(t, "if a then return 1; elif b return 2; else return 3;")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if len(ifs.Elif) != 1 {
		t.Fatalf("Elif = %d, want 1", len(ifs.Elif))
	}
	if len(ifs.Elif[0].Body.Stmts) != 1 {
		t.Fatalf("elif Body.Stmts = %d, want 1", len(ifs.Elif[0].Body.Stmts))
	}
	if ifs.ElseBody == nil {
		t.Fatal("ElseBody = nil, want non-nil")
	}
	if len(ifs.ElseBody.Stmts) != 1 {
		t.Fatalf("ElseBody.Stmts = %d, want 1", len(ifs.ElseBody.Stmts))
	}
}

func TestParseElifThenStmt(t *testing.T) {
	// 'then' is also optional for elif single-line bodies.
	stmt := parseOneStmt(t, "if a { } elif b then return 2;")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if len(ifs.Elif) != 1 {
		t.Fatalf("Elif = %d, want 1", len(ifs.Elif))
	}
	if len(ifs.Elif[0].Body.Stmts) != 1 {
		t.Fatalf("elif Body.Stmts = %d, want 1", len(ifs.Elif[0].Body.Stmts))
	}
}

func TestParseElseBareStmt(t *testing.T) {
	// 'else' takes a bare single statement without 'then'.
	stmt := parseOneStmt(t, "if a { } else return 2;")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if ifs.ElseBody == nil {
		t.Fatal("ElseBody = nil, want non-nil")
	}
	if len(ifs.ElseBody.Stmts) != 1 {
		t.Fatalf("ElseBody.Stmts = %d, want 1", len(ifs.ElseBody.Stmts))
	}
}

func TestParseElseThenError(t *testing.T) {
	// 'then' is never used with 'else'.
	source := "dummy :: proc { if a { } else then return 2; }"
	result := parseTestCase(t, source)
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for 'else then', got none")
	}
	found := false
	for _, d := range result.Diags {
		if d.Severity == SeverityError && d.Message == "'then' is not used with 'else'" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'then is not used with else' error, got %v", result.Diags)
	}
	// The statement after 'then' is still parsed for recovery.
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	ifs, ok := proc.Body.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", proc.Body.Stmts[0])
	}
	if ifs.ElseBody == nil || len(ifs.ElseBody.Stmts) != 1 {
		t.Fatalf("ElseBody = %#v, want one statement", ifs.ElseBody)
	}
}

func TestParseIfParenCond(t *testing.T) {
	// Parenthesized conditions are valid.
	stmt := parseOneStmt(t, "if (a) { exit 0; }")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if _, ok := ifs.Condition.(*ParenExpr); !ok {
		t.Fatalf("Condition type = %T, want *ParenExpr", ifs.Condition)
	}

	stmt = parseOneStmt(t, "if ((a > 1) && a < 10) then return 12;")
	ifs, ok = stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	if _, ok := ifs.Condition.(*ParenExpr); !ok {
		t.Fatalf("Condition type = %T, want *ParenExpr", ifs.Condition)
	}
	if len(ifs.Body.Stmts) != 1 {
		t.Fatalf("Body.Stmts = %d, want 1", len(ifs.Body.Stmts))
	}
}

func TestParseIfThenTolerant(t *testing.T) {
	// The language server parses in tolerant mode; the single-line forms must
	// parse there too.
	source := "main :: proc -> S64 {\n" +
		"    if a then return 1;\n" +
		"    elif b return 2;\n" +
		"    else return 3;\n" +
		"}"
	result := parseTolerantTestCase(t, source)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors in tolerant mode: %v", result.Diags)
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	ifs, ok := proc.Body.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", proc.Body.Stmts[0])
	}
	if len(ifs.Elif) != 1 || ifs.ElseBody == nil {
		t.Fatalf("Elif = %d, ElseBody = %v, want 1 elif and an else", len(ifs.Elif), ifs.ElseBody)
	}
}

// ──────────────────────────────────────────────
// Ifx expressions
// ──────────────────────────────────────────────

func parseIfxFromInit(t *testing.T, init string) *IfxExpr {
	t.Helper()
	expr := parseExpr(t, init)
	ifx, ok := expr.(*IfxExpr)
	if !ok {
		t.Fatalf("expected *IfxExpr, got %T", expr)
	}
	return ifx
}

func TestParseIfxThenElse(t *testing.T) {
	ifx := parseIfxFromInit(t, "ifx cond then 1 else 2")
	if _, ok := ifx.Condition.(*IdentExpr); !ok {
		t.Fatalf("Condition = %T, want *IdentExpr", ifx.Condition)
	}
	if no, ok := ifx.Then.(*IntExpr); !ok || no.Value != "1" {
		t.Fatalf("Then = %#v, want Int(1)", ifx.Then)
	}
	if no, ok := ifx.Else.(*IntExpr); !ok || no.Value != "2" {
		t.Fatalf("Else = %#v, want Int(2)", ifx.Else)
	}
}

func TestParseIfxBareThen(t *testing.T) {
	// 'then' is optional: "ifx cond a else b".
	ifx := parseIfxFromInit(t, "ifx cond 1 else 2")
	if no, ok := ifx.Then.(*IntExpr); !ok || no.Value != "1" {
		t.Fatalf("Then = %#v, want Int(1)", ifx.Then)
	}
	if no, ok := ifx.Else.(*IntExpr); !ok || no.Value != "2" {
		t.Fatalf("Else = %#v, want Int(2)", ifx.Else)
	}
}

func TestParseIfxElseExtends(t *testing.T) {
	// The else branch is a full expression and extends to the end.
	expr := parseExpr(t, "ifx c then 1 else b + 1")
	ifx, ok := expr.(*IfxExpr)
	if !ok {
		t.Fatalf("expected *IfxExpr, got %T", expr)
	}
	if _, ok := ifx.Else.(*BinaryExpr); !ok {
		t.Fatalf("Else = %T, want *BinaryExpr", ifx.Else)
	}
}

func TestParseIfxNested(t *testing.T) {
	// Nested ifx parses: the inner expression consumes its own else first.
	expr := parseExpr(t, "ifx c1 then ifx c2 then 1 else 2 else 3")
	ifx, ok := expr.(*IfxExpr)
	if !ok {
		t.Fatalf("expected *IfxExpr, got %T", expr)
	}
	if _, ok := ifx.Then.(*IfxExpr); !ok {
		t.Fatalf("Then = %T, want *IfxExpr", ifx.Then)
	}
	if no, ok := ifx.Else.(*IntExpr); !ok || no.Value != "3" {
		t.Fatalf("Else = %#v, want Int(3)", ifx.Else)
	}
}

func TestParseIfxMissingElse(t *testing.T) {
	source := "dummy :: proc { x := ifx true then 1; }"
	result := parseTestCase(t, source)
	if !hasError(result.Diags, "expected 'else' in ifx expression") {
		t.Errorf("expected missing-else error, got %v", result.Diags)
	}
}

func TestParseIfxMissingThenValue(t *testing.T) {
	// "ifx cond else value" has no true branch.
	source := "dummy :: proc { x := ifx true else 2; }"
	result := parseTestCase(t, source)
	if !hasError(result.Diags, "expected a value after the condition in ifx expression") {
		t.Errorf("expected missing-then-value error, got %v", result.Diags)
	}
}

func TestParseIfxAsExprStmt(t *testing.T) {
	// An ifx used as a bare statement parses as an expression statement; the
	// type checker rejects it with a precise diagnostic.
	stmt := parseOneStmt(t, "ifx true then 1 else 2;")
	exprStmt, ok := stmt.(*ExprStmt)
	if !ok {
		t.Fatalf("expected *ExprStmt, got %T", stmt)
	}
	if _, ok := exprStmt.Expr.(*IfxExpr); !ok {
		t.Fatalf("Expr = %T, want *IfxExpr", exprStmt.Expr)
	}
}

func TestParsePointerType(t *testing.T) {
	// "*T" and the recursive forms parse; "?" binds to the nearest pointer.
	decl := parseOneDecl(t, "p: *S64;")
	v, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	pt, ok := v.DeclType.(*PointerTypeExpr)
	if !ok {
		t.Fatalf("DeclType = %T, want *PointerTypeExpr", v.DeclType)
	}
	if pt.Nullable {
		t.Error("Nullable = true, want false for *S64")
	}

	decl = parseOneDecl(t, "p: *S64?;")
	v, _ = decl.(*VarDecl)
	pt, _ = v.DeclType.(*PointerTypeExpr)
	if !pt.Nullable {
		t.Error("Nullable = false, want true for *S64?")
	}

	decl = parseOneDecl(t, "p: **S64;")
	v, _ = decl.(*VarDecl)
	outer, ok := v.DeclType.(*PointerTypeExpr)
	if !ok {
		t.Fatalf("DeclType = %T, want *PointerTypeExpr", v.DeclType)
	}
	if _, ok := outer.Elem.(*PointerTypeExpr); !ok {
		t.Fatalf("Elem = %T, want *PointerTypeExpr", outer.Elem)
	}

	decl = parseOneDecl(t, "items: []*S64;")
	v, _ = decl.(*VarDecl)
	arr, ok := v.DeclType.(*ArrayTypeExpr)
	if !ok {
		t.Fatalf("DeclType = %T, want *ArrayTypeExpr", v.DeclType)
	}
	if _, ok := arr.Elem.(*PointerTypeExpr); !ok {
		t.Fatalf("Elem = %T, want *PointerTypeExpr", arr.Elem)
	}
}

func TestParseNonPointerNullableType(t *testing.T) {
	// 'T?' without a pointer is an error; 'null' is a keyword literal.
	source := "dummy :: proc { x: S64?; }"
	result := parseTestCase(t, source)
	if !hasError(result.Diags, "only pointer types can be nullable") {
		t.Errorf("expected nullable-error, got %v", result.Diags)
	}
}

func TestParsePointerSyntaxErrors(t *testing.T) {
	// A '*' with no element type and a '.' that is not followed by '*' or a
	// field name are parse errors.
	result := parseTestCase(t, "dummy :: proc { x: *; }")
	if !hasError(result.Diags, "expected pointer element type after '*'") {
		t.Errorf("expected missing element error, got %v", result.Diags)
	}

	result = parseTestCase(t, "dummy :: proc { x := p.; }")
	if !hasError(result.Diags, "expected '*', '(', or a field name after '.'") {
		t.Errorf("expected dangling-dot error, got %v", result.Diags)
	}
}

func TestParseAddressOfDerefAndField(t *testing.T) {
	// Prefix '*' is address-of; postfix '.*' is dereference; '.field' reads a
	// member. Chaining composes as expected.
	expr := parseExpr(t, "a.*")
	deref, ok := expr.(*DerefExpr)
	if !ok {
		t.Fatalf("expected *DerefExpr for a.*, got %T", expr)
	}
	if id, ok := deref.Operand.(*IdentExpr); !ok || id.Name != "a" {
		t.Fatalf("Deref.Operand = %#v, want Ident(a)", deref.Operand)
	}

	expr = parseExpr(t, "*a")
	unary, ok := expr.(*UnaryExpr)
	if !ok || unary.Op != UnaryOpAddr {
		t.Fatalf("expected Unary(*a), got %#v", expr)
	}

	expr = parseExpr(t, "p.*.field")
	field, ok := expr.(*FieldAccessExpr)
	if !ok {
		t.Fatalf("expected *FieldAccessExpr, got %T", expr)
	}
	if field.Field != "field" {
		t.Fatalf("Field = %q, want %q", field.Field, "field")
	}
	if _, ok := field.Base.(*DerefExpr); !ok {
		t.Fatalf("Field.Base = %T, want *DerefExpr", field.Base)
	}

	// An identifier followed by a member parses as a field access; the type
	// checker decides whether it is an enum member or a struct field.
	expr = parseExpr(t, "Type.MEMBER")
	field, ok = expr.(*FieldAccessExpr)
	if !ok {
		t.Fatalf("expected *FieldAccessExpr for Type.MEMBER, got %T", expr)
	}
	if id, ok := field.Base.(*IdentExpr); !ok || id.Name != "Type" {
		t.Fatalf("Base = %#v, want Ident(Type)", field.Base)
	}

	expr = parseExpr(t, "null")
	if _, ok := expr.(*NullLitExpr); !ok {
		t.Fatalf("expected *NullLitExpr, got %T", expr)
	}
}

func TestParseIfxTolerant(t *testing.T) {
	source := "dummy :: proc { x := ifx true then 1 else 2; }"
	result := parseTolerantTestCase(t, source)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected tolerant-mode errors: %v", result.Diags)
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	decl, ok := proc.Body.Stmts[0].(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", proc.Body.Stmts[0])
	}
	if _, ok := decl.Init.(*IfxExpr); !ok {
		t.Fatalf("Init = %T, want *IfxExpr", decl.Init)
	}
}

// ──────────────────────────────────────────────
// For loops
// ──────────────────────────────────────────────

func TestParseForWhile(t *testing.T) {
	// "for true { ... }" is a while-style loop.
	stmt := parseOneStmt(t, "for true { exit 0; }")
	fs, ok := stmt.(*ForStmt)
	if !ok {
		t.Fatalf("expected *ForStmt, got %T", stmt)
	}
	if fs.Cond == nil {
		t.Fatal("Cond = nil")
	}
	if fs.Range != nil {
		t.Fatal("Range = nil expected for while form")
	}
	if fs.Init != nil || fs.After != nil {
		t.Fatal("Init/After must be nil for while form")
	}
	if len(fs.Body.Stmts) != 1 {
		t.Fatalf("Body.Stmts = %d, want 1", len(fs.Body.Stmts))
	}
}

func TestParseForCFor(t *testing.T) {
	stmt := parseOneStmt(t, "for a := 0; a != 10; a += 1 { exit 0; }")
	fs, ok := stmt.(*ForStmt)
	if !ok {
		t.Fatalf("expected *ForStmt, got %T", stmt)
	}
	if fs.Init == nil {
		t.Fatal("Init = nil")
	}
	if _, ok := fs.Init.(*VarDecl); !ok {
		t.Fatalf("Init type = %T, want *VarDecl", fs.Init)
	}
	if fs.Cond == nil {
		t.Fatal("Cond = nil")
	}
	if _, ok := fs.After.(*CompoundAssignStmt); !ok {
		t.Fatalf("After type = %T, want *CompoundAssignStmt", fs.After)
	}
}

func TestParseForCForIncDecAfter(t *testing.T) {
	// The after clause supports ++, --, +=, -=, =, and calls.
	for _, src := range []string{
		"for a := 0; a < 10; a++ { }",
		"for a := 0; a < 10; a-- { }",
		"for a := 0; a < 10; ++a { }",
		"for a := 0; a < 10; --a { }",
		"for a := 0; a < 10; a -= 1 { }",
		"for a := 0; a < 10; a = a + 1 { }",
		"for a := 0; a < 10; f() { }",
	} {
		stmt := parseOneStmt(t, src)
		if _, ok := stmt.(*ForStmt); !ok {
			t.Fatalf("expected *ForStmt for %q, got %T", src, stmt)
		}
	}
}

func TestParseForRangeElem(t *testing.T) {
	stmt := parseOneStmt(t, "for elem: arr { exit 0; }")
	fs, ok := stmt.(*ForStmt)
	if !ok {
		t.Fatalf("expected *ForStmt, got %T", stmt)
	}
	if fs.Range == nil {
		t.Fatal("Range = nil")
	}
	if fs.ElemName != "elem" {
		t.Fatalf("ElemName = %q, want %q", fs.ElemName, "elem")
	}
	if fs.IndexName != "" {
		t.Fatalf("IndexName = %q, want empty", fs.IndexName)
	}
	if fs.Cond != nil {
		t.Fatal("Cond = nil expected for range form")
	}
}

func TestParseForRangeIdxElem(t *testing.T) {
	source := "for idx, elem: arr { exit 0; }"
	stmt := parseOneStmt(t, source)
	fs, ok := stmt.(*ForStmt)
	if !ok {
		t.Fatalf("expected *ForStmt, got %T", stmt)
	}
	if fs.IndexName != "idx" {
		t.Fatalf("IndexName = %q, want %q", fs.IndexName, "idx")
	}
	if fs.ElemName != "elem" {
		t.Fatalf("ElemName = %q, want %q", fs.ElemName, "elem")
	}
	if got := fs.IndexNameSpan.Start - fs.Span_.Start; got != len("for ") {
		t.Errorf("IndexNameSpan starts %d bytes into loop, want %d", got, len("for "))
	}
	if got := fs.IndexNameSpan.End - fs.IndexNameSpan.Start; got != len("idx") {
		t.Errorf("IndexNameSpan length = %d, want %d", got, len("idx"))
	}
	if got := fs.ElemNameSpan.Start - fs.Span_.Start; got != len("for idx, ") {
		t.Errorf("ElemNameSpan starts %d bytes into loop, want %d", got, len("for idx, "))
	}
	if got := fs.ElemNameSpan.End - fs.ElemNameSpan.Start; got != len("elem") {
		t.Errorf("ElemNameSpan length = %d, want %d", got, len("elem"))
	}
}

func TestParseForImplicitRange(t *testing.T) {
	// "for arr { ... }" parses as the single-expression form; the type
	// checker decides whether it is a while loop or an implicit range.
	stmt := parseOneStmt(t, "for arr { exit 0; }")
	fs, ok := stmt.(*ForStmt)
	if !ok {
		t.Fatalf("expected *ForStmt, got %T", stmt)
	}
	if fs.Cond == nil || fs.Range != nil {
		t.Fatalf("Cond = %v, Range = %v, want Cond set and Range nil", fs.Cond, fs.Range)
	}
}

func TestParseBreakContinue(t *testing.T) {
	block := parseOneStmtBlock(t, "break; continue;")
	if len(block.Stmts) != 2 {
		t.Fatalf("Stmts = %d, want 2", len(block.Stmts))
	}
	if _, ok := block.Stmts[0].(*BreakStmt); !ok {
		t.Fatalf("stmt 0 type = %T, want *BreakStmt", block.Stmts[0])
	}
	if _, ok := block.Stmts[1].(*ContinueStmt); !ok {
		t.Fatalf("stmt 1 type = %T, want *ContinueStmt", block.Stmts[1])
	}
}

func TestParseCompoundAssign(t *testing.T) {
	for _, tt := range []struct {
		src string
		op  BinaryOp
	}{
		{"a += 1;", BinaryOpAdd},
		{"a -= 1;", BinaryOpSub},
	} {
		stmt := parseOneStmt(t, tt.src)
		c, ok := stmt.(*CompoundAssignStmt)
		if !ok {
			t.Fatalf("expected *CompoundAssignStmt for %q, got %T", tt.src, stmt)
		}
		if c.Op != tt.op {
			t.Errorf("%q: Op = %v, want %v", tt.src, c.Op, tt.op)
		}
		if c.Name != "a" {
			t.Errorf("%q: Name = %q, want %q", tt.src, c.Name, "a")
		}
	}
}

func TestParseIncDec(t *testing.T) {
	for _, tt := range []struct {
		src    string
		op     BinaryOp
		prefix bool
	}{
		{"a++;", BinaryOpAdd, false},
		{"a--;", BinaryOpSub, false},
		{"++a;", BinaryOpAdd, true},
		{"--a;", BinaryOpSub, true},
	} {
		stmt := parseOneStmt(t, tt.src)
		inc, ok := stmt.(*IncDecStmt)
		if !ok {
			t.Fatalf("expected *IncDecStmt for %q, got %T", tt.src, stmt)
		}
		if inc.Op != tt.op {
			t.Errorf("%q: Op = %v, want %v", tt.src, inc.Op, tt.op)
		}
		if inc.Prefix != tt.prefix {
			t.Errorf("%q: Prefix = %v, want %v", tt.src, inc.Prefix, tt.prefix)
		}
		if inc.Name != "a" {
			t.Errorf("%q: Name = %q, want %q", tt.src, inc.Name, "a")
		}
	}
}

func TestParseArrayLiteral(t *testing.T) {
	expr := parseExpr(t, "[]S64.{1, 2, 3}")
	al, ok := expr.(*ArrayInitExpr)
	if !ok {
		t.Fatalf("expected *ArrayInitExpr, got %T", expr)
	}
	at, ok := al.Elem.(*IdentExpr)
	if !ok || at.Name != "S64" {
		t.Fatalf("Elem = %#v, want Ident(S64)", al.Elem)
	}
	if len(al.Items) != 3 {
		t.Fatalf("Items = %d, want 3", len(al.Items))
	}
}

func TestParseArrayTypeExpr(t *testing.T) {
	// An array type in a declaration: "items: []S64 = ..."
	decl := parseOneDecl(t, "items: []S64 = []S64.{1};")
	vd, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	at, ok := vd.DeclType.(*ArrayTypeExpr)
	if !ok {
		t.Fatalf("DeclType = %T, want *ArrayTypeExpr", vd.DeclType)
	}
	if _, ok := at.Elem.(*IdentExpr); !ok {
		t.Fatalf("Elem = %T, want *IdentExpr", at.Elem)
	}
}

func TestParseIndex(t *testing.T) {
	expr := parseExpr(t, "arr[0]")
	ix, ok := expr.(*IndexExpr)
	if !ok {
		t.Fatalf("expected *IndexExpr, got %T", expr)
	}
	if _, ok := ix.Base.(*IdentExpr); !ok {
		t.Fatalf("Base = %T, want *IdentExpr", ix.Base)
	}
	if _, ok := ix.Index.(*IntExpr); !ok {
		t.Fatalf("Index = %T, want *IntExpr", ix.Index)
	}
}

func TestParseLoopBuiltins(t *testing.T) {
	stmt := parseOneStmt(t, "for arr { do_something(#this, #index); }")
	fs, ok := stmt.(*ForStmt)
	if !ok {
		t.Fatalf("expected *ForStmt, got %T", stmt)
	}
	exprStmt, ok := fs.Body.Stmts[0].(*ExprStmt)
	if !ok {
		t.Fatalf("expected *ExprStmt, got %T", fs.Body.Stmts[0])
	}
	call, ok := exprStmt.Expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", exprStmt.Expr)
	}
	if len(call.Args) != 2 {
		t.Fatalf("Args = %d, want 2", len(call.Args))
	}
	tb, ok := call.Args[0].(*LoopBuiltinExpr)
	if !ok || tb.Name != "this" {
		t.Fatalf("arg 0 = %#v, want LoopBuiltin(this)", call.Args[0])
	}
	ib, ok := call.Args[1].(*LoopBuiltinExpr)
	if !ok || ib.Name != "index" {
		t.Fatalf("arg 1 = %#v, want LoopBuiltin(index)", call.Args[1])
	}
}

// ──────────────────────────────────────────────
// #shadow directive
// ──────────────────────────────────────────────

func TestParseShadowVarDecl(t *testing.T) {
	// '#shadow' marks a declaration as an explicit shadow.
	for _, src := range []string{
		"#shadow x := 5;",
		"#shadow x :: 5;",
		"#shadow x: S64 = 5;",
	} {
		stmt := parseOneStmt(t, src)
		vd, ok := stmt.(*VarDecl)
		if !ok {
			t.Fatalf("expected *VarDecl for %q, got %T", src, stmt)
		}
		if !vd.Shadow {
			t.Errorf("%q: Shadow = false, want true", src)
		}
	}
}

func TestParseShadowUnlessCatch(t *testing.T) {
	// '#shadow' also applies to the 'unless catch' target.
	stmt := parseOneStmt(t, "#shadow x := f() unless catch { return; }")
	uc, ok := stmt.(*UnlessCatchStmt)
	if !ok {
		t.Fatalf("expected *UnlessCatchStmt, got %T", stmt)
	}
	if !uc.Shadow {
		t.Error("Shadow = false, want true")
	}
	if uc.Target != "x" {
		t.Fatalf("Target = %q, want %q", uc.Target, "x")
	}
}

func TestParseBareUnlessCatchHasNoTargetSpan(t *testing.T) {
	stmt := parseOneStmt(t, "f() unless catch { return; }")
	uc, ok := stmt.(*UnlessCatchStmt)
	if !ok {
		t.Fatalf("expected *UnlessCatchStmt, got %T", stmt)
	}
	if uc.Target != "" || uc.TargetSpan != (Span{}) {
		t.Errorf("bare unless target = %q span=%+v, want empty target and span", uc.Target, uc.TargetSpan)
	}
}

func TestParseShadowTopLevel(t *testing.T) {
	// '#shadow' works for top-level declarations too.
	decl := parseOneDecl(t, "#shadow x := 5;")
	vd, ok := decl.(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", decl)
	}
	if !vd.Shadow {
		t.Error("Shadow = false, want true")
	}
}

func TestParseShadowAllowsProcedureDeclaration(t *testing.T) {
	tokens, _ := Tokenize([]byte("#shadow f :: proc { }"), 0)
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected #shadow procedure error: %v", result.Diags)
	}
	proc := result.Program.Decls[0].(*ProcDecl)
	if !proc.Shadow {
		t.Fatal("Shadow = false, want true")
	}
}

// ──────────────────────────────────────────────
// Expressions — literals
// ──────────────────────────────────────────────

func TestParseIntExpr(t *testing.T) {
	expr := parseExpr(t, "42")
	e, ok := expr.(*IntExpr)
	if !ok {
		t.Fatalf("expected *IntExpr, got %T", expr)
	}
	if e.Value != "42" {
		t.Errorf("Value = %q, want %q", e.Value, "42")
	}
}

func TestParseFloatExpr(t *testing.T) {
	expr := parseExpr(t, "3.14")
	e, ok := expr.(*FloatExpr)
	if !ok {
		t.Fatalf("expected *FloatExpr, got %T", expr)
	}
	if e.Value != "3.14" {
		t.Errorf("Value = %q, want %q", e.Value, "3.14")
	}
}

func TestParseStringExpr(t *testing.T) {
	expr := parseExpr(t, "«hello»")
	e, ok := expr.(*StringExpr)
	if !ok {
		t.Fatalf("expected *StringExpr, got %T", expr)
	}
	if e.Value != "hello" {
		t.Errorf("Value = %q, want %q", e.Value, "hello")
	}
}

func TestParseInterpolatedString(t *testing.T) {
	expr := parseExpr(t, "«hello {name}!»")
	interp, ok := expr.(*InterpolatedStringExpr)
	if !ok {
		t.Fatalf("expected *InterpolatedStringExpr, got %T", expr)
	}
	if len(interp.Parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(interp.Parts))
	}
	if interp.Parts[0].Literal != "hello " {
		t.Errorf("part 0 literal = %q, want %q", interp.Parts[0].Literal, "hello ")
	}
	if interp.Parts[1].Expr == nil {
		t.Fatal("part 1 should be an expression")
	}
	if id, ok := interp.Parts[1].Expr.(*IdentExpr); !ok || id.Name != "name" {
		t.Fatalf("part 1 expr = %#v, want Ident(name)", interp.Parts[1].Expr)
	}
	if interp.Parts[2].Literal != "!" {
		t.Errorf("part 2 literal = %q, want %q", interp.Parts[2].Literal, "!")
	}
}

func TestParseInterpolatedStringLeading(t *testing.T) {
	expr := parseExpr(t, "«{name}»")
	interp, ok := expr.(*InterpolatedStringExpr)
	if !ok {
		t.Fatalf("expected *InterpolatedStringExpr, got %T", expr)
	}
	if len(interp.Parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(interp.Parts))
	}
	if interp.Parts[0].Literal != "" {
		t.Errorf("part 0 literal = %q, want empty", interp.Parts[0].Literal)
	}
	if interp.Parts[1].Expr == nil {
		t.Fatal("part 1 should be an expression")
	}
	if interp.Parts[2].Literal != "" {
		t.Errorf("part 2 literal = %q, want empty", interp.Parts[2].Literal)
	}
}

func TestParseAllocateExpr(t *testing.T) {
	expr := parseExpr(t, "#allocate 16")
	alloc, ok := expr.(*AllocateExpr)
	if !ok {
		t.Fatalf("expected *AllocateExpr, got %T", expr)
	}
	if int, ok := alloc.Size.(*IntExpr); !ok || int.Value != "16" {
		t.Fatalf("Size = %#v, want Int(16)", alloc.Size)
	}
}

func TestParseDeallocateStmt(t *testing.T) {
	stmt := parseOneStmt(t, "#deallocate a;")
	deal, ok := stmt.(*DeallocateStmt)
	if !ok {
		t.Fatalf("expected *DeallocateStmt, got %T", stmt)
	}
	if id, ok := deal.Addr.(*IdentExpr); !ok || id.Name != "a" {
		t.Fatalf("Addr = %#v, want Ident(a)", deal.Addr)
	}
}

func TestParseCastExpr(t *testing.T) {
	expr := parseExpr(t, "a.(*S64)")
	cast, ok := expr.(*CastExpr)
	if !ok {
		t.Fatalf("expected *CastExpr, got %T", expr)
	}
	if id, ok := cast.Value.(*IdentExpr); !ok || id.Name != "a" {
		t.Fatalf("Value = %#v, want Ident(a)", cast.Value)
	}
	if pt, ok := cast.Type.(*PointerTypeExpr); !ok {
		t.Fatalf("Type = %#v, want *PointerTypeExpr", cast.Type)
	} else if id, ok := pt.Elem.(*IdentExpr); !ok || id.Name != "S64" {
		t.Fatalf("Type elem = %#v, want Ident(S64)", pt.Elem)
	}
}

func TestParseBoolExpr(t *testing.T) {
	expr := parseExpr(t, "true")
	e, ok := expr.(*BoolExpr)
	if !ok {
		t.Fatalf("expected *BoolExpr, got %T", expr)
	}
	if !e.Value {
		t.Error("Value = false, want true")
	}
}

func TestParseIdentExpr(t *testing.T) {
	expr := parseExpr(t, "myVar")
	e, ok := expr.(*IdentExpr)
	if !ok {
		t.Fatalf("expected *IdentExpr, got %T", expr)
	}
	if e.Name != "myVar" {
		t.Errorf("Name = %q, want %q", e.Name, "myVar")
	}
}

// ──────────────────────────────────────────────
// Expressions — struct literals
// ──────────────────────────────────────────────

func TestParseStructInitExplicitType(t *testing.T) {
	expr := parseExpr(t, `Something_New.{field1=«hello», field2=10, field3=true}`)
	e, ok := expr.(*StructInitExpr)
	if !ok {
		t.Fatalf("expected *StructInitExpr, got %T", expr)
	}
	if e.Type == nil {
		t.Fatal("Type = nil, want non-nil")
	}
	ident, ok := e.Type.(*IdentExpr)
	if !ok {
		t.Fatalf("Type type = %T, want *IdentExpr", e.Type)
	}
	if ident.Name != "Something_New" {
		t.Errorf("type name = %q, want %q", ident.Name, "Something_New")
	}
	if len(e.Fields) != 3 {
		t.Fatalf("len(Fields) = %d, want 3", len(e.Fields))
	}
	if e.Fields[0].Name != "field1" {
		t.Errorf("Fields[0].Name = %q, want %q", e.Fields[0].Name, "field1")
	}
	if e.Fields[1].Name != "field2" {
		t.Errorf("Fields[1].Name = %q, want %q", e.Fields[1].Name, "field2")
	}
	if e.Fields[2].Name != "field3" {
		t.Errorf("Fields[2].Name = %q, want %q", e.Fields[2].Name, "field3")
	}
}

func TestParseStructInitInferredType(t *testing.T) {
	expr := parseExpr(t, `.{«hello», 10, true}`)
	e, ok := expr.(*StructInitExpr)
	if !ok {
		t.Fatalf("expected *StructInitExpr, got %T", expr)
	}
	if e.Type != nil {
		t.Errorf("Type = %v, want nil for inferred literal", e.Type)
	}
	if len(e.Fields) != 3 {
		t.Fatalf("len(Fields) = %d, want 3", len(e.Fields))
	}
	// Positional fields have empty names.
	for i, f := range e.Fields {
		if f.Name != "" {
			t.Errorf("Fields[%d].Name = %q, want empty", i, f.Name)
		}
		if f.Value == nil {
			t.Errorf("Fields[%d].Value = nil, want non-nil", i)
		}
	}
}

func TestParseStructInitMixedFields(t *testing.T) {
	// Positional fields may precede named fields.
	expr := parseExpr(t, `Something_New.{«hello», field2=10}`)
	e, ok := expr.(*StructInitExpr)
	if !ok {
		t.Fatalf("expected *StructInitExpr, got %T", expr)
	}
	if len(e.Fields) != 2 {
		t.Fatalf("len(Fields) = %d, want 2", len(e.Fields))
	}
	if e.Fields[0].Name != "" {
		t.Errorf("Fields[0].Name = %q, want empty (positional)", e.Fields[0].Name)
	}
	if e.Fields[1].Name != "field2" {
		t.Errorf("Fields[1].Name = %q, want %q", e.Fields[1].Name, "field2")
	}
	result := parseTestCase(t, "x := Something_New.{field2=10, true};")
	if !hasError(result.Diags, "positional struct field cannot follow a named field") {
		t.Fatalf("expected positional-after-named error, got %v", result.Diags)
	}
}

// ──────────────────────────────────────────────
// Expressions — binary operators
// ──────────────────────────────────────────────

func TestParseBinaryAdd(t *testing.T) {
	expr := parseExpr(t, "1 + 2")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpAdd {
		t.Errorf("Op = %d, want %d (BinaryOpAdd)", e.Op, BinaryOpAdd)
	}
}

func TestParseBinaryMul(t *testing.T) {
	expr := parseExpr(t, "3 * 4")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpMul {
		t.Errorf("Op = %d, want %d (BinaryOpMul)", e.Op, BinaryOpMul)
	}
}

func TestParseBinaryMod(t *testing.T) {
	expr := parseExpr(t, "7 % 3")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpMod {
		t.Errorf("Op = %d, want %d (BinaryOpMod)", e.Op, BinaryOpMod)
	}
}

func TestParseBinaryComparison(t *testing.T) {
	ops := []struct {
		text string
		want BinaryOp
	}{
		{"<", BinaryOpLt},
		{">", BinaryOpGt},
		{"<=", BinaryOpLe},
		{">=", BinaryOpGe},
		{"==", BinaryOpEq},
		{"!=", BinaryOpNeq},
	}
	for _, op := range ops {
		t.Run(op.text, func(t *testing.T) {
			expr := parseExpr(t, "1 "+op.text+" 2")
			e, ok := expr.(*BinaryExpr)
			if !ok {
				t.Fatalf("expected *BinaryExpr, got %T", expr)
			}
			if e.Op != op.want {
				t.Errorf("Op = %d, want %d", e.Op, op.want)
			}
		})
	}
}

func TestParseBinaryLogical(t *testing.T) {
	expr := parseExpr(t, "a && b || c")
	// && has higher precedence than ||, so this should be (a && b) || c
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpOr {
		t.Errorf("top op = %d, want %d (BinaryOpOr)", e.Op, BinaryOpOr)
	}
	left, ok := e.Left.(*BinaryExpr)
	if !ok {
		t.Fatalf("left operand type = %T, want *BinaryExpr", e.Left)
	}
	if left.Op != BinaryOpAnd {
		t.Errorf("left op = %d, want %d (BinaryOpAnd)", left.Op, BinaryOpAnd)
	}
}

// ──────────────────────────────────────────────
// Expressions — precedence
// ──────────────────────────────────────────────

func TestPrecedenceAddMul(t *testing.T) {
	// 1 + 2 * 3  should parse as 1 + (2 * 3)
	expr := parseExpr(t, "1 + 2 * 3")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpAdd {
		t.Errorf("top op = %d, want %d (BinaryOpAdd)", e.Op, BinaryOpAdd)
	}
	_, ok = e.Right.(*BinaryExpr)
	if !ok {
		t.Fatalf("right operand type = %T, want *BinaryExpr", e.Right)
	}
}

func TestPrecedenceMulAdd(t *testing.T) {
	// 1 * 2 + 3  should parse as (1 * 2) + 3
	expr := parseExpr(t, "1 * 2 + 3")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpAdd {
		t.Errorf("top op = %d, want %d (BinaryOpAdd)", e.Op, BinaryOpAdd)
	}
	_, ok = e.Left.(*BinaryExpr)
	if !ok {
		t.Fatalf("left operand type = %T, want *BinaryExpr", e.Left)
	}
}

func TestPrecedenceModAdd(t *testing.T) {
	// 1 + 2 % 3  should parse as 1 + (2 % 3), since % binds like * and /
	expr := parseExpr(t, "1 + 2 % 3")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpAdd {
		t.Errorf("top op = %d, want %d (BinaryOpAdd)", e.Op, BinaryOpAdd)
	}
	right, ok := e.Right.(*BinaryExpr)
	if !ok {
		t.Fatalf("right operand type = %T, want *BinaryExpr", e.Right)
	}
	if right.Op != BinaryOpMod {
		t.Errorf("right op = %d, want %d (BinaryOpMod)", right.Op, BinaryOpMod)
	}
}

func TestPrecedenceCmpEq(t *testing.T) {
	// a < b == c  should parse as (a < b) == c
	expr := parseExpr(t, "a < b == c")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpEq {
		t.Errorf("top op = %d, want %d (BinaryOpEq)", e.Op, BinaryOpEq)
	}
}

func TestPrecedenceParens(t *testing.T) {
	// (1 + 2) * 3  should parse as (1+2) * 3
	expr := parseExpr(t, "(1 + 2) * 3")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpMul {
		t.Errorf("top op = %d, want %d (BinaryOpMul)", e.Op, BinaryOpMul)
	}
	_, ok = e.Left.(*ParenExpr)
	if !ok {
		t.Fatalf("left operand type = %T, want *ParenExpr", e.Left)
	}
}

func TestPrecedenceAndOr(t *testing.T) {
	// a && b || c && d  should parse as (a && b) || (c && d)
	expr := parseExpr(t, "a && b || c && d")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpOr {
		t.Errorf("top op = %d, want %d (BinaryOpOr)", e.Op, BinaryOpOr)
	}
	_, leftOk := e.Left.(*BinaryExpr)
	_, rightOk := e.Right.(*BinaryExpr)
	if !leftOk || !rightOk {
		t.Errorf("both sides of || should be BinaryExpr, got left=%T right=%T", e.Left, e.Right)
	}
}

// ──────────────────────────────────────────────
// Expressions — unary
// ──────────────────────────────────────────────

func TestParseUnaryNeg(t *testing.T) {
	expr := parseExpr(t, "-42")
	e, ok := expr.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected *UnaryExpr, got %T", expr)
	}
	if e.Op != UnaryOpNeg {
		t.Errorf("Op = %d, want %d (UnaryOpNeg)", e.Op, UnaryOpNeg)
	}
}

func TestParseUnaryNot(t *testing.T) {
	expr := parseExpr(t, "!flag")
	e, ok := expr.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected *UnaryExpr, got %T", expr)
	}
	if e.Op != UnaryOpNot {
		t.Errorf("Op = %d, want %d (UnaryOpNot)", e.Op, UnaryOpNot)
	}
}

func TestParseUnaryDoubleNeg(t *testing.T) {
	// '--' is now the decrement operator, so double negation must be written
	// with a space ('- -5') or parentheses.
	expr := parseExpr(t, "- -5")
	e, ok := expr.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected *UnaryExpr, got %T", expr)
	}
	inner, ok := e.Operand.(*UnaryExpr)
	if !ok {
		t.Fatalf("inner operand type = %T, want *UnaryExpr", e.Operand)
	}
	if inner.Op != UnaryOpNeg {
		t.Errorf("inner Op = %d, want %d", inner.Op, UnaryOpNeg)
	}
}

// ──────────────────────────────────────────────
// Expressions — calls
// ──────────────────────────────────────────────

func TestParseCallNoArgs(t *testing.T) {
	expr := parseExpr(t, "f()")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	if len(e.Args) != 0 {
		t.Errorf("Args = %d, want 0", len(e.Args))
	}
}

func TestParseCallOneArg(t *testing.T) {
	expr := parseExpr(t, "f(42)")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	if len(e.Args) != 1 {
		t.Fatalf("Args = %d, want 1", len(e.Args))
	}
}

func TestParseCallMultipleArgs(t *testing.T) {
	expr := parseExpr(t, "f(1, 2, 3)")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	if len(e.Args) != 3 {
		t.Fatalf("Args = %d, want 3", len(e.Args))
	}
}

func TestParseCallExprArg(t *testing.T) {
	// f(1 + 2) — arguments should be full expressions, not just base nodes
	expr := parseExpr(t, "f(1 + 2)")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	if len(e.Args) != 1 {
		t.Fatalf("Args = %d, want 1", len(e.Args))
	}
	_, ok = e.Args[0].(*BinaryExpr)
	if !ok {
		t.Fatalf("arg type = %T, want *BinaryExpr", e.Args[0])
	}
}

func TestParseCallChained(t *testing.T) {
	// f()() — chained calls
	expr := parseExpr(t, "f()()")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	inner, ok := e.Func.(*CallExpr)
	if !ok {
		t.Fatalf("func type = %T, want *CallExpr", e.Func)
	}
	if len(inner.Args) != 0 {
		t.Errorf("inner args = %d, want 0", len(inner.Args))
	}
}

// ──────────────────────────────────────────────
// Expressions — mixed
// ──────────────────────────────────────────────

func TestParseCallWithBinaryExprArg(t *testing.T) {
	// f(1 + 2, 3 * 4)
	expr := parseExpr(t, "f(1 + 2, 3 * 4)")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	if len(e.Args) != 2 {
		t.Fatalf("Args = %d, want 2", len(e.Args))
	}
	_, ok = e.Args[0].(*BinaryExpr)
	if !ok {
		t.Errorf("arg[0] type = %T, want *BinaryExpr", e.Args[0])
	}
	_, ok = e.Args[1].(*BinaryExpr)
	if !ok {
		t.Errorf("arg[1] type = %T, want *BinaryExpr", e.Args[1])
	}
}

func TestParseCallWithParenExpr(t *testing.T) {
	expr := parseExpr(t, "f((1 + 2))")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	if len(e.Args) != 1 {
		t.Fatalf("Args = %d, want 1", len(e.Args))
	}
	_, ok = e.Args[0].(*ParenExpr)
	if !ok {
		t.Fatalf("arg[0] type = %T, want *ParenExpr", e.Args[0])
	}
}

// ──────────────────────────────────────────────
// Blocks
// ──────────────────────────────────────────────

func TestParseEmptyBlock(t *testing.T) {
	stmt := parseOneStmt(t, "{}")
	_, ok := stmt.(*BlockStmt)
	if !ok {
		t.Fatalf("expected *BlockStmt, got %T", stmt)
	}
}

func TestParseBlockMultipleStmts(t *testing.T) {
	block := parseOneStmtBlock(t, "x := 1; y := 2;")
	if len(block.Stmts) != 2 {
		t.Fatalf("Stmts = %d, want 2", len(block.Stmts))
	}
}

// ──────────────────────────────────────────────
// Error recovery
// ──────────────────────────────────────────────

func TestParseErrorUnexpectedToken(t *testing.T) {
	result := parseTestCase(t, "+ 42;")
	if !result.Diags.HasErrors() {
		t.Error("expected errors, got none")
	}
}

func TestParseErrorIncompleteExpr(t *testing.T) {
	result := parseTestCase(t, "x :: ;")
	if !result.Diags.HasErrors() {
		t.Error("expected errors, got none")
	}
}

func TestParseErrorMissingSemicolon(t *testing.T) {
	result := parseTestCase(t, "x :: 42 y :: 43;")
	if !result.Diags.HasErrors() {
		t.Error("expected errors, got none")
	}
}

func TestParseErrorUnclosedBlock(t *testing.T) {
	result := parseTestCase(t, "main :: proc { x := 1; ")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for unclosed block, got none")
	}
}

func TestParseErrorBadParam(t *testing.T) {
	result := parseTestCase(t, "f :: proc (a) { }")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for missing param type, got none")
	}
}

func TestParseErrorReturnsAfterError(t *testing.T) {
	// Parsing errors should not prevent the parser from producing valid nodes
	result := parseTestCase(t, "x :: 42; bad; y :: 43;")
	if !result.Diags.HasErrors() {
		t.Error("expected errors, got none")
	}
	// But we should still get the valid declarations
	if len(result.Program.Decls) == 0 {
		t.Error("expected some declarations despite errors")
	}
}

// ──────────────────────────────────────────────
// Multiple declarations
// ──────────────────────────────────────────────

func TestParseMultipleDeclarations(t *testing.T) {
	result := parseTestCase(t, "x :: 1; y :: 2; z :: 3;")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 3 {
		t.Fatalf("Decls = %d, want 3", len(result.Program.Decls))
	}
}

func TestParseProcWithVarDecl(t *testing.T) {
	result := parseTestCase(t, `
		main :: proc -> S64 { return 0; }
		version :: 1;
	`)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 2 {
		t.Fatalf("Decls = %d, want 2", len(result.Program.Decls))
	}
}

// ──────────────────────────────────────────────
// Edge cases
// ──────────────────────────────────────────────

func TestParseStraySemicolons(t *testing.T) {
	result := parseTestCase(t, ";;; x :: 1;;;")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
}

func TestParseNestedBlocks(t *testing.T) {
	stmt := parseOneStmt(t, "{ { { } } }")
	_, ok := stmt.(*BlockStmt)
	if !ok {
		t.Fatalf("expected *BlockStmt, got %T", stmt)
	}
}

func TestParseExitWithExpr(t *testing.T) {
	stmt := parseOneStmt(t, "exit 1 + 2;")
	e, ok := stmt.(*ExitStmt)
	if !ok {
		t.Fatalf("expected *ExitStmt, got %T", stmt)
	}
	_, ok = e.Status.(*BinaryExpr)
	if !ok {
		t.Fatalf("Status type = %T, want *BinaryExpr", e.Status)
	}
}

func TestParseReturnWithBinaryExpr(t *testing.T) {
	stmt := parseOneStmt(t, "return 1 + 2 * 3;")
	r, ok := stmt.(*ReturnStmt)
	if !ok {
		t.Fatalf("expected *ReturnStmt, got %T", stmt)
	}
	_, ok = r.Value.(*BinaryExpr)
	if !ok {
		t.Fatalf("Value type = %T, want *BinaryExpr", r.Value)
	}
}

func TestParseCallStmt(t *testing.T) {
	stmt := parseOneStmt(t, "f(42);")
	_, ok := stmt.(*ExprStmt)
	if !ok {
		t.Fatalf("expected *ExprStmt, got %T", stmt)
	}
}

func TestParseAndOrExpr(t *testing.T) {
	expr := parseExpr(t, "a && b")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpAnd {
		t.Errorf("Op = %d, want %d", e.Op, BinaryOpAnd)
	}
}

func TestParseOrExpr(t *testing.T) {
	expr := parseExpr(t, "a || b")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpOr {
		t.Errorf("Op = %d, want %d", e.Op, BinaryOpOr)
	}
}

func TestParseAssignExpr(t *testing.T) {
	// Test that = is parsed as an assignment, not as an expression operator
	stmt := parseOneStmt(t, "x = 42;")
	_, ok := stmt.(*AssignStmt)
	if !ok {
		t.Fatalf("expected *AssignStmt, got %T", stmt)
	}
}

func TestParseSpanCoverage(t *testing.T) {
	// Verify that spans cover the correct range for a declaration.
	// The span covers from the name through the initializer, not including
	// the terminating semicolon.
	source := "x :: 42;"
	tokens, _ := Tokenize([]byte(source), 0)
	result := ParseProgram(tokens)

	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}

	decl := result.Program.Decls[0]
	span := decl.nodeSpan()
	if span.Start != 0 {
		t.Errorf("span.Start = %d, want 0", span.Start)
	}
	// "x :: 42" spans [0, 7); the ";" is the terminator
	if span.End != 7 {
		t.Errorf("span.End = %d, want 7", span.End)
	}
}

func TestParseProcDeclSpan(t *testing.T) {
	source := "main :: proc { return; }"
	tokens, _ := Tokenize([]byte(source), 0)
	result := ParseProgram(tokens)

	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}

	span := result.Program.Decls[0].nodeSpan()
	if span.Start != 0 {
		t.Errorf("span.Start = %d, want 0", span.Start)
	}
	if span.End != len(source) {
		t.Errorf("span.End = %d, want %d", span.End, len(source))
	}
}

// ──────────────────────────────────────────────
// Regression tests from the old compiler
// ──────────────────────────────────────────────

func TestRegressionFloatLiteral(t *testing.T) {
	// Old compiler turned float literals into int literals
	expr := parseExpr(t, "3.14")
	_, ok := expr.(*FloatExpr)
	if !ok {
		t.Fatalf("expected *FloatExpr, got %T", expr)
	}
}

func TestRegressionReturnVoid(t *testing.T) {
	// Old compiler did not support return;
	stmt := parseOneStmt(t, "return;")
	_, ok := stmt.(*ReturnStmt)
	if !ok {
		t.Fatalf("expected *ReturnStmt, got %T", stmt)
	}
}

func TestRegressionIfBoolCond(t *testing.T) {
	// Old compiler dereferenced nil BinOp for non-binary conditions
	stmt := parseOneStmt(t, "if true { exit 0; }")
	ifs, ok := stmt.(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", stmt)
	}
	_, ok = ifs.Condition.(*BoolExpr)
	if !ok {
		t.Fatalf("Condition type = %T, want *BoolExpr", ifs.Condition)
	}
}

func TestRegressionCallWithExprArg(t *testing.T) {
	// Old compiler only parsed base nodes as call args, so f(1+2) failed
	expr := parseExpr(t, "f(1 + 2)")
	e, ok := expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", expr)
	}
	if len(e.Args) != 1 {
		t.Fatalf("Args = %d, want 1", len(e.Args))
	}
	_, ok = e.Args[0].(*BinaryExpr)
	if !ok {
		t.Fatalf("arg type = %T, want *BinaryExpr", e.Args[0])
	}
}

func TestRegressionPrecedenceOldCompiler(t *testing.T) {
	// Old compiler had a precedence bug: 1 * 2 + 3 would let RHS absorb +
	// New parser should correctly parse as (1 * 2) + 3
	expr := parseExpr(t, "1 * 2 + 3")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpAdd {
		t.Errorf("top op = %d, want %d (BinaryOpAdd)", e.Op, BinaryOpAdd)
	}
	left, ok := e.Left.(*BinaryExpr)
	if !ok {
		t.Fatalf("left type = %T, want *BinaryExpr", e.Left)
	}
	if left.Op != BinaryOpMul {
		t.Errorf("left op = %d, want %d (BinaryOpMul)", left.Op, BinaryOpMul)
	}
}

func TestRegressionLeftAssociativity(t *testing.T) {
	// 10 - 3 - 2 should parse as (10 - 3) - 2, not 10 - (3 - 2)
	expr := parseExpr(t, "10 - 3 - 2")
	e, ok := expr.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected *BinaryExpr, got %T", expr)
	}
	if e.Op != BinaryOpSub {
		t.Errorf("top op = %d, want %d (BinaryOpSub)", e.Op, BinaryOpSub)
	}
	// Left operand should be a BinaryExpr (10 - 3), not IntExpr (10)
	left, ok := e.Left.(*BinaryExpr)
	if !ok {
		t.Fatalf("left type = %T, want *BinaryExpr", e.Left)
	}
	if left.Op != BinaryOpSub {
		t.Errorf("left op = %d, want %d (BinaryOpSub)", left.Op, BinaryOpSub)
	}
	// Right operand should be IntExpr (2)
	_, ok = e.Right.(*IntExpr)
	if !ok {
		t.Fatalf("right type = %T, want *IntExpr", e.Right)
	}
}

func TestRegressionChainedCallStmt(t *testing.T) {
	// f()(); should parse as a chained call expression statement
	stmt := parseOneStmt(t, "f()();")
	e, ok := stmt.(*ExprStmt)
	if !ok {
		t.Fatalf("expected *ExprStmt, got %T", stmt)
	}
	call, ok := e.Expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", e.Expr)
	}
	inner, ok := call.Func.(*CallExpr)
	if !ok {
		t.Fatalf("func type = %T, want *CallExpr", call.Func)
	}
	if len(inner.Args) != 0 {
		t.Errorf("inner args = %d, want 0", len(inner.Args))
	}
}

func TestRegressionBareProcInBody(t *testing.T) {
	// main :: proc { proc } should not hang; it should produce diagnostics
	result := parseTestCase(t, "main :: proc { proc }")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for bare 'proc' in body, got none")
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
}

func TestRegressionMalformedProcResult(t *testing.T) {
	// f :: proc -> S64, { return 0; } should produce a diagnostic
	result := parseTestCase(t, "f :: proc -> S64, { return 0; }")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for malformed proc result, got none")
	}
}

func TestRegressionElseIfSpan(t *testing.T) {
	// "else if" is invalid syntax — must use "elif". The parser should emit
	// an error but still produce a parse tree for recovery.
	source := "main :: proc { if true { } else if false { } }"
	result := parseTestCase(t, source)
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for 'else if', got none")
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if len(proc.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(proc.Body.Stmts))
	}
	ifs, ok := proc.Body.Stmts[0].(*IfStmt)
	if !ok {
		t.Fatalf("expected *IfStmt, got %T", proc.Body.Stmts[0])
	}
	if ifs.ElseBody == nil {
		t.Fatal("ElseBody = nil, want non-nil")
	}
	// The else body span should be larger than just the "if" keyword (3 bytes)
	if ifs.ElseBody.nodeSpan().End-ifs.ElseBody.nodeSpan().Start <= 3 {
		t.Errorf("ElseBody span too small: %#v", ifs.ElseBody.nodeSpan())
	}
	// The parent IfStmt span should cover the full else if branch
	if ifs.nodeSpan().End <= ifs.Body.nodeSpan().End {
		t.Error("IfStmt span should extend past the if body to include else if")
	}
}

func TestRegressionTrailingCommaParam(t *testing.T) {
	// Trailing comma in parameter list should produce an error
	// pointing at the comma, not at the closing parenthesis.
	source := "f :: proc (x: S64,) { }"
	result := parseTestCase(t, source)
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for trailing comma in parameter list")
	}
	// The diagnostic should point at the comma (offset 17)
	if result.Diags[0].Span.Start != 17 || result.Diags[0].Span.End != 18 {
		t.Errorf("diagnostic span = %#v, want offset 17-18 (the comma)", result.Diags[0].Span)
	}
	if result.Diags[0].Message != "trailing comma after parameter" {
		t.Errorf("diagnostic message = %q, want %q", result.Diags[0].Message, "trailing comma after parameter")
	}
	// Verify the parameter was still parsed (error recovery)
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if len(proc.Params) != 1 {
		t.Fatalf("Params = %d, want 1", len(proc.Params))
	}
	if proc.Params[0].Name != "x" {
		t.Errorf("param name = %q, want %q", proc.Params[0].Name, "x")
	}
}

func TestRegressionTrailingCommaArg(t *testing.T) {
	// Trailing comma in call argument list should produce an error
	// pointing at the comma, not at the closing parenthesis.
	source := "main :: proc { f(1,); }"
	result := parseTestCase(t, source)
	if !result.Diags.HasErrors() {
		t.Fatal("expected error for trailing comma in call argument")
	}
	// The diagnostic should point at the comma (offset 18)
	if result.Diags[0].Span.Start != 18 || result.Diags[0].Span.End != 19 {
		t.Errorf("diagnostic span = %#v, want offset 18-19 (the comma)", result.Diags[0].Span)
	}
	if result.Diags[0].Message != "trailing comma in call argument" {
		t.Errorf("diagnostic message = %q, want %q", result.Diags[0].Message, "trailing comma in call argument")
	}
	// Verify the argument was still parsed (error recovery)
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if len(proc.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(proc.Body.Stmts))
	}
	exprStmt, ok := proc.Body.Stmts[0].(*ExprStmt)
	if !ok {
		t.Fatalf("expected *ExprStmt, got %T", proc.Body.Stmts[0])
	}
	call, ok := exprStmt.Expr.(*CallExpr)
	if !ok {
		t.Fatalf("expected *CallExpr, got %T", exprStmt.Expr)
	}
	if len(call.Args) != 1 {
		t.Fatalf("Args = %d, want 1", len(call.Args))
	}
}

// ──────────────────────────────────────────────
// Tolerant mode
// ──────────────────────────────────────────────

func TestTolerantEntryProc(t *testing.T) {
	result := parseTolerantTestCase(t, "main :: #entry proc { return 0; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if proc.Name != "main" {
		t.Errorf("Name = %q, want %q", proc.Name, "main")
	}
}

func TestTolerantEntryProcDirectiveFirst(t *testing.T) {
	result := parseTolerantTestCase(t, "#entry main :: proc () -> I64 { return 10; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if proc.Name != "main" {
		t.Errorf("Name = %q, want %q", proc.Name, "main")
	}
	if len(proc.Results) != 1 {
		t.Fatalf("Results = %d, want 1", len(proc.Results))
	}
}

func TestStrictEntryProcDirectiveFirst(t *testing.T) {
	result := parseTestCase(t, "#entry main :: proc () -> I64 { return 10; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if proc.Name != "main" {
		t.Errorf("Name = %q, want %q", proc.Name, "main")
	}
}

func TestStrictOtherDirectiveStillErrors(t *testing.T) {
	result := parseTestCase(t, "#import «fmt.chaos»;")
	if !result.Diags.HasErrors() {
		t.Error("strict mode should error on #import, got none")
	}
}

func TestTolerantImportDirective(t *testing.T) {
	result := parseTolerantTestCase(t, "#import «fmt.chaos»;")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 0 {
		t.Fatalf("Decls = %d, want 0", len(result.Program.Decls))
	}
}

func TestTolerantStructParsed(t *testing.T) {
	result := parseTolerantTestCase(t, "Foo :: struct { x: S64; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
	if _, ok := result.Program.Decls[0].(*StructDecl); !ok {
		t.Fatalf("Decls[0] type = %T, want *StructDecl", result.Program.Decls[0])
	}
}

func TestTolerantStructAndEnum(t *testing.T) {
	source := "Foo :: struct { x: S64; }\nBar :: enum { A; B; }"
	result := parseTolerantTestCase(t, source)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	// Both struct and enum parse as declarations.
	if len(result.Program.Decls) != 2 {
		t.Fatalf("Decls = %d, want 2", len(result.Program.Decls))
	}
	if _, ok := result.Program.Decls[0].(*StructDecl); !ok {
		t.Fatalf("Decls[0] type = %T, want *StructDecl", result.Program.Decls[0])
	}
	if _, ok := result.Program.Decls[1].(*EnumDecl); !ok {
		t.Fatalf("Decls[1] type = %T, want *EnumDecl", result.Program.Decls[1])
	}
}

func TestTolerantBareErrorBlock(t *testing.T) {
	// Unknown braced forms are skipped, never invented as semantic error types.
	source := "Hash_Table_Error :: {\n    GENERIC;\n    NOT_FOUND;\n}\nsomething :: proc -> (String <> Hash_Table_Error) {\n    return .GENERIC!;\n}"
	result := parseTolerantTestCase(t, source)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors in tolerant mode: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want only the real procedure", len(result.Program.Decls))
	}
	if _, ok := result.Program.Decls[0].(*ProcDecl); !ok {
		t.Fatalf("Decls[0] type = %T, want *ProcDecl", result.Program.Decls[0])
	}

	// Strict mode rejects the same form.
	strict := parseTestCase(t, source)
	if !strict.Diags.HasErrors() {
		t.Fatal("expected errors in strict mode for ':: {' form, got none")
	}
}

func TestTolerantUnknownStmtSkipped(t *testing.T) {
	// 'while' is still an unknown statement keyword in tolerant mode and is
	// skipped as a balanced block; 'for' is now a real keyword.
	result := parseTolerantTestCase(t, "main :: proc { while i := 0; i < 10; i := i + 1 { exit 1; } return; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if len(proc.Body.Stmts) != 1 {
		t.Fatalf("Stmts = %d, want 1 (the return)", len(proc.Body.Stmts))
	}
}

func TestStrictDirectiveStillErrorsAndEnumParses(t *testing.T) {
	strict := parseTestCase(t, "main :: #entry proc { return 0; }")
	if !strict.Diags.HasErrors() {
		t.Error("strict mode should error on #entry proc, got none")
	}
	strictEnum := parseTestCase(t, "Bar :: enum { A: S64; B; }")
	if strictEnum.Diags.HasErrors() {
		t.Fatalf("strict mode should parse a valid enum: %v", strictEnum.Diags)
	}
	if len(strictEnum.Program.Decls) != 1 {
		t.Fatalf("strict enum declarations = %d, want 1", len(strictEnum.Program.Decls))
	}
	if _, ok := strictEnum.Program.Decls[0].(*EnumDecl); !ok {
		t.Fatalf("strict enum declaration = %T, want *EnumDecl", strictEnum.Program.Decls[0])
	}
}

func TestTolerantCommentsDoNotChangeParse(t *testing.T) {
	withComments := parseTolerantTestCase(t, "// header\nx :: 42; /** inline **/ y := 1;")
	withoutComments := parseTolerantTestCase(t, "x :: 42; y := 1;")
	if withComments.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", withComments.Diags)
	}
	if len(withComments.Program.Decls) != len(withoutComments.Program.Decls) {
		t.Fatalf("decl count with comments = %d, want %d", len(withComments.Program.Decls), len(withoutComments.Program.Decls))
	}
}

func TestTolerantBracelessFormDoesNotSwallowNextDecl(t *testing.T) {
	// 'Foo :: enum;' is invalid (enum requires a body), but the next
	// declaration 'x :: 42;' must still be parsed.
	result := parseTolerantTestCase(t, "Foo :: enum; x :: 42;")
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1 (x :: 42)", len(result.Program.Decls))
	}
	decl, ok := result.Program.Decls[0].(*VarDecl)
	if !ok {
		t.Fatalf("expected *VarDecl, got %T", result.Program.Decls[0])
	}
	if decl.Name != "x" {
		t.Errorf("Name = %q, want %q", decl.Name, "x")
	}
}

func TestTolerantRealErrorsStillSurface(t *testing.T) {
	result := parseTolerantTestCase(t, "x := ;")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for 'x := ;', got none")
	}
}

func TestTolerantGenericDeclarationsAreRecoveryOnly(t *testing.T) {
	tests := []string{
		"function5 <T: String | S64> :: proc (input1: T) { }\nnext :: 1;",
		"#entry function5 <T: String | S64> :: proc (input1: T) { }\nnext :: 1;",
		"function5 <T: String | S64> :: #entry proc (input1: T) { }\nnext :: 1;",
		"function8 <T: Array<S64>> :: proc { }\nnext :: 1;",
		"function9 <T: String | S64, U: S64> :: proc { }\nnext :: 1;",
		"Foo <T: S64> :: struct { x: T; }\nnext :: 1;",
	}
	for _, source := range tests {
		result := parseTolerantTestCase(t, source)
		if result.Diags.HasErrors() {
			t.Fatalf("unexpected errors for %q: %v", source, result.Diags)
		}
		if len(result.Program.Decls) != 1 {
			t.Fatalf("Decls = %d, want only the supported declaration for %q", len(result.Program.Decls), source)
		}
		decl, ok := result.Program.Decls[0].(*VarDecl)
		if !ok || decl.Name != "next" {
			t.Fatalf("recovered declaration = %T, want next *VarDecl for %q", result.Program.Decls[0], source)
		}
		if result.Program.EntryDecl != nil {
			t.Fatalf("unsupported generic declaration became entry for %q", source)
		}
	}
}

func TestTolerantGenericProcInBodyIsRecoveryOnly(t *testing.T) {
	result := parseTolerantTestCase(t, "main :: proc { inner <T: S64> :: proc { } value :: 1; }")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	proc := result.Program.Decls[0].(*ProcDecl)
	if len(proc.Body.Stmts) != 1 {
		t.Fatalf("Stmts = %d, want only value declaration", len(proc.Body.Stmts))
	}
	decl, ok := proc.Body.Stmts[0].(*VarDecl)
	if !ok || decl.Name != "value" {
		t.Fatalf("recovered statement = %T, want value *VarDecl", proc.Body.Stmts[0])
	}
}

func TestTolerantErrorDecl(t *testing.T) {
	result := parseTolerantTestCase(t, "Hash_Table_Error :: error {\n\tGENERIC;\n\tOUT_OF_MEMORY;\n}")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
	ed, ok := result.Program.Decls[0].(*ErrorDecl)
	if !ok {
		t.Fatalf("expected *ErrorDecl, got %T", result.Program.Decls[0])
	}
	if len(ed.Members) != 2 {
		t.Fatalf("Members = %d, want 2", len(ed.Members))
	}
}

func TestTolerantErrorReturnSpec(t *testing.T) {
	// The implemented '<>' error-return result parses in tolerant mode,
	// including with the '#entry' directive.
	sources := []string{
		"f :: proc -> (String <> Some_Error) {\n    return .GENERIC!;\n}",
		"#entry main :: proc -> (String <> Some_Error) {\n    return .GENERIC!;\n}",
	}
	for _, src := range sources {
		result := parseTolerantTestCase(t, src)
		if result.Diags.HasErrors() {
			t.Fatalf("unexpected errors for %q: %v", src, result.Diags)
		}
		if len(result.Program.Decls) != 1 {
			t.Fatalf("Decls = %d, want 1 for %q", len(result.Program.Decls), src)
		}
		proc, ok := result.Program.Decls[0].(*ProcDecl)
		if !ok {
			t.Fatalf("expected *ProcDecl, got %T for %q", result.Program.Decls[0], src)
		}
		if proc.ErrorResult == nil {
			t.Errorf("ErrorResult = nil, want Some_Error for %q", src)
		}
	}
}

func TestStrictGenericProcStillErrors(t *testing.T) {
	result := parseTestCase(t, "function5 <T: String | S64> :: proc (input1: T) { }")
	if !result.Diags.HasErrors() {
		t.Error("strict mode should error on generic proc, got none")
	}
}

// ──────────────────────────────────────────────
// Error handling (unless catch, if ... catch)
// ──────────────────────────────────────────────

func TestParseUnlessCatch(t *testing.T) {
	stmt := parseOneStmt(t, "r := f(5) unless catch {\n    return -1;\n}")
	u, ok := stmt.(*UnlessCatchStmt)
	if !ok {
		t.Fatalf("expected *UnlessCatchStmt, got %T", stmt)
	}
	if u.Target != "r" {
		t.Errorf("Target = %q, want r", u.Target)
	}
	if u.CatchName != "" {
		t.Errorf("CatchName = %q, want empty", u.CatchName)
	}
	if u.Init == nil {
		t.Fatal("Init = nil")
	}
	if u.CatchBody == nil {
		t.Fatal("CatchBody = nil")
	}
}

func TestParseUnlessCatchWithBinding(t *testing.T) {
	stmt := parseOneStmt(t, "r := f(5) unless catch err {\n    return -1;\n}")
	u, ok := stmt.(*UnlessCatchStmt)
	if !ok {
		t.Fatalf("expected *UnlessCatchStmt, got %T", stmt)
	}
	if u.CatchName != "err" {
		t.Errorf("CatchName = %q, want err", u.CatchName)
	}
	if u.CatchNameSpan.Start == 0 && u.CatchNameSpan.End == 0 {
		t.Error("CatchNameSpan is zero, want the binding span")
	}
}

func TestParseBareUnlessCatch(t *testing.T) {
	stmt := parseOneStmt(t, "f(5) unless catch {\n    return -1;\n}")
	u, ok := stmt.(*UnlessCatchStmt)
	if !ok {
		t.Fatalf("expected *UnlessCatchStmt, got %T", stmt)
	}
	if u.Target != "" {
		t.Errorf("Target = %q, want empty for bare form", u.Target)
	}
}

func TestParseIfCatch(t *testing.T) {
	stmt := parseOneStmt(t, "if r catch {\n    return -1;\n}")
	ic, ok := stmt.(*IfCatchStmt)
	if !ok {
		t.Fatalf("expected *IfCatchStmt, got %T", stmt)
	}
	if ic.CatchName != "" {
		t.Errorf("CatchName = %q, want empty", ic.CatchName)
	}
	if ic.Cond == nil {
		t.Fatal("Cond = nil")
	}
}

func TestParseIfCatchWithBinding(t *testing.T) {
	stmt := parseOneStmt(t, "if r catch err {\n    if err == .GENERIC! { return 1; }\n    return 2;\n}")
	ic, ok := stmt.(*IfCatchStmt)
	if !ok {
		t.Fatalf("expected *IfCatchStmt, got %T", stmt)
	}
	if ic.CatchName != "err" {
		t.Errorf("CatchName = %q, want err", ic.CatchName)
	}
}

func TestParseUnlessWithoutCatch(t *testing.T) {
	result := parseTestCase(t, "main :: proc { r := f(5) unless; }")
	if !result.Diags.HasErrors() {
		t.Error("expected errors for 'unless' without 'catch', got none")
	}
}

func TestTolerantUnlessCatch(t *testing.T) {
	result := parseTolerantTestCase(t, "main :: proc {\n    r := f(5) unless catch err {\n        return -1;\n    }\n    return r;\n}")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	proc, ok := result.Program.Decls[0].(*ProcDecl)
	if !ok {
		t.Fatalf("expected *ProcDecl, got %T", result.Program.Decls[0])
	}
	if len(proc.Body.Stmts) != 2 {
		t.Fatalf("Stmts = %d, want 2 (unless catch + return)", len(proc.Body.Stmts))
	}
	if _, ok := proc.Body.Stmts[0].(*UnlessCatchStmt); !ok {
		t.Fatalf("Stmts[0] type = %T, want *UnlessCatchStmt", proc.Body.Stmts[0])
	}
}

func TestMultipleResultsRequireParentheses(t *testing.T) {
	bad := parseTestCase(t, "f :: proc -> S64, Bool { return 1, true; }")
	if !hasError(bad.Diags, "multiple procedure results must be parenthesized") {
		t.Fatalf("expected parenthesis diagnostic, got %v", bad.Diags)
	}
	good := parseTestCase(t, "f :: proc -> (S64, Bool) { return 1, true; }")
	if good.Diags.HasErrors() {
		t.Fatalf("parenthesized multiple results failed: %v", good.Diags)
	}
}

func TestProcedureItemLimitIsOneHundred(t *testing.T) {
	makeList := func(count int, item func(int) string) string {
		parts := make([]string, count)
		for i := range parts {
			parts[i] = item(i)
		}
		return strings.Join(parts, ", ")
	}
	params100 := makeList(100, func(i int) string { return "a" + strconv.Itoa(i) + ": S64" })
	args100 := makeList(100, func(int) string { return "0" })
	valid := "f :: proc (" + params100 + ") { }\ng :: proc { f(" + args100 + "); }"
	if result := parseTestCase(t, valid); result.Diags.HasErrors() {
		t.Fatalf("100 inputs should parse: %v", result.Diags)
	}
	params101 := makeList(101, func(i int) string { return "a" + strconv.Itoa(i) + ": S64" })
	result := parseTestCase(t, "f :: proc ("+params101+") { }")
	if !hasError(result.Diags, "Dumb bitch") {
		t.Fatalf("expected profane 101-input diagnostic, got %v", result.Diags)
	}
	results101 := makeList(101, func(int) string { return "S64" })
	result = parseTestCase(t, "f :: proc -> ("+results101+") { return; }")
	if !hasError(result.Diags, "Dumb bitch") {
		t.Fatalf("expected profane 101-result diagnostic, got %v", result.Diags)
	}
}
