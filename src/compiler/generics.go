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
	case *IntExpr, *FloatExpr, *StringExpr, *BoolExpr, *NullLitExpr,
		*LoopBuiltinExpr, *ErrorExpr:
		return n
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
	case *BreakStmt, *ContinueStmt:
		return n
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
	default:
		return s
	}
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
