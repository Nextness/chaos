// Type checking for the Chaos language.
//
// The type checker walks a parsed AST and validates types. It infers the type
// of literals and expressions, checks that assignments and comparisons operate
// on matching types, and verifies return statements against the enclosing
// procedure signature. Chaos is a strict typed language: values of different
// types cannot be assigned, compared, or combined.
package compiler

import "strconv"

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
	diags             DiagnosticList
	scopes            []map[string]Type
	procs             map[string]*ProcDecl
	structs           map[string]*StructDecl
	currentReturnType Type
}

// CheckProgram runs the type checker over a parsed program and returns any
// diagnostics it produced.
func CheckProgram(program *Program) DiagnosticList {
	tc := &TypeChecker{
		procs:   make(map[string]*ProcDecl),
		structs: make(map[string]*StructDecl),
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

// lookup returns the type of a name, searching from the innermost scope
// outward. It returns TypeUnknown when the name is not declared.
func (tc *TypeChecker) lookup(name string) Type {
	for i := len(tc.scopes) - 1; i >= 0; i-- {
		if t, ok := tc.scopes[i][name]; ok {
			return t
		}
	}
	return TypeUnknown
}

func (tc *TypeChecker) checkProgram(program *Program) {
	tc.pushScope() // global scope

	// First pass: register all global declarations so bodies can reference
	// them regardless of source order.
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *ProcDecl:
			tc.procs[d.Name] = d
		case *StructDecl:
			tc.structs[d.Name] = d
			tc.declare(d.Name, Type(d.Name))
		case *VarDecl:
			if d.DeclType != nil {
				tc.declare(d.Name, typeOfTypeExpr(d.DeclType))
			} else {
				tc.declare(d.Name, TypeUnknown)
			}
		}
	}

	// Second pass: check procedure bodies and infer the types of inferred
	// global declarations.
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *ProcDecl:
			tc.checkProc(d)
		case *VarDecl:
			t := tc.checkVarDecl(d)
			if d.DeclType == nil {
				tc.scopes[0][d.Name] = t
			}
		}
	}

	tc.popScope()
}

func (tc *TypeChecker) checkProc(p *ProcDecl) {
	tc.pushScope()
	for _, param := range p.Params {
		tc.declare(param.Name, typeOfTypeExpr(param.Type))
	}
	if len(p.Results) > 0 {
		tc.currentReturnType = typeOfTypeExpr(p.Results[0])
	} else {
		tc.currentReturnType = TypeVoid
	}
	if p.Body != nil {
		tc.checkBlock(p.Body)
	}
	tc.popScope()
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
		tc.declare(n.Name, t)
	case *AssignStmt:
		declType := tc.lookup(n.Name)
		if declType == TypeUnknown {
			tc.diags.Error(n.Span_, "assignment to undeclared variable "+n.Name, "declare the variable before assigning to it")
			return
		}
		tc.checkAssign(n.Span_, declType, n.Value)
	case *ReturnStmt:
		tc.checkReturnStmt(n)
	case *ExitStmt:
		if n.Status != nil {
			tc.inferExpr(n.Status)
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
	}
}

// checkVarDecl infers the type of a variable declaration and checks that the
// initializer matches the declared type when both are present. It returns the
// resolved type of the declaration.
func (tc *TypeChecker) checkVarDecl(d *VarDecl) Type {
	var declType Type
	if d.DeclType != nil {
		declType = typeOfTypeExpr(d.DeclType)
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
	return tc.inferExpr(d.Init)
}

func (tc *TypeChecker) checkReturnStmt(s *ReturnStmt) {
	if s.Value == nil {
		if tc.currentReturnType != TypeVoid {
			tc.diags.Error(s.Span_, "return without a value in a procedure returning "+string(tc.currentReturnType), "return a value of type "+string(tc.currentReturnType))
		}
		return
	}
	if tc.currentReturnType != TypeVoid {
		tc.checkAssign(s.Span_, tc.currentReturnType, s.Value)
	}
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
		return tc.lookup(n.Name)
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
	case *ErrorExpr:
		return TypeUnknown
	}
	return TypeUnknown
}

func (tc *TypeChecker) checkBinaryExpr(n *BinaryExpr) Type {
	lt := tc.inferExpr(n.Left)
	rt := tc.inferExpr(n.Right)
	switch n.Op {
	case BinaryOpAdd, BinaryOpSub, BinaryOpMul, BinaryOpDiv, BinaryOpMod:
		if !tc.operandsCompatible(n.Left, lt, n.Right, rt) {
			tc.diags.Error(n.Span_, "cannot apply "+n.Op.String()+" to "+string(lt)+" and "+string(rt), "operate on values of the same type")
		}
		// The result takes the non-literal operand's type when one side is a
		// literal that adapts to the other.
		if isLiteral(n.Left) && !isLiteral(n.Right) {
			return rt
		}
		return lt
	case BinaryOpLt, BinaryOpGt, BinaryOpLe, BinaryOpGe, BinaryOpEq, BinaryOpNeq:
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
		return typeOfTypeExpr(proc.Results[0])
	}
	return TypeVoid
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

// typeOfTypeExpr extracts the type name from a type expression (currently just
// an identifier).
func typeOfTypeExpr(e Expr) Type {
	if ident, ok := e.(*IdentExpr); ok {
		return Type(ident.Name)
	}
	return TypeUnknown
}

// checkAssign validates that a value can be assigned to a target type. Literals
// adapt to a compatible target type (an integer literal to any integer type, a
// float literal to any float type); other expressions must match exactly.
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
	valType := tc.inferExpr(value)
	if valType != TypeUnknown && valType != target {
		tc.diags.Error(span, "cannot assign "+string(valType)+" to "+string(target), "use a value of type "+string(target))
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
