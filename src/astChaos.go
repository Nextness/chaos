package main

import "fmt"

// import (
//
//	"errors"
//	"fmt"
//	"os"
//	"slices"
//
// )
//
// const PADNUMBER int = 2
//
//	func nodeInfer(tokKind TokenKind) *TokenVarType {
//		result := &TokenVarType{symbol: "infer", tokType: tokInferType, tokKind: tokKind}
//		return result
//	}

type NodeType int

const (
	nodeLiteral NodeType = iota
	nodeVariable
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

func (p *Program) print() {
	for _, node := range p.Nodes {
		if node.NodeType == nodeVariable {
			fmt.Printf("#define %s : infer : %d\n", node.VarName.Symbol, node.VarValue.LiteralInt.Value)
		}
	}
}

func ASTCreateChaosProgram(ls *LexerState) Program {
	assert(ls.cursor == 0, "Cursor is not 0")

	program := Program{
		Nodes: []Node{},
	}

	for ls.cursor < ls.count {
		if ls.MatchAt(0, tokEndOfFile) {
			break
		}

		if ls.MatchAt(0, tokIdentifier) {
			currentVariable := ls.ConsumeAssert(tokIdentifier)
			if ls.MatchTokenSequence(tokColon, tokColon) {
				ls.ConsumeAssertMany(tokColon, tokColon)
				value := ls.Consume()
				ls.ConsumeAssert(tokSemicolon)
				nodeLit := &Node{
					NodeType:   nodeLiteral,
					LiteralInt: value,
				}
				program.Nodes = append(program.Nodes, Node{
					NodeType: nodeVariable,
					VarName:  currentVariable,
					VarValue: nodeLit,
				})
				continue
			}
		}

		panic("Unrecheable")
	}
	program.print()
	return program
}
