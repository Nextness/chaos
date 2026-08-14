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
		symbols:      NewSymbolTable(),
		types:        NewTypeTable(),
		varTypes:     make(map[SymbolID]TypeID),
		procs:        make(map[string]*ProcDecl),
		structs:      make(map[string]*StructDecl),
		errors:       make(map[string]*ErrorDecl),
		errorOrdinal: make(map[string]map[string]int),
		hirStructs:   make(map[string]*HIRStruct),
		unwrapped:    make(map[SymbolID]bool),
		hir:          &HIR{},
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
	symbols      *SymbolTable
	types        *TypeTable
	scopes       []map[string]SymbolID
	varTypes     map[SymbolID]TypeID
	procs        map[string]*ProcDecl
	structs      map[string]*StructDecl
	errors       map[string]*ErrorDecl
	errorOrdinal map[string]map[string]int
	hirStructs   map[string]*HIRStruct
	unwrapped    map[SymbolID]bool // variables whose error was handled
	hir          *HIR
	diags        DiagnosticList
	curResults   []TypeID // result types of the procedure being lowered
	rangeThis    HIRExpr  // expression for '#this' in the innermost range loop
	rangeIndex   SymbolID // symbol for '#index' in the innermost range loop
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

// typeOfTypeExpr resolves a type expression (an identifier or an array type
// "[]T") to a TypeID.
func (l *Lowerer) typeOfTypeExpr(e Expr) TypeID {
	switch n := e.(type) {
	case *IdentExpr:
		if id, ok := l.types.ByName(n.Name); ok {
			return id
		}
	case *ArrayTypeExpr:
		return l.types.InternArray(l.typeOfTypeExpr(n.Elem))
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
		case *ErrorDecl:
			l.errors[d.Name] = d
			l.types.InternError(d.Name)
			sym := l.symbols.Declare(d.Name)
			l.declare(d.Name, sym)
			ordinals := make(map[string]int, len(d.Members))
			for i, m := range d.Members {
				ordinals[m.Name] = i
			}
			l.errorOrdinal[d.Name] = ordinals
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
	if d.ErrorResult != nil {
		// The result is a value-or-error pair; the value side is the
		// non-error type regardless of the written order.
		vt := results[0]
		et := l.typeOfTypeExpr(d.ErrorResult)
		if l.types.Lookup(vt).Kind == TypeKindError && l.types.Lookup(et).Kind != TypeKindError {
			vt, et = et, vt
		}
		results[0] = l.unionTypeID(vt, et)
	}
	prevResults := l.curResults
	l.curResults = results
	var body *HIRBlock
	if d.Body != nil {
		body = l.lowerBlock(d.Body)
		if !blockDiverges(d.Body) && len(results) == 1 && isUnionTypeID(l.types, results[0]) {
			resultType := l.types.Lookup(results[0])
			if resultType.Fields[0].Type == l.types.Void() {
				body.Stmts = append(body.Stmts, l.lowerReturn(&ReturnStmt{Span_: d.Body.Span_}))
			}
		}
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
		value := l.lowerExprAs(n.Value, t)
		// Assigning to a variable whose error was handled wraps the new
		// value in the pair again.
		if l.unwrapped[sym] && isUnionTypeID(l.types, t) {
			vt := l.types.Lookup(t).Fields[0].Type
			et := l.types.Lookup(t).Fields[1].Type
			value = l.lowerUnion(t, l.adaptLiteral(value, vt), l.emptyConst(et, n.Span_), &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: false}, n.Span_)
		}
		return &HIRAssign{
			Span_:  n.Span_,
			Target: sym,
			Value:  value,
		}
	case *ReturnStmt:
		return l.lowerReturn(n)
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
	case *UnlessCatchStmt:
		return l.lowerUnlessCatch(n)
	case *IfCatchStmt:
		return l.lowerIfCatch(n)
	case *ForStmt:
		return l.lowerFor(n)
	case *BreakStmt:
		return &HIRBreak{Span_: n.Span_}
	case *ContinueStmt:
		return &HIRContinue{Span_: n.Span_}
	case *CompoundAssignStmt:
		return l.lowerCompoundAssign(n)
	case *IncDecStmt:
		return l.lowerIncDec(n)
	case *ProcDecl, *StructDecl, *ErrorDecl:
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

// lowerFor lowers a for loop. The range form is desugared into a c-style loop
// over a hidden index variable: the array is bound once, the loop runs while
// the index is below the array length, and the body binds the element (and
// index) and exposes '#this' and '#index'.
func (l *Lowerer) lowerFor(n *ForStmt) HIRStmt {
	if n.Range != nil {
		return l.lowerRangeFor(n, n.Range, n.IndexName, n.ElemName)
	}
	if n.Cond != nil {
		var init, after HIRStmt
		if n.Init != nil {
			init = l.lowerStmt(n.Init)
		}
		cond := l.lowerExpr(n.Cond)
		if l.types.Lookup(cond.hirType()).Kind == TypeKindArray {
			// Implicit range loop: "for arr { ... }" with '#this'/'#index'.
			return l.lowerRangeFor(n, n.Cond, "", "")
		}
		if n.After != nil {
			after = l.lowerStmt(n.After)
		}
		return &HIRFor{Span_: n.Span_, Init: init, Cond: cond, After: after, Body: l.lowerBlock(n.Body)}
	}
	return &HIRFor{Span_: n.Span_, Body: l.lowerBlock(n.Body)}
}

// lowerRangeFor desugars a range loop into a c-style loop. The array is bound
// to a hidden variable so it is evaluated once; a hidden (or named) index
// variable counts from 0 to the array length; the body binds the element and
// exposes '#this' and '#index'.
func (l *Lowerer) lowerRangeFor(n *ForStmt, rangeExpr Expr, indexName, elemName string) HIRStmt {
	arrVal := l.lowerExpr(rangeExpr)
	arrType := arrVal.hirType()
	elemType := l.types.Lookup(arrType).Elem
	l.pushScope()

	// Bind the array once.
	arrSym := l.symbols.Declare("__arr")
	l.declare("__arr", arrSym)
	l.varTypes[arrSym] = arrType
	arrDecl := &HIRVarDecl{Span_: n.Span_, Symbol: arrSym, Name: "__arr", Type: arrType, Init: arrVal, Mutable: false, CompileTime: false}

	// Index variable: the named index (when given) is the loop counter.
	idxName := "__idx"
	if indexName != "" {
		idxName = indexName
	}
	idxSym := l.symbols.Declare(idxName)
	l.declare(idxName, idxSym)
	l.varTypes[idxSym] = l.types.S64()
	idxDecl := &HIRVarDecl{
		Span_:       n.Span_,
		Symbol:      idxSym,
		Name:        idxName,
		Type:        l.types.S64(),
		Init:        &HIRConst{Span_: n.Span_, Type: l.types.S64(), Kind: ConstInt, Int: 0},
		Mutable:     true,
		CompileTime: false,
	}

	arrRef := &HIRRef{Span_: n.Span_, Symbol: arrSym, Type: arrType}
	idxRef := &HIRRef{Span_: n.Span_, Symbol: idxSym, Type: l.types.S64()}
	cond := &HIRBinary{
		Span_: n.Span_,
		Op:    BinaryOpLt,
		Left:  idxRef,
		Right: &HIRArrayLen{Span_: n.Span_, Array: arrRef, Type: l.types.S64()},
		Type:  l.types.Bool(),
	}
	after := &HIRAssign{
		Span_:  n.Span_,
		Target: idxSym,
		Value: &HIRBinary{
			Span_: n.Span_,
			Op:    BinaryOpAdd,
			Left:  idxRef,
			Right: &HIRConst{Span_: n.Span_, Type: l.types.S64(), Kind: ConstInt, Int: 1},
			Type:  l.types.S64(),
		},
	}

	// Lower the body with '#this' and '#index' bound and the element binding
	// prepended.
	prevThis, prevIndex := l.rangeThis, l.rangeIndex
	l.rangeThis = &HIRIndex{Span_: n.Span_, Base: arrRef, Index: idxRef, Type: elemType}
	l.rangeIndex = idxSym

	l.pushScope()
	var bodyStmts []HIRStmt
	if elemName != "" {
		elemSym := l.symbols.Declare(elemName)
		l.declare(elemName, elemSym)
		l.varTypes[elemSym] = elemType
		bodyStmts = append(bodyStmts, &HIRVarDecl{
			Span_:       n.Span_,
			Symbol:      elemSym,
			Name:        elemName,
			Type:        elemType,
			Init:        l.rangeThis,
			Mutable:     false,
			CompileTime: false,
		})
	}
	for _, s := range n.Body.Stmts {
		if hs := l.lowerStmt(s); hs != nil {
			bodyStmts = append(bodyStmts, hs)
		}
	}
	l.popScope()
	l.rangeThis, l.rangeIndex = prevThis, prevIndex

	body := &HIRBlock{Span_: n.Body.Span_, Stmts: bodyStmts}
	forStmt := &HIRFor{Span_: n.Span_, Init: idxDecl, Cond: cond, After: after, Body: body}
	l.popScope()
	return &HIRBlock{Span_: n.Span_, Stmts: []HIRStmt{arrDecl, forStmt}}
}

// lowerCompoundAssign desugars "name += value" into "name = name + value".
func (l *Lowerer) lowerCompoundAssign(n *CompoundAssignStmt) HIRStmt {
	sym := l.lookup(n.Name)
	t := l.varTypes[sym]
	left := &HIRRef{Span_: n.Span_, Symbol: sym, Type: t}
	right := l.lowerExprAs(n.Value, t)
	return &HIRAssign{
		Span_:  n.Span_,
		Target: sym,
		Value:  &HIRBinary{Span_: n.Span_, Op: n.Op, Left: left, Right: right, Type: t},
	}
}

// lowerIncDec desugars "name++" / "++name" into "name = name + 1" (and the
// decrement forms into "name = name - 1").
func (l *Lowerer) lowerIncDec(n *IncDecStmt) HIRStmt {
	sym := l.lookup(n.Name)
	t := l.varTypes[sym]
	one := &HIRConst{Span_: n.Span_, Type: t, Kind: ConstInt, Int: 1}
	return &HIRAssign{
		Span_:  n.Span_,
		Target: sym,
		Value: &HIRBinary{
			Span_: n.Span_,
			Op:    n.Op,
			Left:  &HIRRef{Span_: n.Span_, Symbol: sym, Type: t},
			Right: one,
			Type:  t,
		},
	}
}

// lowerReturn lowers a return statement, wrapping the value in the
// value-or-error pair when the procedure has a '<>' result.
func (l *Lowerer) lowerReturn(n *ReturnStmt) HIRStmt {
	var value HIRExpr
	if len(l.curResults) > 0 && isUnionTypeID(l.types, l.curResults[0]) {
		rt := l.types.Lookup(l.curResults[0])
		vt, et := rt.Fields[0].Type, rt.Fields[1].Type
		if n.Value == nil {
			// A bare return in a 'Void <> E' procedure is a successful
			// return with no value.
			if vt == l.types.Void() {
				value = l.lowerUnion(l.curResults[0], l.emptyConst(vt, n.Span_), l.emptyConst(et, n.Span_), &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: false}, n.Span_)
			}
		} else {
			var lowered HIRExpr
			if em, ok := n.Value.(*ErrorMemberExpr); ok && em.TypeName == "" {
				// A bare error literal resolves against the error side.
				lowered = l.lowerErrorMember(em, l.types.Lookup(et).Name)
			} else {
				lowered = l.lowerExprAs(n.Value, vt)
			}
			switch {
			case lowered.hirType() == l.curResults[0]:
				// Re-raising an already-handled pair: pass it through.
				value = lowered
			case lowered.hirType() == et:
				// An error value: mark the pair as an error.
				value = l.lowerUnion(l.curResults[0], l.emptyConst(vt, n.Span_), lowered, &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: true}, n.Span_)
			default:
				// A value: mark the pair as a success.
				value = l.lowerUnion(l.curResults[0], lowered, l.emptyConst(et, n.Span_), &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: false}, n.Span_)
			}
		}
	} else if n.Value != nil {
		value = l.lowerExpr(n.Value)
		if len(l.curResults) > 0 {
			// Adapt the value to the procedure's result type so that
			// return literals match wider or narrower types.
			value = l.adaptLiteral(value, l.curResults[0])
		}
	}
	return &HIRReturn{Span_: n.Span_, Value: value}
}

// lowerUnlessCatch lowers "target := expr unless catch [err] { body }" (or
// the bare form with Target == "") into a variable declaration holding the
// value-or-error pair followed by the catch check.
func (l *Lowerer) lowerUnlessCatch(n *UnlessCatchStmt) HIRStmt {
	init := l.lowerExpr(n.Init)
	var decl HIRStmt
	cond := init
	if n.Target != "" {
		sym := l.symbols.Declare(n.Target)
		l.declare(n.Target, sym)
		l.varTypes[sym] = init.hirType()
		decl = &HIRVarDecl{
			Span_:       n.Span_,
			Symbol:      sym,
			Name:        n.Target,
			Type:        init.hirType(),
			Init:        init,
			Mutable:     true,
			CompileTime: false,
		}
		cond = &HIRRef{Span_: n.Span_, Symbol: sym, Type: init.hirType()}
		// After the check, the variable holds the unwrapped value.
		l.unwrapped[sym] = true
	}
	ifCatch := l.buildIfCatch(n.Span_, cond, n.CatchName, n.CatchBody)
	if decl != nil {
		return &HIRBlock{Span_: n.Span_, Stmts: []HIRStmt{decl, ifCatch}}
	}
	return ifCatch
}

// lowerIfCatch lowers "if expr catch [err] { body }".
func (l *Lowerer) lowerIfCatch(n *IfCatchStmt) HIRStmt {
	cond := l.lowerExpr(n.Cond)
	ifCatch := l.buildIfCatch(n.Span_, cond, n.CatchName, n.CatchBody)
	// After the check, the variable holds the unwrapped value.
	if ident, ok := n.Cond.(*IdentExpr); ok {
		if sym, ok := l.lookupSymbol(ident.Name); ok {
			l.unwrapped[sym] = true
		}
	}
	return ifCatch
}

// buildIfCatch builds the catch check for a value-or-error pair.
func (l *Lowerer) buildIfCatch(span Span, cond HIRExpr, catchName string, catchBody *BlockStmt) HIRStmt {
	ut := cond.hirType()
	var catchSym SymbolID = NoSymbol
	if catchName != "" {
		catchSym = l.symbols.Declare(catchName)
		l.declare(catchName, catchSym)
		l.varTypes[catchSym] = l.types.Lookup(ut).Fields[1].Type
	}
	return &HIRIfCatch{
		Span_:     span,
		Cond:      cond,
		CatchSym:  catchSym,
		CatchBody: l.lowerBlock(catchBody),
		UnionType: ut,
	}
}

// lookupSymbol resolves a name to its SymbolID in the current scope chain.
func (l *Lowerer) lookupSymbol(name string) (SymbolID, bool) {
	for i := len(l.scopes) - 1; i >= 0; i-- {
		if sym, ok := l.scopes[i][name]; ok {
			return sym, true
		}
	}
	return NoSymbol, false
}

// unionTypeID interns the value-or-error pair type for a '<>' result as a
// struct { value: T, error: E, hasError: Bool }. The synthetic name matches
// the type checker's errorUnionType so both passes agree on the type.
func (l *Lowerer) unionTypeID(valueType, errorType TypeID) TypeID {
	name := l.types.Lookup(valueType).Name + "<>" + l.types.Lookup(errorType).Name
	if id, ok := l.types.ByName(name); ok {
		return id
	}
	id := l.types.InternStruct(name)
	l.types.SetStructFields(id, []TypeField{
		{Symbol: l.symbols.Declare("value"), Name: "value", Type: valueType},
		{Symbol: l.symbols.Declare("error"), Name: "error", Type: errorType},
		{Symbol: l.symbols.Declare("hasError"), Name: "hasError", Type: l.types.Bool()},
	})
	return id
}

// isUnionTypeID reports whether a TypeID is a value-or-error pair type.
func isUnionTypeID(tt *TypeTable, id TypeID) bool {
	t := tt.Lookup(id)
	return t.Kind == TypeKindStruct && strings.Contains(t.Name, "<>")
}

// lowerUnion builds a value-or-error pair value. A nil side is zero-filled by
// the struct init.
func (l *Lowerer) lowerUnion(unionType TypeID, value, err, hasError HIRExpr, span Span) HIRExpr {
	st := l.types.Lookup(unionType)
	fields := []HIRStructInitField{
		{Span_: span, Field: st.Fields[0].Symbol, Value: value},
		{Span_: span, Field: st.Fields[1].Symbol, Value: err},
		{Span_: span, Field: st.Fields[2].Symbol, Value: hasError},
	}
	return &HIRStructInit{Span_: span, Struct: st.Fields[0].Symbol, Type: unionType, Fields: fields}
}

// emptyConst is a zero-filled constant of the given type, used for the
// ignored side of a value-or-error pair.
func (l *Lowerer) emptyConst(t TypeID, span Span) HIRExpr {
	return &HIRConst{Span_: span, Type: t, Kind: ConstUnknown}
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
		t := l.varTypes[sym]
		// A variable whose error was handled holds the value-or-error pair;
		// uses read the value part.
		if l.unwrapped[sym] && isUnionTypeID(l.types, t) {
			return &HIRFieldLoad{Span_: n.Span_, Base: &HIRRef{Span_: n.Span_, Symbol: sym, Type: t}, Field: 0, Type: l.types.Lookup(t).Fields[0].Type}
		}
		return &HIRRef{Span_: n.Span_, Symbol: sym, Type: t}
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
	case *ErrorMemberExpr:
		if n.TypeName != "" {
			return l.lowerErrorMember(n, n.TypeName)
		}
		// Bare error member: the type comes from context, so it is resolved
		// by lowerExprAs.
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	case *ErrorExpr:
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	case *ArrayInitExpr:
		elemType := l.typeOfTypeExpr(n.Elem)
		arrType := l.types.InternArray(elemType)
		items := make([]HIRExpr, len(n.Items))
		for i, item := range n.Items {
			items[i] = l.lowerExprAs(item, elemType)
		}
		return &HIRArrayInit{Span_: n.Span_, Type: arrType, Items: items}
	case *IndexExpr:
		base := l.lowerExpr(n.Base)
		idx := l.lowerExpr(n.Index)
		elemType := l.types.Lookup(base.hirType()).Elem
		return &HIRIndex{Span_: n.Span_, Base: base, Index: idx, Type: elemType}
	case *LoopBuiltinExpr:
		if n.Name == "this" && l.rangeThis != nil {
			return l.rangeThis
		}
		if n.Name == "index" && l.rangeIndex != NoSymbol {
			return &HIRRef{Span_: n.Span_, Symbol: l.rangeIndex, Type: l.types.S64()}
		}
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
	if em, ok := e.(*ErrorMemberExpr); ok && em.TypeName == "" {
		return l.lowerErrorMember(em, l.types.Lookup(target).Name)
	}
	return l.adaptLiteral(l.lowerExpr(e), target)
}

// lowerErrorMember lowers an error member reference to its ordinal constant.
// The explicit "Type.MEMBER" form names the type; the bare ".MEMBER" form is
// resolved against the target type name ("" when unknown).
func (l *Lowerer) lowerErrorMember(n *ErrorMemberExpr, typeName string) HIRExpr {
	ordinals, ok := l.errorOrdinal[typeName]
	if !ok {
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	}
	ord, ok := ordinals[n.Name]
	if !ok {
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	}
	tid, ok := l.types.ByName(typeName)
	if !ok {
		return &HIRConst{Span_: n.Span_, Type: l.types.Unknown(), Kind: ConstUnknown}
	}
	return &HIRConst{Span_: n.Span_, Type: tid, Kind: ConstError, Int: int64(ord), Str: n.Name}
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
		if proc.ErrorResult != nil {
			vt := t
			et := l.typeOfTypeExpr(proc.ErrorResult)
			if l.types.Lookup(vt).Kind == TypeKindError && l.types.Lookup(et).Kind != TypeKindError {
				vt, et = et, vt
			}
			t = l.unionTypeID(vt, et)
		}
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
		// Error values are nominal and never adapt to another type.
		return n.Kind != ConstError
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
