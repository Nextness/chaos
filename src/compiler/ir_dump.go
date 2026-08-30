// Debug dump helpers for the HIR and MIR.
//
// These are exported so the CLI and tests can render the intermediate
// representations of a source file for inspection.
package compiler

import (
	"fmt"
	"strings"
)

// DumpHIR returns a human-readable, indented tree of the HIR.
func DumpHIR(hir *HIR) string {
	var b strings.Builder
	b.WriteString("HIR\n")
	for _, s := range hir.Structs {
		dumpHIRStruct(&b, hir, s, 1)
	}
	for _, g := range hir.Globals {
		dumpHIRGlobal(&b, hir, g, 1)
	}
	for _, p := range hir.Procs {
		dumpHIRProc(&b, hir, p, 1)
	}
	return b.String()
}

func dumpHIRStruct(b *strings.Builder, hir *HIR, s *HIRStruct, depth int) {
	dumpIndent(b, depth)
	fmt.Fprintf(b, "Struct %s\n", s.Name)
	for _, f := range s.Fields {
		dumpIndent(b, depth+1)
		fmt.Fprintf(b, "Field %s: %s\n", f.Name, hir.typeName(f.Type))
	}
}

func dumpHIRGlobal(b *strings.Builder, hir *HIR, g *HIRGlobal, depth int) {
	dumpIndent(b, depth)
	fmt.Fprintf(b, "Global %s: %s", g.Name, hir.typeName(g.Type))
	if g.Init != nil {
		b.WriteString(" = ")
		dumpHIRExpr(b, hir, g.Init)
	}
	fmt.Fprintf(b, " [mutable=%v compileTime=%v]\n", g.Mutable, g.CompileTime)
}

func dumpHIRProc(b *strings.Builder, hir *HIR, p *HIRProc, depth int) {
	dumpIndent(b, depth)
	fmt.Fprintf(b, "Proc %s(", p.Name)
	for i, param := range p.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%s: %s", param.Name, hir.typeName(param.Type))
	}
	b.WriteString(")")
	if len(p.Results) > 0 {
		b.WriteString(" -> ")
		for i, r := range p.Results {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(hir.typeName(r))
		}
	}
	b.WriteString("\n")
	if p.Body != nil {
		dumpHIRBlock(b, hir, p.Body, depth+1)
	}
}

func dumpHIRBlock(b *strings.Builder, hir *HIR, blk *HIRBlock, depth int) {
	dumpIndent(b, depth)
	b.WriteString("Block\n")
	for _, s := range blk.Stmts {
		dumpHIRStmt(b, hir, s, depth+1)
	}
}

func dumpHIRStmt(b *strings.Builder, hir *HIR, s HIRStmt, depth int) {
	switch n := s.(type) {
	case *HIRVarDecl:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "VarDecl %s: %s", n.Name, hir.typeName(n.Type))
		if n.Init != nil {
			b.WriteString(" = ")
			dumpHIRExpr(b, hir, n.Init)
		}
		fmt.Fprintf(b, " [mutable=%v compileTime=%v]\n", n.Mutable, n.CompileTime)
	case *HIRAssign:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "Assign %s = ", hir.symbolName(n.Target))
		dumpHIRExpr(b, hir, n.Value)
		b.WriteString("\n")
	case *HIRAddrStore:
		dumpIndent(b, depth)
		b.WriteString("AddrStore ")
		dumpHIRExpr(b, hir, n.Addr)
		b.WriteString(" = ")
		dumpHIRExpr(b, hir, n.Value)
		b.WriteString("\n")
	case *HIRReturn:
		dumpIndent(b, depth)
		b.WriteString("Return")
		if n.Value != nil {
			b.WriteString(" ")
			dumpHIRExpr(b, hir, n.Value)
		}
		b.WriteString("\n")
	case *HIRExit:
		dumpIndent(b, depth)
		b.WriteString("Exit")
		if n.Status != nil {
			b.WriteString(" status=")
			dumpHIRExpr(b, hir, n.Status)
		}
		if n.Message != nil {
			b.WriteString(" message=")
			dumpHIRExpr(b, hir, n.Message)
		}
		b.WriteString("\n")
	case *HIRIf:
		dumpIndent(b, depth)
		b.WriteString("If condition=")
		dumpHIRExpr(b, hir, n.Condition)
		b.WriteString("\n")
		dumpHIRBlock(b, hir, n.Then, depth+1)
		for _, e := range n.Elif {
			dumpIndent(b, depth)
			b.WriteString("Elif condition=")
			dumpHIRExpr(b, hir, e.Condition)
			b.WriteString("\n")
			dumpHIRBlock(b, hir, e.Then, depth+1)
		}
		if n.Else != nil {
			dumpIndent(b, depth)
			b.WriteString("Else\n")
			dumpHIRBlock(b, hir, n.Else, depth+1)
		}
	case *HIRBlock:
		dumpHIRBlock(b, hir, n, depth)
	case *HIRExprStmt:
		dumpIndent(b, depth)
		b.WriteString("ExprStmt ")
		dumpHIRExpr(b, hir, n.Expr)
		b.WriteString("\n")
	case *HIRIfCatch:
		dumpIndent(b, depth)
		b.WriteString("IfCatch ")
		dumpHIRExpr(b, hir, n.Cond)
		if n.CatchSym != NoSymbol {
			fmt.Fprintf(b, " catch %s", hir.symbolName(n.CatchSym))
		}
		b.WriteString("\n")
		dumpHIRBlock(b, hir, n.CatchBody, depth+1)
	case *HIRFor:
		dumpIndent(b, depth)
		b.WriteString("For")
		if n.Init != nil {
			b.WriteString(" init=")
			dumpHIRStmt(b, hir, n.Init, depth+1)
		}
		b.WriteString(" cond=")
		dumpHIRExpr(b, hir, n.Cond)
		if n.After != nil {
			b.WriteString(" after=")
			dumpHIRStmt(b, hir, n.After, depth+1)
		}
		b.WriteString("\n")
		dumpHIRBlock(b, hir, n.Body, depth+1)
	case *HIRBreak:
		dumpIndent(b, depth)
		b.WriteString("Break\n")
	case *HIRContinue:
		dumpIndent(b, depth)
		b.WriteString("Continue\n")
	default:
		dumpIndent(b, depth)
		fmt.Fprintf(b, "Stmt %T\n", s)
	}
}

func dumpHIRExpr(b *strings.Builder, hir *HIR, e HIRExpr) {
	switch n := e.(type) {
	case *HIRConst:
		switch n.Kind {
		case ConstInt:
			fmt.Fprintf(b, "Int(%s)", n.Str)
		case ConstFloat:
			fmt.Fprintf(b, "Float(%s)", n.Str)
		case ConstString:
			fmt.Fprintf(b, "String(%q)", n.Str)
		case ConstBool:
			fmt.Fprintf(b, "Bool(%v)", n.Bool)
		case ConstError:
			fmt.Fprintf(b, "Error(%s)", n.Str)
		default:
			b.WriteString("Unknown")
		}
	case *HIRRef:
		fmt.Fprintf(b, "Ident(%s)", hir.symbolName(n.Symbol))
	case *HIRBinary:
		fmt.Fprintf(b, "Binary(%s ", n.Op)
		dumpHIRExpr(b, hir, n.Left)
		b.WriteString(" ")
		dumpHIRExpr(b, hir, n.Right)
		b.WriteString(")")
	case *HIRUnary:
		fmt.Fprintf(b, "Unary(%s ", n.Op)
		dumpHIRExpr(b, hir, n.Operand)
		b.WriteString(")")
	case *HIRCall:
		b.WriteString("Call(")
		fmt.Fprintf(b, "%s", hir.symbolName(n.Func))
		for _, a := range n.Args {
			b.WriteString(", ")
			dumpHIRExpr(b, hir, a)
		}
		b.WriteString(")")
	case *HIRStructInit:
		b.WriteString("StructInit(")
		fmt.Fprintf(b, "%s", hir.symbolName(n.Struct))
		for _, f := range n.Fields {
			b.WriteString(", ")
			fmt.Fprintf(b, "%s=", hir.symbolName(f.Field))
			dumpHIRExpr(b, hir, f.Value)
		}
		b.WriteString(")")
	case *HIRFieldLoad:
		b.WriteString("FieldLoad(")
		dumpHIRExpr(b, hir, n.Base)
		fmt.Fprintf(b, ", %d)", n.Field)
	case *HIRDeref:
		b.WriteString("Deref(")
		dumpHIRExpr(b, hir, n.Operand)
		b.WriteString(")")
	case *HIRAddrOf:
		b.WriteString("AddrOf(")
		dumpHIRExpr(b, hir, n.Operand)
		b.WriteString(")")
	case *HIRFieldAddr:
		b.WriteString("FieldAddr(")
		dumpHIRExpr(b, hir, n.Addr)
		fmt.Fprintf(b, ", %d)", n.Field)
	case *HIRArrayElemAddr:
		b.WriteString("ArrayElemAddr(")
		dumpHIRExpr(b, hir, n.Array)
		b.WriteString(", ")
		dumpHIRExpr(b, hir, n.Index)
		b.WriteString(")")
	case *HIRArrayInit:
		b.WriteString("ArrayInit(")
		for i, item := range n.Items {
			if i > 0 {
				b.WriteString(", ")
			}
			dumpHIRExpr(b, hir, item)
		}
		b.WriteString(")")
	case *HIRArrayLen:
		b.WriteString("ArrayLen(")
		dumpHIRExpr(b, hir, n.Array)
		b.WriteString(")")
	case *HIRIndex:
		b.WriteString("Index(")
		dumpHIRExpr(b, hir, n.Base)
		b.WriteString(", ")
		dumpHIRExpr(b, hir, n.Index)
		b.WriteString(")")
	case *HIRIfx:
		b.WriteString("Ifx(")
		dumpHIRExpr(b, hir, n.Cond)
		b.WriteString(" then ")
		dumpHIRExpr(b, hir, n.Then)
		b.WriteString(" else ")
		dumpHIRExpr(b, hir, n.Else)
		b.WriteString(")")
	default:
		fmt.Fprintf(b, "Expr(%T)", e)
	}
}

// typeName renders a TypeID as its name.
func (hir *HIR) typeName(id TypeID) string {
	return hir.Types.Lookup(id).Name
}

// symbolName renders a SymbolID as its name.
func (hir *HIR) symbolName(id SymbolID) string {
	return hir.Symbols.Lookup(id)
}

// DumpMIR returns a human-readable listing of the MIR program.
func DumpMIR(prog *MIRProgram) string {
	var b strings.Builder
	b.WriteString("MIR\n")
	for _, g := range prog.Globals {
		dumpIndent(&b, 1)
		fmt.Fprintf(&b, "Global %s: %s [mutable=%v]\n", g.Name, prog.typeName(g.Type), g.Mutable)
	}
	for _, fn := range prog.Functions {
		dumpMIRFunction(&b, prog, fn)
	}
	return b.String()
}

func dumpMIRFunction(b *strings.Builder, prog *MIRProgram, fn *MIRFunction) {
	dumpIndent(b, 1)
	fmt.Fprintf(b, "Function %s(", fn.Name)
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "l%d: %s", p, prog.typeName(fn.LocalTypes[p]))
	}
	b.WriteString(")")
	if len(fn.Results) > 0 {
		b.WriteString(" -> ")
		for i, r := range fn.Results {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(prog.typeName(r))
		}
	}
	b.WriteString("\n")
	for i, lid := range fn.Locals {
		dumpIndent(b, 2)
		fmt.Fprintf(b, "Local l%d: %s", lid, prog.typeName(fn.LocalTypes[i]))
		if i < len(fn.Params) {
			b.WriteString(" (param)")
		}
		b.WriteString("\n")
	}
	for _, blk := range fn.Blocks {
		dumpMIRBlock(b, prog, fn, blk)
	}
}

func dumpMIRBlock(b *strings.Builder, prog *MIRProgram, fn *MIRFunction, blk *MIRBlock) {
	dumpIndent(b, 2)
	fmt.Fprintf(b, "bb%d:\n", blk.ID)
	for _, ins := range blk.Instrs {
		dumpMIRInstr(b, prog, fn, ins)
	}
	dumpMIRTerminator(b, prog, fn, blk.Term)
}

func dumpMIRInstr(b *strings.Builder, prog *MIRProgram, fn *MIRFunction, ins *MIRInstr) {
	dumpIndent(b, 3)
	if ins.Result != NoValue {
		fmt.Fprintf(b, "v%d:%s = ", ins.Result, prog.typeName(ins.Type))
	}
	fmt.Fprintf(b, "%s", ins.Op)
	// Deref operations carry the pointed-to type on the opcode so the dump
	// shows what is loaded or stored through the address. This is part of the
	// IR's normal textual form, not a debug aid.
	if ins.Op == MIRDerefStore || ins.Op == MIRDerefLoad {
		fmt.Fprintf(b, ":%s", prog.typeName(ins.Type))
	}
	for _, a := range ins.Args {
		fmt.Fprintf(b, " v%d", a)
	}
	switch ins.Imm.Kind {
	case MIRImmInt:
		if ins.Imm.Str != "" {
			fmt.Fprintf(b, " %s", ins.Imm.Str)
		} else {
			fmt.Fprintf(b, " %d", ins.Imm.Int)
		}
	case MIRImmFloat:
		fmt.Fprintf(b, " %v", ins.Imm.Float)
	case MIRImmString:
		fmt.Fprintf(b, " %q", ins.Imm.Str)
	case MIRImmBool:
		fmt.Fprintf(b, " %v", ins.Imm.Bool)
	case MIRImmSymbol:
		fmt.Fprintf(b, " %s", prog.symbolName(ins.Imm.Symbol))
	case MIRImmLocal:
		fmt.Fprintf(b, " l%d", ins.Imm.Local)
	case MIRImmField:
		fmt.Fprintf(b, " field%d", ins.Imm.Int)
	case MIRImmStringList:
		for _, s := range ins.Imm.Strs {
			fmt.Fprintf(b, " %q", s)
		}
	}
	b.WriteString("\n")
}

func dumpMIRTerminator(b *strings.Builder, prog *MIRProgram, fn *MIRFunction, t MIRTerminator) {
	dumpIndent(b, 3)
	fmt.Fprintf(b, "%s", t.Kind)
	switch t.Kind {
	case MIRJump:
		fmt.Fprintf(b, " bb%d", t.Target)
	case MIRBranch:
		fmt.Fprintf(b, " v%d, bb%d, bb%d", t.Cond, t.Then, t.Else)
	case MIRReturn:
		if t.Value != NoValue {
			fmt.Fprintf(b, " v%d", t.Value)
		}
	case MIRTermExit:
		if t.Status != NoValue {
			fmt.Fprintf(b, " status=v%d", t.Status)
		}
		if t.Message != NoValue {
			fmt.Fprintf(b, " message=v%d", t.Message)
		}
	}
	b.WriteString("\n")
}

// typeName renders a TypeID as its name.
func (prog *MIRProgram) typeName(id TypeID) string {
	return prog.Types.Lookup(id).Name
}

// symbolName renders a SymbolID as its name.
func (prog *MIRProgram) symbolName(id SymbolID) string {
	return prog.Symbols.Lookup(id)
}
