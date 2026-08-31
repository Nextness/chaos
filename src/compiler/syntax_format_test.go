package compiler

import "testing"

func TestFormatTypeExpr(t *testing.T) {
	decl := parseOneDecl(t, "items: [COUNT]*Byte? = undefined;").(*VarDecl)
	if got, want := FormatTypeExpr(decl.DeclType), "[COUNT]*Byte?"; got != want {
		t.Fatalf("FormatTypeExpr() = %q, want %q", got, want)
	}
}
