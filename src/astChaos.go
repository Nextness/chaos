package main

import "fmt"

type NodeType int

const (
	nodeIntLiteral NodeType = iota
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
	Status  *Node
	Message *Node
}

type Literal struct {
	Int    Token
	String Token
}

type BinOp struct {
	Operation BinOpOperation
	Lhs       *Node
	Rhs       *Node
}

type Node struct {
	NodeType NodeType
	Literal  *Literal
	VarDecl  *VarDecl
	Exit     Exit
	BinOp    BinOp
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
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				lhs, okLhs := cast[int](node.VarDecl.Assignment.BinOp.Lhs.Literal.Int.Value)
				rhs, okRhs := cast[int](node.VarDecl.Assignment.BinOp.Rhs.Literal.Int.Value)
				if okLhs && okRhs {
					fmt.Printf("%6d. var(%s, %s) = %d + %d\n", idx, name, t, lhs, rhs)
				}
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
				result := castAssert[int](node.VarDecl.Assignment.Literal.Int.Value)
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %d\n", idx, name, t, result)
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeFloatLiteral {
				result := castAssert[float64](node.VarDecl.Assignment.Literal.Int.Value)
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %.2f\n", idx, name, t, result)
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
				result := castAssert[string](node.VarDecl.Assignment.Literal.String.Value)
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = \"%s\"\n", idx, name, t, result)
				continue
			}

			if node.VarDecl.Assignment.NodeType == nodeIdentifier {
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %s\n", idx, name, t, node.VarDecl.Assignment.VarDecl.Name.Symbol)
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
			n := ASTParseExpression(lex)
			nod := Node{
				NodeType: nodeBinOp,
				BinOp: BinOp{
					Operation: opPlus,
					Lhs: &Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							Int: expr,
						},
					},
					Rhs: &n,
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
		lex.ConsumeAssert(tokSemicolon)
		return ident
	}

	assert(false, fmt.Sprintf("Unknown token found - \"%s\"", expr.TokenType.String()))
	return Node{}
}

func ASTParsePrimaryExpression(lex *Lexer) Node {
	if identifier, matches := lex.MatchTokenAndConsumeAssert(tokIdentifier); matches {
		if lex.MatchTokenSequence(tokColon, tokColon) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokColon)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType: nodeIdentifier,
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
				NodeType: nodeIdentifier,
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
				NodeType: nodeIdentifier,
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
				NodeType: nodeIdentifier,
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
				Exit: Exit{
					Status: &Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							Int: status,
						},
					},
					Message: &Node{
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
				Exit: Exit{
					Status: &identValue,
					Message: &Node{
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
	assert(lex.cursor == 0, "Cursor is not 0")

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
