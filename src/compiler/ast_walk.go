package compiler

import "reflect"

// NodeSpan returns a node's source span for generic tooling consumers.
func NodeSpan(node Node) Span {
	if node == nil {
		return Span{}
	}
	return node.nodeSpan()
}

// WalkAST visits every node in source order. Returning false from visit skips
// that node's children. This is the compiler-owned exhaustive traversal used
// by generic validation and no-panic tests; context-sensitive consumers may
// layer their own symbol logic on top of it.
func WalkAST(program *Program, visit func(Node) bool) {
	if program == nil || visit == nil {
		return
	}
	var walkNode func(Node)
	walkNode = func(node Node) {
		if node == nil || (reflect.ValueOf(node).Kind() == reflect.Pointer && reflect.ValueOf(node).IsNil()) || !visit(node) {
			return
		}
		switch n := node.(type) {
		case *Program:
			for _, decl := range n.Decls {
				walkNode(decl)
			}
		case *VarDecl:
			walkNode(n.DeclType)
			walkNode(n.Init)
		case *MultiVarDecl:
			walkNode(n.Init)
		case *AssignStmt:
			walkNode(n.Target)
			walkNode(n.Value)
		case *CompoundAssignStmt:
			walkNode(n.Target)
			walkNode(n.Value)
		case *IncDecStmt:
			walkNode(n.Target)
		case *ReturnStmt:
			for _, value := range n.Values {
				walkNode(value)
			}
		case *ExitStmt:
			walkNode(n.Status)
			walkNode(n.Message)
		case *IfStmt:
			walkNode(n.Condition)
			walkNode(n.Body)
			for _, branch := range n.Elif {
				walkNode(branch)
			}
			walkNode(n.ElseBody)
		case *IfCatchStmt:
			walkNode(n.Cond)
			walkNode(n.CatchBody)
		case *UnlessCatchStmt:
			walkNode(n.Init)
			walkNode(n.CatchBody)
		case *ForStmt:
			walkNode(n.Init)
			walkNode(n.Cond)
			walkNode(n.After)
			walkNode(n.Range)
			walkNode(n.Body)
		case *BlockStmt:
			for _, stmt := range n.Stmts {
				walkNode(stmt)
			}
		case *ProcDecl:
			for i := range n.Params {
				walkNode(n.Params[i])
			}
			for _, result := range n.Results {
				walkNode(result)
			}
			walkNode(n.ErrorResult)
			walkNode(n.Body)
		case *Param:
			walkNode(n.Type)
		case *StructDecl:
			for i := range n.Fields {
				walkNode(n.Fields[i])
			}
		case *StructField:
			walkNode(n.Type)
			walkNode(n.Default)
		case *EnumDecl:
			for i := range n.Members {
				walkNode(n.Members[i])
			}
		case *EnumMember:
			walkNode(n.Type)
			walkNode(n.Value)
		case *ErrorDecl:
			for i := range n.Members {
				walkNode(n.Members[i])
			}
		case *ExprStmt:
			walkNode(n.Expr)
		case *ParenExpr:
			walkNode(n.Inner)
		case *UnaryExpr:
			walkNode(n.Operand)
		case *BinaryExpr:
			walkNode(n.Left)
			walkNode(n.Right)
		case *CallExpr:
			walkNode(n.Func)
			for _, arg := range n.Args {
				walkNode(arg)
			}
		case *StructInitExpr:
			walkNode(n.Type)
			for i := range n.Fields {
				walkNode(&n.Fields[i])
			}
		case *StructInitField:
			walkNode(n.Value)
		case *ArrayTypeExpr:
			walkNode(n.Elem)
		case *PointerTypeExpr:
			walkNode(n.Elem)
		case *ArrayInitExpr:
			walkNode(n.Elem)
			for _, item := range n.Items {
				walkNode(item)
			}
		case *IndexExpr:
			walkNode(n.Base)
			walkNode(n.Index)
		case *DerefExpr:
			walkNode(n.Operand)
		case *FieldAccessExpr:
			walkNode(n.Base)
		case *AllocateExpr:
			walkNode(n.Size)
		case *CastExpr:
			walkNode(n.Value)
			walkNode(n.Type)
		case *InterpolatedStringExpr:
			for _, part := range n.Parts {
				walkNode(part.Expr)
			}
		case *DeallocateStmt:
			walkNode(n.Addr)
		case *IfxExpr:
			walkNode(n.Condition)
			walkNode(n.Then)
			walkNode(n.Else)
		case *IdentExpr, *IntExpr, *FloatExpr, *StringExpr, *BoolExpr,
			*NullLitExpr, *LoopBuiltinExpr, *ErrorExpr, *ErrorMemberExpr,
			*EnumMemberExpr, *BreakStmt, *ContinueStmt, *ErrorMember:
			// Leaves.
		}
	}
	walkNode(program)
}
