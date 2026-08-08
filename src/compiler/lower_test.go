package compiler

import "testing"

// lowerSource tokenizes, parses, type-checks, and lowers a source string to
// both HIR and MIR, failing the test on any error. The MIR is verified as a
// sanity check that lowering produces well-formed output.
func lowerSource(t *testing.T, source string) (*HIR, *MIRProgram) {
	t.Helper()
	tokens, _ := Tokenize([]byte(source), 0)
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		t.Fatalf("parse errors in %q: %v", source, result.Diags)
	}
	if diags := CheckProgram(result.Program); diags.HasErrors() {
		t.Fatalf("type errors in %q: %v", source, diags)
	}
	hir, diags := LowerProgram(result.Program)
	if diags.HasErrors() {
		t.Fatalf("lower errors in %q: %v", source, diags)
	}
	mir, diags := LowerToMIR(hir)
	if diags.HasErrors() {
		t.Fatalf("mir lower errors in %q: %v", source, diags)
	}
	if diags := VerifyMIR(mir); diags.HasErrors() {
		t.Fatalf("verify errors in %q: %v", source, diags)
	}
	return hir, mir
}

func findHIRProc(hir *HIR, name string) *HIRProc {
	for _, p := range hir.Procs {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func typeName(hir *HIR, id TypeID) string {
	return hir.Types.Lookup(id).Name
}

func TestLowerProc(t *testing.T) {
	hir, _ := lowerSource(t, "add :: proc (a: S64, b: S64) -> S64 {\n    return a + b;\n}")
	p := findHIRProc(hir, "add")
	if p == nil {
		t.Fatal("proc add not found")
	}
	if len(p.Params) != 2 || p.Params[0].Name != "a" || p.Params[1].Name != "b" {
		t.Errorf("params = %+v, want a and b", p.Params)
	}
	if len(p.Results) != 1 || typeName(hir, p.Results[0]) != "S64" {
		t.Errorf("results = %+v, want S64", p.Results)
	}
	if p.Body == nil || len(p.Body.Stmts) != 1 {
		t.Fatalf("body = %+v, want one statement", p.Body)
	}
	ret, ok := p.Body.Stmts[0].(*HIRReturn)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRReturn", p.Body.Stmts[0])
	}
	bin, ok := ret.Value.(*HIRBinary)
	if !ok {
		t.Fatalf("return value = %T, want *HIRBinary", ret.Value)
	}
	if bin.Op != BinaryOpAdd {
		t.Errorf("binary op = %v, want +", bin.Op)
	}
	if typeName(hir, bin.Type) != "S64" {
		t.Errorf("binary type = %s, want S64", typeName(hir, bin.Type))
	}
}

func TestLowerVarDeclLiteralAdaptation(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    a: U64 = 10;\n    b := 10;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	typed, ok := p.Body.Stmts[0].(*HIRVarDecl)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRVarDecl", p.Body.Stmts[0])
	}
	if typeName(hir, typed.Type) != "U64" {
		t.Errorf("a type = %s, want U64", typeName(hir, typed.Type))
	}
	lit, ok := typed.Init.(*HIRConst)
	if !ok {
		t.Fatalf("a init = %T, want *HIRConst", typed.Init)
	}
	if typeName(hir, lit.Type) != "U64" {
		t.Errorf("a init literal type = %s, want U64 (literal adaptation)", typeName(hir, lit.Type))
	}
	inferred, ok := p.Body.Stmts[1].(*HIRVarDecl)
	if !ok {
		t.Fatalf("second statement = %T, want *HIRVarDecl", p.Body.Stmts[1])
	}
	if typeName(hir, inferred.Type) != "S64" {
		t.Errorf("b type = %s, want S64", typeName(hir, inferred.Type))
	}
}

func TestLowerBinaryLiteralAdaptation(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    x: U64 = 10;\n    y := x + 1;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	vd, ok := p.Body.Stmts[1].(*HIRVarDecl)
	if !ok {
		t.Fatalf("second statement = %T, want *HIRVarDecl", p.Body.Stmts[1])
	}
	bin, ok := vd.Init.(*HIRBinary)
	if !ok {
		t.Fatalf("y init = %T, want *HIRBinary", vd.Init)
	}
	if typeName(hir, bin.Type) != "U64" {
		t.Errorf("binary result type = %s, want U64", typeName(hir, bin.Type))
	}
	right, ok := bin.Right.(*HIRConst)
	if !ok {
		t.Fatalf("binary right = %T, want *HIRConst", bin.Right)
	}
	if typeName(hir, right.Type) != "U64" {
		t.Errorf("literal operand type = %s, want U64 (literal adaptation)", typeName(hir, right.Type))
	}
}

func TestLowerNegativeLiteralAdaptation(t *testing.T) {
	// A negated literal adapts to the declared type: the inner constant and
	// the unary node both carry the target type.
	hir, _ := lowerSource(t, "main :: proc {\n    a: S128 = -100;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	vd, ok := p.Body.Stmts[0].(*HIRVarDecl)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRVarDecl", p.Body.Stmts[0])
	}
	un, ok := vd.Init.(*HIRUnary)
	if !ok {
		t.Fatalf("a init = %T, want *HIRUnary", vd.Init)
	}
	if un.Op != UnaryOpNeg {
		t.Errorf("unary op = %v, want neg", un.Op)
	}
	if typeName(hir, un.Type) != "S128" {
		t.Errorf("negated literal type = %s, want S128 (literal adaptation)", typeName(hir, un.Type))
	}
	if c, ok := un.Operand.(*HIRConst); !ok || typeName(hir, c.Type) != "S128" {
		t.Errorf("negated literal operand = %#v, want S128-typed constant", un.Operand)
	}
}

func TestLowerReturnLiteralAdaptation(t *testing.T) {
	// A return literal adapts to the procedure's result type.
	hir, _ := lowerSource(t, "f :: proc -> S128 {\n    return -100;\n}")
	p := findHIRProc(hir, "f")
	if p == nil {
		t.Fatal("proc f not found")
	}
	rs, ok := p.Body.Stmts[0].(*HIRReturn)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRReturn", p.Body.Stmts[0])
	}
	un, ok := rs.Value.(*HIRUnary)
	if !ok {
		t.Fatalf("return value = %T, want *HIRUnary", rs.Value)
	}
	if typeName(hir, un.Type) != "S128" {
		t.Errorf("return literal type = %s, want S128 (result adaptation)", typeName(hir, un.Type))
	}
}

func TestLowerIfElifElse(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    a := true;\n    b := false;\n    if a { } elif b { } else { }\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	stmt, ok := p.Body.Stmts[2].(*HIRIf)
	if !ok {
		t.Fatalf("third statement = %T, want *HIRIf", p.Body.Stmts[2])
	}
	if stmt.Condition == nil || stmt.Then == nil {
		t.Errorf("if missing condition or body")
	}
	if len(stmt.Elif) != 1 {
		t.Fatalf("elif count = %d, want 1", len(stmt.Elif))
	}
	if stmt.Elif[0].Condition == nil || stmt.Elif[0].Then == nil {
		t.Errorf("elif missing condition or body")
	}
	if stmt.Else == nil {
		t.Errorf("else body missing")
	}
}

func TestLowerStructAndLiteral(t *testing.T) {
	hir, _ := lowerSource(t, "Point :: struct { x: S64; y: S64; }\nmain :: proc {\n    p := Point.{x=1, y=2};\n}")
	if len(hir.Structs) != 1 || hir.Structs[0].Name != "Point" {
		t.Fatalf("structs = %+v, want Point", hir.Structs)
	}
	if len(hir.Structs[0].Fields) != 2 || hir.Structs[0].Fields[0].Name != "x" {
		t.Errorf("struct fields = %+v, want x and y", hir.Structs[0].Fields)
	}
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	vd, ok := p.Body.Stmts[0].(*HIRVarDecl)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRVarDecl", p.Body.Stmts[0])
	}
	si, ok := vd.Init.(*HIRStructInit)
	if !ok {
		t.Fatalf("p init = %T, want *HIRStructInit", vd.Init)
	}
	if typeName(hir, si.Type) != "Point" {
		t.Errorf("struct init type = %s, want Point", typeName(hir, si.Type))
	}
	if len(si.Fields) != 2 {
		t.Fatalf("struct init fields = %d, want 2", len(si.Fields))
	}
	if hir.Symbols.Lookup(si.Fields[0].Field) != "x" {
		t.Errorf("first field symbol = %s, want x", hir.Symbols.Lookup(si.Fields[0].Field))
	}
}

func TestLowerInferredStructLiteral(t *testing.T) {
	hir, _ := lowerSource(t, "Point :: struct { x: S64; y: S64; }\nmain :: proc {\n    p: Point = .{x=1, y=2};\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	vd, ok := p.Body.Stmts[0].(*HIRVarDecl)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRVarDecl", p.Body.Stmts[0])
	}
	si, ok := vd.Init.(*HIRStructInit)
	if !ok {
		t.Fatalf("p init = %T, want *HIRStructInit", vd.Init)
	}
	if typeName(hir, si.Type) != "Point" {
		t.Errorf("inferred struct init type = %s, want Point", typeName(hir, si.Type))
	}
}

func TestLowerCall(t *testing.T) {
	hir, _ := lowerSource(t, "add :: proc (a: S64, b: S64) -> S64 { return a + b; }\nmain :: proc {\n    x := add(1, 2);\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	vd, ok := p.Body.Stmts[0].(*HIRVarDecl)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRVarDecl", p.Body.Stmts[0])
	}
	call, ok := vd.Init.(*HIRCall)
	if !ok {
		t.Fatalf("x init = %T, want *HIRCall", vd.Init)
	}
	if hir.Symbols.Lookup(call.Func) != "add" {
		t.Errorf("call target = %s, want add", hir.Symbols.Lookup(call.Func))
	}
	if len(call.Args) != 2 {
		t.Fatalf("call args = %d, want 2", len(call.Args))
	}
	if typeName(hir, call.Type) != "S64" {
		t.Errorf("call type = %s, want S64", typeName(hir, call.Type))
	}
}

func TestLowerExit(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    exit 1, «boom»;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	ex, ok := p.Body.Stmts[0].(*HIRExit)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRExit", p.Body.Stmts[0])
	}
	if ex.Status == nil || ex.Message == nil {
		t.Errorf("exit missing status or message")
	}
}

func TestLowerGlobal(t *testing.T) {
	hir, _ := lowerSource(t, "VARIABLE3 :: 10.1;")
	if len(hir.Globals) != 1 {
		t.Fatalf("globals = %d, want 1", len(hir.Globals))
	}
	g := hir.Globals[0]
	if g.Name != "VARIABLE3" {
		t.Errorf("global name = %s, want VARIABLE3", g.Name)
	}
	if typeName(hir, g.Type) != "F64" {
		t.Errorf("global type = %s, want F64", typeName(hir, g.Type))
	}
	if !g.CompileTime || g.Mutable {
		t.Errorf("global flags = mutable=%v compileTime=%v, want compile-time constant", g.Mutable, g.CompileTime)
	}
}

func TestLowerShadowing(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    x := 1;\n    {\n        x := 2;\n    }\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	outer, ok := p.Body.Stmts[0].(*HIRVarDecl)
	if !ok {
		t.Fatalf("first statement = %T, want *HIRVarDecl", p.Body.Stmts[0])
	}
	block, ok := p.Body.Stmts[1].(*HIRBlock)
	if !ok {
		t.Fatalf("second statement = %T, want *HIRBlock", p.Body.Stmts[1])
	}
	inner, ok := block.Stmts[0].(*HIRVarDecl)
	if !ok {
		t.Fatalf("inner statement = %T, want *HIRVarDecl", block.Stmts[0])
	}
	if outer.Symbol == inner.Symbol {
		t.Errorf("shadowed declarations share SymbolID %d", outer.Symbol)
	}
}

func TestLowerUnary(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    flag := false;\n    x := -5;\n    y := !flag;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	neg, ok := p.Body.Stmts[1].(*HIRVarDecl)
	if !ok {
		t.Fatalf("second statement = %T, want *HIRVarDecl", p.Body.Stmts[1])
	}
	un, ok := neg.Init.(*HIRUnary)
	if !ok {
		t.Fatalf("x init = %T, want *HIRUnary", neg.Init)
	}
	if un.Op != UnaryOpNeg || typeName(hir, un.Type) != "S64" {
		t.Errorf("neg = op %v type %s, want - S64", un.Op, typeName(hir, un.Type))
	}
	not, ok := p.Body.Stmts[2].(*HIRVarDecl)
	if !ok {
		t.Fatalf("third statement = %T, want *HIRVarDecl", p.Body.Stmts[2])
	}
	un2, ok := not.Init.(*HIRUnary)
	if !ok {
		t.Fatalf("y init = %T, want *HIRUnary", not.Init)
	}
	if un2.Op != UnaryOpNot || typeName(hir, un2.Type) != "Bool" {
		t.Errorf("not = op %v type %s, want ! Bool", un2.Op, typeName(hir, un2.Type))
	}
}

func TestLowerLogicalOps(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    a := true;\n    b := false;\n    c := a && b;\n    d := a || b;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	and, ok := p.Body.Stmts[2].(*HIRVarDecl)
	if !ok {
		t.Fatalf("third statement = %T, want *HIRVarDecl", p.Body.Stmts[2])
	}
	bin, ok := and.Init.(*HIRBinary)
	if !ok || bin.Op != BinaryOpAnd || typeName(hir, bin.Type) != "Bool" {
		t.Errorf("c init = %+v, want && Bool", and.Init)
	}
	or, ok := p.Body.Stmts[3].(*HIRVarDecl)
	if !ok {
		t.Fatalf("fourth statement = %T, want *HIRVarDecl", p.Body.Stmts[3])
	}
	bin2, ok := or.Init.(*HIRBinary)
	if !ok || bin2.Op != BinaryOpOr || typeName(hir, bin2.Type) != "Bool" {
		t.Errorf("d init = %+v, want || Bool", or.Init)
	}
}

func TestLowerAssign(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    x := 1;\n    x = 2;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	as, ok := p.Body.Stmts[1].(*HIRAssign)
	if !ok {
		t.Fatalf("second statement = %T, want *HIRAssign", p.Body.Stmts[1])
	}
	if hir.Symbols.Lookup(as.Target) != "x" {
		t.Errorf("assign target = %s, want x", hir.Symbols.Lookup(as.Target))
	}
	if typeName(hir, as.Value.hirType()) != "S64" {
		t.Errorf("assign value type = %s, want S64", typeName(hir, as.Value.hirType()))
	}
}

func TestLowerFloatStringBoolConsts(t *testing.T) {
	hir, _ := lowerSource(t, "main :: proc {\n    a := 1.5;\n    b := «hi»;\n    c := true;\n}")
	p := findHIRProc(hir, "main")
	if p == nil {
		t.Fatal("proc main not found")
	}
	checks := []struct {
		idx  int
		kind ConstKind
		name string
	}{
		{0, ConstFloat, "F64"},
		{1, ConstString, "String"},
		{2, ConstBool, "Bool"},
	}
	for _, c := range checks {
		vd, ok := p.Body.Stmts[c.idx].(*HIRVarDecl)
		if !ok {
			t.Fatalf("statement %d = %T, want *HIRVarDecl", c.idx, p.Body.Stmts[c.idx])
		}
		lit, ok := vd.Init.(*HIRConst)
		if !ok || lit.Kind != c.kind {
			t.Errorf("statement %d literal = %+v, want kind %v", c.idx, vd.Init, c.kind)
		}
		if typeName(hir, vd.Type) != c.name {
			t.Errorf("statement %d type = %s, want %s", c.idx, typeName(hir, vd.Type), c.name)
		}
	}
}
