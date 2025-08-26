package main

import "fmt"

type NodeType int

const (
	nodeIntLiteral NodeType = iota
	nodeStringLiteral
	nodeIdentifier
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
			}
			if result, ok := cast[float64](node.VarValue.LiteralInt.Value); ok {
				name := node.VarName.Symbol
				t := "infer"
				if node.VarType.Symbol != "" {
					t = node.VarType.Symbol
				}
				fmt.Printf("%6d. var(%s, %s) = %.2f\n", idx, name, t, result)
			}
		}
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
	} else if expr.TokenType == tokStringLiteral {
		lex.ConsumeAssert(tokSemicolon)
		return Node{
			NodeType:      nodeStringLiteral,
			LiteralString: expr,
		}
	} else {
		assert(false, fmt.Sprintf("Unknown token found at ASTParseExpresion - %s", expr.TokenType.String()))
	}
	return Node{}
}

func ASTParsePrimaryExpression(lex *Lexer) Node {
	if identifier, matches := lex.MatchTokenAndConsumeAssert(tokIdentifier); matches {
		if lex.MatchTokenSequence(tokColon, tokColon) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokColon)
			node := ASTParseExpression(lex)
			n := Node{
				NodeType: nodeIdentifier,
				VarName:  identifier,
				VarValue: &node,
			}
			AllocatedVars[node.LiteralInt.Symbol] = n
			return n
		}

		if lex.MatchTokenSequence(tokColon, tokIdentifier, tokColon) {
			lex.ConsumeAssert(tokColon)
			varType := lex.Consume()
			lex.ConsumeAssert(tokColon)

			node := ASTParseExpression(lex)
			n := Node{
				NodeType: nodeIdentifier,
				VarName:  identifier,
				VarType:  varType,
				VarValue: &node,
			}
			AllocatedVars[node.LiteralInt.Symbol] = n
			return n
		}

		if lex.MatchTokenSequence(tokColon, tokAssignment) {
			lex.ConsumeAssert(tokColon)
			lex.ConsumeAssert(tokAssignment)

			node := ASTParseExpression(lex)
			n := Node{
				NodeType: nodeIdentifier,
				VarName:  identifier,
				VarValue: &node,
			}
			AllocatedVars[node.LiteralInt.Symbol] = n
			return n
		}

		if lex.MatchTokenSequence(tokColon, tokIdentifier, tokAssignment) {
			lex.ConsumeAssert(tokColon)
			varType := lex.Consume()
			lex.ConsumeAssert(tokAssignment)

			node := ASTParseExpression(lex)
			n := Node{
				NodeType: nodeIdentifier,
				VarName:  identifier,
				VarType:  varType,
				VarValue: &node,
			}
			AllocatedVars[node.LiteralInt.Symbol] = n
			return n
		}
	}
	return Node{}
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
