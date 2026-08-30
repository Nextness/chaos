// Type checking for the Chaos language.
//
// The type checker walks a parsed AST and validates types. It infers the type
// of literals and expressions, checks that assignments and comparisons operate
// on matching types, and verifies return statements against the enclosing
// procedure signature. Chaos is a strict typed language: values of different
// types cannot be assigned, compared, or combined.
package compiler

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// Type is a resolved type name in the Chaos type system. Built-in primitive
// types and declared struct names are represented by their name.
type Type string

const (
	TypeString  Type = "String"
	TypeS64     Type = "S64"
	TypeF64     Type = "F64"
	TypeBool    Type = "Bool"
	TypeVoid    Type = "Void"
	TypeAddr    Type = "Addr"
	TypeNull    Type = "null" // the null literal; only valid with nullable pointers
	TypeUnknown Type = ""
)

// Null-state of a pointer binding during flow analysis. 0 is default
// (Unknown) so zero-value Bindings behave conservatively.
const (
	nullStateUnknown uint8 = iota
	nullStateNonNull       // proven non-null by flow (address-of or a null check)
	nullStateNull          // proven null inside a 'p == null' branch
)

// TypeChecker validates the types of a parsed program. It maintains a stack of
// lexical scopes mapping names to their inferred or declared types.
type TypeChecker struct {
	diags                  DiagnosticList
	scopes                 []map[string]Binding
	procs                  map[string]*ProcDecl
	structs                map[string]*StructDecl
	errors                 map[string]*ErrorDecl
	enums                  map[string]*EnumDecl
	declScopes             []declarationScope
	globalPrevious         map[*VarDecl]*VarDecl
	globalTypes            map[*VarDecl]Type
	currentReturnType      Type
	currentReturnTypes     []Type
	currentErrorReturnType Type // error type of a '<>' result ("" when none)
	loopDepth              int  // nesting depth of for loops (for break/continue)
	rangeElemType          Type // element type of the innermost range loop ("" when none)
	rangeIndexType         Type // index type of the innermost range loop ("" when none)
	procValueFloor         int  // outer runtime locals below this scope are not capturable
	procDepth              int
	analysis               *SemanticAnalysis
	nominalTypes           map[Decl]Type
	typeDecls              map[Type]Decl
	procTypes              map[*ProcDecl]Type
	typeProcs              map[Type]*ProcDecl
	nextSemanticID         int
	ifxContext             bool // ifx is valid only as the value of an assignment or return
	typeParamScopes        []map[string]Type
	genericInstances       map[*ProcDecl][]*ProcDecl // generic proc -> instantiated copies
	program                *Program
	structInstances        map[*StructDecl][]*StructDecl // generic struct -> instantiated copies
	instanceTypes          map[*StructDecl]Type          // instantiated struct -> its nominal type
	structLiteralTypes     map[*StructInitExpr]Type      // generic struct literal -> instantiated type
	imports                map[string]*ImportDecl        // namespace name -> resolved import
}

// declarationScope is the lexical compile-time namespace. Procedures and
// nominal types live beside values, but retain their declaration kind so a
// use can be resolved without relying on a mutable spelling-only table.
type declarationScope struct {
	procs   map[string]*ProcDecl
	structs map[string]*StructDecl
	errors  map[string]*ErrorDecl
	enums   map[string]*EnumDecl
}

func newDeclarationScope() declarationScope {
	return declarationScope{
		procs: make(map[string]*ProcDecl), structs: make(map[string]*StructDecl),
		errors: make(map[string]*ErrorDecl), enums: make(map[string]*EnumDecl),
	}
}

// Binding is the semantic state of a value visible in a lexical scope.
// Keeping mutability and definite-initialization beside the type prevents
// later stages from treating compile-time constants or uninitialized locals
// as ordinary mutable values.
type Binding struct {
	Type        Type
	TypeValue   Type // represented type when Type == "Type"
	Mutable     bool
	Initialized bool
	CompileTime bool
	ConstExpr   Expr
	Span        Span
	NullState   uint8 // pointer null-flow state (nullState*); 0 when not narrowed
}

// SemanticAnalysis is the reusable result of name/type analysis. Expression
// types include contextual adaptation (for example an integer literal assigned
// to U8 and a bare enum member resolved by its target type).
type SemanticAnalysis struct {
	ExprTypes          map[Expr]Type
	TypeExprTypes      map[Expr]Type
	DeclTypes          map[Decl]Type
	NominalDecls       map[Type]Decl
	ProcDecls          map[Type]*ProcDecl
	GlobalOrder        []*VarDecl               // dependency order for runtime/constant initialization
	CallInstances      map[*CallExpr]*ProcDecl  // generic call -> instantiated proc
	StructLiteralTypes map[*StructInitExpr]Type // generic struct literal -> instantiated type
	Program            *Program                 // the analyzed program (with prelude and instantiations)
}

// FormatType renders an internal semantic type without exposing the stable
// identity suffixes used to distinguish lexically shadowed nominal types.
func (a *SemanticAnalysis) FormatType(t Type) string {
	return formatSemanticType(t, func(key Type) (string, bool) {
		if a == nil {
			return "", false
		}
		if decl := a.NominalDecls[key]; decl != nil {
			name, _, _ := declarationName(decl)
			return name, true
		}
		if proc := a.ProcDecls[key]; proc != nil {
			return "proc " + proc.Name, true
		}
		return "", false
	})
}

func formatSemanticType(t Type, nominal func(Type) (string, bool)) string {
	if name, ok := nominal(t); ok {
		return name
	}
	s := string(t)
	if strings.HasPrefix(s, "[]") {
		return "[]" + formatSemanticType(Type(strings.TrimPrefix(s, "[]")), nominal)
	}
	if i := strings.Index(s, "<>"); i >= 0 {
		return formatSemanticType(Type(s[:i]), nominal) + " <> " + formatSemanticType(Type(s[i+2:]), nominal)
	}
	// Pointer types: "*" element ("?")?. The '<>' split above runs first so a
	// pointer on the value side of an error union formats correctly.
	if strings.HasPrefix(s, "*") {
		nullable := strings.HasSuffix(s, "?")
		elem := s[1:]
		if nullable {
			elem = elem[:len(elem)-1]
		}
		out := "*" + formatSemanticType(Type(elem), nominal)
		if nullable {
			out += "?"
		}
		return out
	}
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		parts := strings.Split(s[1:len(s)-1], ",")
		for i := range parts {
			parts[i] = formatSemanticType(Type(parts[i]), nominal)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	if strings.HasPrefix(s, "\x00") {
		return "<unresolved semantic type>"
	}
	return s
}

func (tc *TypeChecker) formatType(t Type) string {
	return formatSemanticType(t, func(key Type) (string, bool) {
		if decl := tc.typeDecls[key]; decl != nil {
			name, _, _ := declarationName(decl)
			return name, true
		}
		if proc := tc.typeProcs[key]; proc != nil {
			return "proc " + proc.Name, true
		}
		return "", false
	})
}

// CheckProgram runs the type checker over a parsed program and returns any
// diagnostics it produced.
func CheckProgram(program *Program) DiagnosticList {
	_, diags := AnalyzeProgram(program)
	return diags
}

// AnalyzeProgram type-checks a program and returns facts consumed by lowering
// and editor tooling, so those stages do not establish a competing type truth.
func AnalyzeProgram(program *Program) (*SemanticAnalysis, DiagnosticList) {
	program, importDiags := ResolveImports(program, StdlibDir)
	analysis := &SemanticAnalysis{
		ExprTypes:          make(map[Expr]Type),
		TypeExprTypes:      make(map[Expr]Type),
		DeclTypes:          make(map[Decl]Type),
		NominalDecls:       make(map[Type]Decl),
		ProcDecls:          make(map[Type]*ProcDecl),
		CallInstances:      make(map[*CallExpr]*ProcDecl),
		StructLiteralTypes: make(map[*StructInitExpr]Type),
	}
	tc := &TypeChecker{
		procs:              make(map[string]*ProcDecl),
		structs:            make(map[string]*StructDecl),
		errors:             make(map[string]*ErrorDecl),
		enums:              make(map[string]*EnumDecl),
		globalPrevious:     make(map[*VarDecl]*VarDecl),
		globalTypes:        make(map[*VarDecl]Type),
		nominalTypes:       make(map[Decl]Type),
		typeDecls:          make(map[Type]Decl),
		procTypes:          make(map[*ProcDecl]Type),
		typeProcs:          make(map[Type]*ProcDecl),
		analysis:           analysis,
		genericInstances:   make(map[*ProcDecl][]*ProcDecl),
		structInstances:    make(map[*StructDecl][]*StructDecl),
		instanceTypes:      make(map[*StructDecl]Type),
		structLiteralTypes: make(map[*StructInitExpr]Type),
		imports:            make(map[string]*ImportDecl),
		program:            program,
	}
	if program == nil {
		tc.diags.Error(Span{}, "cannot analyze a nil program", "parse a source program before semantic analysis")
		return analysis, tc.diags
	}
	tc.diags = append(tc.diags, importDiags...)
	tc.checkProgram(program)
	analysis.Program = program
	return analysis, tc.diags
}

func (tc *TypeChecker) pushScope() {
	tc.scopes = append(tc.scopes, map[string]Binding{})
	tc.declScopes = append(tc.declScopes, newDeclarationScope())
}

func (tc *TypeChecker) popScope() {
	tc.scopes = tc.scopes[:len(tc.scopes)-1]
	tc.declScopes = tc.declScopes[:len(tc.declScopes)-1]
}

func (tc *TypeChecker) declare(name string, t Type) {
	tc.declareBinding(name, Binding{Type: t, Mutable: true, Initialized: true})
}

func (tc *TypeChecker) declareBinding(name string, binding Binding) {
	tc.scopes[len(tc.scopes)-1][name] = binding
}

// setBindingNullState marks an existing binding with a null-flow state,
// searching from the innermost scope outward like lookupBinding.
func (tc *TypeChecker) setBindingNullState(name string, state uint8, fromDepth int) {
	for i := len(tc.scopes) - 1; i >= 0 && i >= fromDepth; i-- {
		if binding, ok := tc.scopes[i][name]; ok {
			binding.NullState = state
			tc.scopes[i][name] = binding
			return
		}
	}
}

// resetPointerNullStates clears the narrowed non-null state of every pointer
// binding at or above fromDepth. Used at loop boundaries where the analysis
// cannot prove anything across an arbitrary number of iterations.
func (tc *TypeChecker) resetPointerNullStates(fromDepth int) {
	for i := fromDepth; i < len(tc.scopes); i++ {
		for name, binding := range tc.scopes[i] {
			if binding.NullState != 0 {
				binding.NullState = 0
				tc.scopes[i][name] = binding
			}
		}
	}
}

// lookup returns the type of a name and whether it was declared, searching
// from the innermost scope outward. The boolean distinguishes an undeclared
// name from a declared name whose type is not known yet.
func (tc *TypeChecker) lookup(name string) (Type, bool) {
	binding, _, ok := tc.lookupBinding(name)
	return binding.Type, ok
}

func (tc *TypeChecker) lookupBinding(name string) (Binding, int, bool) {
	for i := len(tc.scopes) - 1; i >= 0; i-- {
		if binding, ok := tc.scopes[i][name]; ok {
			if tc.procDepth > 0 && i > 0 && i < tc.procValueFloor && !binding.CompileTime {
				continue
			}
			return binding, i, true
		}
	}
	return Binding{}, -1, false
}

func (tc *TypeChecker) lookupAnyBinding(name string) (Binding, int, bool) {
	for i := len(tc.scopes) - 1; i >= 0; i-- {
		if binding, ok := tc.scopes[i][name]; ok {
			return binding, i, true
		}
	}
	return Binding{}, -1, false
}

// typeParamType is the abstract type of a generic type parameter within a
// generic declaration body.
func typeParamType(name string) Type { return Type("<" + name + ">") }

// isTypeParam reports whether a type is an abstract generic type parameter.
func isTypeParam(t Type) bool {
	s := string(t)
	return strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">")
}

// typeParamName returns the name of a type parameter type.
func typeParamName(t Type) string { return strings.TrimSuffix(strings.TrimPrefix(string(t), "<"), ">") }

func (tc *TypeChecker) pushTypeParams(params []TypeParam) {
	scope := make(map[string]Type, len(params))
	for _, p := range params {
		scope[p.Name] = typeParamType(p.Name)
	}
	tc.typeParamScopes = append(tc.typeParamScopes, scope)
}

func (tc *TypeChecker) popTypeParams() {
	tc.typeParamScopes = tc.typeParamScopes[:len(tc.typeParamScopes)-1]
}

func (tc *TypeChecker) lookupTypeParam(name string) (Type, bool) {
	for i := len(tc.typeParamScopes) - 1; i >= 0; i-- {
		if t, ok := tc.typeParamScopes[i][name]; ok {
			return t, true
		}
	}
	return TypeUnknown, false
}

func (tc *TypeChecker) lookupProc(name string) (*ProcDecl, bool) {
	for i := len(tc.declScopes) - 1; i >= 0; i-- {
		if d, ok := tc.declScopes[i].procs[name]; ok {
			return d, true
		}
	}
	return nil, false
}

func (tc *TypeChecker) lookupStruct(name string) (*StructDecl, bool) {
	for i := len(tc.declScopes) - 1; i >= 0; i-- {
		if d, ok := tc.declScopes[i].structs[name]; ok {
			return d, true
		}
	}
	return nil, false
}

func (tc *TypeChecker) lookupError(name string) (*ErrorDecl, bool) {
	for i := len(tc.declScopes) - 1; i >= 0; i-- {
		if d, ok := tc.declScopes[i].errors[name]; ok {
			return d, true
		}
	}
	return nil, false
}

func (tc *TypeChecker) lookupEnum(name string) (*EnumDecl, bool) {
	for i := len(tc.declScopes) - 1; i >= 0; i-- {
		if d, ok := tc.declScopes[i].enums[name]; ok {
			return d, true
		}
	}
	return nil, false
}

func (tc *TypeChecker) hasCompileName(name string) bool {
	_, p := tc.lookupProc(name)
	_, s := tc.lookupStruct(name)
	_, e := tc.lookupError(name)
	_, n := tc.lookupEnum(name)
	return p || s || e || n
}

func (tc *TypeChecker) registerProc(d *ProcDecl) {
	tc.declScopes[len(tc.declScopes)-1].procs[d.Name] = d
	if _, exists := tc.procTypes[d]; !exists {
		t := tc.newSemanticType("proc", d.Name)
		tc.procTypes[d] = t
		tc.typeProcs[t] = d
		tc.analysis.ProcDecls[t] = d
		tc.analysis.DeclTypes[d] = t
	}
}
func (tc *TypeChecker) registerStruct(d *StructDecl) {
	tc.declScopes[len(tc.declScopes)-1].structs[d.Name] = d
	tc.registerNominalType(d, "struct", d.Name)
}
func (tc *TypeChecker) registerError(d *ErrorDecl) {
	tc.declScopes[len(tc.declScopes)-1].errors[d.Name] = d
	tc.registerNominalType(d, "error", d.Name)
}
func (tc *TypeChecker) registerEnum(d *EnumDecl) {
	tc.declScopes[len(tc.declScopes)-1].enums[d.Name] = d
	tc.registerNominalType(d, "enum", d.Name)
}

func (tc *TypeChecker) newSemanticType(kind, name string) Type {
	tc.nextSemanticID++
	// NUL cannot occur in a source identifier. It keeps stable semantic
	// identity separate from source spelling without changing the public AST.
	return Type(fmt.Sprintf("\x00%s:%d:%s", kind, tc.nextSemanticID, name))
}

func (tc *TypeChecker) registerNominalType(decl Decl, kind, name string) {
	if _, exists := tc.nominalTypes[decl]; exists {
		return
	}
	t := tc.newSemanticType(kind, name)
	tc.nominalTypes[decl] = t
	tc.typeDecls[t] = decl
	tc.analysis.NominalDecls[t] = decl
	tc.analysis.DeclTypes[decl] = t
}

func (tc *TypeChecker) setBinding(name string, binding Binding) {
	_, scope, ok := tc.lookupBinding(name)
	if ok {
		tc.scopes[scope][name] = binding
	}
}

func (tc *TypeChecker) checkProgram(program *Program) {
	tc.pushScope() // global scope

	// Namespaced imports are checked in their own scope so their members are
	// visible to each other but not to the importing program by bare name.
	// This runs before the program's own declarations are registered so the
	// module's functions never see (or collide with) the importing program's
	// globals. It also runs before the program's procedures so that generic
	// instantiations created while checking the program overwrite the abstract
	// type facts of any nodes shared with the module's generic bodies.
	for _, decl := range program.Decls {
		if d, ok := decl.(*ImportDecl); ok && d.Namespace != "" {
			tc.checkImportModule(d)
		}
	}

	// Top-level declarations are forward visible. Registration is separate
	// from validation so entry and procedure bodies do not depend on file order.
	// Local blocks deliberately use the opposite rule and register declarations
	// only when their statement is reached.
	seenNames := make(map[string]Span)
	for _, decl := range program.Decls {
		name, span, shadow := declarationName(decl)
		if name != "" {
			if isBuiltinTypeName(name) || isReservedInternalName(name) {
				tc.diags.Error(span, "declaration of '"+name+"' collides with a reserved compiler name", "choose a different name")
			} else if _, exists := seenNames[name]; exists && !shadow {
				tc.diags.Error(span, "declaration of '"+name+"' shadows an existing name; use '#shadow' or rename", "add '#shadow' before the declaration or choose a new name")
			}
			seenNames[name] = span
		}
		switch d := decl.(type) {
		case *ProcDecl:
			tc.procs[d.Name] = d
			tc.registerProc(d)
		case *StructDecl:
			tc.structs[d.Name] = d
			tc.registerStruct(d)
		case *ErrorDecl:
			tc.errors[d.Name] = d
			tc.registerError(d)
		case *EnumDecl:
			tc.enums[d.Name] = d
			tc.registerEnum(d)
		case *ImportDecl:
			if d.Namespace != "" {
				tc.imports[d.Namespace] = d
			}
		}
	}

	// Register all global values before validating any declaration defaults.
	// Top-level declarations are forward-visible, including compile-time values
	// referenced by struct defaults. Keep the previous variable declaration for
	// an explicit top-level shadow initializer.
	latestGlobals := make(map[string]*VarDecl)
	for _, decl := range program.Decls {
		d, ok := decl.(*VarDecl)
		if !ok {
			continue
		}
		if previous := latestGlobals[d.Name]; previous != nil {
			tc.globalPrevious[d] = previous
		}
		latestGlobals[d.Name] = d
		declType := TypeUnknown
		if d.DeclType != nil {
			declType = tc.resolveTypeExpr(d.DeclType)
		}
		tc.globalTypes[d] = declType
		tc.declareBinding(d.Name, Binding{Type: declType, Mutable: d.Mutable, Initialized: true, CompileTime: d.CompileTime, ConstExpr: d.Init, Span: d.NameSpan})
	}
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *ErrorDecl:
			tc.checkErrorDecl(d)
		case *EnumDecl:
			tc.checkEnum(d)
		}
	}
	tc.checkStructCycles()

	// Resolve global dependencies before their users. Top-level declarations
	// are forward-visible; only an explicit shadow initializer sees the
	// previous declaration with the same spelling.
	tc.checkGlobals(program, latestGlobals)
	// Procedures see the final global binding regardless of source order.
	for name, decl := range latestGlobals {
		tc.scopes[0][name] = Binding{Type: tc.globalTypes[decl], TypeValue: tc.constTypeValue(decl.Init), Mutable: decl.Mutable, Initialized: true, CompileTime: decl.CompileTime, ConstExpr: decl.Init, Span: decl.NameSpan}
	}
	// Struct defaults can refer to forward top-level compile-time values, so
	// validate them only after global dependency/type resolution is complete.
	for _, decl := range program.Decls {
		if d, ok := decl.(*StructDecl); ok {
			tc.checkStructDecl(d)
		}
	}
	tc.checkEntry(program)
	for _, decl := range program.Decls {
		if d, ok := decl.(*ProcDecl); ok {
			tc.checkProc(d)
		}
	}

	tc.popScope()
}

// checkImportModule registers and type-checks a namespaced import's
// declarations in a dedicated scope. The scope is popped afterwards, so the
// module's members are reachable only through the namespace binding.
func (tc *TypeChecker) checkImportModule(imp *ImportDecl) {
	tc.pushScope()
	for _, d := range imp.Decls {
		switch dd := d.(type) {
		case *ProcDecl:
			tc.registerProc(dd)
		case *StructDecl:
			tc.registerStruct(dd)
		case *ErrorDecl:
			tc.registerError(dd)
		case *EnumDecl:
			tc.registerEnum(dd)
		}
	}
	for _, d := range imp.Decls {
		if s, ok := d.(*StructDecl); ok {
			tc.checkStructDecl(s)
		}
	}
	for _, d := range imp.Decls {
		if p, ok := d.(*ProcDecl); ok {
			tc.checkProc(p)
		}
	}
	tc.popScope()
}

func (tc *TypeChecker) checkGlobals(program *Program, latest map[string]*VarDecl) {
	state := make(map[*VarDecl]uint8)
	path := make([]*VarDecl, 0)
	var visit func(*VarDecl)
	visit = func(decl *VarDecl) {
		if decl == nil || state[decl] == 2 {
			return
		}
		if state[decl] == 1 {
			names := make([]string, 0, len(path)+1)
			for _, item := range path {
				names = append(names, item.Name)
			}
			names = append(names, decl.Name)
			tc.diags.Error(decl.NameSpan, "cyclic global initializer dependency: "+strings.Join(names, " -> "), "break the global initialization cycle")
			return
		}
		state[decl] = 1
		path = append(path, decl)
		identifiers := make(map[string]bool)
		tc.collectInitializerIdentifiers(decl.Init, tc.globalTypes[decl], identifiers, make(map[Type]bool))
		for name := range identifiers {
			dependency := latest[name]
			if name == decl.Name {
				if previous := tc.globalPrevious[decl]; previous != nil {
					dependency = previous
				}
			}
			if dependency != decl {
				visit(dependency)
			}
		}
		path = path[:len(path)-1]

		if previous := tc.globalPrevious[decl]; previous != nil {
			tc.scopes[0][decl.Name] = Binding{Type: tc.globalTypes[previous], TypeValue: tc.constTypeValue(previous.Init), Mutable: previous.Mutable, Initialized: true, CompileTime: previous.CompileTime, ConstExpr: previous.Init, Span: previous.NameSpan}
		} else {
			tc.scopes[0][decl.Name] = Binding{Type: tc.globalTypes[decl], Mutable: decl.Mutable, Initialized: true, CompileTime: decl.CompileTime, ConstExpr: decl.Init, Span: decl.NameSpan}
		}
		t := tc.checkVarDecl(decl)
		tc.globalTypes[decl] = t
		tc.analysis.DeclTypes[decl] = t
		tc.scopes[0][decl.Name] = Binding{Type: t, TypeValue: tc.constTypeValue(decl.Init), Mutable: decl.Mutable, Initialized: true, CompileTime: decl.CompileTime, ConstExpr: decl.Init, Span: decl.NameSpan}
		state[decl] = 2
		tc.analysis.GlobalOrder = append(tc.analysis.GlobalOrder, decl)
	}
	for _, decl := range program.Decls {
		if global, ok := decl.(*VarDecl); ok {
			visit(global)
		}
	}
}

func (tc *TypeChecker) collectInitializerIdentifiers(expr Expr, expected Type, names map[string]bool, visiting map[Type]bool) {
	switch n := expr.(type) {
	case nil:
		return
	case *IdentExpr:
		names[n.Name] = true
	case *ParenExpr:
		tc.collectInitializerIdentifiers(n.Inner, expected, names, visiting)
	case *UnaryExpr:
		tc.collectInitializerIdentifiers(n.Operand, expected, names, visiting)
	case *BinaryExpr:
		tc.collectInitializerIdentifiers(n.Left, TypeUnknown, names, visiting)
		tc.collectInitializerIdentifiers(n.Right, TypeUnknown, names, visiting)
	case *CallExpr:
		tc.collectInitializerIdentifiers(n.Func, TypeUnknown, names, visiting)
		for _, arg := range n.Args {
			tc.collectInitializerIdentifiers(arg, TypeUnknown, names, visiting)
		}
	case *ArrayInitExpr:
		elem := tc.resolveTypeExpr(n.Elem.Elem)
		for _, item := range n.Items {
			tc.collectInitializerIdentifiers(item, elem, names, visiting)
		}
	case *IfxExpr:
		tc.collectInitializerIdentifiers(n.Condition, TypeBool, names, visiting)
		tc.collectInitializerIdentifiers(n.Then, expected, names, visiting)
		tc.collectInitializerIdentifiers(n.Else, expected, names, visiting)
	case *IndexExpr:
		tc.collectInitializerIdentifiers(n.Base, TypeUnknown, names, visiting)
		tc.collectInitializerIdentifiers(n.Index, TypeUnknown, names, visiting)
	case *DerefExpr:
		tc.collectInitializerIdentifiers(n.Operand, TypeUnknown, names, visiting)
	case *FieldAccessExpr:
		tc.collectInitializerIdentifiers(n.Base, TypeUnknown, names, visiting)
	case *StructInitExpr:
		structType := expected
		if n.Type != nil {
			structType = tc.resolveTypeExpr(n.Type)
		}
		decl, ok := tc.structDeclForType(structType)
		if !ok {
			for _, field := range n.Fields {
				tc.collectInitializerIdentifiers(field.Value, TypeUnknown, names, visiting)
			}
			return
		}
		assigned := make(map[int]bool, len(n.Fields))
		fieldIndex := make(map[string]int, len(decl.Fields))
		for i, field := range decl.Fields {
			fieldIndex[field.Name] = i
		}
		positional := 0
		for _, field := range n.Fields {
			index := positional
			if field.Name != "" {
				index = fieldIndex[field.Name]
			} else {
				positional++
			}
			if index >= 0 && index < len(decl.Fields) {
				assigned[index] = true
				tc.collectInitializerIdentifiers(field.Value, tc.resolveTypeExpr(decl.Fields[index].Type), names, visiting)
			}
		}
		if visiting[structType] {
			return
		}
		visiting[structType] = true
		defer delete(visiting, structType)
		for i, field := range decl.Fields {
			if !assigned[i] && field.Default != nil {
				tc.collectInitializerIdentifiers(field.Default, tc.resolveTypeExpr(field.Type), names, visiting)
			}
		}
	}
}

func (tc *TypeChecker) checkProc(p *ProcDecl) {
	previousReturnType := tc.currentReturnType
	previousReturnTypes := tc.currentReturnTypes
	previousErrorType := tc.currentErrorReturnType
	previousFloor, previousDepth := tc.procValueFloor, tc.procDepth
	tc.procValueFloor = len(tc.scopes)
	tc.procDepth++
	tc.pushScope()
	if len(p.TypeParams) > 0 {
		tc.pushTypeParams(p.TypeParams)
		defer tc.popTypeParams()
	}
	seenParams := make(map[string]Span, len(p.Params))
	for _, param := range p.Params {
		tc.checkTypeExprValid(param.Type)
		if _, exists := seenParams[param.Name]; exists {
			tc.diags.Error(param.NameSpan, "duplicate parameter '"+param.Name+"'", "use a unique parameter name")
		}
		seenParams[param.Name] = param.NameSpan
		tc.declareBinding(param.Name, Binding{Type: tc.resolveTypeExpr(param.Type), Mutable: true, Initialized: true, Span: param.NameSpan})
	}
	tc.checkProcResultTypes(p)
	for _, result := range tc.procValueResults(p) {
		if tc.typeContainsArray(result, make(map[Type]bool)) {
			tc.diags.Error(p.NameSpan, "arrays cannot escape through procedure results until owned array storage is implemented", "return a non-array value for now")
		}
	}
	tc.currentReturnType = TypeVoid
	tc.currentReturnTypes = nil
	tc.currentErrorReturnType = ""
	if len(p.Results) > 0 {
		tc.currentReturnTypes = tc.procValueResults(p)
		tc.currentReturnType = tupleType(tc.currentReturnTypes)
	}
	if p.ErrorResult != nil {
		left := tc.resolveTypeExpr(p.Results[0])
		rt := tc.resolveTypeExpr(p.ErrorResult)
		switch {
		case tc.isErrorType(rt):
			if len(p.Results) == 1 && tc.isErrorType(left) {
				tc.diags.Error(p.ErrorResult.nodeSpan(), "a result can carry only one error type", "use one error type after '<>'")
			}
			tc.currentErrorReturnType = rt
		case len(p.Results) == 1 && tc.isErrorType(left):
			tc.currentErrorReturnType = left
		default:
			tc.diags.Error(p.ErrorResult.nodeSpan(), tc.formatType(rt)+" is not an error type", "use a declared error type after '<>'")
			tc.currentErrorReturnType = rt
		}
	}
	if p.Body != nil {
		tc.checkBlock(p.Body)
		if tc.currentReturnType != TypeVoid && !blockDiverges(p.Body) {
			tc.diags.Error(p.Body.Span_, "not every path returns a value from procedure '"+p.Name+"'", "return "+tc.formatTypes(tc.currentReturnTypes)+" on every reachable path")
		}
	}
	tc.popScope()
	tc.currentReturnType = previousReturnType
	tc.currentReturnTypes = previousReturnTypes
	tc.currentErrorReturnType = previousErrorType
	tc.procValueFloor, tc.procDepth = previousFloor, previousDepth
}

// checkProcResultTypes allows Void only when it is the complete value result
// of a procedure. In an error-return signature either side may be the value
// side, so direct Void is allowed on both sides and the existing error-type
// validation determines which side is the error. Nested forms such as
// []Void remain invalid.
func (tc *TypeChecker) checkProcResultTypes(p *ProcDecl) {
	if p.ErrorResult != nil {
		for i, result := range p.Results {
			if i == 0 && len(p.Results) == 1 && tc.isErrorType(tc.resolveTypeExpr(result)) {
				continue
			}
			tc.checkErrorReturnComponent(result)
		}
		if !(len(p.Results) == 1 && tc.isErrorType(tc.resolveTypeExpr(p.Results[0]))) {
			tc.checkErrorReturnComponent(p.ErrorResult)
		} else if ident, ok := p.ErrorResult.(*IdentExpr); !ok || ident.Name != "Void" {
			tc.checkTypeExprValid(p.ErrorResult)
		}
		return
	}
	for _, result := range p.Results {
		if ident, ok := result.(*IdentExpr); ok && ident.Name == "Void" {
			if len(p.Results) != 1 {
				tc.diags.Error(ident.Span_, "Void must be the only function result", "remove the other result types")
			}
			continue
		}
		tc.checkTypeExprValid(result)
	}
}

func (tc *TypeChecker) checkErrorReturnComponent(result Expr) {
	if ident, ok := result.(*IdentExpr); ok && ident.Name == "Void" {
		return
	}
	tc.checkTypeExprValid(result)
}

// procValueResult returns the value result type of a procedure, normalizing
// the '<>' error-return form so the value type is the non-error side.
func (tc *TypeChecker) procValueResult(p *ProcDecl) Type {
	return tupleType(tc.procValueResults(p))
}

func (tc *TypeChecker) procValueResults(p *ProcDecl) []Type {
	if p.ErrorResult != nil && len(p.Results) == 1 {
		left, right := tc.resolveTypeExpr(p.Results[0]), tc.resolveTypeExpr(p.ErrorResult)
		if tc.isErrorType(left) && !tc.isErrorType(right) {
			return []Type{right}
		}
	}
	results := make([]Type, len(p.Results))
	for i, result := range p.Results {
		results[i] = tc.resolveTypeExpr(result)
	}
	return results
}

func (tc *TypeChecker) checkBlock(b *BlockStmt) {
	tc.pushScope()
	for _, stmt := range b.Stmts {
		tc.checkStmt(stmt)
	}
	tc.popScope()
}

func (tc *TypeChecker) checkStmt(s Stmt) {
	switch n := s.(type) {
	case *VarDecl:
		tc.checkShadow(n.Name, n.NameSpan, n.Shadow)
		t := tc.checkVarDecl(n)
		tc.analysis.DeclTypes[n] = t
		initialized := n.Init != nil
		// An uninitialized dynamic array is still initialized: the backend
		// gives it a one-element buffer with capacity 1.
		if !initialized && t != TypeUnknown && arrayKindOf(t) == ArrayDynamic {
			initialized = true
		}
		binding := Binding{Type: t, TypeValue: tc.constTypeValue(n.Init), Mutable: n.Mutable, Initialized: initialized, CompileTime: n.CompileTime, ConstExpr: n.Init, Span: n.NameSpan}
		if n.Init != nil {
			binding.NullState = tc.exprNullState(n.Init)
		}
		tc.declareBinding(n.Name, binding)
	case *MultiVarDecl:
		tc.checkMultiVarDecl(n)
	case *AssignStmt:
		if n.Target != nil {
			tc.checkExprAssign(n.Span_, n.Target, n.Value)
			return
		}
		binding, _, ok := tc.lookupBinding(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "assignment to undeclared variable "+n.Name, "declare the variable before assigning to it")
			return
		}
		if !binding.Mutable {
			tc.diags.Error(n.NameSpan, "cannot assign to immutable binding '"+n.Name+"'", "declare it with ':=' if it must change")
			return
		}
		tc.withIfxContext(n.Value, func() {
			tc.checkAssign(n.Span_, binding.Type, n.Value)
		})
		binding.Initialized = true
		if isPointerType(binding.Type) {
			binding.NullState = tc.exprNullState(n.Value)
		}
		tc.setBinding(n.Name, binding)
	case *ReturnStmt:
		tc.checkReturnStmt(n)
	case *ExitStmt:
		if n.Status != nil {
			st := tc.inferExpr(n.Status)
			if isErrorUnion(st) {
				tc.diags.Error(n.Status.nodeSpan(), "must handle the error before using the value", "use 'unless catch' or 'if ... catch' first")
			} else if st != TypeUnknown && !isIntegerType(st) && !tc.isEnumType(st) && !tc.isErrorType(st) {
				tc.diags.Error(n.Status.nodeSpan(), "exit status must be an integer, enum, or error value, got "+tc.formatType(st), "use an integer-compatible status")
			}
		}
		if n.Message != nil {
			mt := tc.inferExpr(n.Message)
			if mt != TypeUnknown && mt != TypeString {
				tc.diags.Error(n.Message.nodeSpan(), "exit message must be String, got "+tc.formatType(mt), "use a string message")
			}
		}
	case *IfStmt:
		tc.checkIf(n)
	case *BlockStmt:
		tc.checkBlock(n)
	case *ExprStmt:
		tc.inferExpr(n.Expr)
	case *ProcDecl:
		tc.checkShadow(n.Name, n.NameSpan, n.Shadow)
		tc.registerProc(n)
		tc.checkProc(n)
	case *StructDecl:
		tc.checkShadow(n.Name, n.NameSpan, n.Shadow)
		tc.registerStruct(n)
		tc.checkStructDecl(n)
		tc.checkStructCycles()
	case *ErrorDecl:
		tc.checkShadow(n.Name, n.NameSpan, n.Shadow)
		tc.registerError(n)
		tc.checkErrorDecl(n)
	case *EnumDecl:
		tc.checkShadow(n.Name, n.NameSpan, n.Shadow)
		tc.registerEnum(n)
		tc.checkEnum(n)
	case *UnlessCatchStmt:
		tc.checkUnlessCatch(n)
	case *IfCatchStmt:
		tc.checkIfCatch(n)
	case *ForStmt:
		tc.checkFor(n)
	case *BreakStmt:
		if tc.loopDepth == 0 {
			tc.diags.Error(n.Span_, "break outside a loop", "use break inside a for loop")
		}
	case *ContinueStmt:
		if tc.loopDepth == 0 {
			tc.diags.Error(n.Span_, "continue outside a loop", "use continue inside a for loop")
		}
	case *DeallocateStmt:
		at := tc.inferExpr(n.Addr)
		if at != TypeUnknown && at != TypeAddr && !isPointerType(at) {
			tc.diags.Error(n.Addr.nodeSpan(), "#deallocate requires an address, got "+tc.formatType(at), "pass an Addr or pointer value")
		}
	case *CompoundAssignStmt:
		if n.Target != nil {
			tc.checkExprCompound(n.Span_, n.Target, n.Op, n.Value)
			return
		}
		binding, _, ok := tc.lookupBinding(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "assignment to undeclared variable "+n.Name, "declare the variable before assigning to it")
			return
		}
		if !binding.Mutable {
			tc.diags.Error(n.NameSpan, "cannot modify immutable binding '"+n.Name+"'", "declare it with ':=' if it must change")
			return
		}
		if !binding.Initialized {
			tc.diags.Error(n.NameSpan, "binding '"+n.Name+"' is not initialized", "assign it before using a compound assignment")
			return
		}
		if tc.isEnumType(binding.Type) {
			tc.diags.Error(n.Span_, "cannot apply "+n.Op.String()+" to enum values", "enum values support only == and !=")
			return
		}
		tc.checkCompoundOp(n.Span_, binding.Type, n.Op, n.Value)
		if isPointerType(binding.Type) {
			binding.NullState = 0
			tc.setBinding(n.Name, binding)
		}
	case *IncDecStmt:
		if n.Target != nil {
			tc.checkExprIncDec(n.Span_, n.Target, n.Op)
			return
		}
		binding, _, ok := tc.lookupBinding(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "cannot modify undeclared variable "+n.Name, "declare the variable before using it")
			return
		}
		if !binding.Mutable {
			tc.diags.Error(n.NameSpan, "cannot modify immutable binding '"+n.Name+"'", "declare it with ':=' if it must change")
			return
		}
		if !binding.Initialized {
			tc.diags.Error(n.NameSpan, "binding '"+n.Name+"' is not initialized", "assign it before incrementing or decrementing it")
			return
		}
		if !isIntegerType(binding.Type) && !isPointerType(binding.Type) {
			tc.diags.Error(n.Span_, "cannot increment or decrement a value of type "+tc.formatType(binding.Type), "use an integer or pointer variable")
		}
		if isPointerType(binding.Type) {
			binding.NullState = 0
			tc.setBinding(n.Name, binding)
		}
	}
}

// checkUnlessCatch checks "target := expr unless catch [err] { body }" (or
// the bare form with Target == ""). The expression must be an error-returning
// value; the catch body must return or exit, and after the statement the
// target holds the unwrapped value.
func (tc *TypeChecker) checkUnlessCatch(n *UnlessCatchStmt) {
	initType := tc.inferExpr(n.Init)
	if !isErrorUnion(initType) {
		tc.diags.Error(n.Init.nodeSpan(), "unless catch requires an error-returning expression, got "+tc.formatType(initType), "use an expression that can return an error")
		return
	}
	valueTypes := tupleTypes(unionValueType(initType))
	targets, spans := n.Targets, n.TargetSpans
	if len(targets) == 0 && n.Target != "" {
		targets, spans = []string{n.Target}, []Span{n.TargetSpan}
	}
	if len(targets) > 0 && len(valueTypes) == 1 && valueTypes[0] == TypeVoid {
		tc.diags.Error(n.Span_, "cannot bind a Void value; use the bare 'unless catch' form", "remove the target name")
		return
	}
	if len(targets) != 0 && len(targets) != len(valueTypes) {
		tc.diags.Error(n.Span_, fmt.Sprintf("error-returning expression produces %d values, but %d bindings were provided", len(valueTypes), len(targets)), "bind every returned value")
		return
	}
	for i, target := range targets {
		tc.checkShadow(target, spans[i], n.Shadow)
	}
	tc.checkCatchBody(n.CatchBody, unionErrorType(initType), n.CatchName, n.CatchNameSpan)
	for i, target := range targets {
		tc.declareBinding(target, Binding{Type: valueTypes[i], Mutable: true, Initialized: true, Span: spans[i]})
	}
}

func (tc *TypeChecker) checkMultiVarDecl(n *MultiVarDecl) {
	t := tc.inferExpr(n.Init)
	if isErrorUnion(t) {
		tc.diags.Error(n.Init.nodeSpan(), "must handle the error before binding returned values", "add 'unless catch'")
		return
	}
	values := tupleTypes(t)
	if len(values) <= 1 {
		tc.diags.Error(n.Init.nodeSpan(), "multiple bindings require an expression returning multiple values", "bind this expression to one name")
		return
	}
	if len(values) != len(n.Names) {
		tc.diags.Error(n.Span_, fmt.Sprintf("expression returns %d values, but %d bindings were provided", len(values), len(n.Names)), "bind every returned value")
		return
	}
	if n.CompileTime && !tc.isConstExpr(n.Init, make(map[string]bool)) {
		tc.diags.Error(n.Init.nodeSpan(), "compile-time assignment requires a compile-time-known expression", "use ':=' or a constant expression")
	}
	seen := make(map[string]bool, len(n.Names))
	for i, name := range n.Names {
		if seen[name] {
			tc.diags.Error(n.NameSpans[i], "duplicate binding '"+name+"'", "use a unique binding name")
		}
		seen[name] = true
		tc.checkShadow(name, n.NameSpans[i], n.Shadow)
		tc.declareBinding(name, Binding{Type: values[i], Mutable: n.Mutable, Initialized: true, CompileTime: n.CompileTime, Span: n.NameSpans[i]})
	}
}

func cloneBindingScopes(scopes []map[string]Binding) []map[string]Binding {
	copyScopes := make([]map[string]Binding, len(scopes))
	for i, scope := range scopes {
		copyScopes[i] = make(map[string]Binding, len(scope))
		for name, binding := range scope {
			copyScopes[i][name] = binding
		}
	}
	return copyScopes
}

// nullConditionVar decodes a "var == null" / "var != null" test on a nullable
// pointer binding, returning the variable, its state inside the then-branch,
// its state on the else path, and whether the condition narrows at all.
func (tc *TypeChecker) nullConditionVar(cond Expr) (string, uint8, uint8, bool) {
	bin, ok := unwrapParens(cond).(*BinaryExpr)
	if !ok {
		return "", 0, 0, false
	}
	var side Expr
	switch {
	case bin.Op == BinaryOpEq && isNullLit(bin.Left):
		side = bin.Right
	case bin.Op == BinaryOpEq && isNullLit(bin.Right):
		side = bin.Left
	case bin.Op == BinaryOpNeq && isNullLit(bin.Left):
		side = bin.Right
	case bin.Op == BinaryOpNeq && isNullLit(bin.Right):
		side = bin.Left
	default:
		return "", 0, 0, false
	}
	ident, ok := unwrapParens(side).(*IdentExpr)
	if !ok {
		return "", 0, 0, false
	}
	b, _, ok := tc.lookupBinding(ident.Name)
	if !ok || !isPointerType(b.Type) || !pointerNullable(b.Type) {
		return "", 0, 0, false
	}
	if bin.Op == BinaryOpEq {
		return ident.Name, nullStateNull, nullStateNonNull, true
	}
	return ident.Name, nullStateNonNull, nullStateNull, true
}

func (tc *TypeChecker) checkIf(n *IfStmt) {
	condType := tc.inferExpr(n.Condition)
	if condType != TypeUnknown && condType != TypeBool {
		tc.diags.Error(n.Condition.nodeSpan(), "if condition must be Bool, got "+tc.formatType(condType), "use a boolean condition")
	}
	base := cloneBindingScopes(tc.scopes)
	var branches [][]map[string]Binding
	var diverges []bool

	name, thenState, elseState, narrows := tc.nullConditionVar(n.Condition)
	tc.scopes = cloneBindingScopes(base)
	if narrows {
		tc.setBindingNullState(name, thenState, 0)
	}
	tc.checkBlock(n.Body)
	branches = append(branches, cloneBindingScopes(tc.scopes))
	diverges = append(diverges, blockDiverges(n.Body))

	lastName, lastElseState, lastNarrows := name, elseState, narrows
	for _, elif := range n.Elif {
		et := tc.inferExpr(elif.Condition)
		if et != TypeUnknown && et != TypeBool {
			tc.diags.Error(elif.Condition.nodeSpan(), "elif condition must be Bool, got "+tc.formatType(et), "use a boolean condition")
		}
		tc.scopes = cloneBindingScopes(base)
		ename, ethen, eelse, eok := tc.nullConditionVar(elif.Condition)
		if eok {
			tc.setBindingNullState(ename, ethen, 0)
		}
		tc.checkBlock(elif.Body)
		branches = append(branches, cloneBindingScopes(tc.scopes))
		diverges = append(diverges, blockDiverges(elif.Body))
		lastName, lastElseState, lastNarrows = ename, eelse, eok
		tc.scopes = cloneBindingScopes(base)
	}

	if n.ElseBody != nil {
		tc.scopes = cloneBindingScopes(base)
		if lastNarrows {
			tc.setBindingNullState(lastName, lastElseState, 0)
		}
		tc.checkBlock(n.ElseBody)
		branches = append(branches, cloneBindingScopes(tc.scopes))
		diverges = append(diverges, blockDiverges(n.ElseBody))
	} else {
		// No else: the implicit else path is the inverted narrowing of the
		// last condition (the condition did not hold).
		els := cloneBindingScopes(base)
		if lastNarrows {
			saved := tc.scopes
			tc.scopes = els
			tc.setBindingNullState(lastName, lastElseState, 0)
			els = cloneBindingScopes(tc.scopes)
			tc.scopes = saved
		}
		branches = append(branches, els)
		diverges = append(diverges, false)
	}

	tc.scopes = cloneBindingScopes(base)
	for scopeIndex, scope := range tc.scopes {
		for name, original := range scope {
			joined := original
			joined.Initialized = true
			var joinedType Type
			typesAgree := true
			allNonNull := true
			anyContributes := false
			for i, branch := range branches {
				if diverges[i] {
					continue // a diverging branch never reaches the join
				}
				anyContributes = true
				candidate := branch[scopeIndex][name]
				joined.Initialized = joined.Initialized && candidate.Initialized
				if !isPointerType(candidate.Type) || candidate.NullState != nullStateNonNull {
					allNonNull = false
				}
				if joinedType == TypeUnknown {
					joinedType = candidate.Type
				} else if candidate.Type != joinedType {
					typesAgree = false
				}
			}
			if typesAgree {
				joined.Type = joinedType
			} else {
				joined.Type = original.Type
			}
			if isPointerType(joined.Type) {
				if anyContributes && allNonNull {
					joined.NullState = nullStateNonNull
				} else {
					joined.NullState = 0
				}
			}
			tc.scopes[scopeIndex][name] = joined
		}
	}
}

// checkIfCatch checks "if expr catch [err] { body }". The condition must be a
// variable holding an error-returning value; the catch body must return or
// exit, and after the statement the variable holds the unwrapped value.
func (tc *TypeChecker) checkIfCatch(n *IfCatchStmt) {
	condType := tc.inferExpr(n.Cond)
	if !isErrorUnion(condType) {
		tc.diags.Error(n.Cond.nodeSpan(), "catch requires an error-returning value, got "+tc.formatType(condType), "use a value that can return an error")
		return
	}
	ident, ok := n.Cond.(*IdentExpr)
	if !ok {
		tc.diags.Error(n.Cond.nodeSpan(), "catch requires a variable holding the error-returning value", "bind the value to a variable first")
		return
	}
	tc.checkCatchBody(n.CatchBody, unionErrorType(condType), n.CatchName, n.CatchNameSpan)
	tc.updateType(ident.Name, unionValueType(condType))
}

// checkFor checks a for loop. The single-expression form is resolved by type:
// a Bool expression is a while loop, an array expression is an implicit range
// loop. The explicit range form ("for [idx,] elem: arr") declares its bindings
// in the body scope.
func (tc *TypeChecker) checkFor(n *ForStmt) {
	tc.loopDepth++
	if n.Init != nil {
		tc.checkStmt(n.Init)
	}
	// The initializer executes exactly once. The condition, body, and after
	// clause may execute zero times, so their assignments cannot establish
	// definite initialization after the loop.
	postInit := cloneBindingScopes(tc.scopes)
	// Null analysis cannot prove anything across an arbitrary number of
	// iterations: narrowed states inherited from before the loop are dropped
	// before the body is analyzed.
	tc.resetPointerNullStates(0)
	if n.Range != nil {
		rt := tc.inferExpr(n.Range)
		if !isArrayType(rt) {
			tc.diags.Error(n.Range.nodeSpan(), "for range requires an array, got "+tc.formatType(rt), "use an array value")
		}
		elemType := arrayElemType(rt)
		prevElem, prevIndex := tc.rangeElemType, tc.rangeIndexType
		tc.rangeElemType = elemType
		tc.rangeIndexType = TypeS64
		tc.pushScope()
		if n.IndexName != "" {
			tc.checkShadow(n.IndexName, n.IndexNameSpan, false)
			tc.declare(n.IndexName, TypeS64)
		}
		if n.ElemName != "" {
			tc.checkShadow(n.ElemName, n.ElemNameSpan, false)
			tc.declare(n.ElemName, elemType)
		}
		tc.checkBlock(n.Body)
		tc.popScope()
		tc.rangeElemType, tc.rangeIndexType = prevElem, prevIndex
	} else if n.Cond != nil {
		ct := tc.inferExpr(n.Cond)
		if isArrayType(ct) {
			// Implicit range loop: '#this' and '#index' refer to the element
			// and index.
			elemType := arrayElemType(ct)
			prevElem, prevIndex := tc.rangeElemType, tc.rangeIndexType
			tc.rangeElemType = elemType
			tc.rangeIndexType = TypeS64
			tc.checkBlock(n.Body)
			tc.rangeElemType, tc.rangeIndexType = prevElem, prevIndex
		} else {
			if ct != TypeUnknown && ct != TypeBool {
				tc.diags.Error(n.Cond.nodeSpan(), "for condition must be Bool, got "+tc.formatType(ct), "use a boolean condition")
			}
			// The exit condition holds at every entry to the body, so a
			// "p != null" while-condition narrows inside the body.
			if name, thenState, _, ok := tc.nullConditionVar(n.Cond); ok {
				tc.setBindingNullState(name, thenState, 0)
			}
			tc.checkBlock(n.Body)
		}
	}
	if n.After != nil {
		tc.checkStmt(n.After)
	}
	// Post-loop null state: the loop may execute zero times, so a binding is
	// only provably non-null afterwards if it was provably non-null both
	// before the loop and at the exit of the body.
	for scopeIndex, saved := range postInit {
		for name, pre := range saved {
			cur, _, ok := tc.lookupAnyBinding(name)
			if !ok || !isPointerType(pre.Type) || !isPointerType(cur.Type) {
				continue
			}
			if pre.NullState != nullStateNonNull || cur.NullState != nullStateNonNull {
				joined := postInit[scopeIndex][name]
				joined.NullState = 0
				postInit[scopeIndex][name] = joined
			}
		}
	}
	tc.scopes = postInit
	tc.loopDepth--
}

// checkCatchBody checks a catch body: it must return or exit (so the value is
// always defined afterward), and the optional error binding is declared in the
// body's scope.
func (tc *TypeChecker) checkCatchBody(body *BlockStmt, errorType Type, catchName string, catchNameSpan Span) {
	if !blockDiverges(body) {
		tc.diags.Error(body.Span_, "the catch block must diverge", "return, exit, or remain in an infinite loop on every catch path")
	}
	tc.pushScope()
	if catchName != "" {
		tc.checkShadow(catchName, catchNameSpan, false)
		tc.declare(catchName, errorType)
	}
	tc.checkBlock(body)
	tc.popScope()
}

// checkShadow enforces Chaos' single lexical declaration namespace. Reusing a
// visible value, procedure, struct, error, or enum requires '#shadow'; built-in
// and compiler-reserved names cannot be redeclared at all.
func (tc *TypeChecker) checkShadow(name string, span Span, shadow bool) {
	if isBuiltinTypeName(name) || isReservedInternalName(name) {
		tc.diags.Error(span, "declaration of '"+name+"' collides with a reserved compiler name", "choose a different name")
		return
	}
	_, _, valueExists := tc.lookupAnyBinding(name)
	if (valueExists || tc.hasCompileName(name)) && !shadow {
		tc.diags.Error(span, "declaration of '"+name+"' shadows an existing name; use '#shadow' or rename", "add '#shadow' before the declaration or choose a new name")
	}
}

func (tc *TypeChecker) reportTypeShadow(name, kind string, span Span) {
	article := "a"
	if kind == "error" {
		article = "an"
	}
	tc.diags.Error(span, "declaration of '"+name+"' shadows "+article+" "+kind+" type; this is not allowed", "choose a different name")
}

// updateType changes the type of a declared name in the scope where it is
// declared, used for the flow-sensitive unwrap after an error check.
func (tc *TypeChecker) updateType(name string, t Type) {
	for i := len(tc.scopes) - 1; i >= 0; i-- {
		if binding, ok := tc.scopes[i][name]; ok {
			binding.Type = t
			binding.Initialized = true
			tc.scopes[i][name] = binding
			return
		}
	}
}

// blockDiverges reports whether every path reaching some statement in the
// block returns, exits, or remains in an infinite loop. Statements following
// that point are unreachable and cannot make the block fall through again.
func blockDiverges(b *BlockStmt) bool {
	if b == nil {
		return false
	}
	for _, stmt := range b.Stmts {
		if stmtDiverges(stmt) {
			return true
		}
	}
	return false
}

// stmtDiverges reports whether a statement always returns, exits, or loops
// forever. Only a syntactically constant-true loop can establish the latter;
// nontrivial condition reasoning belongs to future optimization passes.
func stmtDiverges(s Stmt) bool {
	switch n := s.(type) {
	case *ReturnStmt, *ExitStmt:
		return true
	case *BlockStmt:
		return blockDiverges(n)
	case *IfStmt:
		if !blockDiverges(n.Body) {
			return false
		}
		for _, elif := range n.Elif {
			if !blockDiverges(elif.Body) {
				return false
			}
		}
		if n.ElseBody == nil {
			return false
		}
		return blockDiverges(n.ElseBody)
	case *ForStmt:
		return n.Range == nil && exprIsTrue(n.Cond) && !blockCanBreakCurrentLoop(n.Body)
	}
	return false
}

func exprIsTrue(expr Expr) bool {
	switch n := expr.(type) {
	case *BoolExpr:
		return n.Value
	case *ParenExpr:
		return exprIsTrue(n.Inner)
	}
	return false
}

// blockCanBreakCurrentLoop finds a reachable break for the loop whose body is
// being inspected. Breaks inside nested loops belong to those loops and do not
// make the outer constant-true loop terminate.
func blockCanBreakCurrentLoop(block *BlockStmt) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.Stmts {
		if stmtCanBreakCurrentLoop(stmt) {
			return true
		}
		if stmtDiverges(stmt) {
			return false
		}
	}
	return false
}

func stmtCanBreakCurrentLoop(stmt Stmt) bool {
	switch n := stmt.(type) {
	case *BreakStmt:
		return true
	case *BlockStmt:
		return blockCanBreakCurrentLoop(n)
	case *IfStmt:
		if blockCanBreakCurrentLoop(n.Body) {
			return true
		}
		for _, branch := range n.Elif {
			if blockCanBreakCurrentLoop(branch.Body) {
				return true
			}
		}
		return blockCanBreakCurrentLoop(n.ElseBody)
	case *IfCatchStmt:
		return blockCanBreakCurrentLoop(n.CatchBody)
	case *UnlessCatchStmt:
		return blockCanBreakCurrentLoop(n.CatchBody)
	case *ForStmt, *ProcDecl:
		return false
	}
	return false
}

func (tc *TypeChecker) checkEntry(program *Program) {
	if program.EntryDecl == nil {
		return
	}
	p := program.EntryDecl
	// The entry procedure may take no arguments or exactly the process
	// argument vector: (argc: S64, argv: *String).
	if len(p.Params) != 0 {
		if len(p.Params) != 2 {
			tc.diags.Error(p.NameSpan, "entry procedure must take no arguments or exactly (argc: S64, argv: *String)", "declare '#entry "+p.Name+" :: proc -> S64' or '#entry "+p.Name+" :: proc (argc: S64, argv: *String) -> S64'")
		} else {
			argcType := tc.resolveTypeExpr(p.Params[0].Type)
			argvType := tc.resolveTypeExpr(p.Params[1].Type)
			if argcType != TypeS64 {
				tc.diags.Error(p.Params[0].NameSpan, "entry argc parameter must be S64", "declare the first parameter as 'argc: S64'")
			}
			if argvType != pointerType(TypeString, false) {
				tc.diags.Error(p.Params[1].NameSpan, "entry argv parameter must be *String", "declare the second parameter as 'argv: *String'")
			}
		}
	}
	if len(p.Results) != 1 || tc.resolveTypeExpr(p.Results[0]) != TypeS64 || p.ErrorResult != nil {
		tc.diags.Error(p.NameSpan, "entry procedure must return exactly S64", "declare '#entry "+p.Name+" :: proc -> S64'")
	}
}

func (tc *TypeChecker) checkStructDecl(d *StructDecl) {
	if len(d.TypeParams) > 0 {
		tc.pushTypeParams(d.TypeParams)
		defer tc.popTypeParams()
	}
	seen := make(map[string]bool, len(d.Fields))
	for _, field := range d.Fields {
		if seen[field.Name] {
			tc.diags.Error(field.NameSpan, "duplicate field '"+field.Name+"' in struct "+d.Name, "use a unique field name")
		}
		seen[field.Name] = true
		tc.checkTypeExprValid(field.Type)
		if field.Default != nil {
			fieldType := tc.resolveTypeExpr(field.Type)
			tc.checkAssign(field.Default.nodeSpan(), fieldType, field.Default)
			if !tc.isConstExpr(field.Default, make(map[string]bool)) {
				tc.diags.Error(field.Default.nodeSpan(), "struct field default must be known at compile time", "use a constant expression")
			} else {
				tc.validateCompileTimeExpr(field.Default, fieldType)
			}
		}
	}
}

func (tc *TypeChecker) checkErrorDecl(d *ErrorDecl) {
	seen := make(map[string]bool, len(d.Members))
	for _, member := range d.Members {
		if seen[member.Name] {
			tc.diags.Error(member.NameSpan, "duplicate error member "+member.Name+" in "+d.Name, "use a unique member name")
		}
		seen[member.Name] = true
	}
}

func (tc *TypeChecker) checkStructCycles() {
	visible := make(map[string]*StructDecl)
	for _, scope := range tc.declScopes {
		for name, decl := range scope.structs {
			visible[name] = decl
		}
	}
	state := make(map[string]uint8)
	var visit func(string, []string)
	visit = func(name string, path []string) {
		if state[name] == 2 {
			return
		}
		if state[name] == 1 {
			decl := visible[name]
			if decl != nil {
				tc.diags.Error(decl.NameSpan, "struct '"+name+"' has infinite size through "+strings.Join(append(path, name), " -> "), "break the cycle with an indirection")
			}
			return
		}
		decl := visible[name]
		if decl == nil {
			return
		}
		state[name] = 1
		for _, field := range decl.Fields {
			// Arrays are pointer/length values and therefore break by-value
			// layout cycles. A nullable pointer field also breaks the cycle:
			// it is the only way a struct may reference itself.
			if _, array := field.Type.(*ArrayTypeExpr); array {
				continue
			}
			if ptr, ok := field.Type.(*PointerTypeExpr); ok {
				if ptr.Nullable {
					continue
				}
				if id, ok := ptr.Elem.(*IdentExpr); ok {
					visit(id.Name, append(path, name))
				}
				continue
			}
			if id, ok := field.Type.(*IdentExpr); ok {
				visit(id.Name, append(path, name))
			}
		}
		state[name] = 2
	}
	for name := range visible {
		visit(name, nil)
	}
}

func (tc *TypeChecker) isConstExpr(expr Expr, visiting map[string]bool) bool {
	switch n := expr.(type) {
	case *IntExpr, *FloatExpr, *StringExpr, *BoolExpr, *EnumMemberExpr, *ErrorMemberExpr:
		return true
	case *ParenExpr:
		return tc.isConstExpr(n.Inner, visiting)
	case *UnaryExpr:
		return tc.isConstExpr(n.Operand, visiting)
	case *BinaryExpr:
		return tc.isConstExpr(n.Left, visiting) && tc.isConstExpr(n.Right, visiting)
	case *ArrayInitExpr:
		for _, item := range n.Items {
			if !tc.isConstExpr(item, visiting) {
				return false
			}
		}
		return true
	case *IfxExpr:
		return tc.isConstExpr(n.Condition, visiting) && tc.isConstExpr(n.Then, visiting) && tc.isConstExpr(n.Else, visiting)
	case *StructInitExpr:
		for _, field := range n.Fields {
			if !tc.isConstExpr(field.Value, visiting) {
				return false
			}
		}
		return true
	case *IdentExpr:
		if isBuiltinTypeName(n.Name) || tc.hasCompileName(n.Name) {
			return true
		}
		binding, _, ok := tc.lookupBinding(n.Name)
		if !ok || !binding.CompileTime || binding.ConstExpr == nil || visiting[n.Name] {
			return false
		}
		visiting[n.Name] = true
		known := tc.isConstExpr(binding.ConstExpr, visiting)
		delete(visiting, n.Name)
		return known
	case *CallExpr:
		// Executing ordinary procedures during constant evaluation is a separate
		// language mechanism; calls are intentionally rejected for now.
		return false
	}
	return false
}

func (tc *TypeChecker) constTypeValue(expr Expr) Type {
	switch n := expr.(type) {
	case *ParenExpr:
		return tc.constTypeValue(n.Inner)
	case *IdentExpr:
		if isBuiltinTypeName(n.Name) {
			return Type(n.Name)
		}
		if d, ok := tc.lookupStruct(n.Name); ok {
			return tc.nominalTypes[d]
		}
		if d, ok := tc.lookupError(n.Name); ok {
			return tc.nominalTypes[d]
		}
		if d, ok := tc.lookupEnum(n.Name); ok {
			return tc.nominalTypes[d]
		}
		if binding, _, ok := tc.lookupBinding(n.Name); ok && binding.CompileTime && binding.Type == "Type" {
			return binding.TypeValue
		}
	}
	return TypeUnknown
}

type constIntFault struct {
	span    Span
	message string
	value   *big.Int
}

type constFloatFault struct {
	span    Span
	message string
}

// evalConstInt evaluates the integer-only subset of compile-time expressions
// with arbitrary precision. target is applied to every intermediate result,
// so typed constant arithmetic cannot hide an overflow that would have
// occurred before the final operation.
func (tc *TypeChecker) evalConstInt(expr Expr, target Type, visiting map[Expr]bool) (*big.Int, bool, *constIntFault) {
	if expr == nil || visiting[expr] {
		return nil, false, nil
	}
	check := func(value *big.Int, span Span) (*big.Int, bool, *constIntFault) {
		if isIntegerType(target) && !fitsIntegerType(target, value) {
			return value, true, &constIntFault{span: span, message: "overflow", value: new(big.Int).Set(value)}
		}
		return value, true, nil
	}
	switch n := expr.(type) {
	case *IntExpr:
		value, ok := new(big.Int).SetString(strings.ReplaceAll(n.Value, "_", ""), 10)
		if !ok {
			return nil, false, nil
		}
		return check(value, n.Span_)
	case *ParenExpr:
		return tc.evalConstInt(n.Inner, target, visiting)
	case *UnaryExpr:
		if n.Op != UnaryOpNeg {
			return nil, false, nil
		}
		// Parse the magnitude without applying the signed target first: -128 is
		// valid for S8 even though its positive magnitude is not.
		value, ok, fault := tc.evalConstInt(n.Operand, TypeUnknown, visiting)
		if !ok || fault != nil {
			return value, ok, fault
		}
		return check(new(big.Int).Neg(value), n.Span_)
	case *BinaryExpr:
		if n.Op < BinaryOpAdd || n.Op > BinaryOpMod {
			return nil, false, nil
		}
		left, leftOK, fault := tc.evalConstInt(n.Left, target, visiting)
		if !leftOK || fault != nil {
			return left, leftOK, fault
		}
		right, rightOK, fault := tc.evalConstInt(n.Right, target, visiting)
		if !rightOK || fault != nil {
			return right, rightOK, fault
		}
		value := new(big.Int)
		switch n.Op {
		case BinaryOpAdd:
			value.Add(left, right)
		case BinaryOpSub:
			value.Sub(left, right)
		case BinaryOpMul:
			value.Mul(left, right)
		case BinaryOpDiv:
			if right.Sign() == 0 {
				return nil, true, &constIntFault{span: n.OpSpan, message: "division by zero"}
			}
			value.Quo(left, right)
		case BinaryOpMod:
			if right.Sign() == 0 {
				return nil, true, &constIntFault{span: n.OpSpan, message: "modulo by zero"}
			}
			value.Rem(left, right)
		}
		return check(value, n.Span_)
	case *IdentExpr:
		binding, _, ok := tc.lookupBinding(n.Name)
		if !ok || !binding.CompileTime || binding.ConstExpr == nil {
			return nil, false, nil
		}
		visiting[expr] = true
		value, known, fault := tc.evalConstInt(binding.ConstExpr, target, visiting)
		delete(visiting, expr)
		return value, known, fault
	case *IfxExpr:
		cond, known := tc.evalConstBool(n.Condition, visiting)
		if !known {
			return nil, false, nil
		}
		if cond {
			return tc.evalConstInt(n.Then, target, visiting)
		}
		return tc.evalConstInt(n.Else, target, visiting)
	}
	return nil, false, nil
}

func (tc *TypeChecker) reportConstIntFault(fault *constIntFault, target Type, literal bool) {
	if fault == nil {
		return
	}
	switch fault.message {
	case "overflow":
		kind := "compile-time integer value"
		if literal {
			kind = "integer literal"
		}
		tc.diags.Error(fault.span, kind+" "+fault.value.String()+" does not fit "+tc.formatType(target), "use a value within the range of "+tc.formatType(target))
	case "division by zero", "modulo by zero":
		tc.diags.Error(fault.span, fault.message+" in compile-time expression", "use a non-zero divisor")
	}
}

func (tc *TypeChecker) markConstIntType(expr Expr, target Type, visiting map[Expr]bool) {
	if expr == nil || visiting[expr] {
		return
	}
	visiting[expr] = true
	tc.analysis.ExprTypes[expr] = target
	switch n := expr.(type) {
	case *ParenExpr:
		tc.markConstIntType(n.Inner, target, visiting)
	case *UnaryExpr:
		tc.markConstIntType(n.Operand, target, visiting)
	case *BinaryExpr:
		tc.markConstIntType(n.Left, target, visiting)
		tc.markConstIntType(n.Right, target, visiting)
	case *IfxExpr:
		tc.markConstIntType(n.Then, target, visiting)
		tc.markConstIntType(n.Else, target, visiting)
	}
}

func (tc *TypeChecker) evalConstFloat(expr Expr, target Type, visiting map[Expr]bool) (float64, bool, *constFloatFault) {
	if expr == nil || visiting[expr] {
		return 0, false, nil
	}
	check := func(value float64, span Span) (float64, bool, *constFloatFault) {
		if target == "F32" {
			value = float64(float32(value))
		}
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return value, true, &constFloatFault{span: span, message: "floating-point overflow"}
		}
		return value, true, nil
	}
	switch n := expr.(type) {
	case *FloatExpr:
		value, err := strconv.ParseFloat(strings.ReplaceAll(n.Value, "_", ""), 64)
		if err != nil {
			return 0, true, &constFloatFault{span: n.Span_, message: "floating-point overflow"}
		}
		return check(value, n.Span_)
	case *ParenExpr:
		return tc.evalConstFloat(n.Inner, target, visiting)
	case *UnaryExpr:
		if n.Op != UnaryOpNeg {
			return 0, false, nil
		}
		value, known, fault := tc.evalConstFloat(n.Operand, target, visiting)
		if !known || fault != nil {
			return value, known, fault
		}
		return check(-value, n.Span_)
	case *BinaryExpr:
		if n.Op < BinaryOpAdd || n.Op > BinaryOpDiv {
			return 0, false, nil
		}
		left, leftOK, fault := tc.evalConstFloat(n.Left, target, visiting)
		if !leftOK || fault != nil {
			return left, leftOK, fault
		}
		right, rightOK, fault := tc.evalConstFloat(n.Right, target, visiting)
		if !rightOK || fault != nil {
			return right, rightOK, fault
		}
		var value float64
		switch n.Op {
		case BinaryOpAdd:
			value = left + right
		case BinaryOpSub:
			value = left - right
		case BinaryOpMul:
			value = left * right
		case BinaryOpDiv:
			if right == 0 {
				return 0, true, &constFloatFault{span: n.OpSpan, message: "division by zero"}
			}
			value = left / right
		}
		return check(value, n.Span_)
	case *IdentExpr:
		binding, _, ok := tc.lookupBinding(n.Name)
		if !ok || !binding.CompileTime || binding.ConstExpr == nil {
			return 0, false, nil
		}
		visiting[expr] = true
		value, known, fault := tc.evalConstFloat(binding.ConstExpr, target, visiting)
		delete(visiting, expr)
		return value, known, fault
	case *IfxExpr:
		cond, known := tc.evalConstBool(n.Condition, visiting)
		if !known {
			return 0, false, nil
		}
		if cond {
			return tc.evalConstFloat(n.Then, target, visiting)
		}
		return tc.evalConstFloat(n.Else, target, visiting)
	}
	return 0, false, nil
}

// evalConstBool evaluates a compile-time-known Bool expression: literals,
// negation, logical operators, integer and floating-point comparisons, and
// references to compile-time Bool bindings. It returns false when the
// expression is not compile-time-known.
func (tc *TypeChecker) evalConstBool(expr Expr, visiting map[Expr]bool) (bool, bool) {
	if expr == nil || visiting[expr] {
		return false, false
	}
	switch n := expr.(type) {
	case *BoolExpr:
		return n.Value, true
	case *ParenExpr:
		return tc.evalConstBool(n.Inner, visiting)
	case *UnaryExpr:
		if n.Op == UnaryOpNot {
			value, known := tc.evalConstBool(n.Operand, visiting)
			return !value, known
		}
	case *BinaryExpr:
		switch n.Op {
		case BinaryOpAnd, BinaryOpOr:
			left, leftOK := tc.evalConstBool(n.Left, visiting)
			right, rightOK := tc.evalConstBool(n.Right, visiting)
			if !leftOK || !rightOK {
				return false, false
			}
			if n.Op == BinaryOpAnd {
				return left && right, true
			}
			return left || right, true
		case BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe, BinaryOpEq, BinaryOpNeq:
			leftInt, leftIntOK, leftIntFault := tc.evalConstInt(n.Left, TypeUnknown, visiting)
			rightInt, rightIntOK, rightIntFault := tc.evalConstInt(n.Right, TypeUnknown, visiting)
			if leftIntOK && leftIntFault == nil && rightIntOK && rightIntFault == nil {
				return compareConstInts(n.Op, leftInt, rightInt), true
			}
			leftFloat, leftFloatOK, leftFloatFault := tc.evalConstFloat(n.Left, TypeUnknown, visiting)
			rightFloat, rightFloatOK, rightFloatFault := tc.evalConstFloat(n.Right, TypeUnknown, visiting)
			if leftFloatOK && leftFloatFault == nil && rightFloatOK && rightFloatFault == nil {
				return compareConstFloats(n.Op, leftFloat, rightFloat), true
			}
		}
	case *IdentExpr:
		binding, _, ok := tc.lookupBinding(n.Name)
		if !ok || !binding.CompileTime || binding.ConstExpr == nil {
			return false, false
		}
		visiting[expr] = true
		value, known := tc.evalConstBool(binding.ConstExpr, visiting)
		delete(visiting, expr)
		return value, known
	}
	return false, false
}

func compareConstInts(op BinaryOp, a, b *big.Int) bool {
	switch op {
	case BinaryOpLt:
		return a.Cmp(b) < 0
	case BinaryOpGt:
		return a.Cmp(b) > 0
	case BinaryOpLe:
		return a.Cmp(b) <= 0
	case BinaryOpGe:
		return a.Cmp(b) >= 0
	case BinaryOpEq:
		return a.Cmp(b) == 0
	case BinaryOpNeq:
		return a.Cmp(b) != 0
	}
	return false
}

func compareConstFloats(op BinaryOp, a, b float64) bool {
	switch op {
	case BinaryOpLt:
		return a < b
	case BinaryOpGt:
		return a > b
	case BinaryOpLe:
		return a <= b
	case BinaryOpGe:
		return a >= b
	case BinaryOpEq:
		return a == b
	case BinaryOpNeq:
		return a != b
	}
	return false
}

func (tc *TypeChecker) reportConstFloatFault(fault *constFloatFault, target Type) {
	if fault == nil {
		return
	}
	switch fault.message {
	case "division by zero":
		tc.diags.Error(fault.span, "division by zero in compile-time expression", "use a non-zero divisor")
	default:
		tc.diags.Error(fault.span, "compile-time floating-point value does not fit "+tc.formatType(target), "use a finite value representable by "+tc.formatType(target))
	}
}

// validateCompileTimeExpr applies the destination type recursively to a
// compile-time value. checkAssign validates ordinary literal leaves, while
// this pass also catches overflow and division by zero in typed arithmetic
// nested inside arrays and structs.
func (tc *TypeChecker) validateCompileTimeExpr(expr Expr, target Type) {
	if expr == nil || target == TypeUnknown || isLiteral(expr) {
		return
	}
	// Pointers are runtime-only: address-of, dereference, and null have no
	// compile-time meaning.
	switch e := expr.(type) {
	case *UnaryExpr:
		if e.Op == UnaryOpAddr {
			tc.diags.Error(e.Span_, "pointers are only available at runtime", "use ':=' or a runtime assignment for pointer values")
			return
		}
	case *DerefExpr:
		tc.diags.Error(e.Span_, "pointers are only available at runtime", "use ':=' or a runtime assignment for pointer values")
		return
	case *NullLitExpr:
		tc.diags.Error(e.Span_, "pointers are only available at runtime", "use ':=' or a runtime assignment for pointer values")
		return
	}
	if ifx, ok := expr.(*IfxExpr); ok {
		tc.validateCompileTimeExpr(ifx.Then, target)
		tc.validateCompileTimeExpr(ifx.Else, target)
		return
	}
	if isIntegerType(target) {
		_, _, fault := tc.evalConstInt(expr, target, make(map[Expr]bool))
		tc.reportConstIntFault(fault, target, false)
		return
	}
	if isFloatType(target) {
		_, _, fault := tc.evalConstFloat(expr, target, make(map[Expr]bool))
		tc.reportConstFloatFault(fault, target)
		return
	}
	if isArrayType(target) {
		if init, ok := expr.(*ArrayInitExpr); ok {
			elemType := arrayElemType(target)
			for _, item := range init.Items {
				tc.validateCompileTimeExpr(item, elemType)
			}
		}
		return
	}
	st, ok := tc.structDeclForType(target)
	if !ok {
		return
	}
	init, ok := expr.(*StructInitExpr)
	if !ok {
		return // A referenced compile-time binding was validated at its declaration.
	}
	fieldIndexes := make(map[string]int, len(st.Fields))
	for i, field := range st.Fields {
		fieldIndexes[field.Name] = i
	}
	positional := 0
	for _, field := range init.Fields {
		index := positional
		if field.Name != "" {
			var exists bool
			index, exists = fieldIndexes[field.Name]
			if !exists {
				continue // checkStructInit already reported the source error.
			}
		} else {
			positional++
		}
		if index < 0 || index >= len(st.Fields) {
			continue
		}
		tc.validateCompileTimeExpr(field.Value, tc.resolveTypeExpr(st.Fields[index].Type))
	}
}

func (tc *TypeChecker) checkLiteralRange(value Expr, target Type) {
	if isIntegerType(target) {
		_, _, fault := tc.evalConstInt(value, target, make(map[Expr]bool))
		// Ordinary runtime arithmetic wraps. Literal spelling itself must fit,
		// while overflow/division in a larger expression is handled only when
		// that expression belongs to a compile-time binding.
		if isLiteral(value) {
			tc.reportConstIntFault(fault, target, true)
		}
		return
	}
	if target == "F32" || target == "F64" {
		literal, ok := value.(*FloatExpr)
		if !ok {
			return
		}
		bits := 64
		if target == "F32" {
			bits = 32
		}
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(literal.Value, "_", ""), bits)
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			tc.diags.Error(value.nodeSpan(), "floating-point literal does not fit "+tc.formatType(target), "use a finite value representable by "+tc.formatType(target))
		}
	}
}

// checkVarDecl infers the type of a variable declaration and checks that the
// initializer matches the declared type when both are present. It returns the
// resolved type of the declaration.
func (tc *TypeChecker) checkVarDecl(d *VarDecl) Type {
	var declType Type
	if d.DeclType != nil {
		declType = tc.resolveTypeExpr(d.DeclType)
		tc.checkTypeExprValid(d.DeclType)
	}
	if d.Init == nil {
		if d.CompileTime {
			tc.diags.Error(d.NameSpan, "compile-time binding '"+d.Name+"' requires an initializer", "assign a compile-time-known value after '::'")
		}
		return declType
	}
	if d.CompileTime && !tc.isConstExpr(d.Init, make(map[string]bool)) {
		tc.diags.Error(d.Init.nodeSpan(), "compile-time assignment requires a compile-time-known expression", "use ':=' or a constant expression")
	}

	// An inferred struct literal (.{...}) takes its type from the declaration.
	if si, ok := d.Init.(*StructInitExpr); ok && si.Type == nil {
		if declType != TypeUnknown {
			if isArrayType(declType) {
				tc.checkArrayInitFromStruct(d.Span_, declType, si)
			} else {
				tc.checkStructInit(si, declType)
			}
			// A generic struct literal resolves to its instantiated type.
			if inst, ok := tc.structLiteralTypes[si]; ok {
				declType = inst
			}
			if d.CompileTime {
				tc.validateCompileTimeExpr(d.Init, declType)
			}
		}
		return declType
	}

	if declType != TypeUnknown {
		tc.withIfxContext(d.Init, func() {
			tc.checkAssign(d.Span_, declType, d.Init)
		})
		if d.CompileTime {
			tc.validateCompileTimeExpr(d.Init, declType)
		}
		if tc.procDepth == 0 && tc.typeContainsArray(declType, make(map[Type]bool)) {
			tc.diags.Error(d.NameSpan, "arrays cannot be stored in globals until owned array storage is implemented", "keep the array local for now")
		}
		return declType
	}
	var t Type
	tc.withIfxContext(d.Init, func() {
		t = tc.inferExpr(d.Init)
	})
	tc.checkLiteralRange(d.Init, t)
	if d.CompileTime {
		tc.validateCompileTimeExpr(d.Init, t)
	}
	if tc.procDepth == 0 && tc.typeContainsArray(t, make(map[Type]bool)) {
		tc.diags.Error(d.NameSpan, "arrays cannot be stored in globals until owned array storage is implemented", "keep the array local for now")
	}
	if t == TypeVoid {
		tc.diags.Error(d.Span_, "Void is only valid as a function result type", "remove the declaration or use a value type")
	}
	return t
}

func (tc *TypeChecker) typeContainsArray(t Type, visiting map[Type]bool) bool {
	if isArrayType(t) {
		return true
	}
	if isPointerType(t) {
		return false // indirection breaks the by-value storage restriction
	}
	if visiting[t] {
		return false
	}
	visiting[t] = true
	defer delete(visiting, t)
	if st, ok := tc.structDeclForType(t); ok {
		for _, field := range st.Fields {
			if tc.typeContainsArray(tc.resolveTypeExpr(field.Type), visiting) {
				return true
			}
		}
	}
	return false
}

// resolveTypeExpr resolves a written type through the current lexical type
// namespace. Nominal declarations receive an identity distinct from their
// spelling, so `#shadow T :: struct ...` does not make old and new T values
// assignment-compatible merely because both were written as "T".
func (tc *TypeChecker) resolveTypeExpr(e Expr) (resolved Type) {
	defer func() {
		if tc.analysis != nil && e != nil {
			tc.analysis.TypeExprTypes[e] = resolved
		}
	}()
	switch n := e.(type) {
	case *IdentExpr:
		if tp, ok := tc.lookupTypeParam(n.Name); ok {
			return tp
		}
		if isBuiltinTypeName(n.Name) {
			return Type(n.Name)
		}
		if binding, _, ok := tc.lookupBinding(n.Name); ok && binding.CompileTime && binding.Type == "Type" && binding.TypeValue != TypeUnknown {
			return binding.TypeValue
		}
		if d, ok := tc.lookupStruct(n.Name); ok {
			return tc.nominalTypes[d]
		}
		if d, ok := tc.lookupError(n.Name); ok {
			return tc.nominalTypes[d]
		}
		if d, ok := tc.lookupEnum(n.Name); ok {
			return tc.nominalTypes[d]
		}
		return Type(n.Name)
	case *ArrayTypeExpr:
		elem := tc.resolveTypeExpr(n.Elem)
		switch n.Kind {
		case ArrayFixed:
			size, ok := tc.arrayFixedSize(n)
			if !ok {
				return TypeUnknown
			}
			return fixedArrayType(elem, size)
		case ArrayDynamic:
			return dynamicArrayType(elem)
		default:
			return arrayType(elem)
		}
	case *PointerTypeExpr:
		return pointerType(tc.resolveTypeExpr(n.Elem), n.Nullable)
	}
	return TypeUnknown
}

func (tc *TypeChecker) structDeclForType(t Type) (*StructDecl, bool) {
	if d, ok := tc.typeDecls[t].(*StructDecl); ok {
		return d, true
	}
	return tc.lookupStruct(string(t))
}

func (tc *TypeChecker) errorDeclForType(t Type) (*ErrorDecl, bool) {
	if d, ok := tc.typeDecls[t].(*ErrorDecl); ok {
		return d, true
	}
	return tc.lookupError(string(t))
}

func (tc *TypeChecker) enumDeclForType(t Type) (*EnumDecl, bool) {
	if d, ok := tc.typeDecls[t].(*EnumDecl); ok {
		return d, true
	}
	return tc.lookupEnum(string(t))
}

// checkTypeExprValid validates a type expression used outside a function
// result position. Void is only valid as a function result type, so any use
// of it here (including as an array element type) is an error.
func (tc *TypeChecker) checkTypeExprValid(e Expr) {
	switch n := e.(type) {
	case *IdentExpr:
		resolved := tc.resolveTypeExpr(n)
		if resolved == TypeVoid {
			tc.diags.Error(n.Span_, "Void is only valid as a function result type", "remove the type or use a value type")
		} else if !isBuiltinTypeName(n.Name) && resolved == Type(n.Name) {
			tc.diags.Error(n.Span_, "unknown type name '"+n.Name+"'", "declare the type before using it in this scope")
		}
	case *ArrayTypeExpr:
		tc.checkTypeExprValid(n.Elem)
		if n.Kind == ArrayFixed {
			tc.arrayFixedSize(n)
		}
	case *PointerTypeExpr:
		tc.checkTypeExprValid(n.Elem)
	}
}

func (tc *TypeChecker) checkReturnStmt(s *ReturnStmt) {
	values := s.Values
	if len(values) == 0 && s.Value != nil {
		values = []Expr{s.Value}
	}
	if len(values) == 0 {
		if tc.currentReturnType != TypeVoid {
			tc.diags.Error(s.Span_, "return without values in a procedure returning "+tc.formatTypes(tc.currentReturnTypes), "return "+tc.formatTypes(tc.currentReturnTypes))
		}
		return
	}
	if tc.currentReturnType == TypeVoid && tc.currentErrorReturnType == "" {
		tc.diags.Error(s.Span_, "return with a value in a void procedure", "return without a value")
		return
	}
	// A bare error literal ('.MEMBER!') takes the error return type.
	if len(values) == 1 {
		if em, ok := values[0].(*ErrorMemberExpr); ok && em.TypeName == "" {
			if tc.currentErrorReturnType != "" {
				tc.checkErrorMember(em, tc.currentErrorReturnType)
				tc.analysis.ExprTypes[em] = tc.currentErrorReturnType
				return
			}
			tc.checkAssign(s.Span_, tc.currentReturnType, values[0])
			return
		}
	}
	// A bare enum member ('.MEMBER') takes the value return type.
	if len(values) == 1 {
		if em, ok := values[0].(*EnumMemberExpr); ok && em.TypeName == "" && len(tc.currentReturnTypes) == 1 {
			tc.checkEnumMember(em, tc.currentReturnType)
			tc.analysis.ExprTypes[em] = tc.currentReturnType
			return
		}
	}
	if tc.currentReturnType != TypeVoid {
		// A '<>' procedure may return either the value type or the error type.
		if tc.currentErrorReturnType != "" && len(values) == 1 {
			var vt Type
			tc.withIfxContext(values[0], func() {
				vt = tc.inferExpr(values[0])
			})
			if vt == tc.currentErrorReturnType {
				return // an error value returned from a '<>' procedure
			}
			if vt == errorUnionType(tc.currentReturnType, tc.currentErrorReturnType) {
				return // re-raise a matching error-returning value
			}
		}
		if len(values) != len(tc.currentReturnTypes) {
			tc.diags.Error(s.Span_, fmt.Sprintf("procedure returns %d values, but return provides %d", len(tc.currentReturnTypes), len(values)), "return "+tc.formatTypes(tc.currentReturnTypes))
			return
		}
		for i, value := range values {
			tc.withIfxContext(value, func() {
				tc.checkAssign(value.nodeSpan(), tc.currentReturnTypes[i], value)
			})
		}
		return
	}
	if len(values) != 1 {
		tc.diags.Error(s.Span_, "an error return must contain one error value", "return one member of "+tc.formatType(tc.currentErrorReturnType))
		return
	}
	tc.withIfxContext(values[0], func() {
		tc.checkAssign(s.Span_, tc.currentErrorReturnType, values[0])
	})
}

// inferExpr returns the type of an expression, reporting type errors it
// encounters along the way.
func (tc *TypeChecker) inferExpr(e Expr) Type {
	if e == nil {
		return TypeUnknown
	}
	t := tc.inferExprInner(e)
	if tc.analysis != nil {
		tc.analysis.ExprTypes[e] = t
	}
	return t
}

func (tc *TypeChecker) inferExprInner(e Expr) Type {
	switch n := e.(type) {
	case *IntExpr:
		return TypeS64
	case *FloatExpr:
		return TypeF64
	case *StringExpr:
		return TypeString
	case *InterpolatedStringExpr:
		for _, part := range n.Parts {
			if part.Expr == nil {
				continue
			}
			et := tc.inferExpr(part.Expr)
			if et != TypeUnknown && et != TypeString {
				tc.diags.Error(part.Expr.nodeSpan(), "interpolation requires a String value, got "+tc.formatType(et), "interpolate a String value")
			}
		}
		return TypeString
	case *BoolExpr:
		return TypeBool
	case *IdentExpr:
		binding, _, ok := tc.lookupBinding(n.Name)
		if ok {
			if !binding.Initialized {
				tc.diags.Error(n.Span_, "binding '"+n.Name+"' is not initialized", "assign it before reading it")
			}
			return binding.Type
		}
		if _, _, captured := tc.lookupAnyBinding(n.Name); captured && tc.procDepth > 1 {
			tc.diags.Error(n.Span_, "nested procedure cannot implicitly capture runtime binding '"+n.Name+"'", "pass it explicitly as a procedure argument")
			return TypeUnknown
		}
		if proc, exists := tc.lookupProc(n.Name); exists {
			return tc.procTypes[proc]
		}
		if isBuiltinTypeName(n.Name) || tc.hasCompileName(n.Name) {
			return Type("Type")
		}
		tc.diags.Error(n.Span_, "use of undeclared variable "+n.Name, "declare it before using it in this scope")
		return TypeUnknown
	case *ParenExpr:
		return tc.inferExpr(n.Inner)
	case *BinaryExpr:
		return tc.checkBinaryExpr(n)
	case *UnaryExpr:
		return tc.checkUnaryExpr(n)
	case *CallExpr:
		return tc.checkCallExpr(n)
	case *StructInitExpr:
		if n.Type != nil {
			t := tc.resolveTypeExpr(n.Type)
			tc.checkStructInit(n, t)
			return t
		}
		return TypeUnknown
	case *ErrorMemberExpr:
		return tc.checkErrorMemberExpr(n)
	case *EnumMemberExpr:
		return tc.checkEnumMemberExpr(n)
	case *ErrorExpr:
		return TypeUnknown
	case *ArrayInitExpr:
		elemType := tc.resolveTypeExpr(n.Elem.Elem)
		tc.checkTypeExprValid(n.Elem)
		for _, item := range n.Items {
			tc.checkAssign(item.nodeSpan(), elemType, item)
		}
		switch n.Elem.Kind {
		case ArrayFixed:
			size, ok := tc.arrayFixedSize(n.Elem)
			if !ok {
				return TypeUnknown
			}
			if len(n.Items) != size {
				tc.diags.Error(n.Span_, fmt.Sprintf("array literal has %d elements but type declares %d", len(n.Items), size), "provide exactly the declared number of elements")
			}
			return fixedArrayType(elemType, size)
		case ArrayDynamic:
			return dynamicArrayType(elemType)
		default:
			return arrayType(elemType)
		}
	case *IndexExpr:
		baseType := tc.inferExpr(n.Base)
		if isPointerType(baseType) {
			// Pointer indexing "p[i]" reads/writes the element at offset i.
			idxType := tc.inferExpr(n.Index)
			if idxType != TypeUnknown && !isIntegerType(idxType) {
				tc.diags.Error(n.Index.nodeSpan(), "pointer index must be an integer, got "+tc.formatType(idxType), "use an integer index")
			}
			if Debug {
				fmt.Printf("DBG TC IndexExpr pointer base=%s elem=%s span=%d\n", tc.formatType(baseType), tc.formatType(pointerElemType(baseType)), n.Span_.Start)
			}
			return pointerElemType(baseType)
		}
		if baseType == TypeString {
			// String indexing "s[i]" reads/writes the byte at offset i.
			idxType := tc.inferExpr(n.Index)
			if idxType != TypeUnknown && !isIntegerType(idxType) {
				tc.diags.Error(n.Index.nodeSpan(), "string index must be an integer, got "+tc.formatType(idxType), "use an integer index")
			}
			return Type("Byte")
		}
		if !isArrayType(baseType) {
			tc.diags.Error(n.Base.nodeSpan(), "cannot index a value of type "+tc.formatType(baseType), "use an array value")
			return TypeUnknown
		}
		idxType := tc.inferExpr(n.Index)
		if idxType != TypeUnknown && !isIntegerType(idxType) {
			tc.diags.Error(n.Index.nodeSpan(), "array index must be an integer, got "+tc.formatType(idxType), "use an integer index")
		}
		// A constant index on a fixed-size array is checked at compile time.
		if arrayKindOf(baseType) == ArrayFixed {
			if size, ok := arrayFixedSizeOf(baseType); ok {
				if idx, known, _ := tc.evalConstInt(n.Index, TypeS64, make(map[Expr]bool)); known {
					if idx.Sign() < 0 || idx.Cmp(big.NewInt(int64(size))) >= 0 {
						tc.diags.Error(n.Index.nodeSpan(), fmt.Sprintf("array index %s is out of bounds for a %d-element array", idx.String(), size), "use an index in the range 0.."+strconv.Itoa(size-1))
					}
				}
			}
		}
		return arrayElemType(baseType)
	case *LoopBuiltinExpr:
		if n.Name == "this" {
			if tc.rangeElemType == "" {
				tc.diags.Error(n.Span_, "'#this' is only available inside a range loop", "use '#this' inside a 'for' loop over an array")
				return TypeUnknown
			}
			return tc.rangeElemType
		}
		if tc.rangeIndexType == "" {
			tc.diags.Error(n.Span_, "'#index' is only available inside a range loop", "use '#index' inside a 'for' loop over an array")
			return TypeUnknown
		}
		return tc.rangeIndexType
	case *NullLitExpr:
		return TypeNull
	case *AllocateExpr:
		sizeType := tc.inferExpr(n.Size)
		if sizeType != TypeUnknown && !isIntegerType(sizeType) {
			tc.diags.Error(n.Size.nodeSpan(), "#allocate size must be an integer, got "+tc.formatType(sizeType), "use an integer size")
		}
		return TypeAddr
	case *SizeOfExpr:
		// size_of resolves its operand as a type expression; if that fails it
		// is a value expression whose type is measured. The result is Size.
		t := tc.resolveTypeExpr(n.Type)
		if t == TypeUnknown {
			t = tc.inferExpr(n.Type)
		}
		if t == TypeUnknown {
			tc.diags.Error(n.Type.nodeSpan(), "size_of requires a known type or value", "use a declared type or a value")
		}
		return Type("Size")
	case *CastExpr:
		target := tc.resolveTypeExpr(n.Type)
		tc.checkTypeExprValid(n.Type)
		if target == TypeUnknown {
			tc.diags.Error(n.Type.nodeSpan(), "cannot cast to an unknown type", "use a declared type")
			return TypeUnknown
		}
		// The cast reinterprets the value; the source type is still checked so
		// undeclared identifiers are reported.
		tc.inferExpr(n.Value)
		return target
	case *DerefExpr:
		return tc.checkDeref(n)
	case *FieldAccessExpr:
		return tc.checkFieldAccess(n)
	case *IfxExpr:
		if !tc.ifxContext {
			tc.diags.Error(n.Span_, "ifx expressions are only supported as the value of an assignment or return", "use ifx directly as the assigned or returned value")
			return TypeUnknown
		}
		return tc.checkIfxExpr(n)
	}
	return TypeUnknown
}

// checkDeref checks "operand.*". The operand must be a pointer, and a
// nullable pointer must be provably non-null at this point (either the type is
// non-nullable or a flow-narrowing null check proved it).
func (tc *TypeChecker) checkDeref(n *DerefExpr) Type {
	ot := tc.inferExpr(n.Operand)
	if !isPointerType(ot) {
		if ot != TypeUnknown {
			tc.diags.Error(n.Operand.nodeSpan(), "cannot dereference a value of type "+tc.formatType(ot), "use a pointer (write '*T' or '*T?')")
		}
		return TypeUnknown
	}
	if pointerNullable(ot) && !tc.exprIsNonNull(n.Operand) {
		tc.diags.Error(n.Operand.nodeSpan(), "cannot dereference "+pointerName(n.Operand)+" because it may be null", "check the pointer against null before dereferencing (for example 'if p == null { return; }')")
	}
	return pointerElemType(ot)
}

// pointerName renders an operand for null-check diagnostics: the variable name
// when the operand is a plain identifier, otherwise a generic description.
func pointerName(e Expr) string {
	if ident, ok := e.(*IdentExpr); ok {
		return "'" + ident.Name + "'"
	}
	return "the pointer"
}

// isLvalue reports whether an expression refers to addressable storage.
func (tc *TypeChecker) isLvalue(e Expr) bool {
	switch n := e.(type) {
	case *IdentExpr:
		b, _, ok := tc.lookupBinding(n.Name)
		return ok && !b.CompileTime
	case *IndexExpr, *FieldAccessExpr, *DerefExpr:
		return true
	case *ParenExpr:
		return tc.isLvalue(n.Inner)
	}
	return false
}

// exprIsNonNull reports whether an expression is provably non-null: an
// address-of result, or a variable whose flow state has been narrowed.
func (tc *TypeChecker) exprIsNonNull(e Expr) bool {
	switch n := e.(type) {
	case *ParenExpr:
		return tc.exprIsNonNull(n.Inner)
	case *UnaryExpr:
		return n.Op == UnaryOpAddr
	case *IdentExpr:
		b, _, ok := tc.lookupBinding(n.Name)
		return ok && b.NullState == nullStateNonNull
	}
	return false
}

// checkFieldAccess checks "base.field" for a struct or enum base. The field
// type is returned for structs; an enum base resolves the member reference
// (and returns the enum type). Assignment targets flow through
// checkAssignableTarget instead.
func (tc *TypeChecker) checkFieldAccess(n *FieldAccessExpr) Type {
	// A base that names an imported module namespace resolves to a member of
	// that module. Procedures cannot be used as values (no function pointers),
	// so a non-call member reference is an error.
	if imp, ok := tc.importNamespace(n.Base); ok {
		if tc.findImportProc(imp, n.Field) != nil {
			tc.diags.Error(n.FieldSpan, "procedure '"+n.Field+"' cannot be used as a value", "call it directly")
			return TypeUnknown
		}
		tc.diags.Error(n.FieldSpan, "module '"+imp.Module+"' has no member '"+n.Field+"'", "use a declared member of the imported module")
		return TypeUnknown
	}
	// A base that names an enum type is an enum member reference. The enum
	// name must be resolved directly: inferring the base as a value would
	// yield the generic "Type" binding rather than the enum itself.
	if name := enumBaseName(n.Base); name != "" {
		if t := tc.resolveTypeExpr(&IdentExpr{Name: name}); tc.isEnumType(t) {
			return tc.checkEnumMemberExpr(&EnumMemberExpr{
				Span_:        n.Span_,
				TypeName:     name,
				TypeNameSpan: n.Base.nodeSpan(),
				Name:         n.Field,
				NameSpan:     n.FieldSpan,
			})
		}
	}
	bt := tc.inferExpr(n.Base)
	if bt == TypeString {
		// String exposes its data (*Byte) and count (Size) fields directly.
		switch n.Field {
		case "data":
			return pointerType(Type("Byte"), false)
		case "count":
			return Type("Size")
		}
		tc.diags.Error(n.FieldSpan, "String has no field '"+n.Field+"'", "use 'data' or 'count'")
		return TypeUnknown
	}
	if isArrayType(bt) {
		return tc.arrayFieldType(n, bt)
	}
	st, ok := tc.structDeclForType(bt)
	if !ok {
		if name := enumBaseName(n.Base); name != "" {
			if t := tc.resolveTypeExpr(&IdentExpr{Name: name}); tc.isErrorType(t) {
				tc.diags.Error(n.Span_, "error values must be instantiated with '!'", "add '!' after the member name")
				return TypeUnknown
			}
		}
		if tc.isErrorType(bt) {
			tc.diags.Error(n.Span_, "error values must be instantiated with '!'", "add '!' after the member name")
			return TypeUnknown
		}
		tc.diags.Error(n.Base.nodeSpan(), "cannot access field '"+n.Field+"' on a value of type "+tc.formatType(bt), "use a struct value")
		return TypeUnknown
	}
	for _, field := range st.Fields {
		if field.Name == n.Field {
			return tc.resolveTypeExpr(field.Type)
		}
	}
	tc.diags.Error(n.FieldSpan, "struct "+tc.formatType(bt)+" has no field '"+n.Field+"'", "use a declared field name")
	return TypeUnknown
}

// arrayFieldType checks "array.field" for an array base. Arrays expose data
// (pointer to the element type) and count (Size); dynamic arrays also expose
// capacity (Size).
func (tc *TypeChecker) arrayFieldType(n *FieldAccessExpr, bt Type) Type {
	elem := arrayElemType(bt)
	switch n.Field {
	case "data":
		return pointerType(elem, false)
	case "count":
		return Type("Size")
	case "capacity":
		if arrayKindOf(bt) == ArrayDynamic {
			return Type("Size")
		}
		tc.diags.Error(n.FieldSpan, "only dynamic arrays have a 'capacity' field", "use 'data' or 'count'")
		return TypeUnknown
	}
	tc.diags.Error(n.FieldSpan, "array has no field '"+n.Field+"'", "use 'data', 'count', or 'capacity'")
	return TypeUnknown
}

// enumBaseName returns the enum type name behind a field-access base.
func enumBaseName(base Expr) string {
	if ident, ok := unwrapParens(base).(*IdentExpr); ok {
		return ident.Name
	}
	return ""
}

// checkAssignableTarget checks an assignment target expression (index, field,
// or deref) and returns the storage type the value must match.
func (tc *TypeChecker) checkAssignableTarget(target Expr) (Type, bool) {
	switch n := target.(type) {
	case *IdentExpr:
		b, _, ok := tc.lookupBinding(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "assignment to undeclared variable "+n.Name, "declare the variable before assigning to it")
			return TypeUnknown, false
		}
		if !b.Mutable {
			tc.diags.Error(n.Span_, "cannot assign to immutable binding '"+n.Name+"'", "declare it with ':=' if it must change")
			return TypeUnknown, false
		}
		return b.Type, true
	case *IndexExpr:
		baseType := tc.inferExpr(n.Base)
		if isPointerType(baseType) {
			idxType := tc.inferExpr(n.Index)
			if idxType != TypeUnknown && !isIntegerType(idxType) {
				tc.diags.Error(n.Index.nodeSpan(), "pointer index must be an integer, got "+tc.formatType(idxType), "use an integer index")
			}
			return pointerElemType(baseType), true
		}
		if baseType == TypeString {
			idxType := tc.inferExpr(n.Index)
			if idxType != TypeUnknown && !isIntegerType(idxType) {
				tc.diags.Error(n.Index.nodeSpan(), "string index must be an integer, got "+tc.formatType(idxType), "use an integer index")
			}
			return Type("Byte"), true
		}
		if !isArrayType(baseType) {
			tc.diags.Error(n.Base.nodeSpan(), "cannot index a value of type "+tc.formatType(baseType), "use an array value")
			return TypeUnknown, false
		}
		idxType := tc.inferExpr(n.Index)
		if idxType != TypeUnknown && !isIntegerType(idxType) {
			tc.diags.Error(n.Index.nodeSpan(), "array index must be an integer, got "+tc.formatType(idxType), "use an integer index")
		}
		return arrayElemType(baseType), true
	case *FieldAccessExpr:
		bt := tc.fieldBaseType(n.Base)
		if bt == TypeString {
			switch n.Field {
			case "data":
				return pointerType(Type("Byte"), false), true
			case "count":
				return Type("Size"), true
			}
			tc.diags.Error(n.FieldSpan, "String has no field '"+n.Field+"'", "use 'data' or 'count'")
			return TypeUnknown, false
		}
		if isArrayType(bt) {
			return tc.arrayFieldType(n, bt), true
		}
		st, ok := tc.structDeclForType(bt)
		if !ok {
			tc.diags.Error(n.Base.nodeSpan(), "cannot access a field on a value of type "+tc.formatType(bt), "use a struct value")
			return TypeUnknown, false
		}
		for _, field := range st.Fields {
			if field.Name == n.Field {
				return tc.resolveTypeExpr(field.Type), true
			}
		}
		tc.diags.Error(n.FieldSpan, "struct "+tc.formatType(bt)+" has no field '"+n.Field+"'", "use a declared field name")
		return TypeUnknown, false
	case *DerefExpr:
		// The deref null-proof runs in checkDeref.
		return tc.checkDeref(n), true
	}
	tc.diags.Error(target.nodeSpan(), "expression is not assignable", "assign to a variable, array element, field, or dereferenced pointer")
	return TypeUnknown, false
}

// checkExprAssign validates "target = value" where target is an index, field,
// or deref expression. The lvalue type is resolved and the value checked
// against it.
func (tc *TypeChecker) checkExprAssign(span Span, target Expr, value Expr) {
	ty, ok := tc.checkAssignableTarget(target)
	if !ok {
		return
	}
	tc.withIfxContext(value, func() {
		tc.checkAssign(span, ty, value)
	})
	// Writing to a field of an uninitialized variable initializes it.
	if fa, isField := target.(*FieldAccessExpr); isField {
		if ident, ok := unwrapParens(fa.Base).(*IdentExpr); ok {
			if b, _, found := tc.lookupBinding(ident.Name); found && !b.Initialized {
				b.Initialized = true
				tc.setBinding(ident.Name, b)
			}
		}
	}
}

// fieldBaseType returns the type of a field-access base without triggering the
// definite-initialization error, so a field write can initialize an otherwise
// uninitialized variable.
func (tc *TypeChecker) fieldBaseType(base Expr) Type {
	if ident, ok := unwrapParens(base).(*IdentExpr); ok {
		if b, _, found := tc.lookupBinding(ident.Name); found {
			return b.Type
		}
	}
	return tc.inferExpr(base)
}

// checkExprCompound validates "target += value" / "target -= value" for an
// index, field, or deref target. Compound assignment is not an ifx context.
func (tc *TypeChecker) checkExprCompound(span Span, target Expr, op BinaryOp, value Expr) {
	ty, ok := tc.checkAssignableTarget(target)
	if !ok {
		return
	}
	tc.checkCompoundOp(span, ty, op, value)
}

// checkExprIncDec validates "target++" / "target--" on an index, field, or
// deref target.
func (tc *TypeChecker) checkExprIncDec(span Span, target Expr, op BinaryOp) {
	ty, ok := tc.checkAssignableTarget(target)
	if !ok {
		return
	}
	if tc.isEnumType(ty) {
		tc.diags.Error(span, "cannot apply "+op.String()+" to enum values", "enum values support only == and !=")
		return
	}
	if !isIntegerType(ty) && !isPointerType(ty) {
		tc.diags.Error(span, "cannot increment or decrement a value of type "+tc.formatType(ty), "use an integer or pointer target")
	}
}

// checkCompoundOp validates a compound operation "+="/"-=" on a value type.
// Compound statements only ever carry Add or Sub; pointer targets require an
// integer offset (scaled by element size, as in C).
func (tc *TypeChecker) checkCompoundOp(span Span, t Type, op BinaryOp, value Expr) {
	if t == TypeUnknown {
		return
	}
	if tc.isEnumType(t) {
		tc.diags.Error(span, "cannot apply "+op.String()+" to enum values", "enum values support only == and !=")
		return
	}
	if isPointerType(t) {
		if value != nil {
			vt := tc.inferExpr(value)
			if vt != TypeUnknown && !isIntegerType(vt) {
				tc.diags.Error(span, "pointer arithmetic requires an integer offset, got "+tc.formatType(vt), "add or subtract an integer: 'p += n'")
			}
		}
		return
	}
	if !isIntegerType(t) && !isFloatType(t) {
		tc.diags.Error(span, "operator "+op.String()+" is not defined for "+tc.formatType(t), "use a numeric target")
		return
	}
	if value != nil {
		tc.checkAssign(span, t, value)
	}
}

// exprNullState returns the null-flow state an expression establishes for a
// pointer binding it is assigned to: address-of results are non-null, null is
// known-null, and identifier copies carry the source binding's state.
func (tc *TypeChecker) exprNullState(e Expr) uint8 {
	switch n := e.(type) {
	case *ParenExpr:
		return tc.exprNullState(n.Inner)
	case *UnaryExpr:
		if n.Op == UnaryOpAddr {
			return nullStateNonNull
		}
	case *NullLitExpr:
		return nullStateNull
	case *IdentExpr:
		b, _, ok := tc.lookupBinding(n.Name)
		if !ok {
			return 0
		}
		if isPointerType(b.Type) && !pointerNullable(b.Type) {
			return nullStateNonNull
		}
		return b.NullState
	}
	return 0
}

// checkErrorMemberExpr infers the type of an error member reference. The
// explicit "Type.MEMBER!" form resolves against the named error type; the bare
// ".MEMBER!" form has no type context here and is reported as unresolvable
// (typed contexts resolve it via checkAssign). Error literals require the
// trailing '!'.
func (tc *TypeChecker) checkErrorMemberExpr(n *ErrorMemberExpr) Type {
	if !n.Bang {
		tc.diags.Error(n.Span_, "error values must be instantiated with '!'", "add '!' after the member name")
		return TypeUnknown
	}
	if n.TypeName == "" {
		tc.diags.Error(n.Span_, "cannot infer the error type of '."+n.Name+"'", "annotate the declaration with an error type")
		return TypeUnknown
	}
	t := tc.resolveTypeExpr(&IdentExpr{Name: n.TypeName})
	if !tc.isErrorType(t) {
		tc.diags.Error(n.Span_, n.TypeName+" is not an error type", "use a declared error type")
		return TypeUnknown
	}
	if !tc.hasErrorMember(n.TypeName, n.Name) {
		tc.diags.Error(n.Span_, "unknown error member "+n.Name+" in "+n.TypeName, "use a declared error member")
		return TypeUnknown
	}
	return t
}

// isErrorType reports whether t names a declared error type.
func (tc *TypeChecker) isErrorType(t Type) bool {
	_, ok := tc.errorDeclForType(t)
	return ok
}

// errorUnionType is the type of a value-or-error pair: "Value<>Error".
// The '<>' separator cannot appear in identifier names, so a type string
// containing it is unambiguously an error union.
func errorUnionType(value, err Type) Type {
	return Type(string(value) + "<>" + string(err))
}

// isErrorUnion reports whether t is a value-or-error pair type.
func isErrorUnion(t Type) bool {
	return strings.Contains(string(t), "<>")
}

// unionValueType returns the value side of an error union type.
func unionValueType(t Type) Type {
	i := strings.Index(string(t), "<>")
	if i < 0 {
		return t
	}
	return t[:i]
}

// unionErrorType returns the error side of an error union type.
func unionErrorType(t Type) Type {
	i := strings.Index(string(t), "<>")
	if i < 0 {
		return ""
	}
	return t[i+2:]
}

// hasErrorMember reports whether the named error type declares the member.
func (tc *TypeChecker) hasErrorMember(typeName, member string) bool {
	t := tc.resolveTypeExpr(&IdentExpr{Name: typeName})
	d, ok := tc.errorDeclForType(t)
	if !ok {
		return false
	}
	for _, m := range d.Members {
		if m.Name == member {
			return true
		}
	}
	return false
}

// checkEnum validates an enum declaration: the first member must declare an
// integer underlying type, only that member may carry a type, member names
// must be unique, and every member value (explicit or sequential) must fit
// the underlying type.
func (tc *TypeChecker) checkEnum(d *EnumDecl) {
	if len(d.Members) == 0 {
		tc.diags.Error(d.Span_, "enum '"+d.Name+"' must have at least one member", "add at least one member")
		return
	}
	first := d.Members[0]
	if first.Type == nil {
		tc.diags.Error(first.Span_, "the first enum member must declare the underlying type", "add ': Type' to the first member")
		return
	}
	underlying := tc.resolveTypeExpr(first.Type)
	if !isIntegerType(underlying) {
		tc.diags.Error(first.Type.nodeSpan(), "enum underlying type must be an integer type, got "+string(underlying), "use an integer type")
		return
	}
	for i := 1; i < len(d.Members); i++ {
		if d.Members[i].Type != nil {
			tc.diags.Error(d.Members[i].Type.nodeSpan(), "only the first enum member may declare the underlying type", "remove the type annotation")
		}
	}
	seen := make(map[string]bool, len(d.Members))
	prev := big.NewInt(0)
	for i, m := range d.Members {
		if seen[m.Name] {
			tc.diags.Error(m.Span_, "duplicate enum member "+m.Name+" in "+d.Name, "use a unique member name")
		}
		seen[m.Name] = true
		val := new(big.Int).Add(prev, big.NewInt(1))
		if i == 0 {
			val.SetInt64(0)
		}
		if m.Value != nil {
			v, ok := evalEnumMemberValue(m.Value)
			if !ok {
				tc.diags.Error(m.Value.nodeSpan(), "enum member value must be an integer literal", "use an integer literal")
				continue
			}
			val = v
		}
		if !fitsIntegerType(underlying, val) {
			tc.diags.Error(m.Span_, "enum member value "+val.String()+" does not fit "+string(underlying), "use a value that fits the underlying type")
		}
		prev.Set(val)
	}
}

// isEnumType reports whether t names a declared enum type.
func (tc *TypeChecker) isEnumType(t Type) bool {
	_, ok := tc.enumDeclForType(t)
	return ok
}

// hasEnumMember reports whether the named enum type declares the member.
func (tc *TypeChecker) hasEnumMember(typeName, member string) bool {
	t := tc.resolveTypeExpr(&IdentExpr{Name: typeName})
	d, ok := tc.enumDeclForType(t)
	if !ok {
		return false
	}
	for _, m := range d.Members {
		if m.Name == member {
			return true
		}
	}
	return false
}

// checkEnumMemberExpr infers the type of an enum member reference. The
// explicit "Enum.MEMBER" form resolves against the named enum type; the bare
// ".MEMBER" form has no type context here and is reported as unresolvable
// (typed contexts resolve it via checkAssign and comparisons).
func (tc *TypeChecker) checkEnumMemberExpr(n *EnumMemberExpr) Type {
	if n.TypeName == "" {
		tc.diags.Error(n.Span_, "cannot infer the enum type of '."+n.Name+"'", "annotate the declaration with an enum type")
		return TypeUnknown
	}
	t := tc.resolveTypeExpr(&IdentExpr{Name: n.TypeName})
	if !tc.isEnumType(t) {
		if tc.isErrorType(t) {
			tc.diags.Error(n.Span_, "error values must be instantiated with '!'", "add '!' after the member name")
		} else {
			tc.diags.Error(n.Span_, n.TypeName+" is not an enum type", "use a declared enum type")
		}
		return TypeUnknown
	}
	if !tc.hasEnumMember(n.TypeName, n.Name) {
		tc.diags.Error(n.Span_, "unknown enum member "+n.Name+" in "+n.TypeName, "use a declared enum member")
		return TypeUnknown
	}
	return t
}

// checkEnumMember validates a bare enum member reference against a target
// enum type.
func (tc *TypeChecker) checkEnumMember(em *EnumMemberExpr, t Type) {
	if !tc.isEnumType(t) {
		tc.diags.Error(em.Span_, tc.formatType(t)+" is not an enum type", "use a declared enum type")
		return
	}
	if !tc.hasEnumMember(string(t), em.Name) {
		tc.diags.Error(em.Span_, "unknown enum member "+em.Name+" in "+tc.formatType(t), "use a declared enum member")
	}
}

// evalEnumMemberValue evaluates an enum member value exactly at compile time.
// Only integer literals (and negated integer literals) are allowed.
func evalEnumMemberValue(e Expr) (*big.Int, bool) {
	switch n := e.(type) {
	case *IntExpr:
		v, ok := new(big.Int).SetString(strings.ReplaceAll(n.Value, "_", ""), 10)
		return v, ok
	case *UnaryExpr:
		if n.Op == UnaryOpNeg {
			if lit, ok := n.Operand.(*IntExpr); ok {
				v, parsed := new(big.Int).SetString(strings.ReplaceAll(lit.Value, "_", ""), 10)
				if parsed {
					return v.Neg(v), true
				}
			}
		}
	}
	return nil, false
}

// fitsIntegerType reports whether v fits the given fixed-width integer type.
// big.Int is used only during compile-time validation so overflow is detected
// before the value is lowered to its declared machine representation.
func fitsIntegerType(t Type, v *big.Int) bool {
	info, ok := LookupBuiltinType(string(t))
	if !ok || info.Kind != BuiltinInteger {
		return false
	}
	if info.Signed {
		limit := new(big.Int).Lsh(big.NewInt(1), uint(info.Bits-1))
		min := new(big.Int).Neg(new(big.Int).Set(limit))
		max := new(big.Int).Sub(limit, big.NewInt(1))
		return v.Cmp(min) >= 0 && v.Cmp(max) <= 0
	}
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(info.Bits)), big.NewInt(1))
	return v.Sign() >= 0 && v.Cmp(max) <= 0
}

func (tc *TypeChecker) checkBinaryExpr(n *BinaryExpr) Type {
	// Infer operand types, deferring bare error members ('.MEMBER') until the
	// other operand's type is known.
	var lt, rt Type
	if em, ok := n.Left.(*ErrorMemberExpr); ok && em.TypeName == "" {
		lt = TypeUnknown
	} else if em, ok := n.Left.(*EnumMemberExpr); ok && em.TypeName == "" {
		lt = TypeUnknown
	} else {
		lt = tc.inferExpr(n.Left)
	}
	if em, ok := n.Right.(*ErrorMemberExpr); ok && em.TypeName == "" {
		rt = TypeUnknown
	} else if em, ok := n.Right.(*EnumMemberExpr); ok && em.TypeName == "" {
		rt = TypeUnknown
	} else {
		rt = tc.inferExpr(n.Right)
	}
	// Resolve a bare error member against the other operand's type when that
	// type is a declared error type.
	if em, ok := n.Left.(*ErrorMemberExpr); ok && em.TypeName == "" {
		if tc.isErrorType(rt) {
			tc.checkErrorMember(em, rt)
			lt = rt
			tc.analysis.ExprTypes[em] = rt
		} else {
			tc.diags.Error(em.Span_, "cannot infer the error type of '."+em.Name+"'", "use the explicit 'Type.MEMBER' form")
		}
	}
	if em, ok := n.Right.(*ErrorMemberExpr); ok && em.TypeName == "" {
		if tc.isErrorType(lt) {
			tc.checkErrorMember(em, lt)
			rt = lt
			tc.analysis.ExprTypes[em] = lt
		} else {
			tc.diags.Error(em.Span_, "cannot infer the error type of '."+em.Name+"'", "use the explicit 'Type.MEMBER' form")
		}
	}
	// Resolve a bare enum member against the other operand's type when that
	// type is a declared enum type.
	if em, ok := n.Left.(*EnumMemberExpr); ok && em.TypeName == "" {
		if tc.isEnumType(rt) {
			tc.checkEnumMember(em, rt)
			lt = rt
			tc.analysis.ExprTypes[em] = rt
		} else {
			tc.diags.Error(em.Span_, "cannot infer the enum type of '."+em.Name+"'", "use the explicit 'Enum.MEMBER' form")
		}
	}
	if em, ok := n.Right.(*EnumMemberExpr); ok && em.TypeName == "" {
		if tc.isEnumType(lt) {
			tc.checkEnumMember(em, lt)
			rt = lt
			tc.analysis.ExprTypes[em] = lt
		} else {
			tc.diags.Error(em.Span_, "cannot infer the enum type of '."+em.Name+"'", "use the explicit 'Enum.MEMBER' form")
		}
	}
	// Error-returning values must be handled before they are used.
	if isErrorUnion(lt) || isErrorUnion(rt) {
		tc.diags.Error(n.Span_, "must handle the error before using the value", "use 'unless catch' or 'if ... catch' first")
		return TypeUnknown
	}
	// Pointer-specific operator rules: + - scaled by element size, equality
	// and ordering between pointers, and ==/!= against null.
	if pt, handled := tc.checkPointerBinary(n, lt, rt); handled {
		return pt
	}
	compatible := tc.operandsCompatible(n.Left, lt, n.Right, rt)
	effective := lt
	if isLiteral(n.Left) && !isLiteral(n.Right) {
		effective = rt
	}
	if compatible {
		if isLiteral(n.Left) {
			tc.checkLiteralRange(n.Left, effective)
			tc.analysis.ExprTypes[n.Left] = effective
		}
		if isLiteral(n.Right) {
			tc.checkLiteralRange(n.Right, effective)
			tc.analysis.ExprTypes[n.Right] = effective
		}
	}
	switch n.Op {
	case BinaryOpAdd, BinaryOpSub, BinaryOpMul, BinaryOpDiv, BinaryOpMod:
		if !compatible {
			tc.diags.Error(n.Span_, "cannot apply "+n.Op.String()+" to "+tc.formatType(lt)+" and "+tc.formatType(rt), "operate on values of the same type")
			return effective
		}
		if !isIntegerType(effective) && !isFloatType(effective) {
			tc.diags.Error(n.Span_, "operator "+n.Op.String()+" is not defined for "+tc.formatType(effective), "use numeric operands")
			return effective
		}
		if n.Op == BinaryOpMod && isFloatType(effective) {
			tc.diags.Error(n.Span_, "operator % is not defined for floating-point values", "use integer operands")
		}
		return effective
	case BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe:
		if !compatible {
			tc.diags.Error(n.Span_, "cannot compare "+tc.formatType(lt)+" and "+tc.formatType(rt), "compare values of the same type")
		} else if !isIntegerType(effective) && !isFloatType(effective) && effective != TypeString {
			tc.diags.Error(n.Span_, "ordering is not defined for "+tc.formatType(effective), "use == or != for nominal and aggregate values")
		}
		return TypeBool
	case BinaryOpEq, BinaryOpNeq:
		if !compatible {
			tc.diags.Error(n.Span_, "cannot compare "+tc.formatType(lt)+" and "+tc.formatType(rt), "compare values of the same type")
		} else if effective == TypeVoid || effective == TypeUnknown || isProcType(effective) || effective == "Type" {
			tc.diags.Error(n.Span_, "equality is not defined for "+tc.formatType(effective), "compare runtime values")
		}
		return TypeBool
	case BinaryOpAnd, BinaryOpOr:
		if lt != TypeUnknown && lt != TypeBool {
			tc.diags.Error(n.Left.nodeSpan(), "logical operator "+n.Op.String()+" requires Bool operands, got "+tc.formatType(lt), "use boolean operands")
		}
		if rt != TypeUnknown && rt != TypeBool {
			tc.diags.Error(n.Right.nodeSpan(), "logical operator "+n.Op.String()+" requires Bool operands, got "+tc.formatType(rt), "use boolean operands")
		}
		return TypeBool
	}
	return TypeUnknown
}

func (tc *TypeChecker) checkUnaryExpr(n *UnaryExpr) Type {
	ot := tc.inferExpr(n.Operand)
	switch n.Op {
	case UnaryOpNeg:
		if !isIntegerType(ot) && !isFloatType(ot) {
			tc.diags.Error(n.Span_, "operator - requires a numeric operand, got "+tc.formatType(ot), "use an integer or floating-point value")
		}
		return ot
	case UnaryOpNot:
		if ot != TypeUnknown && ot != TypeBool {
			tc.diags.Error(n.Span_, "operator ! requires a Bool operand, got "+tc.formatType(ot), "use a boolean operand")
		}
		return TypeBool
	case UnaryOpAddr:
		if ot == TypeUnknown {
			return TypeUnknown
		}
		if ot == TypeNull {
			tc.diags.Error(n.Span_, "cannot take the address of null", "pointers come from address-of, null, or parameters")
			return TypeUnknown
		}
		if !tc.isLvalue(n.Operand) {
			tc.diags.Error(n.Operand.nodeSpan(), "cannot take the address of this expression", "take the address of a variable, array element, field, or dereferenced pointer")
			return TypeUnknown
		}
		// The result is the address of storage, which is never null.
		return pointerType(ot, false)
	}
	return TypeUnknown
}

func (tc *TypeChecker) checkCallExpr(n *CallExpr) Type {
	switch f := n.Func.(type) {
	case *IdentExpr:
		// Compiler builtins (print, read_file, ...) are checked before user
		// declarations so they cannot be shadowed.
		if t, builtin := tc.checkBuiltinCall(n, f); builtin {
			return t
		}
		proc, direct := tc.lookupProc(f.Name)
		if binding, _, valueExists := tc.lookupBinding(f.Name); valueExists {
			if !isProcType(binding.Type) {
				tc.diags.Error(f.Span_, "binding '"+f.Name+"' is not callable", "call a declared procedure")
				return TypeUnknown
			}
			proc = tc.typeProcs[binding.Type]
			direct = proc != nil
		}
		if !direct {
			tc.diags.Error(n.Span_, "call to unknown procedure "+f.Name, "declare the procedure before calling it")
			return TypeUnknown
		}
		return tc.checkCallTo(proc, n)
	case *FieldAccessExpr:
		// 'namespace.member(...)' where namespace is an imported module.
		if imp, ok := tc.importNamespace(f.Base); ok {
			proc := tc.findImportProc(imp, f.Field)
			if proc == nil {
				tc.diags.Error(f.FieldSpan, "module '"+imp.Module+"' has no member '"+f.Field+"'", "use a declared member of the imported module")
				return TypeUnknown
			}
			return tc.checkCallTo(proc, n)
		}
		tc.diags.Error(n.Func.nodeSpan(), "expression is not callable", "call a declared procedure")
		return TypeUnknown
	default:
		tc.diags.Error(n.Func.nodeSpan(), "expression is not callable", "call a declared procedure")
		return TypeUnknown
	}
}

// checkCallTo validates a call against a resolved procedure: generic
// instantiation, argument count, and argument types.
func (tc *TypeChecker) checkCallTo(proc *ProcDecl, n *CallExpr) Type {
	// A generic procedure is instantiated at the call site with concrete type
	// arguments inferred from the argument types.
	if len(proc.TypeParams) > 0 {
		return tc.checkGenericCall(n, proc)
	}
	if len(n.Args) != len(proc.Params) {
		tc.diags.Error(n.Span_, "call to "+proc.Name+" expects "+strconv.Itoa(len(proc.Params))+" arguments, got "+strconv.Itoa(len(n.Args)), "pass the correct number of arguments")
	}
	for i, arg := range n.Args {
		if i < len(proc.Params) {
			paramType := tc.resolveTypeExpr(proc.Params[i].Type)
			tc.checkAssign(arg.nodeSpan(), paramType, arg)
		}
	}
	if len(proc.Results) > 0 {
		if proc.ErrorResult != nil {
			return errorUnionType(tupleType(tc.procValueResults(proc)), tc.procErrorResult(proc))
		}
		return tupleType(tc.procValueResults(proc))
	}
	return TypeVoid
}

// importNamespace reports whether base is an identifier naming an imported
// module namespace, and returns the resolved import.
func (tc *TypeChecker) importNamespace(base Expr) (*ImportDecl, bool) {
	id, ok := base.(*IdentExpr)
	if !ok {
		return nil, false
	}
	imp, ok := tc.imports[id.Name]
	return imp, ok
}

// findImportProc looks up a procedure by name in an import's declarations.
func (tc *TypeChecker) findImportProc(imp *ImportDecl, name string) *ProcDecl {
	for _, d := range imp.Decls {
		if p, ok := d.(*ProcDecl); ok && p.Name == name {
			return p
		}
	}
	return nil
}

// checkBuiltinCall type-checks a call to a compiler builtin procedure. It
// returns the call's result type and true when the name is a builtin. Builtins
// are recognized by name and cannot be shadowed by user declarations.
func (tc *TypeChecker) checkBuiltinCall(n *CallExpr, ident *IdentExpr) (Type, bool) {
	switch ident.Name {
	case "print", "println":
		if len(n.Args) != 1 {
			tc.diags.Error(n.Span_, "call to "+ident.Name+" expects 1 argument, got "+strconv.Itoa(len(n.Args)), "pass a String value")
			return TypeVoid, true
		}
		tc.checkAssign(n.Args[0].nodeSpan(), TypeString, n.Args[0])
		return TypeVoid, true
	case "read_file":
		if len(n.Args) != 1 {
			tc.diags.Error(n.Span_, "call to read_file expects 1 argument, got "+strconv.Itoa(len(n.Args)), "pass a file path String")
			return TypeString, true
		}
		tc.checkAssign(n.Args[0].nodeSpan(), TypeString, n.Args[0])
		return TypeString, true
	case "file_exists":
		if len(n.Args) != 1 {
			tc.diags.Error(n.Span_, "call to file_exists expects 1 argument, got "+strconv.Itoa(len(n.Args)), "pass a file path String")
			return TypeBool, true
		}
		tc.checkAssign(n.Args[0].nodeSpan(), TypeString, n.Args[0])
		return TypeBool, true
	}
	return TypeUnknown, false
}

// checkGenericCall instantiates a generic procedure at a call site: it infers
// the concrete type arguments from the argument types, checks the constraints,
// builds an instantiated copy of the procedure, type-checks the copy, and adds
// it to the program so lowering emits it.
func (tc *TypeChecker) checkGenericCall(n *CallExpr, proc *ProcDecl) Type {
	argTypes := make([]Type, len(n.Args))
	for i, arg := range n.Args {
		argTypes[i] = tc.concreteArgType(arg)
	}
	typeArgs, ok := tc.inferTypeArgs(proc, argTypes)
	if !ok {
		tc.diags.Error(n.Span_, "cannot infer type arguments for generic procedure "+proc.Name, "pass arguments whose types determine the type parameters")
		return TypeUnknown
	}
	if !tc.checkTypeConstraints(proc, typeArgs, n.Span_) {
		return TypeUnknown
	}
	// Build the concrete type-expression mapping for AST substitution.
	mapping := make(map[string]Expr, len(typeArgs))
	for name, t := range typeArgs {
		mapping[name] = tc.typeToExpr(t)
	}
	// Reuse an existing instantiation with the same type arguments so repeated
	// calls do not create duplicate declarations.
	suffix := tc.instanceSuffix(typeArgs)
	for _, existing := range tc.genericInstances[proc] {
		if strings.HasSuffix(existing.Name, suffix) {
			tc.analysis.CallInstances[n] = existing
			return tc.genericCallResult(existing, n)
		}
	}
	instance := instantiateProc(proc, mapping)
	instance.Name = proc.Name + "$" + suffix
	// Type-check the instantiated body and register it for lowering.
	tc.checkProc(instance)
	tc.registerProc(instance)
	tc.genericInstances[proc] = append(tc.genericInstances[proc], instance)
	if tc.program != nil {
		tc.program.Decls = append(tc.program.Decls, instance)
	}
	tc.analysis.CallInstances[n] = instance
	// Type-check the call against the instantiated signature.
	return tc.genericCallResult(instance, n)
}

// genericCallResult type-checks a call against an instantiated generic
// procedure's signature and returns the call's result type.
func (tc *TypeChecker) genericCallResult(instance *ProcDecl, n *CallExpr) Type {
	if len(n.Args) != len(instance.Params) {
		tc.diags.Error(n.Span_, "call to "+instance.Name+" expects "+strconv.Itoa(len(instance.Params))+" arguments, got "+strconv.Itoa(len(n.Args)), "pass the correct number of arguments")
	}
	for i, arg := range n.Args {
		if i < len(instance.Params) {
			paramType := tc.resolveTypeExpr(instance.Params[i].Type)
			tc.checkAssign(arg.nodeSpan(), paramType, arg)
		}
	}
	if len(instance.Results) > 0 {
		if instance.ErrorResult != nil {
			return errorUnionType(tupleType(tc.procValueResults(instance)), tc.procErrorResult(instance))
		}
		return tupleType(tc.procValueResults(instance))
	}
	return TypeVoid
}

// inferTypeArgs infers concrete types for a generic procedure's type
// parameters by matching each argument type against the corresponding
// parameter type.
func (tc *TypeChecker) inferTypeArgs(proc *ProcDecl, argTypes []Type) (map[string]Type, bool) {
	tc.pushTypeParams(proc.TypeParams)
	defer tc.popTypeParams()
	mapping := make(map[string]Type)
	for i, arg := range argTypes {
		if i >= len(proc.Params) {
			break
		}
		paramType := tc.resolveTypeExpr(proc.Params[i].Type)
		if !tc.matchType(paramType, arg, mapping) {
			return nil, false
		}
	}
	return mapping, true
}

// concreteArgType returns a concrete type for a call argument, resolving
// context-dependent literals to their default type so generic type inference
// can bind type parameters.
func (tc *TypeChecker) concreteArgType(e Expr) Type {
	t := tc.inferExpr(e)
	if t != TypeUnknown {
		return t
	}
	switch e.(type) {
	case *IntExpr:
		return TypeS64
	case *FloatExpr:
		return TypeF64
	case *StringExpr:
		return TypeString
	case *BoolExpr:
		return TypeBool
	}
	return TypeUnknown
}

// matchType structurally matches an argument type against a parameter type,
// binding type parameters as they are encountered. It returns false when the
// types cannot match.
func (tc *TypeChecker) matchType(param, arg Type, mapping map[string]Type) bool {
	if isTypeParam(param) {
		name := typeParamName(param)
		if existing, ok := mapping[name]; ok {
			return existing == arg
		}
		mapping[name] = arg
		return true
	}
	if isArrayType(param) && isArrayType(arg) {
		if arrayKindOf(param) != arrayKindOf(arg) {
			return false
		}
		if arrayKindOf(param) == ArrayFixed {
			ps, pok := arrayFixedSizeOf(param)
			as, aok := arrayFixedSizeOf(arg)
			if !pok || !aok || ps != as {
				return false
			}
		}
		return tc.matchType(arrayElemType(param), arrayElemType(arg), mapping)
	}
	if isPointerType(param) && isPointerType(arg) {
		if pointerNullable(param) != pointerNullable(arg) {
			return false
		}
		return tc.matchType(pointerElemType(param), pointerElemType(arg), mapping)
	}
	return param == arg
}

// checkTypeConstraints verifies that each inferred type argument satisfies its
// type parameter's constraint list.
func (tc *TypeChecker) checkTypeConstraints(proc *ProcDecl, typeArgs map[string]Type, span Span) bool {
	ok := true
	for _, tp := range proc.TypeParams {
		arg, found := typeArgs[tp.Name]
		if !found {
			continue
		}
		if len(tp.Constraints) == 0 {
			continue
		}
		satisfied := false
		for _, c := range tp.Constraints {
			if tc.resolveTypeExpr(c) == arg {
				satisfied = true
				break
			}
		}
		if !satisfied {
			tc.diags.Error(span, "type argument "+tc.formatType(arg)+" does not satisfy the constraints of type parameter '"+tp.Name+"'", "use one of the allowed constraint types")
			ok = false
		}
	}
	return ok
}

// typeToExpr converts a resolved Type back into a type expression for AST
// substitution.
func (tc *TypeChecker) typeToExpr(t Type) Expr {
	if isArrayType(t) {
		elem := tc.typeToExpr(arrayElemType(t))
		switch arrayKindOf(t) {
		case ArrayFixed:
			if size, ok := arrayFixedSizeOf(t); ok {
				return &ArrayTypeExpr{Span_: Span{}, Elem: elem, Kind: ArrayFixed, Size: &IntExpr{Value: strconv.Itoa(size)}}
			}
		case ArrayDynamic:
			return &ArrayTypeExpr{Span_: Span{}, Elem: elem, Kind: ArrayDynamic}
		default:
			return &ArrayTypeExpr{Span_: Span{}, Elem: elem, Kind: ArrayRuntime}
		}
	}
	if isPointerType(t) {
		return &PointerTypeExpr{Span_: Span{}, Elem: tc.typeToExpr(pointerElemType(t)), Nullable: pointerNullable(t)}
	}
	// A nominal type (struct, enum, error) resolves back to its declared
	// source name; the semantic type name is not a valid type expression.
	if decl, ok := tc.typeDecls[t]; ok {
		if name, _, _ := declarationName(decl); name != "" {
			return &IdentExpr{Span_: Span{}, Name: name}
		}
	}
	return &IdentExpr{Span_: Span{}, Name: string(t)}
}

// instanceSuffix builds a stable suffix for an instantiated procedure name
// from its concrete type arguments.
func (tc *TypeChecker) instanceSuffix(typeArgs map[string]Type) string {
	// Sort for determinism.
	names := make([]string, 0, len(typeArgs))
	for name := range typeArgs {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteString("=")
		b.WriteString(string(typeArgs[name]))
		b.WriteString("_")
	}
	return b.String()
}

// procErrorResult returns the error type of a '<>' procedure result,
// normalizing the written order.
func (tc *TypeChecker) procErrorResult(p *ProcDecl) Type {
	lt := tc.resolveTypeExpr(p.Results[0])
	rt := tc.resolveTypeExpr(p.ErrorResult)
	if len(p.Results) == 1 && tc.isErrorType(lt) && !tc.isErrorType(rt) {
		return lt
	}
	return rt
}

// checkArrayInitFromStruct checks an inferred ".{...}" literal assigned to an
// array type. The parser produces a StructInitExpr for ".{...}"; when the
// target is an array, its positional values are the array elements.
func (tc *TypeChecker) checkArrayInitFromStruct(span Span, target Type, si *StructInitExpr) {
	elem := arrayElemType(target)
	for _, field := range si.Fields {
		if field.Name != "" {
			tc.diags.Error(field.Span_, "array literal elements cannot be named", "use positional values")
			continue
		}
		tc.checkAssign(field.Span_, elem, field.Value)
	}
	if arrayKindOf(target) == ArrayFixed {
		if size, ok := arrayFixedSizeOf(target); ok && len(si.Fields) != size {
			tc.diags.Error(span, fmt.Sprintf("array literal has %d elements but type declares %d", len(si.Fields), size), "provide exactly the declared number of elements")
		}
	}
}

func (tc *TypeChecker) checkStructInit(si *StructInitExpr, structType Type) {
	st, ok := tc.structDeclForType(structType)
	if !ok {
		tc.diags.Error(si.Span_, "unknown struct type "+tc.formatType(structType), "use a declared struct type")
		return
	}
	// A generic struct is instantiated at the use site: infer the type
	// arguments from the struct literal's field values, then check the
	// literal against the instantiated type.
	if len(st.TypeParams) > 0 {
		tc.checkGenericStructInit(si, st)
		return
	}
	fieldTypes := make(map[string]Type, len(st.Fields))
	fieldIndexes := make(map[string]int, len(st.Fields))
	for i, f := range st.Fields {
		fieldTypes[f.Name] = tc.resolveTypeExpr(f.Type)
		fieldIndexes[f.Name] = i
	}
	assigned := make(map[int]bool, len(si.Fields))
	positional := 0
	for _, field := range si.Fields {
		if field.Name != "" {
			ft, ok := fieldTypes[field.Name]
			if !ok {
				tc.diags.Error(field.Span_, "unknown field "+field.Name+" in struct "+tc.formatType(structType), "use a declared field name")
				continue
			}
			index := fieldIndexes[field.Name]
			if assigned[index] {
				tc.diags.Error(field.NameSpan, "field '"+field.Name+"' is initialized more than once", "remove the duplicate field initializer")
				continue
			}
			assigned[index] = true
			tc.checkAssign(field.Span_, ft, field.Value)
			continue
		}
		// Positional fields occupy the next declaration-order slot.
		if positional < len(st.Fields) {
			assigned[positional] = true
			ft := tc.resolveTypeExpr(st.Fields[positional].Type)
			tc.checkAssign(field.Span_, ft, field.Value)
			positional++
		} else {
			tc.diags.Error(field.Span_, "too many fields in struct literal for "+tc.formatType(structType), "remove the extra field")
		}
	}
}

// checkGenericStructInit instantiates a generic struct at a use site by
// inferring the type arguments from the struct literal's field values, then
// checks the literal against the instantiated type.
func (tc *TypeChecker) checkGenericStructInit(si *StructInitExpr, st *StructDecl) {
	tc.pushTypeParams(st.TypeParams)
	defer tc.popTypeParams()
	// Infer type arguments by matching each field's declared type (which may
	// contain type parameters) against the field value's concrete type.
	typeArgs := make(map[string]Type)
	positional := 0
	for _, field := range si.Fields {
		var fieldDecl *StructField
		if field.Name != "" {
			for i := range st.Fields {
				if st.Fields[i].Name == field.Name {
					fieldDecl = &st.Fields[i]
					break
				}
			}
		} else {
			// Positional: match against the next declaration-order field.
			if positional < len(st.Fields) {
				fieldDecl = &st.Fields[positional]
			}
			positional++
		}
		if fieldDecl == nil {
			continue
		}
		paramType := tc.resolveTypeExpr(fieldDecl.Type)
		argType := tc.concreteArgType(field.Value)
		if argType != TypeUnknown {
			tc.matchType(paramType, argType, typeArgs)
		}
	}
	// Check constraints.
	for _, tp := range st.TypeParams {
		arg, found := typeArgs[tp.Name]
		if !found || len(tp.Constraints) == 0 {
			continue
		}
		satisfied := false
		for _, c := range tp.Constraints {
			if tc.resolveTypeExpr(c) == arg {
				satisfied = true
				break
			}
		}
		if !satisfied {
			tc.diags.Error(si.Span_, "type argument "+tc.formatType(arg)+" does not satisfy the constraints of type parameter '"+tp.Name+"'", "use one of the allowed constraint types")
		}
	}
	// Build the concrete type-expression mapping and substitute field types.
	mapping := make(map[string]Expr, len(typeArgs))
	for name, t := range typeArgs {
		mapping[name] = tc.typeToExpr(t)
	}
	// Create an instantiated struct type with the substituted field types and
	// record it so field access resolves to the concrete types.
	instance := &StructDecl{
		Span_:    st.Span_,
		Name:     st.Name + "$" + tc.instanceSuffix(typeArgs),
		NameSpan: st.NameSpan,
		Fields:   make([]StructField, len(st.Fields)),
	}
	for i, f := range st.Fields {
		instance.Fields[i] = f
		instance.Fields[i].Type = substituteTypeExpr(f.Type, mapping)
	}
	instType := tc.newSemanticType("struct", instance.Name)
	tc.instanceTypes[instance] = instType
	tc.typeDecls[instType] = instance
	tc.analysis.NominalDecls[instType] = instance
	tc.structInstances[st] = append(tc.structInstances[st], instance)
	tc.structLiteralTypes[si] = instType
	tc.analysis.StructLiteralTypes[si] = instType
	if tc.program != nil {
		tc.program.Decls = append(tc.program.Decls, instance)
	}
	fieldTypes := make(map[string]Type, len(instance.Fields))
	fieldIndexes := make(map[string]int, len(instance.Fields))
	for i, f := range instance.Fields {
		fieldTypes[f.Name] = tc.resolveTypeExpr(f.Type)
		fieldIndexes[f.Name] = i
	}
	assigned := make(map[int]bool, len(si.Fields))
	positional = 0
	for _, field := range si.Fields {
		if field.Name != "" {
			ft, ok := fieldTypes[field.Name]
			if !ok {
				tc.diags.Error(field.Span_, "unknown field "+field.Name+" in struct "+st.Name, "use a declared field name")
				continue
			}
			index := fieldIndexes[field.Name]
			if assigned[index] {
				tc.diags.Error(field.NameSpan, "field '"+field.Name+"' is initialized more than once", "remove the duplicate field initializer")
				continue
			}
			assigned[index] = true
			tc.checkAssign(field.Span_, ft, field.Value)
			continue
		}
		if positional < len(instance.Fields) {
			assigned[positional] = true
			ft := tc.resolveTypeExpr(instance.Fields[positional].Type)
			tc.checkAssign(field.Span_, ft, field.Value)
			positional++
		} else {
			tc.diags.Error(field.Span_, "too many fields in struct literal for "+st.Name, "remove the extra field")
		}
	}
}

// typeOfTypeExpr extracts the type name from a type expression: an identifier
// or an array type "[]T".
func typeOfTypeExpr(e Expr) Type {
	switch n := e.(type) {
	case *IdentExpr:
		return Type(n.Name)
	case *ArrayTypeExpr:
		elem := typeOfTypeExpr(n.Elem)
		switch n.Kind {
		case ArrayFixed:
			if lit, ok := n.Size.(*IntExpr); ok {
				if sz, err := strconv.Atoi(strings.ReplaceAll(lit.Value, "_", "")); err == nil {
					return fixedArrayType(elem, sz)
				}
			}
			return TypeUnknown
		case ArrayDynamic:
			return dynamicArrayType(elem)
		default:
			return arrayType(elem)
		}
	}
	return TypeUnknown
}

func declarationName(d Decl) (string, Span, bool) {
	switch n := d.(type) {
	case *VarDecl:
		return n.Name, n.NameSpan, n.Shadow
	case *ProcDecl:
		return n.Name, n.NameSpan, n.Shadow
	case *StructDecl:
		return n.Name, n.NameSpan, n.Shadow
	case *ErrorDecl:
		return n.Name, n.NameSpan, n.Shadow
	case *EnumDecl:
		return n.Name, n.NameSpan, n.Shadow
	case *ImportDecl:
		// A namespaced import binds its namespace name like any other
		// declaration, so 'c :: #import «core»;' collides with a second
		// top-level 'c'. Flat imports bind nothing.
		if n.Namespace != "" {
			return n.Namespace, n.Span_, false
		}
	}
	return "", Span{}, false
}

func isReservedInternalName(name string) bool { return strings.HasPrefix(name, "__chaos_") }

func tupleType(types []Type) Type {
	if len(types) == 0 {
		return TypeVoid
	}
	if len(types) == 1 {
		return types[0]
	}
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = string(t)
	}
	return Type("(" + strings.Join(parts, ",") + ")")
}

func tupleTypes(t Type) []Type {
	s := string(t)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return []Type{t}
	}
	parts := strings.Split(s[1:len(s)-1], ",")
	result := make([]Type, len(parts))
	for i, part := range parts {
		result[i] = Type(part)
	}
	return result
}

func (tc *TypeChecker) formatTypes(types []Type) string {
	if len(types) == 0 {
		return "no values"
	}
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = tc.formatType(t)
	}
	return strings.Join(parts, ", ")
}

func isProcType(t Type) bool { return strings.HasPrefix(string(t), "\x00proc:") }

// arrayType is the type name of a runtime-sized array "[]T".
func arrayType(elem Type) Type {
	return Type("[]" + string(elem))
}

// fixedArrayType is the type name of a fixed-size array "[N]T".
func fixedArrayType(elem Type, n int) Type {
	return Type(fmt.Sprintf("[%d]%s", n, string(elem)))
}

// dynamicArrayType is the type name of a dynamic array "[dyn]T".
func dynamicArrayType(elem Type) Type {
	return Type("[dyn]" + string(elem))
}

// isArrayType reports whether t is an array type ("[]T", "[N]T", or "[dyn]T").
func isArrayType(t Type) bool {
	s := string(t)
	if strings.HasPrefix(s, "[]") || strings.HasPrefix(s, "[dyn]") {
		return true
	}
	if len(s) > 0 && s[0] == '[' {
		if i := strings.IndexByte(s, ']'); i > 0 {
			for j := 1; j < i; j++ {
				if s[j] < '0' || s[j] > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}

// arrayElemType returns the element type of an array type.
func arrayElemType(t Type) Type {
	s := string(t)
	if strings.HasPrefix(s, "[]") {
		return Type(s[2:])
	}
	if strings.HasPrefix(s, "[dyn]") {
		return Type(s[5:])
	}
	if i := strings.IndexByte(s, ']'); i > 0 {
		return Type(s[i+1:])
	}
	return t
}

// arrayKindOf returns the array kind of an array type.
func arrayKindOf(t Type) ArrayKind {
	s := string(t)
	if strings.HasPrefix(s, "[dyn]") {
		return ArrayDynamic
	}
	if strings.HasPrefix(s, "[]") {
		return ArrayRuntime
	}
	return ArrayFixed
}

// arrayFixedSizeOf returns the compile-time size of a fixed-size array type.
func arrayFixedSizeOf(t Type) (int, bool) {
	s := string(t)
	if !strings.HasPrefix(s, "[") {
		return 0, false
	}
	i := strings.IndexByte(s, ']')
	if i <= 1 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(s[1:i], "_", ""))
	if err != nil {
		return 0, false
	}
	return n, true
}

// arrayFixedSize resolves the compile-time size of a fixed-size array type
// expression "[N]T". The size must be a non-negative integer literal.
func (tc *TypeChecker) arrayFixedSize(at *ArrayTypeExpr) (int, bool) {
	if at.Size == nil {
		tc.diags.Error(at.Span_, "fixed-size array is missing its size", "write '[N]T' with an integer size")
		return 0, false
	}
	lit, ok := at.Size.(*IntExpr)
	if !ok {
		tc.diags.Error(at.Size.nodeSpan(), "array size must be a compile-time integer constant", "use a literal integer size")
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(lit.Value, "_", ""))
	if err != nil || n < 0 {
		tc.diags.Error(at.Size.nodeSpan(), "invalid array size", "use a non-negative integer size")
		return 0, false
	}
	return n, true
}

// pointerType is the type name of a pointer with the given element type.
func pointerType(elem Type, nullable bool) Type {
	if nullable {
		return Type("*" + string(elem) + "?")
	}
	return Type("*" + string(elem))
}

// isPointerType reports whether t is a pointer type.
func isPointerType(t Type) bool {
	return strings.HasPrefix(string(t), "*")
}

// pointerElemType returns the pointed-to element type of a pointer type.
func pointerElemType(t Type) Type {
	s := string(t)
	if strings.HasSuffix(s, "?") {
		s = s[:len(s)-1]
	}
	return Type(strings.TrimPrefix(s, "*"))
}

// pointerNullable reports whether a pointer type is nullable.
func pointerNullable(t Type) bool {
	return strings.HasSuffix(string(t), "?")
}

// checkPointerBinary validates binary operators with pointer or null
// operands. It returns (resultType, true) when the operator involves a
// pointer or null; otherwise (TypeUnknown, false) leaves the general rules to
// their ordinary path.
func (tc *TypeChecker) checkPointerBinary(b *BinaryExpr, lt, rt Type) (Type, bool) {
	leftPtr, rightPtr := isPointerType(lt), isPointerType(rt)
	nullLT, nullRT := lt == TypeNull, rt == TypeNull

	switch b.Op {
	case BinaryOpAdd, BinaryOpSub:
		if leftPtr && rightPtr {
			if b.Op == BinaryOpAdd {
				tc.diags.Error(b.Span_, "cannot add two pointers", "add an integer offset: 'p + n'")
				return TypeUnknown, true
			}
			if lt != rt {
				tc.diags.Error(b.Span_, "cannot subtract pointers of different types", "subtract pointers of the same type: 'p - q'")
			}
			return TypeS64, true // element difference
		}
		if leftPtr && isIntegerType(rt) {
			return lt, true // scaled by sizeof(element)
		}
		if rightPtr && isIntegerType(lt) {
			return rt, true
		}
		if leftPtr || rightPtr {
			other := lt
			if leftPtr {
				other = rt
			}
			tc.diags.Error(b.Span_, "pointer arithmetic requires an integer offset, got "+tc.formatType(other), "add or subtract an integer: 'p + n'")
			return TypeUnknown, true
		}
		return TypeUnknown, false

	case BinaryOpEq, BinaryOpNeq:
		if leftPtr && rightPtr {
			if lt != rt {
				tc.diags.Error(b.Span_, "cannot compare "+tc.formatType(lt)+" and "+tc.formatType(rt), "compare pointers of the same type")
			}
			return TypeBool, true
		}
		if (leftPtr && nullRT) || (rightPtr && nullLT) {
			ptrType := lt
			if rightPtr {
				ptrType = rt
			}
			if !pointerNullable(ptrType) {
				tc.diags.Error(b.Span_, "cannot compare a non-nullable pointer with null", "non-nullable pointers can never be null")
			}
			// Record the null operand's pointer type so lowering can
			// materialize it as a zero address of the right type.
			if tc.analysis != nil {
				if b.Left != nil {
					if _, isNull := b.Left.(*NullLitExpr); isNull {
						tc.analysis.ExprTypes[b.Left] = ptrType
					}
				}
				if b.Right != nil {
					if _, isNull := b.Right.(*NullLitExpr); isNull {
						tc.analysis.ExprTypes[b.Right] = ptrType
					}
				}
			}
			return TypeBool, true
		}
		if nullLT || nullRT {
			tc.diags.Error(b.Span_, "null can only be compared with a nullable pointer", "compare null with a nullable pointer type like '*T?'")
			return TypeBool, true
		}
		return TypeUnknown, false

	case BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe:
		if leftPtr && rightPtr {
			if lt != rt {
				tc.diags.Error(b.Span_, "cannot compare "+tc.formatType(lt)+" and "+tc.formatType(rt), "compare pointers of the same type")
			}
			return TypeBool, true
		}
		if leftPtr || rightPtr || nullLT || nullRT {
			tc.diags.Error(b.Span_, "ordering is not defined for pointers and null", "use == or != to compare against null")
			return TypeBool, true
		}
		return TypeUnknown, false
	}
	return TypeUnknown, false
}

// checkAssign validates that a value can be assigned to a target type. Literals
// adapt to a compatible target type (an integer literal to any integer type, a
// float literal to any float type); other expressions must match exactly. A
// bare error member ('.MEMBER') takes the target error type.
func (tc *TypeChecker) checkAssign(span Span, target Type, value Expr) {
	if target == TypeUnknown || value == nil {
		return
	}
	if ifx, ok := unwrapParens(value).(*IfxExpr); ok {
		if !tc.ifxContext {
			tc.diags.Error(ifx.Span_, "ifx expressions are only supported as the value of an assignment or return", "use ifx directly as the assigned or returned value")
			return
		}
		tc.checkIfxAssign(ifx, target)
		tc.analysis.ExprTypes[value] = target
		return
	}
	// The null literal and pointer targets follow dedicated rules.
	if _, isNull := value.(*NullLitExpr); isNull {
		if isPointerType(target) {
			if pointerNullable(target) {
				tc.analysis.ExprTypes[value] = target
				return
			}
			tc.diags.Error(span, "cannot assign null to a non-nullable pointer of type "+tc.formatType(target), "declare the variable as a nullable pointer '"+tc.formatType(target)+"?' if it may hold null")
		} else {
			tc.diags.Error(span, "cannot assign null to "+tc.formatType(target), "only nullable pointers can hold null")
		}
		tc.analysis.ExprTypes[value] = target
		return
	}
	if target == TypeAddr {
		// Addr accepts any pointer value implicitly; it is an opaque address.
		valType := tc.inferExpr(value)
		if isPointerType(valType) || valType == TypeAddr {
			tc.analysis.ExprTypes[value] = target
			return
		}
		tc.diags.Error(span, "cannot assign "+tc.formatType(valType)+" to Addr", "use a pointer value")
		tc.analysis.ExprTypes[value] = target
		return
	}
	if isPointerType(target) {
		valType := tc.inferExpr(value)
		if valType == target {
			tc.analysis.ExprTypes[value] = target
			return
		}
		if isPointerType(valType) {
			sameBase := pointerType(pointerElemType(valType), false) == pointerType(pointerElemType(target), false)
			switch {
			case sameBase && pointerNullable(target):
				// A nullable target accepts any same-base pointer, including
				// values that are provably non-null.
				tc.analysis.ExprTypes[value] = target
				return
			case sameBase && !pointerNullable(target) && pointerNullable(valType) && !tc.exprIsNonNull(value):
				tc.diags.Error(span, "cannot assign the nullable pointer to "+tc.formatType(target), "prove the value is not null first (for example 'if p == null { return; }')")
			case sameBase:
				// Same-base non-nullable assignment, or a flow-narrowed
				// nullable value assigned to a non-null target.
				tc.analysis.ExprTypes[value] = target
				return
			default:
				tc.diags.Error(span, "cannot assign "+tc.formatType(valType)+" to "+tc.formatType(target), "use a pointer value of type "+tc.formatType(target))
			}
		} else {
			tc.diags.Error(span, "cannot assign "+tc.formatType(valType)+" to "+tc.formatType(target), "use a pointer value of type "+tc.formatType(target))
		}
		tc.analysis.ExprTypes[value] = target
		return
	}
	if isLiteral(value) {
		if !literalCompatible(value, target) {
			valType := tc.inferExpr(value)
			tc.diags.Error(span, "cannot assign "+tc.formatType(valType)+" to "+tc.formatType(target), "use a value of type "+tc.formatType(target))
		}
		tc.checkLiteralRange(value, target)
		tc.analysis.ExprTypes[value] = target
		return
	}
	if isIntegerType(target) {
		_, known, _ := tc.evalConstInt(value, target, make(map[Expr]bool))
		if known {
			tc.markConstIntType(value, target, make(map[Expr]bool))
			return
		}
	}
	if isFloatType(target) {
		_, known, _ := tc.evalConstFloat(value, target, make(map[Expr]bool))
		if known {
			tc.markConstIntType(value, target, make(map[Expr]bool))
			return
		}
	}
	if init, ok := value.(*StructInitExpr); ok && init.Type == nil {
		if isArrayType(target) {
			tc.checkArrayInitFromStruct(span, target, init)
			tc.analysis.ExprTypes[value] = target
			return
		}
		tc.checkStructInit(init, target)
		tc.analysis.ExprTypes[value] = target
		return
	}
	if em, ok := value.(*ErrorMemberExpr); ok && em.TypeName == "" {
		if tc.isErrorType(target) {
			tc.checkErrorMember(em, target)
			tc.analysis.ExprTypes[value] = target
			return
		}
		tc.diags.Error(span, "cannot assign an error member to "+tc.formatType(target), "use a value of type "+tc.formatType(target))
		return
	}
	if em, ok := value.(*EnumMemberExpr); ok && em.TypeName == "" {
		if tc.isEnumType(target) {
			tc.checkEnumMember(em, target)
			tc.analysis.ExprTypes[value] = target
			return
		}
		if tc.isErrorType(target) {
			tc.diags.Error(span, "error values must be instantiated with '!'", "add '!' after the member name")
			return
		}
		tc.diags.Error(span, "cannot assign an enum member to "+tc.formatType(target), "use a value of type "+tc.formatType(target))
		return
	}
	valType := tc.inferExpr(value)
	if isErrorUnion(valType) {
		tc.diags.Error(span, "must handle the error before using the value", "use 'unless catch' or 'if ... catch' first")
		return
	}
	if valType != TypeUnknown && valType != target {
		tc.diags.Error(span, "cannot assign "+tc.formatType(valType)+" to "+tc.formatType(target), "use a value of type "+tc.formatType(target))
	}
}

// unwrapParens strips transparent parenthesization from an expression.
func unwrapParens(e Expr) Expr {
	for {
		p, ok := e.(*ParenExpr)
		if !ok {
			return e
		}
		e = p.Inner
	}
}

// isIfxValue reports whether an expression is an ifx expression, looking
// through transparent parentheses.
func isIfxValue(e Expr) bool {
	_, ok := unwrapParens(e).(*IfxExpr)
	return ok
}

// withIfxContext runs fn with the ifx context flag set when value is an ifx
// expression (possibly parenthesized). The flag permits an ifx expression to
// be type-checked as the complete value of an assignment or return; every
// other expression position leaves it unset so the type checker can reject
// ifx with a precise diagnostic.
func (tc *TypeChecker) withIfxContext(value Expr, fn func()) {
	if isIfxValue(value) {
		prev := tc.ifxContext
		tc.ifxContext = true
		fn()
		tc.ifxContext = prev
		return
	}
	fn()
}

// checkIfxExpr infers the type of an ifx expression with no expected target
// type. The condition must be Bool and both branches must have the same type;
// a literal branch adapts to the other branch's type.
func (tc *TypeChecker) checkIfxExpr(n *IfxExpr) Type {
	// A nested ifx expression is not the direct value of an assignment or
	// return, so the condition and both branches are checked with the flag
	// cleared.
	prev := tc.ifxContext
	tc.ifxContext = false
	condType := tc.inferExpr(n.Condition)
	thenType := tc.inferExpr(n.Then)
	elseType := tc.inferExpr(n.Else)
	tc.ifxContext = prev
	if condType != TypeUnknown && condType != TypeBool {
		tc.diags.Error(n.Condition.nodeSpan(), "ifx condition must be Bool, got "+tc.formatType(condType), "use a boolean condition")
	}
	compatible := tc.operandsCompatible(n.Then, thenType, n.Else, elseType)
	effective := thenType
	if isLiteral(n.Then) && !isLiteral(n.Else) {
		effective = elseType
	}
	if compatible {
		if isLiteral(n.Then) {
			tc.checkLiteralRange(n.Then, effective)
			tc.analysis.ExprTypes[n.Then] = effective
		}
		if isLiteral(n.Else) {
			tc.checkLiteralRange(n.Else, effective)
			tc.analysis.ExprTypes[n.Else] = effective
		}
	} else {
		tc.diags.Error(n.Span_, "ifx branches must have the same type, got "+tc.formatType(thenType)+" and "+tc.formatType(elseType), "use values of the same type")
	}
	return effective
}

// checkIfxAssign checks an ifx expression against a known target type: the
// condition must be Bool and both branches must be assignable to the target.
func (tc *TypeChecker) checkIfxAssign(n *IfxExpr, target Type) {
	// A nested ifx expression is not the direct value of an assignment or
	// return, so the condition and both branches are checked with the flag
	// cleared.
	prev := tc.ifxContext
	tc.ifxContext = false
	condType := tc.inferExpr(n.Condition)
	tc.checkAssign(n.Then.nodeSpan(), target, n.Then)
	tc.checkAssign(n.Else.nodeSpan(), target, n.Else)
	tc.ifxContext = prev
	if condType != TypeUnknown && condType != TypeBool {
		tc.diags.Error(n.Condition.nodeSpan(), "ifx condition must be Bool, got "+tc.formatType(condType), "use a boolean condition")
	}
	tc.analysis.ExprTypes[n] = target
}

// checkErrorMember validates a bare error member reference against a target
// error type. Error literals require the trailing '!'.
func (tc *TypeChecker) checkErrorMember(em *ErrorMemberExpr, t Type) {
	if !em.Bang {
		tc.diags.Error(em.Span_, "error values must be instantiated with '!'", "add '!' after the member name")
		return
	}
	if !tc.isErrorType(t) {
		tc.diags.Error(em.Span_, tc.formatType(t)+" is not an error type", "use a declared error type")
		return
	}
	if !tc.hasErrorMember(string(t), em.Name) {
		tc.diags.Error(em.Span_, "unknown error member "+em.Name+" in "+tc.formatType(t), "use a declared error member")
	}
}

// operandsCompatible reports whether two operands can be combined by an
// operator. Unknown types are treated as compatible; a literal may adapt to the
// other operand's type.
func (tc *TypeChecker) operandsCompatible(left Expr, lt Type, right Expr, rt Type) bool {
	if lt == TypeUnknown || rt == TypeUnknown {
		return true
	}
	if lt == rt {
		return true
	}
	if isLiteral(left) && literalCompatible(left, rt) {
		return true
	}
	if isLiteral(right) && literalCompatible(right, lt) {
		return true
	}
	return false
}

// isLiteral reports whether an expression is a literal that can adapt to a
// target type. A negated literal (for example -7) is treated as a literal too.
func isLiteral(e Expr) bool {
	switch n := e.(type) {
	case *IntExpr, *FloatExpr, *StringExpr, *BoolExpr:
		return true
	case *UnaryExpr:
		return n.Op == UnaryOpNeg && isLiteral(n.Operand)
	}
	return false
}

// isNullLit reports whether an expression is the null literal, looking
// through transparent parentheses.
func isNullLit(e Expr) bool {
	_, ok := unwrapParens(e).(*NullLitExpr)
	return ok
}

// literalCompatible reports whether a literal can be assigned to a target type.
func literalCompatible(lit Expr, target Type) bool {
	switch n := lit.(type) {
	case *IntExpr:
		return isIntegerType(target)
	case *FloatExpr:
		return isFloatType(target)
	case *StringExpr:
		return target == TypeString
	case *BoolExpr:
		return target == TypeBool
	case *UnaryExpr:
		return n.Op == UnaryOpNeg && literalCompatible(n.Operand, target)
	}
	return false
}

func isIntegerType(t Type) bool {
	info, ok := LookupBuiltinType(string(t))
	return ok && info.Kind == BuiltinInteger
}

func isFloatType(t Type) bool {
	info, ok := LookupBuiltinType(string(t))
	return ok && info.Kind == BuiltinFloat
}
