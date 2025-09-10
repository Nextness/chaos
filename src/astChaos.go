package main

import "fmt"

type NodeType int

const (
	nodeNull NodeType = iota
	nodeIntLiteral
	nodeStringLiteral
	nodeFloatLiteral
	nodeIdentifier
	nodeExit
	nodeBinOp
	nodeProcDef
	nodeCall
)

type BinOpOperation int

const (
	opNull BinOpOperation = iota
	opPlus
	opLessThan
)

type VarDecl struct {
	Name        Token
	Type        Token
	Assignment  Node
	Initialized bool
}

type Exit struct {
	Status  Node
	Message Node
}

type Literal struct {
	Int    Token
	String Token
}

type BinOp struct {
	Operation BinOpOperation
	Lhs       Node
	Rhs       Node
}

type Proc struct {
	Scope  []Node
	Inputs []Node
}

type Call struct {
	Name Token
}

type Node struct {
	NodeType     NodeType
	Reassignable bool
	Literal      *Literal
	VarDecl      *VarDecl
	Exit         *Exit
	BinOp        *BinOp
	Proc         *Proc
	Call         *Call
}

type Program struct {
	Nodes          []Node
	AllocatedVars  map[string]Node
	AllocatedProcs map[string]Node
}

// TODO: Make these variables allocate VarDecl and
// Proc respectively instead of Node
var AllocatedVars map[string]Node = map[string]Node{}
var AllocatedProcs map[string]Node = map[string]Node{}

func (node *Node) print(idx int) {

	if node.NodeType == nodeCall {
		name := node.Call.Name.Symbol
		fmt.Printf("%6d. call(%s, Void, Void)\n", idx, name)
		return
	}

	if node.NodeType == nodeIdentifier {
		reassinableType := "const"
		if node.Reassignable {
			reassinableType = "var"
		}

		name := node.VarDecl.Name.Symbol
		t := "infer"
		if node.VarDecl.Type.Symbol != "" {
			t = node.VarDecl.Type.Symbol
		}

		if node.VarDecl.Assignment.NodeType == nodeProcDef {
			var inputs string = ""
			if node.VarDecl.Assignment.Proc.Inputs != nil {
				size := len(node.VarDecl.Assignment.Proc.Inputs)
				for id, n := range node.VarDecl.Assignment.Proc.Inputs {
					if id == size-1 {
						inputs = fmt.Sprintf("%s%s: %s", inputs, n.VarDecl.Name.Symbol, n.VarDecl.Type.Symbol)
						continue
					}
					inputs = fmt.Sprintf("%s%s: %s, ", inputs, n.VarDecl.Name.Symbol, n.VarDecl.Type.Symbol)
				}
			}
			fmt.Printf("%6d. %s(%s) -> Void {\n", idx, name, inputs)
			for _, nod := range node.VarDecl.Assignment.Proc.Scope {
				nod.print(idx)
			}
			fmt.Printf("%6d. }\n", idx)
			return
		}

		if node.VarDecl.Assignment.NodeType == nodeBinOp {
			var lhs, rhs Node
			var okLhs, okRhs bool = false, false
			if node.VarDecl.Assignment.BinOp.Lhs.NodeType != nodeNull {
				okLhs = true
				lhs = node.VarDecl.Assignment.BinOp.Lhs
			}

			if node.VarDecl.Assignment.BinOp.Rhs.NodeType != nodeNull {
				okRhs = true
				rhs = node.VarDecl.Assignment.BinOp.Rhs
			}

			if node.VarDecl.Assignment.BinOp.Operation == opPlus {
				if okLhs && okRhs {
					if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIntLiteral {
						l := castAssert[int](lhs.Literal.Int.Value)
						r := castAssert[int](rhs.Literal.Int.Value)
						fmt.Printf("%6d. %s(%s, %s) = %d + %d\n", idx, reassinableType, name, t, l, r)
						return
					}
					if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIdentifier {
						l := castAssert[int](lhs.Literal.Int.Value)
						r := rhs.VarDecl.Name.Symbol
						fmt.Printf("%6d. %s(%s, %s) = %d + %s\n", idx, reassinableType, name, t, l, r)
						return
					}
					if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIntLiteral {
						l := lhs.VarDecl.Name.Symbol
						r := castAssert[int](rhs.Literal.Int.Value)
						fmt.Printf("%6d. %s(%s, %s) = %s + %d\n", idx, reassinableType, name, t, l, r)
						return
					}
					if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIdentifier {
						l := lhs.VarDecl.Name.Symbol
						r := rhs.VarDecl.Name.Symbol
						fmt.Printf("%6d. %s(%s, %s) = %s + %s\n", idx, reassinableType, name, t, l, r)
						return
					}
				}
				return
			}
			if node.VarDecl.Assignment.BinOp.Operation == opLessThan {
				if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIntLiteral {
					l := castAssert[int](lhs.Literal.Int.Value)
					r := castAssert[int](rhs.Literal.Int.Value)
					fmt.Printf("%6d. %s(%s, %s) = %d < %d\n", idx, reassinableType, name, t, l, r)
					return
				}
				if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIdentifier {
					l := castAssert[int](lhs.Literal.Int.Value)
					r := rhs.VarDecl.Name.Symbol
					fmt.Printf("%6d. %s(%s, %s) = %d < %s\n", idx, reassinableType, name, t, l, r)
					return
				}
				if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIntLiteral {
					l := lhs.VarDecl.Name.Symbol
					r := castAssert[int](rhs.Literal.Int.Value)
					fmt.Printf("%6d. %s(%s, %s) = %s < %d\n", idx, reassinableType, name, t, l, r)
					return
				}
				if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIdentifier {
					l := lhs.VarDecl.Name.Symbol
					r := rhs.VarDecl.Name.Symbol
					fmt.Printf("%6d. %s(%s, %s) = %s < %s\n", idx, reassinableType, name, t, l, r)
					return
				}
				return
			}

		}

		if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
			result := castAssert[int](node.VarDecl.Assignment.Literal.Int.Value)
			fmt.Printf("%6d. %s(%s, %s) = %d\n", idx, reassinableType, name, t, result)
			return
		}

		if node.VarDecl.Assignment.NodeType == nodeFloatLiteral {
			result := castAssert[float64](node.VarDecl.Assignment.Literal.Int.Value)
			fmt.Printf("%6d. %s(%s, %s) = %.2f\n", idx, reassinableType, name, t, result)
			return
		}

		if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
			result := castAssert[string](node.VarDecl.Assignment.Literal.String.Value)
			fmt.Printf("%6d. %s(%s, %s) = \"%s\"\n", idx, reassinableType, name, t, result)
			return
		}

		if !node.VarDecl.Initialized {
			reassinableType = "var"
			name := node.VarDecl.Name.Symbol
			fmt.Printf("%6d. %s(%s, %s)\n", idx, reassinableType, name, t)
			return
		}
		if node.VarDecl.Assignment.NodeType == nodeIdentifier {
			name := node.VarDecl.Name.Symbol
			fmt.Printf("%6d. %s(%s, %s) = %s\n", idx, reassinableType, name, t, node.VarDecl.Assignment.VarDecl.Name.Symbol)
			return
		}

		fmt.Printf("[ERROR] Not implemented :: %+v\n", node.VarDecl.Assignment)
		return
	}

	if node.NodeType == nodeExit {
		msg := ""
		if node.Exit.Message.NodeType == nodeStringLiteral {
			msg = castAssert[string](node.Exit.Message.Literal.String.Value)
		}

		if node.Exit.Status.NodeType == nodeIdentifier {
			status := node.Exit.Status.VarDecl.Name.Symbol
			fmt.Printf("%6d. exit(%s, \"%s\")\n", idx, status, msg)
		}
		if node.Exit.Status.NodeType == nodeIntLiteral {
			status := castAssert[int](node.Exit.Status.Literal.Int.Value)
			fmt.Printf("%6d. exit(%d, \"%s\")\n", idx, status, msg)
		}
		return
	}

	assert[any](false, fmt.Sprintf("Unknown node '%v'", node))
}

func ASTParseProc(lex *Lexer) Node {
	node := Node{
		NodeType: nodeProcDef,
		Proc: &Proc{
			Scope:  []Node{},
			Inputs: []Node{},
		},
	}

	if lex.MatchTokenSequence(tokOpenParen) {
		lex.ConsumeAssert(tokOpenParen)
		for lex.GetToken(0).TokenType != tokCloseParen {
			ident := lex.ConsumeAssert(tokIdentifier)
			lex.ConsumeAssert(tokColon)
			varType := lex.ConsumeAssert(tokIdentifier)
			lex.MatchTokenSequenceAndConsumeAssert(tokComma)
			n := Node{
				NodeType: nodeIdentifier,
				VarDecl: &VarDecl{
					Name:        ident,
					Type:        varType,
					Initialized: false,
				},
			}
			node.Proc.Inputs = append(node.Proc.Inputs, n)
		}
		lex.ConsumeAssert(tokCloseParen)
	}

	lex.ConsumeAssert(tokOpenBraket)
	for lex.GetToken(0).TokenType != tokCloseBraket {
		stmt := ASTParseStatement(lex)
		node.Proc.Scope = append(node.Proc.Scope, stmt)
	}
	lex.ConsumeAssert(tokCloseBraket)
	return node
}

func ASTParseExpression(lex *Lexer) Node {
	expr := lex.Consume()

	if expr.TokenType == tokProc {
		return ASTParseProc(lex)
	}

	if expr.TokenType == tokNumberLiteral {
		if lex.GetToken(0).TokenType == tokLessThan {
			lex.ConsumeAssert(tokLessThan)
			rhs := ASTParseExpression(lex)
			nod := Node{
				NodeType: nodeBinOp,
				BinOp: &BinOp{
					Operation: opLessThan,
					Lhs: Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							Int: expr,
						},
					},
					Rhs: rhs,
				},
			}
			return nod
		}
		if lex.GetToken(0).TokenType == tokPlus {
			lex.ConsumeAssert(tokPlus)
			rhs := ASTParseExpression(lex)
			nod := Node{
				NodeType: nodeBinOp,
				BinOp: &BinOp{
					Operation: opPlus,
					Lhs: Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							Int: expr,
						},
					},
					Rhs: rhs,
				},
			}
			return nod
		}
		return Node{
			NodeType: nodeIntLiteral,
			Literal: &Literal{
				Int: expr,
			},
		}
	}

	if expr.TokenType == tokStringLiteral {
		return Node{
			NodeType: nodeStringLiteral,
			Literal: &Literal{
				String: expr,
			},
		}
	}

	if expr.TokenType == tokIdentifier {
		ident := AllocatedVars[expr.Symbol]
		if lex.GetToken(0).TokenType == tokLessThan {
			lex.ConsumeAssert(tokLessThan)
			rhs := ASTParseExpression(lex)
			nod := Node{
				NodeType: nodeBinOp,
				BinOp: &BinOp{
					Operation: opLessThan,
					Lhs:       ident,
					Rhs:       rhs,
				},
			}
			return nod
		}
		if lex.GetToken(0).TokenType == tokPlus {
			lex.ConsumeAssert(tokPlus)
			rhs := ASTParseExpression(lex)
			nod := Node{
				NodeType: nodeBinOp,
				BinOp: &BinOp{
					Operation: opPlus,
					Lhs:       ident,
					Rhs:       rhs,
				},
			}
			return nod
		}
		return ident
	}

	token := lex.GetToken(0)
	return assert[Node](false, fmt.Sprintf("Unknown token found - \"%s\"", token.TokenType.String()))
}

func ASTParseProcDefinition(lex *Lexer) (Node, bool) {
	if lex.MatchTokenSequence(tokIdentifier, tokColon, tokColon, tokProc) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssertSequence(tokColon, tokColon, tokProc)
		node := Node{
			NodeType:     nodeIdentifier,
			Reassignable: false,
			VarDecl: &VarDecl{
				Name:        ident,
				Assignment:  ASTParseProc(lex),
				Initialized: true,
			},
		}
		AllocatedProcs[ident.Symbol] = node
		return node, true
	}
	return Node{}, false
}

func ASTParseVariableDefinition(lex *Lexer) (Node, bool) {
	if lex.MatchTokenSequence(tokIdentifier, tokColon, tokIdentifier, tokSemicolon) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssert(tokColon)
		varType := lex.Consume()
		node := Node{
			NodeType:     nodeIdentifier,
			Reassignable: true,
			VarDecl: &VarDecl{
				Name:        ident,
				Type:        varType,
				Initialized: false,
			},
		}
		AllocatedVars[ident.Symbol] = node
		return node, true
	}
	return Node{}, false
}

func ASTParseVariableReassignment(lex *Lexer) bool {
	if lex.MatchTokenSequence(tokIdentifier, tokAssignment) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssert(tokAssignment)
		node, ok := AllocatedVars[ident.Symbol]
		assert[any](ok, "Variable is not allocated")
		node.VarDecl.Assignment = ASTParseExpression(lex)

		node.VarDecl.Initialized = true
		node.Reassignable = true
		AllocatedVars[ident.Symbol] = node
		return true
	}
	return false
}

func ASTParseNewConstAssignment(lex *Lexer) (Node, bool) {
	if lex.MatchTokenSequence(tokIdentifier, tokColon, tokColon) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssertSequence(tokColon, tokColon)
		node := Node{
			NodeType:     nodeIdentifier,
			Reassignable: false,
			VarDecl: &VarDecl{
				Name:        ident,
				Assignment:  ASTParseExpression(lex),
				Initialized: true,
			},
		}
		AllocatedVars[ident.Symbol] = node
		return node, true
	}
	if lex.MatchTokenSequence(tokIdentifier, tokColon, tokIdentifier, tokColon) {
		ident := lex.Consume()
		lex.ConsumeAssert(tokColon)
		varType := lex.Consume()
		lex.ConsumeAssert(tokColon)
		node := Node{
			NodeType:     nodeIdentifier,
			Reassignable: false,
			VarDecl: &VarDecl{
				Name:        ident,
				Type:        varType,
				Assignment:  ASTParseExpression(lex),
				Initialized: true,
			},
		}
		AllocatedVars[ident.Symbol] = node
		return node, true
	}
	return Node{}, false
}

func ASTParseNewVariableAssignment(lex *Lexer) (Node, bool) {
	if lex.MatchTokenSequence(tokIdentifier, tokColon, tokAssignment) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssertSequence(tokColon, tokAssignment)
		node := Node{
			NodeType:     nodeIdentifier,
			Reassignable: false,
			VarDecl: &VarDecl{
				Name:        ident,
				Assignment:  ASTParseExpression(lex),
				Initialized: true,
			},
		}
		AllocatedVars[ident.Symbol] = node
		return node, true
	}

	if lex.MatchTokenSequence(tokIdentifier, tokColon, tokIdentifier, tokAssignment) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssert(tokColon)
		varType := lex.Consume()
		lex.ConsumeAssert(tokAssignment)
		node := Node{
			NodeType:     nodeIdentifier,
			Reassignable: true,
			VarDecl: &VarDecl{
				Name:        ident,
				Type:        varType,
				Assignment:  ASTParseExpression(lex),
				Initialized: true,
			},
		}
		AllocatedVars[ident.Symbol] = node
		return node, true
	}
	return Node{}, false
}

func ASTParseProcCall(lex *Lexer) (Node, bool) {
	if lex.MatchTokenSequence(tokIdentifier, tokOpenParen) {
		ident := lex.ConsumeAssert(tokIdentifier)
		if _, ok := AllocatedProcs[ident.Symbol]; !ok {
			assert[any](false, "Proc doesn't exist")
		}
		lex.ConsumeAssert(tokOpenParen)
		lex.ConsumeAssert(tokCloseParen)
		node := Node{
			NodeType: nodeCall,
			Call: &Call{
				Name: ident,
			},
		}
		return node, true
	}
	return Node{}, false
}

func ASTParseExit(lex *Lexer) (Node, bool) {
	if lex.MatchTokenSequenceAndConsumeAssert(tokExit) {
		var message Node
		status := ASTParseExpression(lex)
		if lex.MatchTokenSequenceAndConsumeAssert(tokComma) {
			message = ASTParseExpression(lex)
		}
		node := Node{
			NodeType: nodeExit,
			Exit: &Exit{
				Status:  status,
				Message: message,
			},
		}
		return node, true
	}
	return Node{}, false
}

func ASTParsePrimaryExpression(lex *Lexer) Node {
	if node, ok := ASTParseProcDefinition(lex); ok {
		return node
	}

	if node, ok := ASTParseVariableDefinition(lex); ok {
		lex.ConsumeAssert(tokSemicolon)
		return node
	}

	if node, ok := ASTParseNewVariableAssignment(lex); ok {
		lex.ConsumeAssert(tokSemicolon)
		return node
	}

	if node, ok := ASTParseNewConstAssignment(lex); ok {
		lex.ConsumeAssert(tokSemicolon)
		return node
	}

	if ASTParseVariableReassignment(lex) {
		lex.ConsumeAssert(tokSemicolon)
		return Node{}
	}

	if node, ok := ASTParseProcCall(lex); ok {
		lex.ConsumeAssert(tokSemicolon)
		return node
	}

	if node, ok := ASTParseExit(lex); ok {
		lex.ConsumeAssert(tokSemicolon)
		return node
	}

	if lex.GetToken(0).TokenType == tokEndOfFile {
		return Node{}
	}

	return assert[Node](false, fmt.Sprintf("[ERROR] Unexpected token \"%s\"", lex.GetToken(0).Symbol))
}

func ASTParseStatement(lex *Lexer) Node {
	if lex.MatchAt(0, tokIdentifier, tokExit) {
		return ASTParsePrimaryExpression(lex)
	}
	return Node{}
}

func ASTCreateChaosProgram(lex *Lexer) Program {
	assert[any](lex.cursor == 0, "Cursor is not 0")

	program := Program{
		Nodes: []Node{},
	}

	for !lex.MatchAt(0, tokEndOfFile) {
		node := ASTParseStatement(lex)
		if node == (Node{}) {
			continue
		}
		program.Nodes = append(program.Nodes, node)
	}

	size := len(AllocatedVars)
	program.AllocatedVars = AllocatedVars
	program.AllocatedProcs = AllocatedProcs

	fmt.Printf("Allocated Vars [%d] - Node list:\n", size)
	for idx, node := range program.Nodes {
		node.print(idx)
		fmt.Print("\n")
	}

	return program
}
