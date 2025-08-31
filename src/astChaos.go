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
)

type BinOpOperation int

const (
	opPlus BinOpOperation = iota
)

type VarDecl struct {
	Name       Token
	Type       Token
	Assignment Node
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

type Node struct {
	NodeType     NodeType
	Reassignable bool
	Literal      *Literal
	VarDecl      *VarDecl
	Exit         *Exit
	BinOp        *BinOp
}

type Program struct {
	Nodes         []Node
	AllocatedVars map[string]Node
}

var AllocatedVars map[string]Node = map[string]Node{}

func (p *Program) print() {
	fmt.Print("Node list:\n")
	for idx, node := range p.Nodes {
		if node.NodeType == nodeIdentifier {

			if node.VarDecl.Assignment.NodeType == nodeBinOp {
				reassinableType := "const"
				if node.Reassignable {
					reassinableType = "var"
				}

				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}

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
						continue
					}
					if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIdentifier {
						l := castAssert[int](lhs.Literal.Int.Value)
						r := rhs.VarDecl.Name.Symbol
						fmt.Printf("%6d. %s(%s, %s) = %d + %s\n", idx, reassinableType, name, t, l, r)
						continue
					}
					if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIntLiteral {
						l := lhs.VarDecl.Name.Symbol
						r := castAssert[int](rhs.Literal.Int.Value)
						fmt.Printf("%6d. %s(%s, %s) = %s + %d\n", idx, reassinableType, name, t, l, r)
						continue
					}
					if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIdentifier {
						l := lhs.VarDecl.Name.Symbol
						r := rhs.VarDecl.Name.Symbol
						fmt.Printf("%6d. %s(%s, %s) = %s + %s\n", idx, reassinableType, name, t, l, r)
						continue
					}
				}
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
				reassinableType := "const"
				if node.Reassignable {
					reassinableType = "var"
				}
				result := castAssert[int](node.VarDecl.Assignment.Literal.Int.Value)
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. %s(%s, %s) = %d\n", idx, reassinableType, name, t, result)
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeFloatLiteral {
				reassinableType := "const"
				if node.Reassignable {
					reassinableType = "var"
				}
				result := castAssert[float64](node.VarDecl.Assignment.Literal.Int.Value)
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. %s(%s, %s) = %.2f\n", idx, reassinableType, name, t, result)
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
				reassinableType := "const"
				if node.Reassignable {
					reassinableType = "var"
				}
				result := castAssert[string](node.VarDecl.Assignment.Literal.String.Value)
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. %s(%s, %s) = \"%s\"\n", idx, reassinableType, name, t, result)
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeIdentifier {
				reassinableType := "const"
				if node.Reassignable {
					reassinableType = "var"
				}
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. %s(%s, %s) = %s\n", idx, reassinableType, name, t, node.VarDecl.Assignment.VarDecl.Name.Symbol)
				continue
			}

			fmt.Printf("[ERROR] Not implemented :: %+v\n", node.VarDecl.Assignment)
			continue
		}

		if node.NodeType == nodeExit {
			msg := ""
			if node.Exit.Message.Literal.String.Value != "" {
				msg = castAssert[string](node.Exit.Message.Literal.String.Value)
			}

			if node.Exit.Status.VarDecl.Name.Symbol != "" {
				status := node.Exit.Status.VarDecl.Name.Symbol
				fmt.Printf("%6d. exit(%s, \"%s\")\n", idx, status, msg)
			} else if status, ok := cast[int](node.Exit.Status.Literal.Int.Value); ok {
				fmt.Printf("%6d. exit(%d, \"%s\")\n", idx, status, msg)
			}
			continue
		}

		panic(fmt.Sprintf("[ERROR] Unknown node '%+v'", node))
	}
}

func ASTParseExpression(lex *Lexer) Node {
	expr := lex.Consume()
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

	return assert[Node](false, fmt.Sprintf("Unknown token found - \"%s\"", expr.TokenType.String()))
}

func ASTParsePrimaryExpression(lex *Lexer) Node {
	if identifier, matches := lex.MatchTokenAndConsumeAssert(tokIdentifier); matches {
		if lex.MatchTokenSequence(tokColon, tokColon) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokColon)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType:     nodeIdentifier,
				Reassignable: false,
				VarDecl: &VarDecl{
					Name:       identifier,
					Assignment: rhs,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node
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
					Name:       identifier,
					Type:       varType,
					Assignment: rhs,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node
		}

		if lex.MatchTokenSequence(tokColon, tokAssignment) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokAssignment)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType:     nodeIdentifier,
				Reassignable: true,
				VarDecl: &VarDecl{
					Name:       identifier,
					Assignment: rhs,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node
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
					Name:       identifier,
					Type:       varType,
					Assignment: rhs,
				},
			}
			AllocatedVars[identifier.Symbol] = node
			return node
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
			return node
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
			return node

		}
	}

	panic(fmt.Sprintf("[ERROR] Unknown token \"%s\"", lex.GetToken(0).Symbol))
}

func ASTParseStatement(lex *Lexer) Node {
	node := ASTParsePrimaryExpression(lex)
	return node
}

func ASTCreateChaosProgram(lex *Lexer) Program {
	assert[any](lex.cursor == 0, "Cursor is not 0")

	program := Program{
		Nodes: []Node{},
	}

	for !lex.MatchAt(0, tokEndOfFile) {
		node := ASTParseStatement(lex)
		program.Nodes = append(program.Nodes, node)
	}

	program.AllocatedVars = AllocatedVars

	program.print()
	return program
}
