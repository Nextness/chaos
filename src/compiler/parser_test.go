package compiler

import (
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
	decl := parseOneDecl(t, "divmod :: proc (a: S64, b: S64) -> S64, S64 { return 0; }")
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
	expr := parseExpr(t, "--5")
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
	source := "Foo :: struct { x: S64; }\nBar :: enum { A, B }"
	result := parseTolerantTestCase(t, source)
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
	// struct parses as a declaration; enum is still skipped.
	if len(result.Program.Decls) != 1 {
		t.Fatalf("Decls = %d, want 1", len(result.Program.Decls))
	}
	if _, ok := result.Program.Decls[0].(*StructDecl); !ok {
		t.Fatalf("Decls[0] type = %T, want *StructDecl", result.Program.Decls[0])
	}
}

func TestTolerantUnknownStmtSkipped(t *testing.T) {
	result := parseTolerantTestCase(t, "main :: proc { for i := 0; i < 10; i := i + 1 { exit 1; } return; }")
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

func TestTolerantStrictStillErrors(t *testing.T) {
	strict := parseTestCase(t, "main :: #entry proc { return 0; }")
	if !strict.Diags.HasErrors() {
		t.Error("strict mode should error on #entry proc, got none")
	}
	strictEnum := parseTestCase(t, "Bar :: enum { A, B }")
	if !strictEnum.Diags.HasErrors() {
		t.Error("strict mode should error on enum, got none")
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
	result := parseTolerantTestCase(t, "Foo :: enum; x :: 42;")
	if result.Diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", result.Diags)
	}
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
