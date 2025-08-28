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
	VarName  Token
	VarType  Token
	VarValue *Node
}

type Exit struct {
	ExVal *Node
	ExMsg *Node
}

type Node struct {
	NodeType NodeType

	// Literal
	LiteralInt    Token
	LiteralString Token

	// Variable Declaration
	VarDecl VarDecl

	// Exit
	Exit Exit
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
			if result, ok := cast[int](node.VarDecl.VarValue.LiteralInt.Value); ok {
				name := node.VarDecl.VarName.Symbol
				t := "infer"
				if node.VarDecl.VarType.Symbol != "" {
					t = node.VarDecl.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %d\n", idx, name, t, result)
				continue
			}

			if result, ok := cast[float64](node.VarDecl.VarValue.LiteralInt.Value); ok {
				name := node.VarDecl.VarName.Symbol
				t := "infer"
				if node.VarDecl.VarType.Symbol != "" {
					t = node.VarDecl.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %.2f\n", idx, name, t, result)
				continue
			}

			if result, ok := cast[string](node.VarDecl.VarValue.LiteralString.Value); ok && result != "" {
				name := node.VarDecl.VarName.Symbol
				t := "infer"
				if node.VarDecl.VarType.Symbol != "" {
					t = node.VarDecl.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = \"%s\"\n", idx, name, t, result)
				continue
			}

			if node.VarDecl.VarValue != nil {
				name := node.VarDecl.VarName.Symbol
				t := "infer"
				if node.VarDecl.VarType.Symbol != "" {
					t = node.VarDecl.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %s\n", idx, name, t, node.VarDecl.VarValue.VarDecl.VarName.Symbol)
				continue
			}

			fmt.Printf("[ERROR] Not implemented :: %+v\n", node.VarDecl.VarValue)
			continue
		}

		if node.NodeType == nodeExit {
			msg := ""
			if node.Exit.ExMsg.LiteralString.Value != "" {
				msg = castAssert[string](node.Exit.ExMsg.LiteralString.Value)
			}

			if node.Exit.ExVal.VarDecl.VarName.Symbol != "" {
				status := node.Exit.ExVal.VarDecl.VarName.Symbol
				fmt.Printf("%6d. exit(%s, \"%s\")\n", idx, status, msg)
			} else if status, ok := cast[int](node.Exit.ExVal.LiteralInt.Value); ok {
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
			NodeType:   nodeIntLiteral,
			LiteralInt: expr,
		}
	}

	if expr.TokenType == tokStringLiteral {
		lex.ConsumeAssert(tokSemicolon)
		return Node{
			NodeType:      nodeStringLiteral,
			LiteralString: expr,
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
					VarName:  identifier,
					VarValue: &rhs,
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
					VarName:  identifier,
					VarType:  varType,
					VarValue: &rhs,
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
					VarName:  identifier,
					VarValue: &rhs,
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
					VarName:  identifier,
					VarType:  varType,
					VarValue: &rhs,
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
					ExVal: &Node{
						NodeType:   nodeIntLiteral,
						LiteralInt: status,
					},
					ExMsg: &Node{
						NodeType:      nodeStringLiteral,
						LiteralString: exMsg,
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
					ExVal: &identValue,
					ExMsg: &Node{
						NodeType:      nodeStringLiteral,
						LiteralString: exMsg,
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
