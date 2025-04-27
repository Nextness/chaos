package main

import (
	"fmt"
	"os"
)

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
	fmt.Printf("    Type:   '%s'\n", n.nodeType.asString())
	fmt.Printf("    State:  '%s'\n", n.identifier.stt)
	// TODO: Improve this printing and handling of type in Nodes
	if n.identifier.tpe == "String" {
		fmt.Printf("    Symbol: '%s (%s) = «%s»'\n", n.identifier.sym, n.identifier.tpe, n.identifier.val)
		return
	}
	fmt.Printf("    Symbol: '%s (%s) = %s'\n", n.identifier.sym, n.identifier.tpe, n.identifier.val)
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
	varName := ts.consume().value
	if _, ok := ts.variables[varName]; !ok {
		ts.variables[varName] = ""
	}

	if !ts.expects(0, tokVarType, tokAssignment) {
		errMsg := fmt.Sprintf("Expected a type or assignment but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
		config.printAndExitLexerError(ts)
	}

	if ts.expects(0, tokVarType) {
		newNode.identifier.tpe = ts.consume().value
	}

	if ts.expects(0, tokAssignment) {
		if newNode.identifier.tpe == "" {
			newNode.identifier.tpe = "must-infer"
		}
		ts.consume() // Consume '='
		if !ts.expects(0, tokNumber, tokIdentifier, tokString) {
			errMsg := fmt.Sprintf("Expected a number, or identifier or string but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
			config.printAndExitLexerError(ts)
		}
		if ts.expects(0, tokNumber) {
			value := ts.consume().value
			newNode.identifier.val = value
			newNode.identifier.stt = "initialized"
			ts.variables[varName] = value
		}
		if ts.expects(0, tokString) {
			value := ts.consume().value
			newNode.identifier.val = value
			newNode.identifier.stt = "initialized"
			ts.variables[varName] = value
		}
		if ts.expects(0, tokIdentifier) {
			if val, ok := ts.variables[ts.current().value]; ok {
				name := ts.consume().value
				ts.variables[name] = val
				newNode.identifier.val = name
				newNode.identifier.stt = "initialized"
			} else {
				errMsg := fmt.Sprintf("Undefined identifier %s\n", ts.current().value)
				examples := []string{
					"let something U64\n",
					"let something U64 = 1\n",
					"let something U64 = 1\n" +
						"            let somethingElse = something\n",
					"let something String = «string literal»\n",
					"let something String = «string literal»\n" +
						"            let somethingElse = something\n",
				}
				config := newLexerErroConfig(errMsg, examples...)
				config.printAndExitLexerError(ts)
			}
		}
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
	lexerState.count = len(lexerState.data)
	return &lexerState
}
