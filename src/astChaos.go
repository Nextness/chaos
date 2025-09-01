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
	Scope []Node
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
			fmt.Printf("%6d. %s() -> Void {\n", idx, name)
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

	assert[any](false, fmt.Sprintf("[ERROR] Unknown node '%+v'", node))
}

func ASTParseProc(lex *Lexer) Node {
	node := Node{
		NodeType: nodeProcDef,
		Proc: &Proc{
			Scope: []Node{},
		},
	}
	lex.ConsumeAssert(tokOpenBraket)
	for lex.GetToken(0).TokenType != tokCloseBraket {
		stmt, _ := ASTParseStatement(lex)
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
		lex.ConsumeAssert(tokSemicolon)
		return Node{
			NodeType: nodeIntLiteral,
			Literal: &Literal{
				Int: expr,
			},
		}
	}

	if expr.TokenType == tokStringLiteral {
		lex.ConsumeAssert(tokSemicolon)
		return Node{
			NodeType: nodeStringLiteral,
			Literal: &Literal{
				String: expr,
			},
		}
	}

	if expr.TokenType == tokIdentifier {
		ident := AllocatedVars[expr.Symbol]
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
		lex.ConsumeAssert(tokSemicolon)
		return ident
	}

	token := lex.GetToken(0)
	return assert[Node](false, fmt.Sprintf("Unknown token found - \"%s\"", token.TokenType.String()))
}

func ASTParsePrimaryExpression(lex *Lexer) (Node, bool) {
	if identifier, matches := lex.MatchTokenAndConsumeAssert(tokIdentifier); matches {
		if _, ok := AllocatedProcs[identifier.Symbol]; ok {
			lex.ConsumeAssert(tokOpenParen)
			lex.ConsumeAssert(tokCloseParen)
			lex.ConsumeAssert(tokSemicolon)
			node := Node{
				NodeType: nodeCall,
				Call: &Call{
					Name: identifier,
				},
			}
			return node, false
		}

		if lex.MatchAt(0, tokAssignment) {
			lex.ConsumeAssert(tokAssignment)
			rhs := ASTParseExpression(lex)
			node := AllocatedVars[identifier.Symbol]
			node.VarDecl.Assignment = rhs
			node.VarDecl.Initialized = true
			node.Reassignable = true
			AllocatedVars[identifier.Symbol] = node
			return Node{}, true
		}

		if lex.MatchTokenSequence(tokColon, tokIdentifier, tokSemicolon) {
			lex.ConsumeAssert(tokColon)
			varType := lex.Consume()
			lex.ConsumeAssert(tokSemicolon)
			node := Node{
				NodeType:     nodeIdentifier,
				Reassignable: true,
				VarDecl: &VarDecl{
					Name:        identifier,
					Type:        varType,
					Initialized: false,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node, false
		}

		if lex.MatchTokenSequence(tokColon, tokColon) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokColon)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType:     nodeIdentifier,
				Reassignable: false,
				VarDecl: &VarDecl{
					Name:        identifier,
					Assignment:  rhs,
					Initialized: true,
				},
			}
			if rhs.NodeType == nodeProcDef {
				AllocatedProcs[identifier.Symbol] = node
			} else {
				AllocatedVars[identifier.Symbol] = node
			}
			return node, false
		}

		if lex.MatchTokenSequence(tokColon, tokIdentifier, tokColon) {
			lex.ConsumeAssert(tokColon)
			varType := lex.Consume()
			lex.ConsumeAssert(tokColon)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType:     nodeIdentifier,
				Reassignable: false,
				VarDecl: &VarDecl{
					Name:        identifier,
					Type:        varType,
					Assignment:  rhs,
					Initialized: true,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node, false
		}

		if lex.MatchTokenSequence(tokColon, tokAssignment) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokAssignment)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType:     nodeIdentifier,
				Reassignable: true,
				VarDecl: &VarDecl{
					Name:        identifier,
					Assignment:  rhs,
					Initialized: true,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node, false
		}

		if lex.MatchTokenSequence(tokColon, tokIdentifier, tokAssignment) {
			lex.ConsumeAssert(tokColon)
			varType := lex.Consume()
			lex.ConsumeAssert(tokAssignment)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType:     nodeIdentifier,
				Reassignable: true,
				VarDecl: &VarDecl{
					Name:        identifier,
					Type:        varType,
					Assignment:  rhs,
					Initialized: true,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node, false
		}
	}

	if _, matches := lex.MatchTokenAndConsumeAssert(tokExit); matches {
		if status, matches := lex.MatchTokenAndConsumeAssert(tokNumberLiteral); matches {
			var exMsg Token
			if lex.MatchAt(0, tokComma) {
				lex.ConsumeAssert(tokComma)
				exMsg = lex.ConsumeAssert(tokStringLiteral)
			}
			lex.ConsumeAssert(tokSemicolon)

			node := Node{
				NodeType: nodeExit,
				Exit: &Exit{
					Status: Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							Int: status,
						},
					},
					Message: Node{
						NodeType: nodeStringLiteral,
						Literal: &Literal{
							String: exMsg,
						},
					},
				},
			}
			return node, false
		}

		if token, matches := lex.MatchTokenAndConsumeAssert(tokIdentifier); matches {
			var exMsg Token
			if lex.MatchAt(0, tokComma) {
				lex.ConsumeAssert(tokComma)
				exMsg = lex.ConsumeAssert(tokStringLiteral)
			}
			lex.ConsumeAssert(tokSemicolon)
			identValue := AllocatedVars[token.Symbol]

			node := Node{
				NodeType: nodeExit,
				Exit: &Exit{
					Status: identValue,
					Message: Node{
						NodeType: nodeStringLiteral,
						Literal: &Literal{
							String: exMsg,
						},
					},
				},
			}
			return node, false

		}
	}

	return assert[Node](false, fmt.Sprintf("[ERROR] Unknown token \"%s\"", lex.GetToken(0).Symbol)), false
}

func ASTParseStatement(lex *Lexer) (Node, bool) {
	node, skip := ASTParsePrimaryExpression(lex)
	return node, skip
}

func ASTCreateChaosProgram(lex *Lexer) Program {
	assert[any](lex.cursor == 0, "Cursor is not 0")

	program := Program{
		Nodes: []Node{},
	}

	for !lex.MatchAt(0, tokEndOfFile) {
		node, skip := ASTParseStatement(lex)
		if !skip {
			program.Nodes = append(program.Nodes, node)
		}
	}

	size := len(AllocatedVars)
	program.AllocatedVars = AllocatedVars
	program.AllocatedProcs = AllocatedProcs

	fmt.Printf("Allocated Vars [%d] - Node list:\n", size)
	for idx, node := range program.Nodes {
		node.print(idx)
	}

	return program
}
