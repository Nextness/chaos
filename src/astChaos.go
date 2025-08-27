package main

import "fmt"

type NodeType int

const (
	nodeIntLiteral NodeType = iota
	nodeStringLiteral
	nodeIdentifier
	nodeExit
)

type Node struct {
	NodeType NodeType

	// Literal
	LiteralInt    Token
	LiteralString Token

	// Variable Declaration
	VarName  Token
	VarType  Token
	VarValue *Node

	// Exit
	ExVal int
	ExMsg string
}

type Program struct {
	Nodes []Node
}

var AllocatedVars map[string]Node = map[string]Node{}

func (p *Program) print() {
	fmt.Print("Node list:\n")
	for idx, node := range p.Nodes {
		if node.NodeType == nodeIdentifier {
			if result, ok := cast[int](node.VarValue.LiteralInt.Value); ok {
				name := node.VarName.Symbol
				t := "infer"
				if node.VarType.Symbol != "" {
					t = node.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %d\n", idx, name, t, result)
				continue
			}

			if result, ok := cast[float64](node.VarValue.LiteralInt.Value); ok {
				name := node.VarName.Symbol
				t := "infer"
				if node.VarType.Symbol != "" {
					t = node.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %.2f\n", idx, name, t, result)
				continue
			}

			if result := castAssert[string](node.VarValue.LiteralString.Value); result != "" {
				name := node.VarName.Symbol
				t := "infer"
				if node.VarType.Symbol != "" {
					t = node.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = \"%s\"\n", idx, name, t, result)
				continue
			}
			fmt.Printf("[ERROR] Not implemented :: %+v\n", node.VarValue)
			continue
		}

		if node.NodeType == nodeExit {
			status := node.ExVal
			if node.ExMsg != "" {
				fmt.Printf("%6d. exit(%d, \"%s\")\n", idx, status, node.ExMsg)
				continue
			}
			fmt.Printf("%6d. exit(%d)\n", idx, status)
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

	panic(fmt.Sprintf("Unknown token found at ASTParseExpresion - \"%s\"", expr.TokenType.String()))
}

func ASTParsePrimaryExpression(lex *Lexer) Node {
	if identifier, matches := lex.MatchTokenAndConsumeAssert(tokIdentifier); matches {
		if lex.MatchTokenSequence(tokColon, tokColon) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokColon)

			rhs := ASTParseExpression(lex)
			node := Node{
				NodeType: nodeIdentifier,
				VarName:  identifier,
				VarValue: &rhs,
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
				VarName:  identifier,
				VarType:  varType,
				VarValue: &rhs,
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
				VarName:  identifier,
				VarValue: &rhs,
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
				VarName:  identifier,
				VarType:  varType,
				VarValue: &rhs,
			}
			AllocatedVars[identifier.Symbol] = node
			return node
		}
	}

	if _, matches := lex.MatchTokenAndConsumeAssert(tokExit); matches {
		if status, matches := lex.MatchTokenAndConsumeAssert(tokNumberLiteral); matches {
			exMsg := ""
			if lex.MatchAt(0, tokComma) {
				lex.ConsumeAssert(tokComma)
				exMsg = castAssert[string](lex.ConsumeAssert(tokStringLiteral).Value)
			}
			lex.ConsumeAssert(tokSemicolon)

			node := Node{
				NodeType: nodeExit,
				ExVal:    castAssert[int](status.Value),
				ExMsg:    exMsg,
			}
			return node
		}
		if status, matches := lex.MatchTokenAndConsumeAssert(tokIdentifier); matches {
			exMsg := ""
			if lex.MatchAt(0, tokComma) {
				lex.ConsumeAssert(tokComma)
				exMsg = castAssert[string](lex.ConsumeAssert(tokStringLiteral).Value)
			}
			lex.ConsumeAssert(tokSemicolon)

			identValue := AllocatedVars[status.Symbol]
			node := Node{
				NodeType: nodeExit,
				ExVal:    castAssert[int](identValue.VarValue.LiteralInt.Value),
				ExMsg:    exMsg,
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

	program.print()
	return program
}
