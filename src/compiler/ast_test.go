package compiler

import (
	"testing"
)

func TestNodeSpanAccessors(t *testing.T) {
	span := Span{File: 1, Start: 10, End: 20}

	tests := []struct {
		name string
		node Node
		want Span
	}{
		{name: "IdentExpr", node: &IdentExpr{Span_: span, Name: "x"}, want: span},
		{name: "IntExpr", node: &IntExpr{Span_: span, Value: "42"}, want: span},
		{name: "FloatExpr", node: &FloatExpr{Span_: span, Value: "3.14"}, want: span},
		{name: "StringExpr", node: &StringExpr{Span_: span, Value: "hello"}, want: span},
		{name: "BoolExpr", node: &BoolExpr{Span_: span, Value: true}, want: span},
		{name: "BinaryExpr", node: &BinaryExpr{Span_: span, Left: &IntExpr{Span_: span, Value: "1"}, Right: &IntExpr{Span_: span, Value: "2"}, Op: BinaryOpAdd}, want: span},
		{name: "UnaryExpr", node: &UnaryExpr{Span_: span, Op: UnaryOpNeg, Operand: &IntExpr{Span_: span, Value: "1"}}, want: span},
		{name: "CallExpr", node: &CallExpr{Span_: span, Func: &IdentExpr{Span_: span, Name: "f"}}, want: span},
		{name: "ParenExpr", node: &ParenExpr{Span_: span, Inner: &IntExpr{Span_: span, Value: "1"}}, want: span},
		{name: "ErrorExpr", node: &ErrorExpr{Span_: span}, want: span},
		{name: "VarDecl", node: &VarDecl{Span_: span, Name: "x"}, want: span},
		{name: "AssignStmt", node: &AssignStmt{Span_: span, Name: "x"}, want: span},
		{name: "ReturnStmt", node: &ReturnStmt{Span_: span}, want: span},
		{name: "ExitStmt", node: &ExitStmt{Span_: span, Status: &IntExpr{Span_: span, Value: "0"}}, want: span},
		{name: "IfStmt", node: &IfStmt{Span_: span, Condition: &BoolExpr{Span_: span, Value: true}, Body: &BlockStmt{Span_: span}}, want: span},
		{name: "BlockStmt", node: &BlockStmt{Span_: span}, want: span},
		{name: "ProcDecl", node: &ProcDecl{Span_: span, Name: "main", Body: &BlockStmt{Span_: span}}, want: span},
		{name: "ExprStmt", node: &ExprStmt{Span_: span, Expr: &IntExpr{Span_: span, Value: "1"}}, want: span},
		{name: "Param", node: Param{Span_: span, Name: "x", Type: &IdentExpr{Span_: span, Name: "S64"}}, want: span},
		{name: "Program", node: &Program{Decls: []Decl{&VarDecl{Span_: span, Name: "x"}}}, want: span},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.nodeSpan(); got != tt.want {
				t.Errorf("nodeSpan() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExprTypeAssertions(t *testing.T) {
	span := Span{File: 0, Start: 0, End: 1}
	exprs := []Expr{
		&IdentExpr{Span_: span, Name: "x"},
		&IntExpr{Span_: span, Value: "1"},
		&FloatExpr{Span_: span, Value: "1.0"},
		&StringExpr{Span_: span, Value: "s"},
		&BoolExpr{Span_: span, Value: true},
		&BinaryExpr{Span_: span, Left: &IntExpr{Span_: span, Value: "1"}, Right: &IntExpr{Span_: span, Value: "2"}},
		&UnaryExpr{Span_: span, Op: UnaryOpNeg, Operand: &IntExpr{Span_: span, Value: "1"}},
		&CallExpr{Span_: span, Func: &IdentExpr{Span_: span, Name: "f"}},
		&ParenExpr{Span_: span, Inner: &IntExpr{Span_: span, Value: "1"}},
		&ErrorExpr{Span_: span},
	}

	for _, e := range exprs {
		// All Expr values must implement Node (via exprNode marker)
		_ = Node(e)
		// All must have a span
		if e.nodeSpan() != span {
			t.Errorf("unexpected span for %T", e)
		}
	}
}

func TestStmtTypeAssertions(t *testing.T) {
	span := Span{File: 0, Start: 0, End: 1}
	stmts := []Stmt{
		&VarDecl{Span_: span, Name: "x"},
		&AssignStmt{Span_: span, Name: "x"},
		&ReturnStmt{Span_: span},
		&ExitStmt{Span_: span, Status: &IntExpr{Span_: span, Value: "0"}},
		&IfStmt{Span_: span, Condition: &BoolExpr{Span_: span, Value: true}, Body: &BlockStmt{Span_: span}},
		&BlockStmt{Span_: span},
		&ProcDecl{Span_: span, Name: "main", Body: &BlockStmt{Span_: span}},
		&ExprStmt{Span_: span, Expr: &IntExpr{Span_: span, Value: "1"}},
	}

	for _, s := range stmts {
		_ = Node(s)
		if s.nodeSpan() != span {
			t.Errorf("unexpected span for %T", s)
		}
	}
}

func TestDeclTypeAssertions(t *testing.T) {
	span := Span{File: 0, Start: 0, End: 1}
	decls := []Decl{
		&VarDecl{Span_: span, Name: "x"},
		&ProcDecl{Span_: span, Name: "main", Body: &BlockStmt{Span_: span}},
	}

	for _, d := range decls {
		_ = Stmt(d)
		_ = Node(d)
		if d.nodeSpan() != span {
			t.Errorf("unexpected span for %T", d)
		}
	}
}

func TestBinaryOpValues(t *testing.T) {
	// Verify that all BinaryOp values are distinct and within range.
	seen := make(map[BinaryOp]bool)
	ops := []BinaryOp{
		BinaryOpAdd, BinaryOpSub, BinaryOpMul, BinaryOpDiv,
		BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe,
		BinaryOpEq, BinaryOpNeq, BinaryOpAnd, BinaryOpOr,
	}
	for _, op := range ops {
		if seen[op] {
			t.Errorf("duplicate BinaryOp value %d", op)
		}
		seen[op] = true
	}
	if len(seen) != len(ops) {
		t.Errorf("expected %d unique BinaryOp values, got %d", len(ops), len(seen))
	}
}

func TestUnaryOpValues(t *testing.T) {
	seen := make(map[UnaryOp]bool)
	ops := []UnaryOp{UnaryOpNeg, UnaryOpNot}
	for _, op := range ops {
		if seen[op] {
			t.Errorf("duplicate UnaryOp value %d", op)
		}
		seen[op] = true
	}
	if len(seen) != len(ops) {
		t.Errorf("expected %d unique UnaryOp values, got %d", len(ops), len(seen))
	}
}

func TestSpanUnion(t *testing.T) {
	tests := []struct {
		name string
		a, b Span
		want Span
	}{
		{
			name: "a before b",
			a:    Span{File: 0, Start: 5, End: 10},
			b:    Span{File: 0, Start: 15, End: 20},
			want: Span{File: 0, Start: 5, End: 20},
		},
		{
			name: "b before a",
			a:    Span{File: 0, Start: 15, End: 20},
			b:    Span{File: 0, Start: 5, End: 10},
			want: Span{File: 0, Start: 5, End: 20},
		},
		{
			name: "overlapping",
			a:    Span{File: 0, Start: 5, End: 15},
			b:    Span{File: 0, Start: 10, End: 20},
			want: Span{File: 0, Start: 5, End: 20},
		},
		{
			name: "same",
			a:    Span{File: 0, Start: 5, End: 10},
			b:    Span{File: 0, Start: 5, End: 10},
			want: Span{File: 0, Start: 5, End: 10},
		},
		{
			name: "different files uses a's file",
			a:    Span{File: 1, Start: 5, End: 10},
			b:    Span{File: 2, Start: 0, End: 20},
			want: Span{File: 1, Start: 0, End: 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spanUnion(tt.a, tt.b); got != tt.want {
				t.Errorf("spanUnion(%#v, %#v) = %#v, want %#v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestTokToBinaryOp(t *testing.T) {
	tests := []struct {
		kind TokenKind
		want int
	}{
		{TkPlus, int(BinaryOpAdd)},
		{TkMinus, int(BinaryOpSub)},
		{TkStar, int(BinaryOpMul)},
		{TkSlash, int(BinaryOpDiv)},
		{TkPercent, int(BinaryOpMod)},
		{TkLt, int(BinaryOpLt)},
		{TkGt, int(BinaryOpGt)},
		{TkLe, int(BinaryOpLe)},
		{TkGe, int(BinaryOpGe)},
		{TkEq, int(BinaryOpEq)},
		{TkNeq, int(BinaryOpNeq)},
		{TkAnd, int(BinaryOpAnd)},
		{TkOr, int(BinaryOpOr)},
		{TkEOF, -1},
		{TkIdent, -1},
		{TkAssign, -1},
	}

	for _, tt := range tests {
		if got := tokToBinaryOp(tt.kind); got != tt.want {
			t.Errorf("tokToBinaryOp(%s) = %d, want %d", tt.kind, got, tt.want)
		}
	}
}

func TestTokenPrecedence(t *testing.T) {
	tests := []struct {
		kind TokenKind
		want int
	}{
		{TkOr, precOr},
		{TkAnd, precAnd},
		{TkEq, precEq},
		{TkNeq, precEq},
		{TkLt, precCmp},
		{TkGt, precCmp},
		{TkLe, precCmp},
		{TkGe, precCmp},
		{TkPlus, precAdd},
		{TkMinus, precAdd},
		{TkStar, precMul},
		{TkSlash, precMul},
		{TkPercent, precMul},
		{TkLParen, precCall},
		{TkEOF, 0},
		{TkIdent, 0},
		{TkSemicolon, 0},
	}

	for _, tt := range tests {
		if got := tokenPrecedence(tt.kind); got != tt.want {
			t.Errorf("tokenPrecedence(%s) = %d, want %d", tt.kind, got, tt.want)
		}
	}
}
