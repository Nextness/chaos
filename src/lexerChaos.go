package main

import (
	"fmt"
	"os"
	"strconv"
)

type Node interface {
	print()
}

type LexerState struct {
	data   []Node
	count  int
	cursor int
}

type NodeExitWith struct {
	sym string
	val int
	msg string
}

type NodeIdentifier struct {
	sym string
	stt string
	tpe string
	val string
}

func (n *NodeExitWith) print() {
	fmt.Printf("Node\n")
	fmt.Printf("├─➜ Type:   'chaosIntrisic'\n")
	fmt.Printf("├─➜ Symbol: 'exitWith'\n")
	if n.msg != "" {
		fmt.Printf("├─➜ Value:  '%d'\n", n.val)
		fmt.Printf("└─➜ Msg:    '%s'\n", n.msg)
		return
	}
	fmt.Printf("└─➜ Value:  '%d'\n", n.val)
}

func (n *NodeIdentifier) print() {
	fmt.Printf("Node\n")
	fmt.Printf("├─➜ State:  '%s'\n", n.stt)
	// TODO: Improve this printing and handling of type in Nodes
	if n.tpe == kindString.asString() {
		fmt.Printf("└─➜ Symbol: '%s (%s) = «%s»'\n", n.sym, n.tpe, n.val)
		return
	}
	fmt.Printf("└─➜ Symbol: '%s (%s) = %s'\n", n.sym, n.tpe, n.val)
}

func lexChaosLet(ts *TokenizerState) *NodeIdentifier {
	newNode := NodeIdentifier{}

	ts.consume() // consume the 'let'
	newNode.sym = ts.current().value

	if !ts.matchAt(0, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected an identifier but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
		config.printAndExitLexerError(ts)
	}
	varName := ts.consume().value
	if _, ok := ts.variables[varName]; !ok {
		ts.variables[varName] = ""
	}

	if !ts.matchAt(0, tokVarType, tokAssignment) {
		errMsg := fmt.Sprintf("Expected a type or assignment but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
		config.printAndExitLexerError(ts)
	}

	if ts.matchAt(0, tokVarType) {
		token := ts.consume()
		newNode.tpe = token.tokKind.asString()
	}

	if ts.matchAt(0, tokAssignment) {
		if newNode.tpe == "" {
			newNode.tpe = "must-infer"
		}
		ts.consume() // Consume '='
		if !ts.matchAt(0, tokNumber, tokIdentifier, tokString) {
			errMsg := fmt.Sprintf("Expected a number, or identifier or string but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
			config.printAndExitLexerError(ts)
		}
		if ts.matchAt(0, tokNumber) {
			value := ts.consume().value
			newNode.val = value
			newNode.stt = "initialized"
			ts.variables[varName] = value
		}
		if ts.matchAt(0, tokString) {
			value := ts.consume().value
			newNode.val = value
			newNode.stt = "initialized"
			ts.variables[varName] = value
		}
		if ts.matchAt(0, tokIdentifier) {
			if val, ok := ts.variables[ts.current().value]; ok {
				name := ts.consume().value
				ts.variables[name] = val
				newNode.val = name
				newNode.stt = "initialized"
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
	} else if ts.matchAt(0, tokNewline) {
		newNode.val = ""
		newNode.stt = "not-initialized"
	} else {
		errMsg := fmt.Sprintf("Unexpected a token found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg, "let something U64\n", "let something U64 = 1\n")
		config.printAndExitLexerError(ts)
	}

	ts.consume() // consume the 'newline'
	return &newNode
}

func lexExitWithChaos(ts *TokenizerState) *NodeExitWith {
	newNode := NodeExitWith{}

	ts.consume()
	if !ts.matchAt(0, tokNumber, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected a number or identifier but found %s\n", ts.current().tokType.asString())
		examples := []string{
			"exitWith 1\n",
			"exitWith 1, «reason for exiting early»\n",
			"let code U64 = 1" +
				"            exitWith code\n",
			"let code U64 = 1" +
				"            exitWith code, «reason for exitin early»\n",
		}
		config := newLexerErroConfig(errMsg, examples...)
		config.printAndExitLexerError(ts)
	}

	if ts.matchAt(0, tokNumber) {
		token := ts.consume()
		asInt, err := strconv.Atoi(token.value)
		if err != nil {
			panic("failed to convert str to number")
		}
		newNode.val = asInt
	}
	if ts.matchAt(0, tokIdentifier) {
		token := ts.consume()
		val := ts.variables[token.value]
		asInt, err := strconv.Atoi(val)
		if err != nil {
			panic("failed to convert str to number")
		}
		newNode.sym = token.value
		newNode.val = asInt
	}

	if ts.matchAt(0, tokComma) {
		if ts.matchAt(1, tokString) {
			ts.consume()
			newNode.msg = ts.consume().value
		} else {
			if ts.matchAt(0, tokComma) {
				ts.consume()
			}
			errMsg := fmt.Sprintf("Expected a string but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg, "exitWith 1, «reason for exiting early»\n")
			config.printAndExitLexerError(ts)
		}
	}

	ts.consume() // consume the 'newline'
	return &newNode
}

func lexerChaos(ts *TokenizerState) *LexerState {
	assert(ts.cursor == 0, "Cursor is not 0")
	lexerState := LexerState{data: []Node{}, count: 0, cursor: 0}

	for ts.cursor < ts.count {
		if ts.matchAt(0, tokEndOfFile) {
			break
		}

		if ts.matchAt(0, tokLet) {
			node := &NodeIdentifier{}
			// TODO: better hanadle lexing errors
			if node = lexChaosLet(ts); node == nil {
				os.Exit(1)
			}
			node.print()
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokExitWith) {
			node := &NodeExitWith{}
			if node = lexExitWithChaos(ts); node == nil {
				os.Exit(1)
			}
			node.print()
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokNewline) {
			ts.consume()
			continue
		}

		fmt.Fprintf(os.Stderr, "[ERROR] The token '%s' is not expected\n", ts.current().value)
		panic("Expected token found")
	}

	assert(tokEndOfFile == ts.current().tokType, "Expected eof")
	ts.consume()
	lexerState.count = len(lexerState.data)
	return &lexerState
}
