// Type checking for the Chaos language.
//
// The type checker walks a parsed AST and validates types. It infers the type
// of literals and expressions, checks that assignments and comparisons operate
// on matching types, and verifies return statements against the enclosing
// procedure signature. Chaos is a strict typed language: values of different
// types cannot be assigned, compared, or combined.
package compiler

import (
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
	TypeUnknown Type = ""
)

// TypeChecker validates the types of a parsed program. It maintains a stack of
// lexical scopes mapping names to their inferred or declared types.
type TypeChecker struct {
	diags                  DiagnosticList
	scopes                 []map[string]Type
	procs                  map[string]*ProcDecl
	structs                map[string]*StructDecl
	errors                 map[string]*ErrorDecl
	globalPrevious         map[*VarDecl]*VarDecl
	globalTypes            map[*VarDecl]Type
	currentReturnType      Type
	currentErrorReturnType Type // error type of a '<>' result ("" when none)
	loopDepth              int  // nesting depth of for loops (for break/continue)
	rangeElemType          Type // element type of the innermost range loop ("" when none)
	rangeIndexType         Type // index type of the innermost range loop ("" when none)
}

// CheckProgram runs the type checker over a parsed program and returns any
// diagnostics it produced.
func CheckProgram(program *Program) DiagnosticList {
	tc := &TypeChecker{
		procs:          make(map[string]*ProcDecl),
		structs:        make(map[string]*StructDecl),
		errors:         make(map[string]*ErrorDecl),
		globalPrevious: make(map[*VarDecl]*VarDecl),
		globalTypes:    make(map[*VarDecl]Type),
	}
	tc.checkProgram(program)
	return tc.diags
}

func (tc *TypeChecker) pushScope() {
	tc.scopes = append(tc.scopes, map[string]Type{})
}

func (tc *TypeChecker) popScope() {
	tc.scopes = tc.scopes[:len(tc.scopes)-1]
}

func (tc *TypeChecker) declare(name string, t Type) {
	tc.scopes[len(tc.scopes)-1][name] = t
}

// lookup returns the type of a name and whether it was declared, searching
// from the innermost scope outward. The boolean distinguishes an undeclared
// name from a declared name whose type is not known yet.
func (tc *TypeChecker) lookup(name string) (Type, bool) {
	for i := len(tc.scopes) - 1; i >= 0; i-- {
		if t, ok := tc.scopes[i][name]; ok {
			return t, true
		}
	}
	return TypeUnknown, false
}

func (tc *TypeChecker) checkProgram(program *Program) {
	tc.pushScope() // global scope

	// Register named declarations first so a variable cannot evade the
	// no-shadowing rule merely by appearing before a type declaration.
	seenProcs := make(map[string]bool)
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *ProcDecl:
			if seenProcs[d.Name] {
				tc.diags.Error(d.Span_, "procedure '"+d.Name+"' is declared more than once", "choose a different procedure name")
			}
			seenProcs[d.Name] = true
			tc.procs[d.Name] = d
		case *StructDecl:
			tc.structs[d.Name] = d
		case *ErrorDecl:
			tc.errors[d.Name] = d
		}
	}

	// Register all global values and types so bodies can reference them
	// regardless of source order. Keep the previous variable declaration for
	// an explicit top-level shadow initializer.
	latestGlobals := make(map[string]*VarDecl)
	seenTypes := make(map[string]string)
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *ProcDecl:
		case *StructDecl:
			if previous, ok := seenTypes[d.Name]; ok {
				tc.reportTypeShadow(d.Name, previous, d.Span_)
			}
			seenTypes[d.Name] = "struct"
			tc.declare(d.Name, Type(d.Name))
			// Void is only valid as a function result type, so a struct field
			// of type Void is rejected.
			for _, f := range d.Fields {
				tc.checkTypeExprValid(f.Type)
			}
		case *ErrorDecl:
			if previous, ok := seenTypes[d.Name]; ok {
				tc.reportTypeShadow(d.Name, previous, d.Span_)
			}
			seenTypes[d.Name] = "error"
			tc.declare(d.Name, Type(d.Name))
			// Error members are numbered sequentially from 0; duplicate
			// member names would make the numbering ambiguous.
			seen := make(map[string]bool, len(d.Members))
			for _, m := range d.Members {
				if seen[m.Name] {
					tc.diags.Error(m.Span_, "duplicate error member "+m.Name+" in "+d.Name, "use a unique member name")
				}
				seen[m.Name] = true
			}
		case *VarDecl:
			tc.checkShadow(d.Name, d.Span_, d.Shadow)
			if previous := latestGlobals[d.Name]; previous != nil {
				tc.globalPrevious[d] = previous
			}
			latestGlobals[d.Name] = d
			declType := TypeUnknown
			if d.DeclType != nil {
				declType = typeOfTypeExpr(d.DeclType)
			}
			tc.globalTypes[d] = declType
			tc.declare(d.Name, declType)
		}
	}

	// Resolve globals in declaration order. A shadowing initializer sees the
	// previous declaration, while unrelated globals retain forward visibility.
	for _, decl := range program.Decls {
		d, ok := decl.(*VarDecl)
		if !ok {
			continue
		}
		if previous := tc.globalPrevious[d]; previous != nil {
			tc.scopes[0][d.Name] = tc.globalTypes[previous]
		} else {
			tc.scopes[0][d.Name] = tc.globalTypes[d]
		}
		t := tc.checkVarDecl(d)
		tc.globalTypes[d] = t
		tc.scopes[0][d.Name] = t
	}
	// Procedures see the final global binding regardless of source order.
	for name, decl := range latestGlobals {
		tc.scopes[0][name] = tc.globalTypes[decl]
	}
	for _, decl := range program.Decls {
		if d, ok := decl.(*ProcDecl); ok {
			tc.checkProc(d)
		}
	}

	tc.popScope()
}

func (tc *TypeChecker) checkProc(p *ProcDecl) {
	tc.pushScope()
	for _, param := range p.Params {
		tc.checkTypeExprValid(param.Type)
		tc.declare(param.Name, typeOfTypeExpr(param.Type))
	}
	tc.checkProcResultTypes(p)
	tc.currentReturnType = TypeVoid
	tc.currentErrorReturnType = ""
	if len(p.Results) > 0 {
		tc.currentReturnType = tc.procValueResult(p)
	}
	if p.ErrorResult != nil {
		lt := typeOfTypeExpr(p.Results[0])
		rt := typeOfTypeExpr(p.ErrorResult)
		switch {
		case tc.isErrorType(rt) && !tc.isErrorType(lt):
			tc.currentErrorReturnType = rt
		case tc.isErrorType(lt) && !tc.isErrorType(rt):
			// 'Some_Error <> String' is the same as 'String <> Some_Error'.
			tc.currentErrorReturnType = lt
		case tc.isErrorType(lt) && tc.isErrorType(rt):
			tc.diags.Error(p.ErrorResult.nodeSpan(), "a result can carry only one error type", "use one error type after '<>'")
			tc.currentErrorReturnType = rt
		default:
			tc.diags.Error(p.ErrorResult.nodeSpan(), string(rt)+" is not an error type", "use a declared error type after '<>'")
			tc.currentErrorReturnType = rt
		}
	}
	if p.Body != nil {
		tc.checkBlock(p.Body)
	}
	tc.popScope()
}

// checkProcResultTypes allows Void only when it is the complete value result
// of a procedure. In an error-return signature either side may be the value
// side, so direct Void is allowed on both sides and the existing error-type
// validation determines which side is the error. Nested forms such as
// []Void remain invalid.
func (tc *TypeChecker) checkProcResultTypes(p *ProcDecl) {
	if p.ErrorResult != nil {
		tc.checkErrorReturnComponent(p.Results[0])
		tc.checkErrorReturnComponent(p.ErrorResult)
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
	vt := typeOfTypeExpr(p.Results[0])
	if p.ErrorResult != nil {
		et := typeOfTypeExpr(p.ErrorResult)
		if tc.isErrorType(vt) && !tc.isErrorType(et) {
			return et
		}
	}
	return vt
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
		t := tc.checkVarDecl(n)
		tc.checkShadow(n.Name, n.Span_, n.Shadow)
		tc.declare(n.Name, t)
	case *AssignStmt:
		declType, ok := tc.lookup(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "assignment to undeclared variable "+n.Name, "declare the variable before assigning to it")
			return
		}
		tc.checkAssign(n.Span_, declType, n.Value)
	case *ReturnStmt:
		tc.checkReturnStmt(n)
	case *ExitStmt:
		if n.Status != nil {
			st := tc.inferExpr(n.Status)
			if isErrorUnion(st) {
				tc.diags.Error(n.Status.nodeSpan(), "must handle the error before using the value", "use 'unless catch' or 'if ... catch' first")
			}
		}
		if n.Message != nil {
			tc.inferExpr(n.Message)
		}
	case *IfStmt:
		condType := tc.inferExpr(n.Condition)
		if condType != TypeUnknown && condType != TypeBool {
			tc.diags.Error(n.Condition.nodeSpan(), "if condition must be Bool, got "+string(condType), "use a boolean condition")
		}
		tc.checkBlock(n.Body)
		for _, elif := range n.Elif {
			et := tc.inferExpr(elif.Condition)
			if et != TypeUnknown && et != TypeBool {
				tc.diags.Error(elif.Condition.nodeSpan(), "elif condition must be Bool, got "+string(et), "use a boolean condition")
			}
			tc.checkBlock(elif.Body)
		}
		if n.ElseBody != nil {
			tc.checkBlock(n.ElseBody)
		}
	case *BlockStmt:
		tc.checkBlock(n)
	case *ExprStmt:
		tc.inferExpr(n.Expr)
	case *ProcDecl:
		tc.checkProc(n)
	case *StructDecl:
		// Struct declarations are registered in the first pass.
	case *ErrorDecl:
		// Error declarations are registered in the first pass.
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
	case *CompoundAssignStmt:
		declType, ok := tc.lookup(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "assignment to undeclared variable "+n.Name, "declare the variable before assigning to it")
			return
		}
		tc.checkAssign(n.Span_, declType, n.Value)
	case *IncDecStmt:
		declType, ok := tc.lookup(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "cannot modify undeclared variable "+n.Name, "declare the variable before using it")
			return
		}
		if !isIntegerType(declType) {
			tc.diags.Error(n.Span_, "cannot increment or decrement a value of type "+string(declType), "use an integer variable")
		}
	}
}

// checkUnlessCatch checks "target := expr unless catch [err] { body }" (or
// the bare form with Target == ""). The expression must be an error-returning
// value; the catch body must return or exit, and after the statement the
// target holds the unwrapped value.
func (tc *TypeChecker) checkUnlessCatch(n *UnlessCatchStmt) {
	initType := tc.inferExpr(n.Init)
	if n.Target != "" {
		// A 'Void <> E' value has nothing to bind; only the bare form is
		// meaningful.
		if unionValueType(initType) == TypeVoid {
			tc.diags.Error(n.Span_, "cannot bind a Void value; use the bare 'unless catch' form", "remove the target name")
			return
		}
		tc.checkShadow(n.Target, n.TargetSpan, n.Shadow)
		tc.declare(n.Target, initType)
	}
	if !isErrorUnion(initType) {
		tc.diags.Error(n.Init.nodeSpan(), "unless catch requires an error-returning expression, got "+string(initType), "use an expression that can return an error")
		return
	}
	tc.checkCatchBody(n.CatchBody, unionErrorType(initType), n.CatchName, n.CatchNameSpan)
	if n.Target != "" {
		tc.updateType(n.Target, unionValueType(initType))
	}
}

// checkIfCatch checks "if expr catch [err] { body }". The condition must be a
// variable holding an error-returning value; the catch body must return or
// exit, and after the statement the variable holds the unwrapped value.
func (tc *TypeChecker) checkIfCatch(n *IfCatchStmt) {
	condType := tc.inferExpr(n.Cond)
	if !isErrorUnion(condType) {
		tc.diags.Error(n.Cond.nodeSpan(), "catch requires an error-returning value, got "+string(condType), "use a value that can return an error")
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
	if n.Range != nil {
		rt := tc.inferExpr(n.Range)
		if !isArrayType(rt) {
			tc.diags.Error(n.Range.nodeSpan(), "for range requires an array, got "+string(rt), "use an array value")
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
				tc.diags.Error(n.Cond.nodeSpan(), "for condition must be Bool, got "+string(ct), "use a boolean condition")
			}
			tc.checkBlock(n.Body)
		}
	}
	if n.After != nil {
		tc.checkStmt(n.After)
	}
	tc.loopDepth--
}

// checkCatchBody checks a catch body: it must return or exit (so the value is
// always defined afterward), and the optional error binding is declared in the
// body's scope.
func (tc *TypeChecker) checkCatchBody(body *BlockStmt, errorType Type, catchName string, catchNameSpan Span) {
	if !blockDiverges(body) {
		tc.diags.Error(body.Span_, "the catch block must return or exit", "end the catch block with a return or exit")
	}
	tc.pushScope()
	if catchName != "" {
		tc.checkShadow(catchName, catchNameSpan, false)
		tc.declare(catchName, errorType)
	}
	tc.checkBlock(body)
	tc.popScope()
}

// checkShadow reports whether declaring 'name' shadows an existing name and
// emits the appropriate diagnostic. Shadowing a struct or error type is never
// allowed; shadowing a value name requires the explicit '#shadow' directive.
// Parameter names are placeholders and are never checked.
func (tc *TypeChecker) checkShadow(name string, span Span, shadow bool) {
	if _, ok := tc.structs[name]; ok {
		tc.diags.Error(span, "declaration of '"+name+"' shadows a struct type; this is not allowed", "choose a different name")
		return
	}
	if _, ok := tc.errors[name]; ok {
		tc.diags.Error(span, "declaration of '"+name+"' shadows an error type; this is not allowed", "choose a different name")
		return
	}
	if _, ok := tc.procs[name]; ok {
		if shadow && len(tc.scopes) == 1 {
			tc.diags.Error(span, "a top-level variable cannot shadow procedure '"+name+"'", "choose a different variable name")
			return
		}
		if !shadow {
			tc.diags.Error(span, "declaration of '"+name+"' shadows an existing procedure; use '#shadow' or rename", "add '#shadow' before the declaration or choose a new name")
			return
		}
	}
	if _, ok := tc.lookup(name); ok && !shadow {
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
		if _, ok := tc.scopes[i][name]; ok {
			tc.scopes[i][name] = t
			return
		}
	}
}

// blockDiverges reports whether a block always returns or exits: its last
// statement diverges.
func blockDiverges(b *BlockStmt) bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	return stmtDiverges(b.Stmts[len(b.Stmts)-1])
}

// stmtDiverges reports whether a statement always returns or exits.
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
	}
	return false
}

// checkVarDecl infers the type of a variable declaration and checks that the
// initializer matches the declared type when both are present. It returns the
// resolved type of the declaration.
func (tc *TypeChecker) checkVarDecl(d *VarDecl) Type {
	var declType Type
	if d.DeclType != nil {
		declType = typeOfTypeExpr(d.DeclType)
		tc.checkTypeExprValid(d.DeclType)
	}
	if d.Init == nil {
		return declType
	}

	// An inferred struct literal (.{...}) takes its type from the declaration.
	if si, ok := d.Init.(*StructInitExpr); ok && si.Type == nil {
		if declType != TypeUnknown {
			tc.checkStructInit(si, declType)
		}
		return declType
	}

	if declType != TypeUnknown {
		tc.checkAssign(d.Span_, declType, d.Init)
		return declType
	}
	t := tc.inferExpr(d.Init)
	if t == TypeVoid {
		tc.diags.Error(d.Span_, "Void is only valid as a function result type", "remove the declaration or use a value type")
	}
	return t
}

// checkTypeExprValid validates a type expression used outside a function
// result position. Void is only valid as a function result type, so any use
// of it here (including as an array element type) is an error.
func (tc *TypeChecker) checkTypeExprValid(e Expr) {
	switch n := e.(type) {
	case *IdentExpr:
		if n.Name == "Void" {
			tc.diags.Error(n.Span_, "Void is only valid as a function result type", "remove the type or use a value type")
		}
	case *ArrayTypeExpr:
		tc.checkTypeExprValid(n.Elem)
	}
}

func (tc *TypeChecker) checkReturnStmt(s *ReturnStmt) {
	if s.Value == nil {
		if tc.currentReturnType != TypeVoid {
			tc.diags.Error(s.Span_, "return without a value in a procedure returning "+string(tc.currentReturnType), "return a value of type "+string(tc.currentReturnType))
		}
		return
	}
	if tc.currentReturnType == TypeVoid && tc.currentErrorReturnType == "" {
		tc.diags.Error(s.Span_, "return with a value in a void procedure", "return without a value")
		return
	}
	// A bare error literal ('.MEMBER!') takes the error return type.
	if em, ok := s.Value.(*ErrorMemberExpr); ok && em.TypeName == "" {
		if tc.currentErrorReturnType != "" {
			tc.checkErrorMember(em, tc.currentErrorReturnType)
			return
		}
		tc.checkAssign(s.Span_, tc.currentReturnType, s.Value)
		return
	}
	if tc.currentReturnType != TypeVoid {
		// A '<>' procedure may return either the value type or the error type.
		if tc.currentErrorReturnType != "" {
			vt := tc.inferExpr(s.Value)
			if vt == tc.currentErrorReturnType {
				return // an error value returned from a '<>' procedure
			}
			if vt == errorUnionType(tc.currentReturnType, tc.currentErrorReturnType) {
				return // re-raise a matching error-returning value
			}
		}
		tc.checkAssign(s.Span_, tc.currentReturnType, s.Value)
		return
	}
	tc.checkAssign(s.Span_, tc.currentErrorReturnType, s.Value)
}

// inferExpr returns the type of an expression, reporting type errors it
// encounters along the way.
func (tc *TypeChecker) inferExpr(e Expr) Type {
	switch n := e.(type) {
	case *IntExpr:
		return TypeS64
	case *FloatExpr:
		return TypeF64
	case *StringExpr:
		return TypeString
	case *BoolExpr:
		return TypeBool
	case *IdentExpr:
		t, ok := tc.lookup(n.Name)
		if !ok {
			tc.diags.Error(n.Span_, "use of undeclared variable "+n.Name, "declare the variable before using it")
		}
		return t
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
			t := typeOfTypeExpr(n.Type)
			tc.checkStructInit(n, t)
			return t
		}
		return TypeUnknown
	case *ErrorMemberExpr:
		return tc.checkErrorMemberExpr(n)
	case *ErrorExpr:
		return TypeUnknown
	case *ArrayInitExpr:
		elemType := typeOfTypeExpr(n.Elem)
		tc.checkTypeExprValid(n.Elem)
		for _, item := range n.Items {
			tc.checkAssign(item.nodeSpan(), elemType, item)
		}
		return arrayType(elemType)
	case *IndexExpr:
		baseType := tc.inferExpr(n.Base)
		if !isArrayType(baseType) {
			tc.diags.Error(n.Base.nodeSpan(), "cannot index a value of type "+string(baseType), "use an array value")
			return TypeUnknown
		}
		idxType := tc.inferExpr(n.Index)
		if idxType != TypeUnknown && !isIntegerType(idxType) {
			tc.diags.Error(n.Index.nodeSpan(), "array index must be an integer, got "+string(idxType), "use an integer index")
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
	}
	return TypeUnknown
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
		tc.diags.Error(n.Span_, "cannot infer the error type of '. "+n.Name+"'", "annotate the declaration with an error type")
		return TypeUnknown
	}
	if !tc.isErrorType(Type(n.TypeName)) {
		tc.diags.Error(n.Span_, n.TypeName+" is not an error type", "use a declared error type")
		return TypeUnknown
	}
	if !tc.hasErrorMember(n.TypeName, n.Name) {
		tc.diags.Error(n.Span_, "unknown error member "+n.Name+" in "+n.TypeName, "use a declared error member")
		return TypeUnknown
	}
	return Type(n.TypeName)
}

// isErrorType reports whether t names a declared error type.
func (tc *TypeChecker) isErrorType(t Type) bool {
	_, ok := tc.errors[string(t)]
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
	d, ok := tc.errors[typeName]
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

func (tc *TypeChecker) checkBinaryExpr(n *BinaryExpr) Type {
	// Infer operand types, deferring bare error members ('.MEMBER') until the
	// other operand's type is known.
	var lt, rt Type
	if em, ok := n.Left.(*ErrorMemberExpr); ok && em.TypeName == "" {
		lt = TypeUnknown
	} else {
		lt = tc.inferExpr(n.Left)
	}
	if em, ok := n.Right.(*ErrorMemberExpr); ok && em.TypeName == "" {
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
		} else {
			tc.diags.Error(em.Span_, "cannot infer the error type of '. "+em.Name+"'", "use the explicit 'Type.MEMBER' form")
		}
	}
	if em, ok := n.Right.(*ErrorMemberExpr); ok && em.TypeName == "" {
		if tc.isErrorType(lt) {
			tc.checkErrorMember(em, lt)
			rt = lt
		} else {
			tc.diags.Error(em.Span_, "cannot infer the error type of '. "+em.Name+"'", "use the explicit 'Type.MEMBER' form")
		}
	}
	// Error-returning values must be handled before they are used.
	if isErrorUnion(lt) || isErrorUnion(rt) {
		tc.diags.Error(n.Span_, "must handle the error before using the value", "use 'unless catch' or 'if ... catch' first")
		return TypeUnknown
	}
	switch n.Op {
	case BinaryOpAdd, BinaryOpSub, BinaryOpMul, BinaryOpDiv, BinaryOpMod:
		if tc.isErrorType(lt) || tc.isErrorType(rt) {
			tc.diags.Error(n.Span_, "cannot apply "+n.Op.String()+" to error values", "error values support only == and !=")
			return lt
		}
		if !tc.operandsCompatible(n.Left, lt, n.Right, rt) {
			tc.diags.Error(n.Span_, "cannot apply "+n.Op.String()+" to "+string(lt)+" and "+string(rt), "operate on values of the same type")
		}
		// The result takes the non-literal operand's type when one side is a
		// literal that adapts to the other.
		if isLiteral(n.Left) && !isLiteral(n.Right) {
			return rt
		}
		return lt
	case BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe:
		if tc.isErrorType(lt) || tc.isErrorType(rt) {
			tc.diags.Error(n.Span_, "cannot order error values with "+n.Op.String(), "error values support only == and !=")
			return TypeBool
		}
		if !tc.operandsCompatible(n.Left, lt, n.Right, rt) {
			tc.diags.Error(n.Span_, "cannot compare "+string(lt)+" and "+string(rt), "compare values of the same type")
		}
		return TypeBool
	case BinaryOpEq, BinaryOpNeq:
		if !tc.operandsCompatible(n.Left, lt, n.Right, rt) {
			tc.diags.Error(n.Span_, "cannot compare "+string(lt)+" and "+string(rt), "compare values of the same type")
		}
		return TypeBool
	case BinaryOpAnd, BinaryOpOr:
		if lt != TypeUnknown && lt != TypeBool {
			tc.diags.Error(n.Left.nodeSpan(), "logical operator "+n.Op.String()+" requires Bool operands, got "+string(lt), "use boolean operands")
		}
		if rt != TypeUnknown && rt != TypeBool {
			tc.diags.Error(n.Right.nodeSpan(), "logical operator "+n.Op.String()+" requires Bool operands, got "+string(rt), "use boolean operands")
		}
		return TypeBool
	}
	return TypeUnknown
}

func (tc *TypeChecker) checkUnaryExpr(n *UnaryExpr) Type {
	ot := tc.inferExpr(n.Operand)
	switch n.Op {
	case UnaryOpNeg:
		return ot
	case UnaryOpNot:
		if ot != TypeUnknown && ot != TypeBool {
			tc.diags.Error(n.Span_, "operator ! requires a Bool operand, got "+string(ot), "use a boolean operand")
		}
		return TypeBool
	}
	return TypeUnknown
}

func (tc *TypeChecker) checkCallExpr(n *CallExpr) Type {
	ident, ok := n.Func.(*IdentExpr)
	if !ok {
		return TypeUnknown
	}
	proc, ok := tc.procs[ident.Name]
	if !ok {
		tc.diags.Error(n.Span_, "call to unknown procedure "+ident.Name, "declare the procedure before calling it")
		return TypeUnknown
	}
	if len(n.Args) != len(proc.Params) {
		tc.diags.Error(n.Span_, "call to "+ident.Name+" expects "+strconv.Itoa(len(proc.Params))+" arguments, got "+strconv.Itoa(len(n.Args)), "pass the correct number of arguments")
	}
	for i, arg := range n.Args {
		if i < len(proc.Params) {
			paramType := typeOfTypeExpr(proc.Params[i].Type)
			tc.checkAssign(arg.nodeSpan(), paramType, arg)
		}
	}
	if len(proc.Results) > 0 {
		if proc.ErrorResult != nil {
			return errorUnionType(tc.procValueResult(proc), tc.procErrorResult(proc))
		}
		return tc.procValueResult(proc)
	}
	return TypeVoid
}

// procErrorResult returns the error type of a '<>' procedure result,
// normalizing the written order.
func (tc *TypeChecker) procErrorResult(p *ProcDecl) Type {
	lt := typeOfTypeExpr(p.Results[0])
	rt := typeOfTypeExpr(p.ErrorResult)
	if tc.isErrorType(lt) && !tc.isErrorType(rt) {
		return lt
	}
	return rt
}

func (tc *TypeChecker) checkStructInit(si *StructInitExpr, structType Type) {
	st, ok := tc.structs[string(structType)]
	if !ok {
		tc.diags.Error(si.Span_, "unknown struct type "+string(structType), "use a declared struct type")
		return
	}
	fieldTypes := make(map[string]Type, len(st.Fields))
	for _, f := range st.Fields {
		fieldTypes[f.Name] = typeOfTypeExpr(f.Type)
	}
	for i, field := range si.Fields {
		if field.Name != "" {
			ft, ok := fieldTypes[field.Name]
			if !ok {
				tc.diags.Error(field.Span_, "unknown field "+field.Name+" in struct "+string(structType), "use a declared field name")
				continue
			}
			tc.checkAssign(field.Span_, ft, field.Value)
			continue
		}
		// Positional field: maps to the struct field at the same index.
		if i < len(st.Fields) {
			ft := typeOfTypeExpr(st.Fields[i].Type)
			tc.checkAssign(field.Span_, ft, field.Value)
		} else {
			tc.diags.Error(field.Span_, "too many fields in struct literal for "+string(structType), "remove the extra field")
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
		return arrayType(typeOfTypeExpr(n.Elem))
	}
	return TypeUnknown
}

// arrayType is the type name of an array with the given element type.
func arrayType(elem Type) Type {
	return Type("[]" + string(elem))
}

// isArrayType reports whether t is an array type.
func isArrayType(t Type) bool {
	return strings.HasPrefix(string(t), "[]")
}

// arrayElemType returns the element type of an array type.
func arrayElemType(t Type) Type {
	return Type(strings.TrimPrefix(string(t), "[]"))
}

// checkAssign validates that a value can be assigned to a target type. Literals
// adapt to a compatible target type (an integer literal to any integer type, a
// float literal to any float type); other expressions must match exactly. A
// bare error member ('.MEMBER') takes the target error type.
func (tc *TypeChecker) checkAssign(span Span, target Type, value Expr) {
	if target == TypeUnknown || value == nil {
		return
	}
	if isLiteral(value) {
		if !literalCompatible(value, target) {
			valType := tc.inferExpr(value)
			tc.diags.Error(span, "cannot assign "+string(valType)+" to "+string(target), "use a value of type "+string(target))
		}
		return
	}
	if em, ok := value.(*ErrorMemberExpr); ok && em.TypeName == "" {
		if tc.isErrorType(target) {
			tc.checkErrorMember(em, target)
			return
		}
		tc.diags.Error(span, "cannot assign an error member to "+string(target), "use a value of type "+string(target))
		return
	}
	valType := tc.inferExpr(value)
	if isErrorUnion(valType) {
		tc.diags.Error(span, "must handle the error before using the value", "use 'unless catch' or 'if ... catch' first")
		return
	}
	if valType != TypeUnknown && valType != target {
		tc.diags.Error(span, "cannot assign "+string(valType)+" to "+string(target), "use a value of type "+string(target))
	}
}

// checkErrorMember validates a bare error member reference against a target
// error type. Error literals require the trailing '!'.
func (tc *TypeChecker) checkErrorMember(em *ErrorMemberExpr, t Type) {
	if !em.Bang {
		tc.diags.Error(em.Span_, "error values must be instantiated with '!'", "add '!' after the member name")
		return
	}
	if !tc.isErrorType(t) {
		tc.diags.Error(em.Span_, string(t)+" is not an error type", "use a declared error type")
		return
	}
	if !tc.hasErrorMember(string(t), em.Name) {
		tc.diags.Error(em.Span_, "unknown error member "+em.Name+" in "+string(t), "use a declared error member")
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
	switch t {
	case "S8", "S16", "S32", "S64", "S128",
		"U8", "U16", "U32", "U64", "U128",
		"Size", "Byte":
		return true
	}
	return false
}

func isFloatType(t Type) bool {
	switch t {
	case "F16", "F32", "F64", "F128":
		return true
	}
	return false
}
