package main

import (
	"fmt"
	"os"
)

type LexerArray struct {
	data   []any
	count  int
	cursor int
}

type LexerState struct {
	data   []any
	count  int
	cursor int
}

type NodeType int

const (
	identifierNode NodeType = iota
	literalNode
)

type NodeToStringMap map[NodeType]string

var nodeMapping = NodeToStringMap{
	identifierNode: "identifierNode",
	literalNode:    "literalNode",
}

func (tok NodeType) asString() string {
	if tokType, ok := nodeMapping[tok]; ok {
		return tokType
	}
	errorMsg := fmt.Sprintf("[ERROR] Unexpected node '%d' - fix this shitty code :)\n", tok)
	panic(errorMsg)
}

type Node struct {
	nodeType   NodeType
	identifier struct {
		sym string
		stt string
		tpe string
		val string
	}
}

func (n *Node) print() {
	fmt.Printf("Node:\n")
	fmt.Printf("  Type:   '%s'\n", n.nodeType.asString())
	fmt.Printf("  State:  '%s'\n", n.identifier.stt)
	fmt.Printf("  Symbol: '%s (%s) = «%s»'\n", n.identifier.sym, n.identifier.tpe, n.identifier.val)
}

func lexChaosLet(ts *TokenizerState) *Node {
	var newNode Node

	ts.consume() // consume the 'let'
	newNode.nodeType = identifierNode
	newNode.identifier.sym = ts.current().value

	if !ts.expects(0, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected an identifier but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
		config.printAndExitLexerError(ts)
	}
	ts.consume()

	if !ts.expects(0, tokVarType, tokAssignment) {
		errMsg := fmt.Sprintf("Expected a type or assignment but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
		config.printAndExitLexerError(ts)
	}

	if ts.expects(0, tokVarType) {
		newNode.identifier.tpe = ts.consume().value
	}

	if ts.expects(0, tokAssignment) {
		newNode.identifier.tpe = "must-infer"
		ts.consume() // Consume '='
		if !ts.expects(0, tokNumber) {
			errMsg := fmt.Sprintf("Expected a number but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
			config.printAndExitLexerError(ts)
		}
		newNode.identifier.val = ts.consume().value
		newNode.identifier.stt = "initialized"
	} else if ts.expects(0, tokNewline) {
		newNode.identifier.val = ""
		newNode.identifier.stt = "not-initialized"
	} else {
		errMsg := fmt.Sprintf("Unexpected a token found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
		config.printAndExitLexerError(ts)
	}

	ts.consume() // consume the 'newline'
	return &newNode
}

func lexerChaos(ts *TokenizerState) *LexerState {
	assert(ts.cursor == 0, "Cursor is not 0")
	lexerState := LexerState{data: []any{}, count: 0, cursor: 0}
	var node *Node = nil

	for ts.cursor < ts.count {
		if ts.expects(0, tokEndOfFile) {
			break
		}

		if ts.expects(0, tokLet) {
			// TODO: better hanadle lexing errors
			if node = lexChaosLet(ts); node == nil {
				os.Exit(1)
			}
			node.print()
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.expects(0, tokNewline) {
			ts.consume()
			continue
		}
	}

	assert(tokEndOfFile == ts.current().tokType, "Expected eof")
	ts.consume()
	return &lexerState
}
