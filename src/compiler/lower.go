// AST to HIR lowering for the Chaos compiler.
//
// LowerProgram walks a parsed (and type-checked) AST and produces the typed
// HIR. It resolves names to SymbolIDs, type expressions to TypeIDs, and
// infers the type of every expression using the same rules as the type
// checker. The program is assumed to be well-typed; lowering still degrades
// gracefully on unresolved names.
package compiler

import (
	"strconv"
	"strings"
)

// LowerProgram lowers a parsed AST into the typed HIR.
func LowerProgram(program *Program) (*HIR, DiagnosticList) {
	l := &Lowerer{
		symbols:    NewSymbolTable(),
		types:      NewTypeTable(),
		varTypes:   make(map[SymbolID]TypeID),
		procs:      make(map[string]*ProcDecl),
		structs:    make(map[string]*StructDecl),
		hirStructs: make(map[string]*HIRStruct),
		hir:        &HIR{},
	}
	l.hir.Symbols = l.symbols
	l.hir.Types = l.types
	l.hir.Entry = program.Entry
	l.lowerProgram(program)
	return l.hir, l.diags
}

// Lowerer lowers an AST into the HIR. It maintains a stack of lexical scopes
// mapping names to SymbolIDs and a map from each declared symbol to its type.
type Lowerer struct {
	symbols    *SymbolTable
	types      *TypeTable
	scopes     []map[string]SymbolID
	varTypes   map[SymbolID]TypeID
	procs      map[string]*ProcDecl
	structs    map[string]*StructDecl
	hirStructs map[string]*HIRStruct
	hir        *HIR
	diags      DiagnosticList
	curResults []TypeID // result types of the procedure being lowered
}

func (l *Lowerer) pushScope() {
	l.scopes = append(l.scopes, map[string]SymbolID{})
}

func (l *Lowerer) popScope() {
	l.scopes = l.scopes[:len(l.scopes)-1]
}

func (l *Lowerer) declare(name string, sym SymbolID) {
	l.scopes[len(l.scopes)-1][name] = sym
}

// lookup resolves a name to its SymbolID, searching from the innermost scope
// outward. Unknown names receive a fresh symbol so lowering can continue.
func (l *Lowerer) lookup(name string) SymbolID {
	for i := len(l.scopes) - 1; i >= 0; i-- {
		if sym, ok := l.scopes[i][name]; ok {
			return sym
		}
	}
	return l.symbols.Declare(name)
}

// typeOfTypeExpr resolves a type expression (currently just an identifier) to
// a TypeID.
func (l *Lowerer) typeOfTypeExpr(e Expr) TypeID {
	if ident, ok := e.(*IdentExpr); ok {
		if id, ok := l.types.ByName(ident.Name); ok {
			return id
		}
	}
	return l.types.Unknown()
}

func (l *Lowerer) lowerProgram(program *Program) {
	l.pushScope() // global scope

	// Pass 1: register all top-level declarations so bodies can reference
	// them regardless of source order.
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *StructDecl:
			l.structs[d.Name] = d
			tid := l.types.InternStruct(d.Name)
			sym := l.symbols.Declare(d.Name)
			l.declare(d.Name, sym)
			hs := &HIRStruct{Symbol: sym, Name: d.Name, Type: tid, Span: d.Span_}
			l.hirStructs[d.Name] = hs
			l.hir.Structs = append(l.hir.Structs, hs)
		case *ProcDecl:
			l.procs[d.Name] = d
			sym := l.symbols.Declare(d.Name)
			l.declare(d.Name, sym)
		case *VarDecl:
			sym := l.symbols.Declare(d.Name)
			l.declare(d.Name, sym)
		}
	}

	// Pass 2: resolve struct field types now that every type name is known.
	for _, hs := range l.hir.Structs {
		decl := l.structs[hs.Name]
		fields := make([]TypeField, len(decl.Fields))
		hirFields := make([]HIRField, len(decl.Fields))
		for i, f := range decl.Fields {
			ft := l.typeOfTypeExpr(f.Type)
			fsym := l.symbols.Declare(f.Name)
			fields[i] = TypeField{Symbol: fsym, Name: f.Name, Type: ft}
			hirFields[i] = HIRField{Symbol: fsym, Name: f.Name, Type: ft, Span: f.Span_}
		}
		l.types.SetStructFields(hs.Type, fields)
		hs.Fields = hirFields
	}

	// Pass 3: lower globals and procedure bodies.
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *VarDecl:
			l.lowerGlobal(d)
		case *ProcDecl:
			l.lowerProc(d)
		}
	}

	l.popScope()
}

func (l *Lowerer) lowerGlobal(d *VarDecl) {
	sym := l.lookup(d.Name)
	var t TypeID = l.types.Unknown()
	if d.DeclType != nil {
		t = l.typeOfTypeExpr(d.DeclType)
	}
	var init HIRExpr
	if d.Init != nil {
		init = l.lowerExprAs(d.Init, t)
		if t == l.types.Unknown() {
			t = init.hirType()
		}
	}
	l.varTypes[sym] = t
	l.hir.Globals = append(l.hir.Globals, &HIRGlobal{
		Symbol:      sym,
		Name:        d.Name,
		Type:        t,
		Init:        init,
		Mutable:     d.Mutable,
		CompileTime: d.CompileTime,
		Span:        d.Span_,
	})
}

func (l *Lowerer) lowerProc(d *ProcDecl) {
	sym := l.lookup(d.Name)
	l.pushScope()
	params := make([]HIRParam, len(d.Params))
	for i, p := range d.Params {
		pt := l.typeOfTypeExpr(p.Type)
		psym := l.symbols.Declare(p.Name)
		l.declare(p.Name, psym)
		l.varTypes[psym] = pt
		params[i] = HIRParam{Symbol: psym, Name: p.Name, Type: pt, Span: p.Span_}
	}
	results := make([]TypeID, len(d.Results))
	for i, r := range d.Results {
		results[i] = l.typeOfTypeExpr(r)
	}
	prevResults := l.curResults
	l.curResults = results
	var body *HIRBlock
	if d.Body != nil {
		body = l.lowerBlock(d.Body)
	}
	l.curResults = prevResults
	l.popScope()
	l.hir.Procs = append(l.hir.Procs, &HIRProc{
		Symbol:  sym,
		Name:    d.Name,
		Params:  params,
		Results: results,
		Body:    body,
		Span:    d.Span_,
	})
}

func (l *Lowerer) lowerBlock(b *BlockStmt) *HIRBlock {
	l.pushScope()
	hb := &HIRBlock{Span_: b.Span_}
	for _, stmt := range b.Stmts {
		if hs := l.lowerStmt(stmt); hs != nil {
			hb.Stmts = append(hb.Stmts, hs)
		}
	}
	l.popScope()
	return hb
}

func (l *Lowerer) lowerStmt(s Stmt) HIRStmt {
	switch n := s.(type) {
	case *VarDecl:
		return l.lowerVarDecl(n)
	case *AssignStmt:
		sym := l.lookup(n.Name)
		t := l.varTypes[sym]
		return &HIRAssign{
			Span_:  n.Span_,
			Target: sym,
			Value:  l.lowerExprAs(n.Value, t),
		}
	case *ReturnStmt:
		var value HIRExpr
		if n.Value != nil {
			// Adapt the value to the procedure's result type so that return
			// literals match wider or narrower types.
			value = l.lowerExpr(n.Value)
			if len(l.curResults) > 0 {
				value = l.adaptLiteral(value, l.curResults[0])
			}
		}
		return &HIRReturn{Span_: n.Span_, Value: value}
	case *ExitStmt:
		var status, message HIRExpr
		if n.Status != nil {
			status = l.lowerExpr(n.Status)
		}
		if n.Message != nil {
			message = l.lowerExpr(n.Message)
		}
		return &HIRExit{Span_: n.Span_, Status: status, Message: message}
	case *IfStmt:
		return l.lowerIf(n)
	case *BlockStmt:
		return l.lowerBlock(n)
	case *ExprStmt:
		return &HIRExprStmt{Span_: n.Span_, Expr: l.lowerExpr(n.Expr)}
	case *ProcDecl, *StructDecl:
		// Nested declarations are not supported inside bodies.
		return nil
	}
	return nil
}

func (l *Lowerer) lowerVarDecl(d *VarDecl) HIRStmt {
	sym := l.symbols.Declare(d.Name)
	l.declare(d.Name, sym)
	var t TypeID = l.types.Unknown()
	if d.DeclType != nil {
		t = l.typeOfTypeExpr(d.DeclType)
	}
	var init HIRExpr
	if d.Init != nil {
		init = l.lowerExprAs(d.Init, t)
		if t == l.types.Unknown() {
			t = init.hirType()
		}
	}
	l.varTypes[sym] = t
	return &HIRVarDecl{
		Span_:       d.Span_,
		Symbol:      sym,
		Name:        d.Name,
		Type:        t,
		Init:        init,
		Mutable:     d.Mutable,
		CompileTime: d.CompileTime,
	}
}

func (l *Lowerer) lowerIf(n *IfStmt) HIRStmt {
	cond := l.lowerExpr(n.Condition)
	then := l.lowerBlock(n.Body)
	var elifs []*HIRIf
	for _, e := range n.Elif {
		elifs = append(elifs, &HIRIf{
			Span_:     e.Span_,
			Condition: l.lowerExpr(e.Condition),
			Then:      l.lowerBlock(e.Body),
		})
	}
	var els *HIRBlock
	if n.ElseBody != nil {
		els = l.lowerBlock(n.ElseBody)
	}
	return &HIRIf{Span_: n.Span_, Condition: cond, Then: then, Elif: elifs, Else: els}
}

func (l *Lowerer) lowerExpr(e Expr) HIRExpr {
	switch n := e.(type) {
	case *IntExpr:
		return &HIRConst{
			Span_: n.Span_,
			Type:  l.types.S64(),
			Kind:  ConstInt,
			Int:   parseIntLiteral(n.Value),
			Str:   n.Value,
		}
	case *FloatExpr:
		return &HIRConst{
			Span_: n.Span_,
			Type:  l.types.F64(),
			Kind:  ConstFloat,
			Float: parseFloatLiteral(n.Value),
			Str:   n.Value,
		}
	case *StringExpr:
		return &HIRConst{Span_: n.Span_, Type: l.types.String(), Kind: ConstString, Str: n.Value}
	case *BoolExpr:
		return &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: n.Value}
	case *IdentExpr:
		sym := l.lookup(n.Name)
		return &HIRRef{Span_: n.Span_, Symbol: sym, Type: l.varTypes[sym]}
	case *ParenExpr:
		return l.lowerExpr(n.Inner)
	case *BinaryExpr:
		return l.lowerBinary(n)
	case *UnaryExpr:
		return l.lowerUnary(n)
	case *CallExpr:
		return l.lowerCall(n)
	case *StructInitExpr:
		if n.Type != nil {
			return l.lowerStructInit(n, l.typeOfTypeExpr(n.Type))
		}
		// Inferred struct literal: the type comes from context, so it is
		// resolved by lowerExprAs.
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	case *ErrorExpr:
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	}
	return &HIRConst{Span_: e.nodeSpan(), Type: l.types.Unknown(), Kind: ConstUnknown}
}

// lowerExprAs lowers an expression in a context with a known target type. An
// inferred struct literal takes the target type; a literal adapts to the
// target when compatible.
func (l *Lowerer) lowerExprAs(e Expr, target TypeID) HIRExpr {
	if si, ok := e.(*StructInitExpr); ok && si.Type == nil {
		return l.lowerStructInit(si, target)
	}
	return l.adaptLiteral(l.lowerExpr(e), target)
}

func (l *Lowerer) lowerBinary(n *BinaryExpr) HIRExpr {
	left := l.lowerExpr(n.Left)
	right := l.lowerExpr(n.Right)
	switch n.Op {
	case BinaryOpAdd, BinaryOpSub, BinaryOpMul, BinaryOpDiv, BinaryOpMod:
		left, right = l.adaptLiteralTypes(left, right)
		t := left.hirType()
		if t == l.types.Unknown() {
			t = right.hirType()
		}
		return &HIRBinary{Span_: n.Span_, Op: n.Op, Left: left, Right: right, Type: t}
	case BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe, BinaryOpEq, BinaryOpNeq:
		left, right = l.adaptLiteralTypes(left, right)
		return &HIRBinary{Span_: n.Span_, Op: n.Op, Left: left, Right: right, Type: l.types.Bool()}
	case BinaryOpAnd, BinaryOpOr:
		return &HIRBinary{Span_: n.Span_, Op: n.Op, Left: left, Right: right, Type: l.types.Bool()}
	}
	return &HIRBinary{Span_: n.Span_, Op: n.Op, Left: left, Right: right, Type: l.types.Unknown()}
}

func (l *Lowerer) lowerUnary(n *UnaryExpr) HIRExpr {
	operand := l.lowerExpr(n.Operand)
	t := operand.hirType()
	if n.Op == UnaryOpNot {
		t = l.types.Bool()
	}
	return &HIRUnary{Span_: n.Span_, Op: n.Op, Operand: operand, Type: t}
}

func (l *Lowerer) lowerCall(n *CallExpr) HIRExpr {
	ident, ok := n.Func.(*IdentExpr)
	if !ok {
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	}
	proc, ok := l.procs[ident.Name]
	if !ok {
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	}
	sym, _ := l.symbols.ByName(ident.Name)
	args := make([]HIRExpr, len(n.Args))
	for i, arg := range n.Args {
		var target TypeID
		if i < len(proc.Params) {
			target = l.typeOfTypeExpr(proc.Params[i].Type)
		}
		args[i] = l.lowerExprAs(arg, target)
	}
	t := l.types.Void()
	if len(proc.Results) > 0 {
		t = l.typeOfTypeExpr(proc.Results[0])
	}
	return &HIRCall{Span_: n.Span_, Func: sym, Args: args, Type: t}
}

func (l *Lowerer) lowerStructInit(n *StructInitExpr, structType TypeID) HIRExpr {
	st := l.types.Lookup(structType)
	hs, ok := l.hirStructs[st.Name]
	if !ok {
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	}
	fieldTypes := make(map[string]TypeID, len(hs.Fields))
	fieldSyms := make(map[string]SymbolID, len(hs.Fields))
	for _, f := range hs.Fields {
		fieldTypes[f.Name] = f.Type
		fieldSyms[f.Name] = f.Symbol
	}
	fields := make([]HIRStructInitField, len(n.Fields))
	for i, field := range n.Fields {
		var ft TypeID
		var fsym SymbolID
		if field.Name != "" {
			ft = fieldTypes[field.Name]
			fsym = fieldSyms[field.Name]
		} else if i < len(st.Fields) {
			ft = st.Fields[i].Type
			fsym = fieldSyms[st.Fields[i].Name]
		}
		fields[i] = HIRStructInitField{
			Span_: field.Span_,
			Field: fsym,
			Value: l.lowerExprAs(field.Value, ft),
		}
	}
	return &HIRStructInit{
		Span_:  n.Span_,
		Struct: hs.Symbol,
		Type:   structType,
		Fields: fields,
	}
}

// adaptLiteralTypes adapts a literal operand to the other operand's type when
// the other operand is not a literal, mirroring the type checker's rule that
// a literal may adapt to the other operand's type.
func (l *Lowerer) adaptLiteralTypes(a, b HIRExpr) (HIRExpr, HIRExpr) {
	if isHIRLiteral(a) && !isHIRLiteral(b) {
		a = l.adaptLiteral(a, b.hirType())
	} else if isHIRLiteral(b) && !isHIRLiteral(a) {
		b = l.adaptLiteral(b, a.hirType())
	}
	return a, b
}

// adaptLiteral retypes a literal constant (or a negated literal) to a
// compatible target type.
func (l *Lowerer) adaptLiteral(e HIRExpr, target TypeID) HIRExpr {
	if target == l.types.Unknown() {
		return e
	}
	switch n := e.(type) {
	case *HIRConst:
		if l.literalCompatible(n, target) {
			n.Type = target
		}
	case *HIRUnary:
		if n.Op == UnaryOpNeg {
			if c, ok := n.Operand.(*HIRConst); ok && l.literalCompatible(c, target) {
				c.Type = target
				n.Type = target
			}
		}
	}
	return e
}

func (l *Lowerer) literalCompatible(c *HIRConst, target TypeID) bool {
	tt := l.types.Lookup(target)
	switch c.Kind {
	case ConstInt:
		return tt.Kind == TypeKindInt
	case ConstFloat:
		return tt.Kind == TypeKindFloat
	case ConstString:
		return tt.Kind == TypeKindString
	case ConstBool:
		return tt.Kind == TypeKindBool
	}
	return false
}

func isHIRLiteral(e HIRExpr) bool {
	switch n := e.(type) {
	case *HIRConst:
		return true
	case *HIRUnary:
		return n.Op == UnaryOpNeg && isHIRLiteral(n.Operand)
	}
	return false
}

// parseIntLiteral parses a decimal integer literal, ignoring underscore
// separators. Values outside int64 range are clamped to 0; the raw text is
// preserved on the HIRConst for wider types.
func parseIntLiteral(s string) int64 {
	v, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseFloatLiteral parses a decimal float literal, ignoring underscore
// separators.
func parseFloatLiteral(s string) float64 {
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil {
		return 0
	}
	return v
}
