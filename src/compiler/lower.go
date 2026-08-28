// AST to HIR lowering for the Chaos compiler.
//
// LowerProgram walks a parsed (and type-checked) AST and produces the typed
// HIR. It resolves names to SymbolIDs, type expressions to TypeIDs, and
// infers the type of every expression using the same rules as the type
// checker. The program is assumed to be well-typed; lowering still degrades
// gracefully on unresolved names.
package compiler

import (
	"maps"
	"math/big"
	"strconv"
	"strings"
)

// LowerProgram lowers a parsed AST into the typed HIR.
func LowerProgram(program *Program) (*HIR, DiagnosticList) {
	analysis, semanticDiags := AnalyzeProgram(program)
	hir, lowerDiags := LowerAnalyzedProgram(program, analysis)
	return hir, append(semanticDiags, lowerDiags...)
}

// LowerAnalyzedProgram lowers with the exact semantic facts produced by
// AnalyzeProgram. Pipeline drivers should prefer this over re-analysis.
func LowerAnalyzedProgram(program *Program, analysis *SemanticAnalysis) (*HIR, DiagnosticList) {
	if program == nil {
		var diags DiagnosticList
		diags.Error(Span{}, "cannot lower a nil program", "parse and analyze a source program before lowering")
		return &HIR{Symbols: NewSymbolTable(), Types: NewTypeTable(), Entry: NoSymbol}, diags
	}
	if analysis == nil {
		var diags DiagnosticList
		diags.Error(Span{}, "lowering requires semantic analysis facts", "run AnalyzeProgram before LowerAnalyzedProgram")
		return &HIR{Symbols: NewSymbolTable(), Types: NewTypeTable(), Entry: NoSymbol, Sources: program.Sources}, diags
	}
	l := &Lowerer{
		symbols:           NewSymbolTable(),
		types:             NewTypeTable(),
		varTypes:          make(map[SymbolID]TypeID),
		procSymbols:       make(map[*ProcDecl]SymbolID),
		structTypes:       make(map[*StructDecl]TypeID),
		errorTypes:        make(map[*ErrorDecl]TypeID),
		enumTypes:         make(map[*EnumDecl]TypeID),
		structDecls:       make(map[TypeID]*StructDecl),
		errorOrdinal:      make(map[TypeID]map[string]int),
		enumValues:        make(map[TypeID]map[string]string),
		hirStructs:        make(map[TypeID]*HIRStruct),
		globalSymbols:     make(map[*VarDecl]SymbolID),
		globalPrevious:    make(map[*VarDecl]SymbolID),
		symbolCompileTime: make(map[SymbolID]bool),
		constValues:       make(map[SymbolID]HIRExpr),
		procAliases:       make(map[SymbolID]*ProcDecl),
		unwrapped:         make(map[SymbolID]bool),
		analysis:          analysis,
		hir:               &HIR{},
	}
	l.hir.Symbols = l.symbols
	l.hir.Types = l.types
	l.hir.Entry = NoSymbol
	l.hir.Sources = program.Sources
	l.lowerProgram(program)
	return l.hir, l.diags
}

// Lowerer lowers an AST into the HIR. It maintains a stack of lexical scopes
// mapping names to SymbolIDs and a map from each declared symbol to its type.
type Lowerer struct {
	symbols           *SymbolTable
	types             *TypeTable
	scopes            []map[string]SymbolID
	varTypes          map[SymbolID]TypeID
	procScopes        []map[string]*ProcDecl
	typeScopes        []map[string]TypeID
	procSymbols       map[*ProcDecl]SymbolID
	structTypes       map[*StructDecl]TypeID
	errorTypes        map[*ErrorDecl]TypeID
	enumTypes         map[*EnumDecl]TypeID
	structDecls       map[TypeID]*StructDecl
	errorOrdinal      map[TypeID]map[string]int
	enumValues        map[TypeID]map[string]string
	hirStructs        map[TypeID]*HIRStruct
	globalSymbols     map[*VarDecl]SymbolID
	globalPrevious    map[*VarDecl]SymbolID
	symbolCompileTime map[SymbolID]bool
	constValues       map[SymbolID]HIRExpr
	procAliases       map[SymbolID]*ProcDecl
	unwrapped         map[SymbolID]bool // variables whose error was handled
	hir               *HIR
	diags             DiagnosticList
	curResults        []TypeID // result types of the procedure being lowered
	rangeThis         HIRExpr  // expression for '#this' in the innermost range loop
	rangeIndex        SymbolID // symbol for '#index' in the innermost range loop
	procValueFloor    int
	procDepth         int
	analysis          *SemanticAnalysis
}

func (l *Lowerer) pushScope() {
	l.scopes = append(l.scopes, map[string]SymbolID{})
	l.procScopes = append(l.procScopes, map[string]*ProcDecl{})
	l.typeScopes = append(l.typeScopes, map[string]TypeID{})
}

func (l *Lowerer) popScope() {
	l.scopes = l.scopes[:len(l.scopes)-1]
	l.procScopes = l.procScopes[:len(l.procScopes)-1]
	l.typeScopes = l.typeScopes[:len(l.typeScopes)-1]
}

func (l *Lowerer) declare(name string, sym SymbolID) {
	l.scopes[len(l.scopes)-1][name] = sym
}

// lookup resolves a name to its SymbolID, searching from the innermost scope
// outward. Unknown names receive a fresh symbol so lowering can continue.
func (l *Lowerer) lookup(name string) SymbolID {
	for i := len(l.scopes) - 1; i >= 0; i-- {
		if sym, ok := l.scopes[i][name]; ok {
			if l.procDepth > 0 && i > 0 && i < l.procValueFloor && !l.symbolCompileTime[sym] {
				continue
			}
			return sym
		}
	}
	return NoSymbol
}

func (l *Lowerer) poison(span Span, message string) HIRExpr {
	l.diags.Error(span, "internal lowering error: "+message, "report this compiler bug")
	return &HIRPoison{Span_: span, Type: l.types.Unknown()}
}

func (l *Lowerer) lookupProc(name string) (*ProcDecl, bool) {
	for i := len(l.procScopes) - 1; i >= 0; i-- {
		if p, ok := l.procScopes[i][name]; ok {
			return p, true
		}
	}
	return nil, false
}

func (l *Lowerer) lookupType(name string) (TypeID, bool) {
	for i := len(l.typeScopes) - 1; i >= 0; i-- {
		if t, ok := l.typeScopes[i][name]; ok {
			return t, true
		}
	}
	return l.types.ByName(name)
}

// typeOfTypeExpr resolves a type expression (an identifier, an array type
// "[]T", or a pointer type "*T" / "*T?") to a TypeID.
func (l *Lowerer) typeOfTypeExpr(e Expr) TypeID {
	switch n := e.(type) {
	case *IdentExpr:
		if id, ok := l.lookupType(n.Name); ok {
			return id
		}
	case *ArrayTypeExpr:
		return l.types.InternArray(l.typeOfTypeExpr(n.Elem))
	case *PointerTypeExpr:
		return l.types.InternPointer(l.typeOfTypeExpr(n.Elem), n.Nullable)
	}
	return l.types.Unknown()
}

func (l *Lowerer) lowerProgram(program *Program) {
	l.pushScope() // global scope
	latestGlobals := make(map[string]SymbolID)

	// Pass 1: register all top-level declarations so bodies can reference
	// them regardless of source order.
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *StructDecl:
			l.registerStruct(d)
		case *ProcDecl:
			l.registerProc(d)
		case *ErrorDecl:
			l.registerError(d)
		case *EnumDecl:
			l.registerEnum(d)
		case *VarDecl:
			if previous, ok := latestGlobals[d.Name]; ok {
				l.globalPrevious[d] = previous
			}
			sym := l.symbols.Declare(d.Name)
			l.globalSymbols[d] = sym
			latestGlobals[d.Name] = sym
			l.declare(d.Name, sym)
			l.symbolCompileTime[sym] = d.CompileTime
		}
	}
	// Resolve forward top-level type aliases before any annotated global,
	// field, or signature is lowered. Local aliases remain source-ordered.
	for pass := 0; pass < len(program.Decls); pass++ {
		changed := false
		for _, decl := range program.Decls {
			d, ok := decl.(*VarDecl)
			if !ok || !d.CompileTime {
				continue
			}
			if _, exists := l.typeScopes[0][d.Name]; exists {
				continue
			}
			if target, ok := l.typeAliasTarget(d.Init); ok {
				l.typeScopes[0][d.Name] = target
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	// Every global receives its semantic type before any initializer/default is
	// lowered. This removes source-order dependence from expression lowering.
	for _, decl := range program.Decls {
		if d, ok := decl.(*VarDecl); ok && d.DeclType != nil {
			l.varTypes[l.globalSymbols[d]] = l.typeOfTypeExpr(d.DeclType)
		}
	}
	for pass := 0; pass < len(program.Decls); pass++ {
		changed := false
		for _, decl := range program.Decls {
			d, ok := decl.(*VarDecl)
			if !ok {
				continue
			}
			sym := l.globalSymbols[d]
			if l.varTypes[sym] != 0 && l.varTypes[sym] != l.types.Unknown() {
				continue
			}
			if typ := l.inferASTType(d.Init); typ != l.types.Unknown() {
				l.varTypes[sym] = typ
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	if program.EntryDecl != nil {
		l.hir.Entry = l.procSymbols[program.EntryDecl]
	}

	// Pass 2: resolve struct field types now that every type name is known.
	for _, decl := range program.Decls {
		if d, ok := decl.(*StructDecl); ok {
			l.finishStruct(d)
		}
	}

	// Pass 3: lower globals in stable dependency order, then procedures.
	l.lowerGlobals(program)
	for _, decl := range program.Decls {
		if d, ok := decl.(*ProcDecl); ok {
			l.lowerProc(d)
		}
	}

	l.popScope()
}

func (l *Lowerer) lowerGlobals(program *Program) {
	seen := make(map[*VarDecl]bool)
	for _, decl := range l.analysis.GlobalOrder {
		if _, belongsToProgram := l.globalSymbols[decl]; belongsToProgram && !seen[decl] {
			l.lowerGlobal(decl)
			seen[decl] = true
		}
	}
	// Invalid/incomplete analyses do not normally reach lowering. Keep a
	// deterministic recovery path so direct library use produces diagnostics
	// rather than silently dropping a global.
	for _, decl := range program.Decls {
		if global, ok := decl.(*VarDecl); ok && !seen[global] {
			l.diags.Error(global.NameSpan, "semantic analysis did not provide a global initialization order", "fix semantic errors before lowering")
			l.lowerGlobal(global)
		}
	}
}

func (l *Lowerer) inferASTType(expr Expr) TypeID {
	if l.analysis != nil {
		if semanticType, ok := l.analysis.ExprTypes[expr]; ok && semanticType != TypeUnknown {
			if id := l.semanticTypeID(semanticType); id != l.types.Unknown() {
				return id
			}
		}
	}
	switch n := expr.(type) {
	case nil:
		return l.types.Unknown()
	case *IntExpr:
		return l.types.S64()
	case *FloatExpr:
		return l.types.F64()
	case *StringExpr:
		return l.types.String()
	case *BoolExpr:
		return l.types.Bool()
	case *IdentExpr:
		sym := l.lookup(n.Name)
		if sym != NoSymbol {
			return l.varTypes[sym]
		}
	case *ParenExpr:
		return l.inferASTType(n.Inner)
	case *UnaryExpr:
		if n.Op == UnaryOpAddr {
			// Address-of: result type is the pointer to the operand type.
			return l.types.InternPointer(l.inferASTType(n.Operand), false)
		}
		return l.inferASTType(n.Operand)
	case *DerefExpr:
		ot := l.inferASTType(n.Operand)
		if ot != l.types.Unknown() && l.types.Lookup(ot).Kind == TypeKindPointer {
			return l.types.PointerElemType(ot)
		}
		return l.types.Unknown()
	case *FieldAccessExpr:
		bt := l.inferASTType(n.Base)
		if hs, ok := l.hirStructs[bt]; ok {
			for _, field := range hs.Fields {
				if field.Name == n.Field {
					return field.Type
				}
			}
		}
		return l.types.Unknown()
	case *BinaryExpr:
		if n.Op >= BinaryOpLt {
			return l.types.Bool()
		}
		left := l.inferASTType(n.Left)
		if left != l.types.Unknown() {
			return left
		}
		return l.inferASTType(n.Right)
	case *CallExpr:
		if id, ok := n.Func.(*IdentExpr); ok {
			if p, found := l.lookupProc(id.Name); found {
				t := l.tupleTypeID(l.procValueTypes(p))
				if p.ErrorResult != nil {
					t = l.unionTypeID(t, l.procErrorType(p))
				}
				return t
			}
		}
	case *StructInitExpr:
		if n.Type != nil {
			return l.typeOfTypeExpr(n.Type)
		}
	case *ArrayInitExpr:
		return l.types.InternArray(l.typeOfTypeExpr(n.Elem))
	case *IndexExpr:
		base := l.types.Lookup(l.inferASTType(n.Base))
		if base.Kind == TypeKindArray {
			return base.Elem
		}
	case *ErrorMemberExpr:
		if n.TypeName != "" {
			if t, ok := l.lookupType(n.TypeName); ok {
				return t
			}
		}
	case *EnumMemberExpr:
		if n.TypeName != "" {
			if t, ok := l.lookupType(n.TypeName); ok {
				return t
			}
		}
	case *IfxExpr:
		thenType := l.inferASTType(n.Then)
		if thenType != l.types.Unknown() {
			return thenType
		}
		return l.inferASTType(n.Else)
	}
	return l.types.Unknown()
}

func (l *Lowerer) semanticTypeID(t Type) TypeID {
	if l.analysis != nil {
		switch decl := l.analysis.NominalDecls[t].(type) {
		case *StructDecl:
			if id, ok := l.structTypes[decl]; ok {
				return id
			}
		case *ErrorDecl:
			if id, ok := l.errorTypes[decl]; ok {
				return id
			}
		case *EnumDecl:
			if id, ok := l.enumTypes[decl]; ok {
				return id
			}
		}
	}
	name := string(t)
	if strings.HasPrefix(name, "[]") {
		return l.types.InternArray(l.semanticTypeID(Type(strings.TrimPrefix(name, "[]"))))
	}
	if strings.HasPrefix(name, "*") {
		nullable := strings.HasSuffix(name, "?")
		elem := strings.TrimSuffix(strings.TrimPrefix(name, "*"), "?")
		return l.types.InternPointer(l.semanticTypeID(Type(elem)), nullable)
	}
	if id, ok := l.lookupType(name); ok {
		return id
	}
	if id, ok := l.types.ByName(name); ok {
		return id
	}
	return l.types.Unknown()
}

func (l *Lowerer) registerProc(d *ProcDecl) {
	sym := l.symbols.Declare(d.Name)
	l.procSymbols[d] = sym
	l.procScopes[len(l.procScopes)-1][d.Name] = d
}

func (l *Lowerer) registerStruct(d *StructDecl) {
	tid := l.types.InternScopedStruct(d.Name)
	l.structTypes[d], l.structDecls[tid] = tid, d
	l.typeScopes[len(l.typeScopes)-1][d.Name] = tid
	hs := &HIRStruct{Symbol: l.symbols.Declare(d.Name), Name: d.Name, Type: tid, Span: d.Span_}
	l.hirStructs[tid] = hs
	l.hir.Structs = append(l.hir.Structs, hs)
}

func (l *Lowerer) finishStruct(d *StructDecl) {
	tid := l.structTypes[d]
	hs := l.hirStructs[tid]
	fields := make([]TypeField, len(d.Fields))
	hirFields := make([]HIRField, len(d.Fields))
	for i, field := range d.Fields {
		ft := l.typeOfTypeExpr(field.Type)
		fsym := l.symbols.Declare(field.Name)
		fields[i] = TypeField{Symbol: fsym, Name: field.Name, Type: ft}
		hirFields[i] = HIRField{Symbol: fsym, Name: field.Name, Type: ft, Span: field.Span_}
	}
	l.types.SetStructFields(tid, fields)
	hs.Fields = hirFields
}

func (l *Lowerer) registerError(d *ErrorDecl) {
	tid := l.types.InternScopedError(d.Name)
	l.errorTypes[d] = tid
	l.typeScopes[len(l.typeScopes)-1][d.Name] = tid
	ordinals := make(map[string]int, len(d.Members))
	for i, member := range d.Members {
		ordinals[member.Name] = i
	}
	l.errorOrdinal[tid] = ordinals
}

func (l *Lowerer) registerEnum(d *EnumDecl) {
	if len(d.Members) == 0 || d.Members[0].Type == nil {
		l.diags.Error(d.Span_, "cannot lower enum without a typed first member", "fix semantic errors before lowering")
		return
	}
	underlying := l.typeOfTypeExpr(d.Members[0].Type)
	tid := l.types.InternScopedEnum(d.Name, underlying)
	l.enumTypes[d] = tid
	l.typeScopes[len(l.typeScopes)-1][d.Name] = tid
	values := make(map[string]string, len(d.Members))
	prev := big.NewInt(0)
	for i, member := range d.Members {
		value := new(big.Int).Add(prev, big.NewInt(1))
		if i == 0 {
			value.SetInt64(0)
		}
		if member.Value != nil {
			if explicit, ok := evalEnumMemberValue(member.Value); ok {
				value.Set(explicit)
			}
		}
		values[member.Name] = value.String()
		prev.Set(value)
	}
	l.enumValues[tid] = values
}

func (l *Lowerer) lowerGlobal(d *VarDecl) {
	sym := l.globalSymbols[d]
	t := l.varTypes[sym]
	if t == 0 {
		t = l.types.Unknown()
	}
	if d.DeclType != nil {
		t = l.typeOfTypeExpr(d.DeclType)
	}
	var init HIRExpr
	if d.CompileTime {
		if _, found := l.typeAliasTarget(d.Init); found {
			l.varTypes[sym] = l.types.Void()
			l.hir.Globals = append(l.hir.Globals, &HIRGlobal{Symbol: sym, Name: d.Name, Type: l.types.Void(), CompileTime: true, Span: d.Span_})
			return
		}
		if ident, ok := d.Init.(*IdentExpr); ok {
			if proc, found := l.lookupProc(ident.Name); found {
				l.procAliases[sym] = proc
				l.varTypes[sym] = l.types.Void()
				l.hir.Globals = append(l.hir.Globals, &HIRGlobal{Symbol: sym, Name: d.Name, Type: l.types.Void(), CompileTime: true, Span: d.Span_})
				return
			}
		}
	}
	if d.Init != nil {
		// A top-level shadow initializer references the previous global
		// declaration. Other code continues to see the final global binding.
		final := l.scopes[0][d.Name]
		initializerBinding := sym
		if previous, ok := l.globalPrevious[d]; ok {
			initializerBinding = previous
		}
		l.scopes[0][d.Name] = initializerBinding
		init = l.lowerExprAs(d.Init, t)
		l.scopes[0][d.Name] = final
		if t == l.types.Unknown() {
			t = init.hirType()
		}
	}
	if init == nil {
		init = l.zeroValue(t, d.Span_)
	}
	l.varTypes[sym] = t
	if d.CompileTime {
		init = l.adaptLiteral(l.foldConstant(init), t)
		l.constValues[sym] = init
	}
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

func (l *Lowerer) procValueTypes(d *ProcDecl) []TypeID {
	if d.ErrorResult != nil && len(d.Results) == 1 {
		left, right := l.typeOfTypeExpr(d.Results[0]), l.typeOfTypeExpr(d.ErrorResult)
		if l.types.Lookup(left).Kind == TypeKindError && l.types.Lookup(right).Kind != TypeKindError {
			return []TypeID{right}
		}
	}
	results := make([]TypeID, len(d.Results))
	for i, result := range d.Results {
		results[i] = l.typeOfTypeExpr(result)
	}
	return results
}

func (l *Lowerer) procErrorType(d *ProcDecl) TypeID {
	left, right := l.typeOfTypeExpr(d.Results[0]), l.typeOfTypeExpr(d.ErrorResult)
	if len(d.Results) == 1 && l.types.Lookup(left).Kind == TypeKindError && l.types.Lookup(right).Kind != TypeKindError {
		return left
	}
	return right
}

func (l *Lowerer) tupleTypeID(types []TypeID) TypeID {
	if len(types) == 0 {
		return l.types.Void()
	}
	if len(types) == 1 {
		return types[0]
	}
	parts := make([]string, len(types))
	fields := make([]TypeField, len(types))
	for i, typ := range types {
		parts[i] = strconv.FormatUint(uint64(typ), 10)
		name := strconv.Itoa(i)
		fields[i] = TypeField{Symbol: l.symbols.Declare("__chaos_tuple_" + name), Name: name, Type: typ}
	}
	return l.types.InternTuple("("+strings.Join(parts, ",")+")", fields)
}

func (l *Lowerer) lowerProc(d *ProcDecl) {
	sym, ok := l.procSymbols[d]
	if !ok {
		l.diags.Error(d.NameSpan, "internal error: procedure has no registered symbol", "report this compiler bug")
		return
	}
	previousFloor, previousDepth := l.procValueFloor, l.procDepth
	l.procValueFloor, l.procDepth = len(l.scopes), l.procDepth+1
	l.pushScope()
	params := make([]HIRParam, len(d.Params))
	for i, p := range d.Params {
		pt := l.typeOfTypeExpr(p.Type)
		psym := l.symbols.Declare(p.Name)
		l.declare(p.Name, psym)
		l.varTypes[psym] = pt
		l.symbolCompileTime[psym] = false
		params[i] = HIRParam{Symbol: psym, Name: p.Name, Type: pt, Span: p.Span_}
	}
	valueResults := l.procValueTypes(d)
	resultType := l.tupleTypeID(valueResults)
	if d.ErrorResult != nil {
		et := l.procErrorType(d)
		resultType = l.unionTypeID(resultType, et)
	}
	var results []TypeID
	if resultType != l.types.Void() {
		results = []TypeID{resultType}
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
	l.procValueFloor, l.procDepth = previousFloor, previousDepth
	l.hir.Procs = append(l.hir.Procs, &HIRProc{
		Symbol:     sym,
		Name:       d.Name,
		Params:     params,
		Results:    results,
		ResultType: resultType,
		Body:       body,
		Span:       d.Span_,
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
	case *MultiVarDecl:
		return l.lowerMultiVarDecl(n)
	case *AssignStmt:
		if n.Target != nil {
			return l.lowerTargetAssign(n)
		}
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
	case *DeallocateStmt:
		return &HIRDeallocate{Span_: n.Span_, Addr: l.lowerExpr(n.Addr)}
	case *CompoundAssignStmt:
		if n.Target != nil {
			return l.lowerTargetCompound(n)
		}
		return l.lowerCompoundAssign(n)
	case *IncDecStmt:
		if n.Target != nil {
			return l.lowerTargetIncDec(n)
		}
		return l.lowerIncDec(n)
	case *ProcDecl:
		l.registerProc(n)
		l.lowerProc(n)
		return nil
	case *StructDecl:
		l.registerStruct(n)
		l.finishStruct(n)
		return nil
	case *ErrorDecl:
		l.registerError(n)
		return nil
	case *EnumDecl:
		l.registerEnum(n)
		return nil
	}
	return nil
}

func (l *Lowerer) lowerVarDecl(d *VarDecl) HIRStmt {
	var t TypeID = l.types.Unknown()
	if d.DeclType != nil {
		t = l.typeOfTypeExpr(d.DeclType)
	}
	// The initializer is lowered before the symbol is declared so that a
	// shadowing declaration ("#shadow x := x + 1") references the outer
	// binding, matching the type checker's scoping.
	var init HIRExpr
	if d.CompileTime {
		if target, ok := l.typeAliasTarget(d.Init); ok {
			sym := l.symbols.Declare(d.Name)
			l.declare(d.Name, sym)
			l.typeScopes[len(l.typeScopes)-1][d.Name] = target
			l.symbolCompileTime[sym] = true
			return nil
		}
		if ident, ok := d.Init.(*IdentExpr); ok {
			proc, found := l.lookupProc(ident.Name)
			if source := l.lookup(ident.Name); source != NoSymbol && l.procAliases[source] != nil {
				proc, found = l.procAliases[source], true
			}
			if found {
				sym := l.symbols.Declare(d.Name)
				l.declare(d.Name, sym)
				l.procAliases[sym] = proc
				l.symbolCompileTime[sym] = true
				return nil
			}
			if _, found := l.lookupType(ident.Name); found {
				sym := l.symbols.Declare(d.Name)
				l.declare(d.Name, sym)
				l.symbolCompileTime[sym] = true
				return nil
			}
		}
	}
	if d.Init != nil {
		init = l.lowerExprAs(d.Init, t)
		if t == l.types.Unknown() {
			t = init.hirType()
		}
	}
	sym := l.symbols.Declare(d.Name)
	l.declare(d.Name, sym)
	l.varTypes[sym] = t
	l.symbolCompileTime[sym] = d.CompileTime
	if d.CompileTime {
		init = l.adaptLiteral(l.foldConstant(init), t)
		l.constValues[sym] = init
		return nil
	}
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

func (l *Lowerer) typeAliasTarget(expr Expr) (TypeID, bool) {
	switch n := expr.(type) {
	case *ParenExpr:
		return l.typeAliasTarget(n.Inner)
	case *IdentExpr:
		return l.lookupType(n.Name)
	}
	return l.types.Unknown(), false
}

func (l *Lowerer) lowerMultiVarDecl(d *MultiVarDecl) HIRStmt {
	init := l.lowerExpr(d.Init)
	tuple := l.types.Lookup(init.hirType())
	temp := l.symbols.Declare("__chaos_multi")
	l.varTypes[temp] = init.hirType()
	stmts := []HIRStmt{&HIRVarDecl{Span_: d.Span_, Symbol: temp, Type: init.hirType(), Init: init}}
	for i, name := range d.Names {
		sym := l.symbols.Declare(name)
		l.declare(name, sym)
		fieldType := tuple.Fields[i].Type
		l.varTypes[sym] = fieldType
		l.symbolCompileTime[sym] = d.CompileTime
		stmts = append(stmts, &HIRVarDecl{
			Span_: d.NameSpans[i], Symbol: sym, Name: name, Type: fieldType,
			Init:    &HIRFieldLoad{Span_: d.Span_, Base: &HIRRef{Span_: d.Span_, Symbol: temp, Type: init.hirType()}, Field: i, Type: fieldType},
			Mutable: d.Mutable, CompileTime: d.CompileTime,
		})
	}
	return &HIRBlock{Span_: d.Span_, Stmts: stmts}
}

func (l *Lowerer) lowerIf(n *IfStmt) HIRStmt {
	cond := l.lowerExpr(n.Condition)
	base := maps.Clone(l.unwrapped)
	then := l.lowerBlock(n.Body)
	branchStates := []map[SymbolID]bool{maps.Clone(l.unwrapped)}
	l.unwrapped = maps.Clone(base)
	var elifs []*HIRIf
	for _, e := range n.Elif {
		elifs = append(elifs, &HIRIf{
			Span_:     e.Span_,
			Condition: l.lowerExpr(e.Condition),
			Then:      l.lowerBlock(e.Body),
		})
		branchStates = append(branchStates, maps.Clone(l.unwrapped))
		l.unwrapped = maps.Clone(base)
	}
	var els *HIRBlock
	if n.ElseBody != nil {
		els = l.lowerBlock(n.ElseBody)
		branchStates = append(branchStates, maps.Clone(l.unwrapped))
	} else {
		branchStates = append(branchStates, base)
	}
	l.unwrapped = maps.Clone(base)
	for symbol := range l.unwrapped {
		for _, state := range branchStates {
			if !state[symbol] {
				delete(l.unwrapped, symbol)
				break
			}
		}
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
	const hiddenArrayName = "__chaos_range_array"
	arrSym := l.symbols.Declare(hiddenArrayName)
	l.varTypes[arrSym] = arrType
	arrDecl := &HIRVarDecl{Span_: n.Span_, Symbol: arrSym, Name: hiddenArrayName, Type: arrType, Init: arrVal, Mutable: false, CompileTime: false}

	// Index variable: the named index (when given) is the loop counter.
	idxName := "__chaos_range_index"
	if indexName != "" {
		idxName = indexName
	}
	idxSym := l.symbols.Declare(idxName)
	if indexName != "" {
		l.declare(idxName, idxSym)
	}
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

// lowerTargetAssign lowers "target = value" for an index, field, or deref
// target: compute the target's address once and store through it.
func (l *Lowerer) lowerTargetAssign(n *AssignStmt) HIRStmt {
	addr := l.addressOf(n.Target)
	t := l.inferASTType(n.Target)
	if t == l.types.Unknown() {
		t = l.inferASTType(n.Value)
	}
	value := l.lowerExprAs(n.Value, t)
	return &HIRAddrStore{Span_: n.Span_, Addr: addr, Value: value, Type: t}
}

// lowerTargetCompound desugars "target += value" / "target -= value" for a
// compound target into a read, an operation, and a store through the address.
func (l *Lowerer) lowerTargetCompound(n *CompoundAssignStmt) HIRStmt {
	addr := l.addressOf(n.Target)
	vt := l.valueExpr(n.Target)
	t := vt.hirType()
	right := l.lowerExprAs(n.Value, t)
	return &HIRAddrStore{
		Span_: n.Span_,
		Addr:  addr,
		Value: &HIRBinary{Span_: n.Span_, Op: n.Op, Left: vt, Right: right, Type: t},
		Type:  t,
	}
}

// lowerTargetIncDec desugars "target++" / "target--" for a non-identifier
// target into a read, an add/sub of one, and a store through the address.
// A pointer target steps by one element (the literal is typed S64).
func (l *Lowerer) lowerTargetIncDec(n *IncDecStmt) HIRStmt {
	addr := l.addressOf(n.Target)
	vt := l.valueExpr(n.Target)
	t := vt.hirType()
	oneType := t
	if l.types.Lookup(t).Kind == TypeKindPointer {
		oneType = l.types.S64()
	}
	one := &HIRConst{Span_: n.Span_, Type: oneType, Kind: ConstInt, Int: 1}
	return &HIRAddrStore{
		Span_: n.Span_,
		Addr:  addr,
		Value: &HIRBinary{Span_: n.Span_, Op: n.Op, Left: vt, Right: one, Type: t},
		Type:  t,
	}
}

// valueExpr produces the HIR expression that reads the current value of an
// lvalue expression (index, field, or deref target).
func (l *Lowerer) valueExpr(target Expr) HIRExpr {
	switch n := target.(type) {
	case *IdentExpr:
		sym := l.lookup(n.Name)
		return &HIRRef{Span_: n.Span_, Type: l.varTypes[sym], Symbol: sym}
	case *IndexExpr:
		base := l.lowerExpr(n.Base)
		idx := l.lowerExpr(n.Index)
		return &HIRIndex{Span_: n.Span_, Base: base, Index: idx, Type: l.inferASTType(n)}
	case *FieldAccessExpr:
		return l.lowerExpr(n)
	case *DerefExpr:
		return l.lowerExpr(n)
	}
	return l.poison(target.nodeSpan(), "unsupported lvalue read")
}

// addressOf produces the HIR expression computing the address of an lvalue:
// a local/global, a dereferenced pointer (its address IS the pointer value),
// an array element, or a struct field.
func (l *Lowerer) addressOf(target Expr) HIRExpr {
	switch n := target.(type) {
	case *IdentExpr:
		sym := l.lookup(n.Name)
		if sym == NoSymbol {
			return l.poison(n.Span_, "unresolved lvalue '"+n.Name+"'")
		}
		return &HIRAddrOf{Span_: n.Span_, Operand: &HIRRef{Span_: n.Span_, Symbol: sym, Type: l.varTypes[sym]}, Type: l.types.InternPointer(l.varTypes[sym], false)}
	case *DerefExpr:
		// The address behind "p.*" is the pointer value of p, loaded as a
		// value; no address computation is needed.
		return l.lowerExpr(n.Operand)
	case *IndexExpr:
		arr := l.lowerExpr(n.Base)
		idx := l.lowerExpr(n.Index)
		elemType := l.typeOfIndexElem(n)
		return &HIRArrayElemAddr{Span_: n.Span_, Array: arr, Index: idx, Type: l.types.InternPointer(elemType, false)}
	case *FieldAccessExpr:
		baseAddr := l.addressOf(n.Base)
		idx := l.typeIndex(n.Base, n.Field)
		if idx < 0 {
			return l.poison(n.Span_, "unknown struct field '"+n.Field+"'")
		}
		return &HIRFieldAddr{Span_: n.Span_, Addr: baseAddr, Field: idx, Type: l.types.InternPointer(l.typeOfField(n.Base, n.Field), false)}
	}
	return l.poison(target.nodeSpan(), "unsupported address-of target")
}

// typeIndex returns the declaration index of the named field on the struct
// type of base, or -1 when base is not a struct.
func (l *Lowerer) typeIndex(base Expr, field string) int {
	bt := l.inferASTType(base)
	if bt == l.types.String() {
		switch field {
		case "data":
			return 0
		case "count":
			return 1
		}
		return -1
	}
	hs, ok := l.hirStructs[bt]
	if !ok {
		return -1
	}
	for i, f := range hs.Fields {
		if f.Name == field {
			return i
		}
	}
	return -1
}

// typeOfField returns the resolved field type of the named field on the
// struct type of base, or Unknown when base is not a struct.
func (l *Lowerer) typeOfField(base Expr, field string) TypeID {
	bt := l.inferASTType(base)
	if bt == l.types.String() {
		switch field {
		case "data":
			return l.types.InternPointer(l.types.Byte(), false)
		case "count":
			return l.types.Size()
		}
		return l.types.Unknown()
	}
	hs, ok := l.hirStructs[bt]
	if !ok {
		return l.types.Unknown()
	}
	for _, f := range hs.Fields {
		if f.Name == field {
			return f.Type
		}
	}
	return l.types.Unknown()
}

// typeOfIndexElem returns the element type of the indexed array.
func (l *Lowerer) typeOfIndexElem(n *IndexExpr) TypeID {
	at := l.inferASTType(n.Base)
	if at != l.types.Unknown() && l.types.Lookup(at).Kind == TypeKindArray {
		return l.types.Lookup(at).Elem
	}
	return l.inferASTType(n)
}

// lowerIncDec desugars "name++" / "++name" into "name = name + 1" (and the
// decrement forms into "name = name - 1"). A pointer target increments by one
// element: the literal is typed S64 so the MIR ptr.* opcodes apply the scale.
func (l *Lowerer) lowerIncDec(n *IncDecStmt) HIRStmt {
	sym := l.lookup(n.Name)
	t := l.varTypes[sym]
	oneType := t
	if l.types.Lookup(t).Kind == TypeKindPointer {
		oneType = l.types.S64()
	}
	one := &HIRConst{Span_: n.Span_, Type: oneType, Kind: ConstInt, Int: 1}
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
	values := n.Values
	if len(values) == 0 && n.Value != nil {
		values = []Expr{n.Value}
	}
	var value HIRExpr
	if len(l.curResults) > 0 && isUnionTypeID(l.types, l.curResults[0]) {
		rt := l.types.Lookup(l.curResults[0])
		vt, et := rt.Fields[0].Type, rt.Fields[1].Type
		if len(values) == 0 {
			// A bare return in a 'Void <> E' procedure is a successful
			// return with no value.
			if vt == l.types.Void() {
				value = l.lowerUnion(l.curResults[0], l.emptyConst(vt, n.Span_), l.emptyConst(et, n.Span_), &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: false}, n.Span_)
			}
		} else if len(values) == 1 {
			var lowered HIRExpr
			if em, ok := values[0].(*ErrorMemberExpr); ok && em.TypeName == "" {
				// A bare error literal resolves against the error side.
				lowered = l.lowerErrorMember(em, l.types.Lookup(et).Name)
			} else {
				lowered = l.lowerExprAs(values[0], vt)
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
		} else {
			lowered := l.lowerResultValues(values, vt, n.Span_)
			value = l.lowerUnion(l.curResults[0], lowered, l.emptyConst(et, n.Span_), &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: false}, n.Span_)
		}
	} else if len(values) > 0 {
		if len(l.curResults) > 0 {
			value = l.lowerResultValues(values, l.curResults[0], n.Span_)
		} else {
			value = l.lowerExpr(values[0])
		}
	}
	return &HIRReturn{Span_: n.Span_, Value: value}
}

func (l *Lowerer) lowerResultValues(values []Expr, resultType TypeID, span Span) HIRExpr {
	if len(values) == 1 {
		return l.lowerExprAs(values[0], resultType)
	}
	tuple := l.types.Lookup(resultType)
	fields := make([]HIRStructInitField, len(values))
	for i, expr := range values {
		fields[i] = HIRStructInitField{Span_: expr.nodeSpan(), Field: tuple.Fields[i].Symbol, Value: l.lowerExprAs(expr, tuple.Fields[i].Type)}
	}
	return &HIRStructInit{Span_: span, Struct: NoSymbol, Type: resultType, Fields: fields}
}

// lowerUnlessCatch lowers "target := expr unless catch [err] { body }" (or
// the bare form with Target == "") into a variable declaration holding the
// value-or-error pair followed by the catch check.
func (l *Lowerer) lowerUnlessCatch(n *UnlessCatchStmt) HIRStmt {
	init := l.lowerExpr(n.Init)
	targets := n.Targets
	spans := n.TargetSpans
	if len(targets) == 0 && n.Target != "" {
		targets, spans = []string{n.Target}, []Span{n.TargetSpan}
	}
	var stmts []HIRStmt
	cond := HIRExpr(init)
	temp := NoSymbol
	if len(targets) > 0 {
		temp = l.symbols.Declare("__chaos_union")
		l.varTypes[temp] = init.hirType()
		stmts = append(stmts, &HIRVarDecl{Span_: n.Span_, Symbol: temp, Type: init.hirType(), Init: init})
		cond = &HIRRef{Span_: n.Span_, Symbol: temp, Type: init.hirType()}
	}
	ifCatch := l.buildIfCatch(n.Span_, cond, n.CatchName, n.CatchBody)
	stmts = append(stmts, ifCatch)
	if len(targets) > 0 {
		union := l.types.Lookup(init.hirType())
		valueType := union.Fields[0].Type
		value := HIRExpr(&HIRFieldLoad{Span_: n.Span_, Base: &HIRRef{Span_: n.Span_, Symbol: temp, Type: init.hirType()}, Field: 0, Type: valueType})
		for i, target := range targets {
			fieldType, fieldValue := valueType, value
			if len(targets) > 1 {
				tuple := l.types.Lookup(valueType)
				fieldType = tuple.Fields[i].Type
				fieldValue = &HIRFieldLoad{Span_: n.Span_, Base: value, Field: i, Type: fieldType}
			}
			sym := l.symbols.Declare(target)
			l.declare(target, sym)
			l.varTypes[sym] = fieldType
			stmts = append(stmts, &HIRVarDecl{Span_: spans[i], Symbol: sym, Name: target, Type: fieldType, Init: fieldValue, Mutable: true})
		}
	}
	if len(stmts) == 1 {
		return stmts[0]
	}
	return &HIRBlock{Span_: n.Span_, Stmts: stmts}
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
	l.pushScope()
	if catchName != "" {
		catchSym = l.symbols.Declare(catchName)
		l.declare(catchName, catchSym)
		l.varTypes[catchSym] = l.types.Lookup(ut).Fields[1].Type
	}
	body := l.lowerBlock(catchBody)
	l.popScope()
	return &HIRIfCatch{
		Span_:     span,
		Cond:      cond,
		CatchSym:  catchSym,
		CatchBody: body,
		UnionType: ut,
	}
}

// lookupSymbol resolves a name to its SymbolID in the current scope chain.
func (l *Lowerer) lookupSymbol(name string) (SymbolID, bool) {
	for i := len(l.scopes) - 1; i >= 0; i-- {
		if sym, ok := l.scopes[i][name]; ok {
			if l.procDepth > 0 && i > 0 && i < l.procValueFloor && !l.symbolCompileTime[sym] {
				continue
			}
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
	return l.zeroValue(t, span)
}

func (l *Lowerer) zeroValue(t TypeID, span Span) HIRExpr {
	typ := l.types.Lookup(t)
	switch typ.Kind {
	case TypeKindBool:
		return &HIRConst{Span_: span, Type: t, Kind: ConstBool, Bool: false}
	case TypeKindString:
		return &HIRConst{Span_: span, Type: t, Kind: ConstString, Str: ""}
	case TypeKindFloat:
		return &HIRConst{Span_: span, Type: t, Kind: ConstFloat, Float: 0, Str: "0.0"}
	case TypeKindInt, TypeKindEnum:
		return &HIRConst{Span_: span, Type: t, Kind: ConstInt, Int: 0, Str: "0"}
	case TypeKindError:
		return &HIRConst{Span_: span, Type: t, Kind: ConstError, Int: 0, Str: "0"}
	case TypeKindArray:
		return &HIRArrayInit{Span_: span, Type: t}
	case TypeKindStruct, TypeKindTuple:
		fields := make([]HIRStructInitField, len(typ.Fields))
		for i, field := range typ.Fields {
			value := l.zeroValue(field.Type, span)
			if decl := l.structDecls[t]; decl != nil && decl.Fields[i].Default != nil {
				value = l.lowerExprAs(decl.Fields[i].Default, field.Type)
			}
			fields[i] = HIRStructInitField{Span_: span, Field: field.Symbol, Value: value}
		}
		return &HIRStructInit{Span_: span, Struct: NoSymbol, Type: t, Fields: fields}
	case TypeKindPointer:
		return &HIRConst{Span_: span, Type: t, Kind: ConstInt, Int: 0}
	case TypeKindVoid:
		return &HIRZero{Span_: span, Type: t}
	}
	return l.poison(span, "cannot construct zero value of unresolved type")
}

func (l *Lowerer) lowerExpr(e Expr) HIRExpr {
	switch n := e.(type) {
	case *IntExpr:
		value, ok := parseIntLiteral(n.Value)
		if !ok {
			return l.poison(n.Span_, "invalid integer literal reached lowering")
		}
		return &HIRConst{
			Span_: n.Span_,
			Type:  l.types.S64(),
			Kind:  ConstInt,
			Int:   value,
			Str:   n.Value,
		}
	case *FloatExpr:
		value, ok := parseFloatLiteral(n.Value)
		if !ok {
			return l.poison(n.Span_, "invalid or non-finite floating-point literal reached lowering")
		}
		return &HIRConst{
			Span_: n.Span_,
			Type:  l.types.F64(),
			Kind:  ConstFloat,
			Float: value,
			Str:   n.Value,
		}
	case *StringExpr:
		return &HIRConst{Span_: n.Span_, Type: l.types.String(), Kind: ConstString, Str: n.Value}
	case *BoolExpr:
		return &HIRConst{Span_: n.Span_, Type: l.types.Bool(), Kind: ConstBool, Bool: n.Value}
	case *IdentExpr:
		sym := l.lookup(n.Name)
		if sym == NoSymbol {
			return l.poison(n.Span_, "unresolved value '"+n.Name+"'")
		}
		if value := l.constValues[sym]; value != nil {
			return value
		}
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
		if n.Op == UnaryOpAddr {
			// '*target' is the address of an lvalue.
			return l.addressOf(n.Operand)
		}
		return l.lowerUnary(n)
	case *NullLitExpr:
		t := l.inferASTType(n)
		if t == l.types.Unknown() || l.types.Lookup(t).Kind != TypeKindPointer {
			return l.poison(n.Span_, "null has no pointer target type")
		}
		return &HIRConst{Span_: n.Span_, Type: t, Kind: ConstInt, Int: 0}
	case *CallExpr:
		return l.lowerCall(n)
	case *AllocateExpr:
		return &HIRAllocate{Span_: n.Span_, Size: l.lowerExpr(n.Size), Type: l.types.Addr()}
	case *CastExpr:
		return &HIRCast{Span_: n.Span_, Value: l.lowerExpr(n.Value), Type: l.typeOfTypeExpr(n.Type)}
	case *InterpolatedStringExpr:
		var literals []string
		var values []HIRExpr
		for _, part := range n.Parts {
			if part.Expr != nil {
				values = append(values, l.lowerExpr(part.Expr))
			} else {
				literals = append(literals, part.Literal)
			}
		}
		return &HIRInterpolate{Span_: n.Span_, Literals: literals, Values: values, Type: l.types.String()}
	case *StructInitExpr:
		if n.Type != nil {
			return l.lowerStructInit(n, l.typeOfTypeExpr(n.Type))
		}
		// Inferred struct literal: the type comes from context, so it is
		// resolved by lowerExprAs.
		return &HIRPoison{Span_: n.Span_, Type: l.types.Unknown()}
	case *ErrorMemberExpr:
		if n.TypeName != "" {
			return l.lowerErrorMember(n, n.TypeName)
		}
		// Bare error member: the type comes from context, so it is resolved
		// by lowerExprAs.
		return &HIRPoison{Span_: n.Span_, Type: l.types.Unknown()}
	case *EnumMemberExpr:
		if n.TypeName != "" {
			return l.lowerEnumMember(n, n.TypeName)
		}
		// Bare enum member: the type comes from context, so it is resolved
		// by lowerExprAs.
		return l.poison(n.Span_, "bare enum member has no target type")
	case *ErrorExpr:
		return l.poison(n.Span_, "parser recovery expression reached lowering")
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
	case *DerefExpr:
		operand := l.lowerExpr(n.Operand)
		t := l.inferASTType(n)
		if t == l.types.Unknown() {
			t = l.types.PointerElemType(operand.hirType())
		}
		return &HIRDeref{Span_: n.Span_, Operand: operand, Type: t}
	case *FieldAccessExpr:
		// A base that names an enum type is an enum member reference and
		// lowers to the member's constant.
		if name := enumBaseName(n.Base); name != "" {
			if t := l.typeOfTypeExpr(&IdentExpr{Name: name}); t != l.types.Unknown() && l.types.Lookup(t).Kind == TypeKindEnum {
				return l.lowerEnumMember(&EnumMemberExpr{
					Span_:    n.Span_,
					TypeName: name,
					Name:     n.Field,
				}, name)
			}
		}
		// A field read on a struct value (including one produced by a deref).
		base := l.lowerExpr(n.Base)
		ft := l.inferASTType(n)
		idx := l.typeIndex(n.Base, n.Field)
		if idx < 0 {
			return l.poison(n.Span_, "unknown struct field '"+n.Field+"'")
		}
		return &HIRFieldLoad{Span_: n.Span_, Base: base, Field: idx, Type: ft}
	case *LoopBuiltinExpr:
		if n.Name == "this" && l.rangeThis != nil {
			return l.rangeThis
		}
		if n.Name == "index" && l.rangeIndex != NoSymbol {
			return &HIRRef{Span_: n.Span_, Symbol: l.rangeIndex, Type: l.types.S64()}
		}
		return l.poison(n.Span_, "loop builtin used outside a range loop")
	case *IfxExpr:
		return l.lowerIfx(n)
	}
	return l.poison(e.nodeSpan(), "unsupported expression")
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
	if em, ok := e.(*EnumMemberExpr); ok && em.TypeName == "" {
		return l.lowerEnumMember(em, l.types.Lookup(target).Name)
	}
	lowered := l.lowerExpr(e)
	// A pointer value assigned to Addr is implicitly cast to the opaque
	// address type.
	if target == l.types.Addr() && lowered.hirType() != target {
		if lt := l.types.Lookup(lowered.hirType()); lt.Kind == TypeKindPointer || lt.Kind == TypeKindAddr {
			return &HIRCast{Span_: e.nodeSpan(), Value: lowered, Type: target}
		}
	}
	return l.adaptLiteral(lowered, target)
}

// lowerErrorMember lowers an error member reference to its ordinal constant.
// The explicit "Type.MEMBER" form names the type; the bare ".MEMBER" form is
// resolved against the target type name ("" when unknown).
func (l *Lowerer) lowerErrorMember(n *ErrorMemberExpr, typeName string) HIRExpr {
	tid, typeOK := l.lookupType(typeName)
	ordinals, ok := l.errorOrdinal[tid]
	if !ok {
		return l.poison(n.Span_, "unresolved error type '"+typeName+"'")
	}
	ord, ok := ordinals[n.Name]
	if !ok {
		return l.poison(n.Span_, "unresolved error member '"+n.Name+"'")
	}
	if !typeOK {
		return l.poison(n.Span_, "unresolved error type '"+typeName+"'")
	}
	return &HIRConst{Span_: n.Span_, Type: tid, Kind: ConstError, Int: int64(ord), Str: n.Name}
}

// lowerEnumMember lowers an enum member reference to its integer value as a
// constant of the enum type.
func (l *Lowerer) lowerEnumMember(n *EnumMemberExpr, typeName string) HIRExpr {
	tid, typeOK := l.lookupType(typeName)
	values, ok := l.enumValues[tid]
	if !ok {
		return l.poison(n.Span_, "unresolved enum type '"+typeName+"'")
	}
	value, ok := values[n.Name]
	if !ok {
		return l.poison(n.Span_, "unresolved enum member '"+n.Name+"'")
	}
	if !typeOK {
		return l.poison(n.Span_, "unresolved enum type '"+typeName+"'")
	}
	parsed, _ := new(big.Int).SetString(value, 10)
	return &HIRConst{Span_: n.Span_, Type: tid, Kind: ConstInt, Int: parsed.Int64(), Str: value}
}

func (l *Lowerer) lowerBinary(n *BinaryExpr) HIRExpr {
	leftBare := isBareMemberExpr(n.Left)
	rightBare := isBareMemberExpr(n.Right)
	var left, right HIRExpr
	if !leftBare {
		left = l.lowerExpr(n.Left)
	}
	if !rightBare {
		right = l.lowerExpr(n.Right)
	}
	if leftBare && right != nil {
		left = l.lowerExprAs(n.Left, right.hirType())
	}
	if rightBare && left != nil {
		right = l.lowerExprAs(n.Right, left.hirType())
	}
	// The type checker rejects expressions where both sides lack context.
	if left == nil {
		left = l.lowerExpr(n.Left)
	}
	if right == nil {
		right = l.lowerExpr(n.Right)
	}
	switch n.Op {
	case BinaryOpAdd, BinaryOpSub, BinaryOpMul, BinaryOpDiv, BinaryOpMod:
		left, right = l.adaptLiteralTypes(left, right)
		t := l.inferASTType(n)
		if t == l.types.Unknown() {
			t = left.hirType()
		}
		if t == l.types.Unknown() {
			t = right.hirType()
		}
		left = l.adaptLiteral(left, t)
		right = l.adaptLiteral(right, t)
		return &HIRBinary{Span_: n.Span_, OpSpan: n.OpSpan, Op: n.Op, Left: left, Right: right, Type: t}
	case BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe, BinaryOpEq, BinaryOpNeq:
		left, right = l.adaptLiteralTypes(left, right)
		return &HIRBinary{Span_: n.Span_, OpSpan: n.OpSpan, Op: n.Op, Left: left, Right: right, Type: l.types.Bool()}
	case BinaryOpAnd, BinaryOpOr:
		return &HIRBinary{Span_: n.Span_, OpSpan: n.OpSpan, Op: n.Op, Left: left, Right: right, Type: l.types.Bool()}
	}
	return &HIRBinary{Span_: n.Span_, OpSpan: n.OpSpan, Op: n.Op, Left: left, Right: right, Type: l.types.Unknown()}
}

func isBareMemberExpr(e Expr) bool {
	switch n := e.(type) {
	case *ErrorMemberExpr:
		return n.TypeName == ""
	case *EnumMemberExpr:
		return n.TypeName == ""
	}
	return false
}

func (l *Lowerer) lowerUnary(n *UnaryExpr) HIRExpr {
	operand := l.lowerExpr(n.Operand)
	t := l.inferASTType(n)
	if t == l.types.Unknown() {
		t = operand.hirType()
	}
	if n.Op == UnaryOpNot {
		t = l.types.Bool()
	}
	operand = l.adaptLiteral(operand, t)
	unary := &HIRUnary{Span_: n.Span_, Op: n.Op, Operand: operand, Type: t}
	if _, constant := operand.(*HIRConst); constant {
		// Materialize signed literal values directly. Keeping -128 as a
		// target-typed +128 followed by neg would temporarily violate the
		// fixed-width MIR constant contract even though -128 itself is valid.
		return l.foldConstant(unary)
	}
	return unary
}

// lowerIfx lowers an ifx expression. Both branches are adapted to the
// resolved type of the expression, which comes from the semantic analysis and
// falls back to the branch types.
func (l *Lowerer) lowerIfx(n *IfxExpr) HIRExpr {
	cond := l.lowerExpr(n.Condition)
	t := l.inferASTType(n)
	then := l.lowerExprAs(n.Then, t)
	els := l.lowerExprAs(n.Else, t)
	return &HIRIfx{Span_: n.Span_, Cond: cond, Then: then, Else: els, Type: t}
}

func (l *Lowerer) lowerCall(n *CallExpr) HIRExpr {
	ident, ok := n.Func.(*IdentExpr)
	if !ok {
		return l.poison(n.Span_, "non-identifier callable")
	}
	proc, ok := l.lookupProc(ident.Name)
	if sym := l.lookup(ident.Name); sym != NoSymbol {
		if alias := l.procAliases[sym]; alias != nil {
			proc, ok = alias, true
		} else {
			ok = false
		}
	}
	if !ok {
		return l.poison(n.Span_, "unresolved procedure '"+ident.Name+"'")
	}
	sym := l.procSymbols[proc]
	args := make([]HIRExpr, len(n.Args))
	for i, arg := range n.Args {
		var target TypeID
		if i < len(proc.Params) {
			target = l.typeOfTypeExpr(proc.Params[i].Type)
		}
		args[i] = l.lowerExprAs(arg, target)
	}
	t := l.tupleTypeID(l.procValueTypes(proc))
	if proc.ErrorResult != nil {
		t = l.unionTypeID(t, l.procErrorType(proc))
	}
	return &HIRCall{Span_: n.Span_, Func: sym, Args: args, Type: t}
}

func (l *Lowerer) lowerStructInit(n *StructInitExpr, structType TypeID) HIRExpr {
	st := l.types.Lookup(structType)
	hs, ok := l.hirStructs[structType]
	if !ok {
		return l.poison(n.Span_, "struct literal has unresolved type")
	}
	indexes := make(map[string]int, len(hs.Fields))
	for i, field := range hs.Fields {
		indexes[field.Name] = i
	}
	assigned := make(map[int]HIRExpr, len(n.Fields))
	positional := 0
	for _, field := range n.Fields {
		index := positional
		if field.Name != "" {
			var exists bool
			index, exists = indexes[field.Name]
			if !exists {
				return l.poison(field.Span_, "unknown struct field reached lowering")
			}
		} else {
			positional++
		}
		if index < 0 || index >= len(st.Fields) {
			return l.poison(field.Span_, "excess positional struct field reached lowering")
		}
		if _, duplicate := assigned[index]; duplicate {
			return l.poison(field.Span_, "duplicate struct field reached lowering")
		}
		assigned[index] = l.lowerExprAs(field.Value, st.Fields[index].Type)
	}
	fields := make([]HIRStructInitField, len(st.Fields))
	for i, field := range st.Fields {
		value := assigned[i]
		if value == nil {
			value = l.zeroValue(field.Type, n.Span_)
			if decl := l.structDecls[structType]; decl != nil && decl.Fields[i].Default != nil {
				value = l.lowerExprAs(decl.Fields[i].Default, field.Type)
			}
		}
		fields[i] = HIRStructInitField{Span_: n.Span_, Field: field.Symbol, Value: value}
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
		copy := *n
		if l.literalCompatible(&copy, target) {
			copy.Type = target
		}
		return &copy
	case *HIRUnary:
		copy := *n
		if n.Op == UnaryOpNeg {
			if c, ok := n.Operand.(*HIRConst); ok && l.literalCompatible(c, target) {
				operand := *c
				operand.Type = target
				copy.Operand = &operand
				copy.Type = target
			}
		}
		return &copy
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

// parseIntLiteral parses a decimal integer exactly. HIRConst retains the raw
// text for 128-bit encoding; Int is the low 64-bit two's-complement fragment.
func parseIntLiteral(s string) (int64, bool) {
	v, ok := new(big.Int).SetString(strings.ReplaceAll(s, "_", ""), 10)
	if !ok {
		return 0, false
	}
	return v.Int64(), true
}

// parseFloatLiteral parses a decimal float literal, ignoring underscore
// separators.
func parseFloatLiteral(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64)
	if err != nil || v != v || v > 1.7976931348623157e308 || v < -1.7976931348623157e308 {
		return 0, false
	}
	return v, true
}
