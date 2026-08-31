// Generic procedure monomorphization for the Chaos compiler.
//
// A generic procedure ("Name <T: A | B> :: proc (...) { ... }") is validated
// once with its type parameters as abstract types. Each call site supplies
// concrete type arguments; the compiler creates an instantiated copy of the
// procedure with the type parameters substituted and lowers that copy. This
// file implements the AST substitution used to build those copies.
package compiler

// substituteTypeExpr substitutes generic type parameters in a type expression.
// mapping maps a type-parameter name to the concrete type expression to
// substitute. Only type positions are rewritten; value expressions are handled
// by substituteExpr/substituteStmt.
func substituteTypeExpr(e Expr, mapping map[string]Expr) Expr {
	if e == nil {
		return nil
	}
	switch n := e.(type) {
	case *IdentExpr:
		if repl, ok := mapping[n.Name]; ok {
			return repl
		}
		return n
	case *ArrayTypeExpr:
		cp := *n
		cp.Elem = substituteTypeExpr(n.Elem, mapping)
		cp.Size = substituteExpr(n.Size, mapping)
		return &cp
	case *PointerTypeExpr:
		cp := *n
		cp.Elem = substituteTypeExpr(n.Elem, mapping)
		return &cp
	case *ParenExpr:
		cp := *n
		cp.Inner = substituteTypeExpr(n.Inner, mapping)
		return &cp
	default:
		return substituteExpr(e, mapping)
	}
}

// substituteExpr substitutes generic type parameters throughout an expression,
// rewriting type positions (casts, array/pointer element types, struct literal
// type names) and recursing into subexpressions.
func substituteExpr(e Expr, mapping map[string]Expr) Expr {
	if e == nil {
		return nil
	}
	switch n := e.(type) {
	case *IdentExpr:
		// Copy the identifier so each generic instance owns its node. Sharing
		// the original would let one instance's type-check overwrite the
		// ExprTypes entry that another instance reads during lowering.
		cp := *n
		return &cp
	case *IntExpr:
		cp := *n
		return &cp
	case *FloatExpr:
		cp := *n
		return &cp
	case *StringExpr:
		cp := *n
		return &cp
	case *BoolExpr:
		cp := *n
		return &cp
	case *NullLitExpr:
		cp := *n
		return &cp
	case *LoopBuiltinExpr:
		cp := *n
		return &cp
	case *ErrorExpr:
		cp := *n
		return &cp
	case *ErrorMemberExpr:
		cp := *n
		return &cp
	case *EnumMemberExpr:
		cp := *n
		return &cp
	case *BinaryExpr:
		cp := *n
		cp.Left = substituteExpr(n.Left, mapping)
		cp.Right = substituteExpr(n.Right, mapping)
		return &cp
	case *UnaryExpr:
		cp := *n
		cp.Operand = substituteExpr(n.Operand, mapping)
		return &cp
	case *CallExpr:
		cp := *n
		cp.Func = substituteExpr(n.Func, mapping)
		cp.Args = make([]Expr, len(n.Args))
		for i, a := range n.Args {
			cp.Args[i] = substituteExpr(a, mapping)
		}
		return &cp
	case *ParenExpr:
		cp := *n
		cp.Inner = substituteExpr(n.Inner, mapping)
		return &cp
	case *StructInitExpr:
		cp := *n
		cp.Type = substituteTypeExpr(n.Type, mapping)
		cp.Fields = make([]StructInitField, len(n.Fields))
		for i, f := range n.Fields {
			cp.Fields[i] = f
			cp.Fields[i].Value = substituteExpr(f.Value, mapping)
		}
		return &cp
	case *ArrayInitExpr:
		cp := *n
		cp.Elem = substituteArrayType(n.Elem, mapping)
		cp.Items = make([]Expr, len(n.Items))
		for i, it := range n.Items {
			cp.Items[i] = substituteExpr(it, mapping)
		}
		return &cp
	case *IndexExpr:
		cp := *n
		cp.Base = substituteExpr(n.Base, mapping)
		cp.Index = substituteExpr(n.Index, mapping)
		return &cp
	case *IfxExpr:
		cp := *n
		cp.Condition = substituteExpr(n.Condition, mapping)
		cp.Then = substituteExpr(n.Then, mapping)
		cp.Else = substituteExpr(n.Else, mapping)
		return &cp
	case *AllocateExpr:
		cp := *n
		cp.Size = substituteExpr(n.Size, mapping)
		return &cp
	case *SizeOfExpr:
		cp := *n
		cp.Type = substituteTypeExpr(n.Type, mapping)
		return &cp
	case *CastExpr:
		cp := *n
		cp.Value = substituteExpr(n.Value, mapping)
		cp.Type = substituteTypeExpr(n.Type, mapping)
		return &cp
	case *InterpolatedStringExpr:
		cp := *n
		cp.Parts = make([]InterpPart, len(n.Parts))
		for i, part := range n.Parts {
			cp.Parts[i] = part
			cp.Parts[i].Expr = substituteExpr(part.Expr, mapping)
		}
		return &cp
	case *DerefExpr:
		cp := *n
		cp.Operand = substituteExpr(n.Operand, mapping)
		return &cp
	case *FieldAccessExpr:
		cp := *n
		cp.Base = substituteExpr(n.Base, mapping)
		return &cp
	default:
		return e
	}
}

// substituteArrayType substitutes the element type of an array type
// expression.
func substituteArrayType(at *ArrayTypeExpr, mapping map[string]Expr) *ArrayTypeExpr {
	if at == nil {
		return nil
	}
	cp := *at
	cp.Elem = substituteTypeExpr(at.Elem, mapping)
	cp.Size = substituteExpr(at.Size, mapping)
	return &cp
}

// substituteStmt substitutes generic type parameters throughout a statement.
func substituteStmt(s Stmt, mapping map[string]Expr) Stmt {
	if s == nil {
		return nil
	}
	switch n := s.(type) {
	case *VarDecl:
		cp := *n
		cp.DeclType = substituteTypeExpr(n.DeclType, mapping)
		cp.Init = substituteExpr(n.Init, mapping)
		return &cp
	case *AssignStmt:
		cp := *n
		cp.Target = substituteExpr(n.Target, mapping)
		cp.Value = substituteExpr(n.Value, mapping)
		return &cp
	case *ReturnStmt:
		cp := *n
		cp.Value = substituteExpr(n.Value, mapping)
		cp.Values = make([]Expr, len(n.Values))
		for i, v := range n.Values {
			cp.Values[i] = substituteExpr(v, mapping)
		}
		return &cp
	case *ExitStmt:
		cp := *n
		cp.Status = substituteExpr(n.Status, mapping)
		cp.Message = substituteExpr(n.Message, mapping)
		return &cp
	case *IfStmt:
		cp := *n
		cp.Condition = substituteExpr(n.Condition, mapping)
		cp.Body = substituteBlock(n.Body, mapping)
		cp.Elif = make([]*IfStmt, len(n.Elif))
		for i, el := range n.Elif {
			cp.Elif[i] = substituteStmt(el, mapping).(*IfStmt)
		}
		cp.ElseBody = substituteBlock(n.ElseBody, mapping)
		return &cp
	case *IfCatchStmt:
		cp := *n
		cp.Cond = substituteExpr(n.Cond, mapping)
		cp.CatchBody = substituteBlock(n.CatchBody, mapping)
		return &cp
	case *UnlessCatchStmt:
		cp := *n
		cp.Init = substituteExpr(n.Init, mapping)
		cp.CatchBody = substituteBlock(n.CatchBody, mapping)
		return &cp
	case *ForStmt:
		cp := *n
		cp.Init = substituteStmt(n.Init, mapping)
		cp.Cond = substituteExpr(n.Cond, mapping)
		cp.After = substituteStmt(n.After, mapping)
		cp.Range = substituteExpr(n.Range, mapping)
		cp.Body = substituteBlock(n.Body, mapping)
		return &cp
	case *BreakStmt:
		cp := *n
		return &cp
	case *ContinueStmt:
		cp := *n
		return &cp
	case *CompoundAssignStmt:
		cp := *n
		cp.Target = substituteExpr(n.Target, mapping)
		cp.Value = substituteExpr(n.Value, mapping)
		return &cp
	case *IncDecStmt:
		cp := *n
		cp.Target = substituteExpr(n.Target, mapping)
		return &cp
	case *DeallocateStmt:
		cp := *n
		cp.Addr = substituteExpr(n.Addr, mapping)
		return &cp
	case *BlockStmt:
		return substituteBlock(n, mapping)
	case *ExprStmt:
		cp := *n
		cp.Expr = substituteExpr(n.Expr, mapping)
		return &cp
	case *MultiVarDecl:
		cp := *n
		cp.Names = append([]string(nil), n.Names...)
		cp.NameSpans = append([]Span(nil), n.NameSpans...)
		cp.Init = substituteExpr(n.Init, mapping)
		return &cp
	case *ProcDecl:
		return substituteProcDecl(n, mapping)
	case *StructDecl:
		return substituteStructDecl(n, mapping)
	case *ErrorDecl:
		cp := *n
		cp.Members = append([]ErrorMember(nil), n.Members...)
		return &cp
	case *EnumDecl:
		cp := *n
		cp.Members = make([]EnumMember, len(n.Members))
		for i, member := range n.Members {
			cp.Members[i] = member
			cp.Members[i].Type = substituteTypeExpr(member.Type, mapping)
			cp.Members[i].Value = substituteExpr(member.Value, mapping)
		}
		return &cp
	default:
		return s
	}
}

func substituteProcDecl(proc *ProcDecl, mapping map[string]Expr) *ProcDecl {
	scoped := mappingWithoutTypeParams(mapping, proc.TypeParams)
	cp := *proc
	cp.TypeParams = substituteTypeParams(proc.TypeParams, scoped)
	cp.Params = make([]Param, len(proc.Params))
	for i, param := range proc.Params {
		cp.Params[i] = param
		cp.Params[i].Type = substituteTypeExpr(param.Type, scoped)
	}
	cp.Results = make([]Expr, len(proc.Results))
	for i, result := range proc.Results {
		cp.Results[i] = substituteTypeExpr(result, scoped)
	}
	cp.ErrorResult = substituteTypeExpr(proc.ErrorResult, scoped)
	cp.Body = substituteBlock(proc.Body, scoped)
	return &cp
}

func substituteStructDecl(st *StructDecl, mapping map[string]Expr) *StructDecl {
	scoped := mappingWithoutTypeParams(mapping, st.TypeParams)
	cp := *st
	cp.TypeParams = substituteTypeParams(st.TypeParams, scoped)
	cp.Fields = make([]StructField, len(st.Fields))
	for i, field := range st.Fields {
		cp.Fields[i] = field
		cp.Fields[i].Type = substituteTypeExpr(field.Type, scoped)
		cp.Fields[i].Default = substituteExpr(field.Default, scoped)
	}
	return &cp
}

func substituteTypeParams(params []TypeParam, mapping map[string]Expr) []TypeParam {
	result := make([]TypeParam, len(params))
	for i, param := range params {
		result[i] = param
		result[i].Constraints = make([]Expr, len(param.Constraints))
		for j, constraint := range param.Constraints {
			result[i].Constraints[j] = substituteTypeExpr(constraint, mapping)
		}
	}
	return result
}

func mappingWithoutTypeParams(mapping map[string]Expr, params []TypeParam) map[string]Expr {
	result := make(map[string]Expr, len(mapping))
	for name, expr := range mapping {
		result[name] = expr
	}
	for _, param := range params {
		delete(result, param.Name)
	}
	return result
}

func substituteBlock(b *BlockStmt, mapping map[string]Expr) *BlockStmt {
	if b == nil {
		return nil
	}
	cp := *b
	cp.Stmts = make([]Stmt, len(b.Stmts))
	for i, st := range b.Stmts {
		cp.Stmts[i] = substituteStmt(st, mapping)
	}
	return &cp
}

// instantiateProc builds a concrete copy of a generic procedure with the type
// parameters substituted by the given concrete type expressions.
func instantiateProc(p *ProcDecl, mapping map[string]Expr) *ProcDecl {
	cp := *p
	cp.TypeParams = nil
	cp.Params = make([]Param, len(p.Params))
	for i, param := range p.Params {
		cp.Params[i] = param
		cp.Params[i].Type = substituteTypeExpr(param.Type, mapping)
	}
	cp.Results = make([]Expr, len(p.Results))
	for i, r := range p.Results {
		cp.Results[i] = substituteTypeExpr(r, mapping)
	}
	cp.ErrorResult = substituteTypeExpr(p.ErrorResult, mapping)
	cp.Body = substituteBlock(p.Body, mapping)
	return &cp
}
