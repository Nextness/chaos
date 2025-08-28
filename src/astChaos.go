package main

import "fmt"

type NodeType int

const (
	nodeIntLiteral NodeType = iota
	nodeStringLiteral
	nodeIdentifier
	nodeExit
)

type VarDecl struct {
	Name  Token
	Type  Token
	Value *Node
}

type Exit struct {
	Value   *Node
	Message *Node
}

type Literal struct {
	Int    Token
	String Token
}

type Node struct {
	NodeType NodeType
	Literal  Literal
	VarDecl  VarDecl
	Exit     Exit
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
			if result, ok := cast[int](node.VarDecl.Value.Literal.Int.Value); ok {
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %d\n", idx, name, t, result)
				continue
			}

			if result, ok := cast[float64](node.VarDecl.Value.Literal.Int.Value); ok {
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %.2f\n", idx, name, t, result)
				continue
			}

			if result, ok := cast[string](node.VarDecl.Value.Literal.String.Value); ok && result != "" {
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = \"%s\"\n", idx, name, t, result)
				continue
			}

			if node.VarDecl.Value != nil {
				name := node.VarDecl.Name.Symbol
				t := "infer"
				if node.VarDecl.Type.Symbol != "" {
					t = node.VarDecl.Type.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %s\n", idx, name, t, node.VarDecl.Value.VarDecl.Name.Symbol)
				continue
			}

			fmt.Printf("[ERROR] Not implemented :: %+v\n", node.VarDecl.Value)
			continue
		}

		if node.NodeType == nodeExit {
			msg := ""
			if node.Exit.Message.Literal.String.Value != "" {
				msg = castAssert[string](node.Exit.Message.Literal.String.Value)
			}

			if node.Exit.Value.VarDecl.Name.Symbol != "" {
				status := node.Exit.Value.VarDecl.Name.Symbol
				fmt.Printf("%6d. exit(%s, \"%s\")\n", idx, status, msg)
			} else if status, ok := cast[int](node.Exit.Value.Literal.Int.Value); ok {
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
		lex.ConsumeAssert(tokSemicolon)
		return Node{
			NodeType: nodeIntLiteral,
			Literal: Literal{
				Int: expr,
			},
		}
	}

	if expr.TokenType == tokStringLiteral {
		lex.ConsumeAssert(tokSemicolon)
		return Node{
			NodeType: nodeStringLiteral,
			Literal: Literal{
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
				VarDecl: VarDecl{
					Name:  identifier,
					Value: &rhs,
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
				VarDecl: VarDecl{
					Name:  identifier,
					Type:  varType,
					Value: &rhs,
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
				VarDecl: VarDecl{
					Name:  identifier,
					Value: &rhs,
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
				VarDecl: VarDecl{
					Name:  identifier,
					Type:  varType,
					Value: &rhs,
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
					Value: &Node{
						NodeType: nodeIntLiteral,
						Literal: Literal{
							Int: status,
						},
					},
					Message: &Node{
						NodeType: nodeStringLiteral,
						Literal: Literal{
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
					Value: &identValue,
					Message: &Node{
						NodeType: nodeStringLiteral,
						Literal: Literal{
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
