// Debug dump helpers for the tokenizer, parser, and AST.
//
// These are exported so the CLI and tests can render the intermediate
// representations of a source file for inspection.
package compiler

import (
	"fmt"
	"strings"
)

// DumpTokens returns a human-readable listing of the token stream.
func DumpTokens(tokens []Token) string {
	var b strings.Builder
	for i, tok := range tokens {
		fmt.Fprintf(&b, "  [%d] %s", i, tok.Kind.String())
		if tok.Value != "" {
			fmt.Fprintf(&b, "(%s)", tok.Value)
		}
		fmt.Fprintf(&b, " span=[%d,%d)\n", tok.Span.Start, tok.Span.End)
	}
	return b.String()
}

// DumpParseResult returns a human-readable dump of a parse result: the
// diagnostics followed by the AST.
func DumpParseResult(result ParseResult) string {
	var b strings.Builder
	b.WriteString("Diagnostics:\n")
	if len(result.Diags) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, d := range result.Diags {
		fmt.Fprintf(&b, "  %s: %s\n", d.Severity, d.Message)
	}
	b.WriteString("AST:\n")
	b.WriteString(DumpAST(result.Program))
	return b.String()
}

// DumpAST returns a human-readable, indented tree of the AST.
func DumpAST(program *Program) string {
	var b strings.Builder
	b.WriteString("Program\n")
	for _, d := range program.Decls {
		dumpDecl(&b, d, 1)
	}
	return b.String()
}

func dumpIndent(b *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
}

func dumpDecl(b *strings.Builder, d Decl, depth int) {
	switch n := d.(type) {
	case *ProcDecl:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "ProcDecl %s\n", n.Name)
		for _, p := range n.Params {
			dumpIndent(b, depth+1)
			fmt.Fprintf(b, "Param %s: ", p.Name)
			dumpExpr(b, p.Type)
			b.WriteString("\n")
		}
		if len(n.Results) > 0 {
			dumpIndent(b, depth+1)
			b.WriteString("Results: ")
			for i, r := range n.Results {
				if i > 0 {
					b.WriteString(", ")
				}
				dumpExpr(b, r)
			}
			b.WriteString("\n")
		}
		dumpBlock(b, n.Body, depth+1)
	case *StructDecl:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "StructDecl %s\n", n.Name)
		for _, f := range n.Fields {
			dumpIndent(b, depth+1)
			fmt.Fprintf(b, "Field %s: ", f.Name)
			dumpExpr(b, f.Type)
			b.WriteString("\n")
		}
	case *VarDecl:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "VarDecl %s", n.Name)
		if n.DeclType != nil {
			b.WriteString(": ")
			dumpExpr(b, n.DeclType)
		}
		if n.Init != nil {
			b.WriteString(" = ")
			dumpExpr(b, n.Init)
		}
		fmt.Fprintf(b, " [mutable=%v compileTime=%v]\n", n.Mutable, n.CompileTime)
	default:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "Decl %T\n", d)
	}
}

func dumpStmt(b *strings.Builder, s Stmt, depth int) {
	switch n := s.(type) {
	case *VarDecl, *ProcDecl, *StructDecl:
		dumpDecl(b, n.(Decl), depth)
	case *AssignStmt:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "AssignStmt %s = ", n.Name)
		dumpExpr(b, n.Value)
		b.WriteString("\n")
	case *ReturnStmt:
		dumpIndent(b, depth)
		b.WriteString("ReturnStmt")
		if n.Value != nil {
			b.WriteString(" ")
			dumpExpr(b, n.Value)
		}
		b.WriteString("\n")
	case *ExitStmt:
		dumpIndent(b, depth)
		b.WriteString("ExitStmt")
		if n.Status != nil {
			b.WriteString(" status=")
			dumpExpr(b, n.Status)
		}
		if n.Message != nil {
			b.WriteString(" message=")
			dumpExpr(b, n.Message)
		}
		b.WriteString("\n")
	case *IfStmt:
		dumpIndent(b, depth)
		b.WriteString("IfStmt condition=")
		dumpExpr(b, n.Condition)
		b.WriteString("\n")
		dumpBlock(b, n.Body, depth+1)
		for _, e := range n.Elif {
			dumpIndent(b, depth)
			b.WriteString("Elif condition=")
			dumpExpr(b, e.Condition)
			b.WriteString("\n")
			dumpBlock(b, e.Body, depth+1)
		}
		if n.ElseBody != nil {
			dumpIndent(b, depth)
			b.WriteString("Else\n")
			dumpBlock(b, n.ElseBody, depth+1)
		}
	case *BlockStmt:
		dumpBlock(b, n, depth)
	case *ExprStmt:
		dumpIndent(b, depth)
		b.WriteString("ExprStmt ")
		dumpExpr(b, n.Expr)
		b.WriteString("\n")
	default:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "Stmt %T\n", s)
	}
}

func dumpBlock(b *strings.Builder, blk *BlockStmt, depth int) {
	dumpIndent(b, depth)
	b.WriteString("Block\n")
	for _, s := range blk.Stmts {
		dumpStmt(b, s, depth+1)
	}
}

func dumpExpr(b *strings.Builder, e Expr) {
	switch n := e.(type) {
	case *IdentExpr:
		fmt.Fprintf(b, "Ident(%s)", n.Name)
	case *IntExpr:
		fmt.Fprintf(b, "Int(%s)", n.Value)
	case *FloatExpr:
		fmt.Fprintf(b, "Float(%s)", n.Value)
	case *StringExpr:
		fmt.Fprintf(b, "String(%q)", n.Value)
	case *BoolExpr:
		fmt.Fprintf(b, "Bool(%v)", n.Value)
	case *BinaryExpr:
		fmt.Fprintf(b, "Binary(%s ", n.Op)
		dumpExpr(b, n.Left)
		b.WriteString(" ")
		dumpExpr(b, n.Right)
		b.WriteString(")")
	case *UnaryExpr:
		fmt.Fprintf(b, "Unary(%s ", n.Op)
		dumpExpr(b, n.Operand)
		b.WriteString(")")
	case *CallExpr:
		b.WriteString("Call(")
		dumpExpr(b, n.Func)
		for _, a := range n.Args {
			b.WriteString(", ")
			dumpExpr(b, a)
		}
		b.WriteString(")")
	case *ParenExpr:
		b.WriteString("Paren(")
		dumpExpr(b, n.Inner)
		b.WriteString(")")
	case *StructInitExpr:
		b.WriteString("StructInit(")
		if n.Type != nil {
			dumpExpr(b, n.Type)
		} else {
			b.WriteString("inferred")
		}
		for _, f := range n.Fields {
			b.WriteString(", ")
			if f.Name != "" {
				fmt.Fprintf(b, "%s=", f.Name)
			}
			dumpExpr(b, f.Value)
		}
		b.WriteString(")")
	case *ErrorExpr:
		b.WriteString("ErrorExpr")
	default:
		fmt.Fprintf(b, "Expr(%T)", e)
	}
}
