package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Node interface {
	print(int)
}

type LexerState struct {
	data   []Node
	count  int
	cursor int
}

type NodeCondition struct {
	conditionCount int
	conditions     []NodeBinOp
	scope          map[int][]Node
}

type NodeBinOp struct {
	lhs string
	rhs string
	op  string
}

type NodeExitWith struct {
	sym string
	msg string
	val int
}

type NodeIdentifier struct {
	sym string
	stt string
	tpe string
	val any
}

var _ Node = &NodeExitWith{}
var _ Node = &NodeIdentifier{}
var _ Node = &NodeBinOp{}

func makePad(padSize int) string {
	pad := strings.Repeat(" ", padSize)
	return pad
}

func (n *NodeCondition) print(padSize int) {
	pad := makePad(padSize)
	fmt.Printf("%sNodeCondition\n", pad)
	for _, binOp := range n.conditions {
		binOp.print(padSize + 4)
	}
	var i int
	for i <= n.conditionCount {
		for _, expr := range n.scope[i] {
			expr.print(padSize + 4)
		}
		i++
	}
	fmt.Printf("%s\n", pad)
}

func (n *NodeBinOp) print(padSize int) {
	pad := makePad(padSize)
	fmt.Printf("%sNodeBinOp\n", pad)
	fmt.Printf("%s├─➜ lhs: '%s'\n", pad, n.lhs)
	fmt.Printf("%s├─➜ rhs: '%s'\n", pad, n.rhs)
	fmt.Printf("%s└─➜ op:  '%s'\n", pad, n.op)
}

func (n *NodeExitWith) print(padSize int) {
	pad := strings.Join([]string{strings.Repeat(" ", padSize)}, "")
	fmt.Printf("%sNodeExitWith\n", pad)
	fmt.Printf("%s├─➜ Type:   'chaosIntrisic'\n", pad)
	fmt.Printf("%s├─➜ Symbol: 'exitWith'\n", pad)
	if n.msg != "" {
		fmt.Printf("%s├─➜ Value:  '%d'\n", pad, n.val)
		fmt.Printf("%s└─➜ Msg:    '%s'\n", pad, n.msg)
		return
	}
	fmt.Printf("%s└─➜ Value:  '%d'\n", pad, n.val)
}

func (n *NodeIdentifier) print(padSize int) {
	pad := strings.Join([]string{strings.Repeat(" ", padSize)}, "")
	fmt.Printf("%sNodeIdentifier\n", pad)
	fmt.Printf("%s├─➜ State:  '%s'\n", pad, n.stt)
	switch v := n.val.(type) {
	case string:
		if n.tpe == kindString.asString() {
			fmt.Printf("%s└─➜ Symbol: '%s (%s) = «%s»'\n", pad, n.sym, n.tpe, n.val)
			return
		}
		fmt.Printf("%s└─➜ Symbol: '%s (%s) = %s'\n", pad, n.sym, n.tpe, n.val)
	case NodeBinOp:
		fmt.Printf("%s└─➜ Symbol: '%s (%s) = <NodeBinOp>'\n", pad, n.sym, n.tpe)
		v.print(padSize + 4)
	}
}

func lexChaosConditions(ts *TokenizerState) *NodeCondition {
	newNode := NodeCondition{
		conditionCount: 0,
		conditions:     []NodeBinOp{},
		scope:          map[int][]Node{},
	}

	ts.consume() // consume the 'if'

	if !ts.matchAt(0, tokNumber) {
		errMsg := fmt.Sprintf("Expected a Number but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("if 1 < 2 do ... endif")
		config.printAndExitLexerError(ts)
	}

	if ts.matchAt(1, tokLessThan, tokGreaterThan) {
		lhs := ts.consume()
		op := ts.consume()
		if !ts.matchAt(0, tokNumber) {
			errMsg := fmt.Sprintf("Expected a number but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg)
			config.printAndExitLexerError(ts)
		}
		rhs := ts.consume()
		binOp := NodeBinOp{
			lhs: lhs.value,
			rhs: rhs.value,
			op:  op.value,
		}
		newNode.conditions = append(newNode.conditions, binOp)
		newNode.conditionCount++
	}

	if !ts.matchAt(0, tokDo) {
		errMsg := fmt.Sprintf("Expected 'do' but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("if 1 < 2 do ... endif")
		config.printAndExitLexerError(ts)
	}

	ts.consume(2) // consume the 'do' and 'newline'

	if ts.matchAt(0, tokLet) {
		node := &NodeIdentifier{}
		// TODO: better hanadle lexing errors
		if node = lexChaosLet(ts); node == nil {
			os.Exit(1)
		}
		newNode.scope[newNode.conditionCount] = append(newNode.scope[newNode.conditionCount], node)
	}

	if ts.matchAt(0, tokExitWith) {
		node := &NodeExitWith{}
		if node = lexExitWithChaos(ts); node == nil {
			os.Exit(1)
		}
		newNode.scope[newNode.conditionCount] = append(newNode.scope[newNode.conditionCount], node)
	}

	if !ts.matchAt(0, tokEndIf) {
		errMsg := fmt.Sprintf("Expected 'endif' but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("if 1 < 2 do ... endif")
		config.printAndExitLexerError(ts)
	}
	ts.consume() // consume the 'endif'

	return &newNode
}

func lexChaosLet(ts *TokenizerState) *NodeIdentifier {
	newNode := NodeIdentifier{}
	newNode.val = ""
	newNode.stt = "not-initialized"

	ts.consume() // consume the 'let'
	newNode.sym = ts.current().value

	if !ts.matchAt(0, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected an identifier but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
		config.printAndExitLexerError(ts)
	}
	varName := ts.consume().value
	if _, ok := ts.variables[varName]; !ok {
		ts.variables[varName] = ""
	}

	if !ts.matchAt(0, tokVarType, tokAssignment) {
		errMsg := fmt.Sprintf("Expected a type or assignment but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
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
			config := newLexerErroConfig(errMsg)
			config.newExample("let something U64")
			config.newExample("let something U64 = 1")
			config.printAndExitLexerError(ts)
		}
		if ts.matchAt(0, tokNumber) {
			newNode.stt = "initialized"
			if ts.matchAt(1, tokPlus, tokMinus) {
				lhs := ts.consume()
				op := ts.consume()
				if !ts.matchAt(0, tokNumber) {
					errMsg := fmt.Sprintf("Expected a number but found %s\n", ts.current().tokType.asString())
					config := newLexerErroConfig(errMsg)
					config.printAndExitLexerError(ts)
				}
				rhs := ts.consume()
				binOp := NodeBinOp{
					lhs: lhs.value,
					rhs: rhs.value,
					op:  op.value,
				}
				newNode.val = binOp
			} else {
				value := ts.consume().value
				newNode.val = value
				ts.variables[varName] = value
			}
		} else if ts.matchAt(0, tokString) {
			value := ts.consume().value
			newNode.val = value
			newNode.stt = "initialized"
			ts.variables[varName] = value
		} else if ts.matchAt(0, tokIdentifier) {
			if val, ok := ts.variables[ts.current().value]; ok {
				name := ts.consume().value
				ts.variables[name] = val
				newNode.val = name
				newNode.stt = "initialized"
			} else {
				errMsg := fmt.Sprintf("Undefined identifier %s\n", ts.current().value)

				config := newLexerErroConfig(errMsg)
				config.newExample("let something U64")
				config.newExample("let something U64 = 1")
				config.newExample("let something U64 = 1", "let somethingElse = something")
				config.newExample("let something String = «string literal»")
				config.newExample("let something String = «string literal»", "let somethingElse = something")
				config.printAndExitLexerError(ts)
			}
		}
	}
	if !ts.matchAt(0, tokNewline) {
		errMsg := fmt.Sprintf("Unexpected token found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
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
		config := newLexerErroConfig(errMsg)
		config.newExample("exitWith 1")
		config.newExample("exitWith 1, «reason for exiting early»")
		config.newExample("let code U64 = 1", "exitWith code")
		config.newExample("let code U64 = 1", "exitWith code, «reason for exitin early»")
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
			config := newLexerErroConfig(errMsg)
			config.newExample("exitWith 1")
			config.newExample("exitWith 1, «reason for exiting early»")
			config.newExample("let code U64 = 1", "exitWith code")
			config.newExample("let code U64 = 1", "exitWith code, «reason for exitin early»")
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
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokIf) {
			node := &NodeCondition{}
			if node = lexChaosConditions(ts); node == nil {
				os.Exit(1)
			}
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokExitWith) {
			node := &NodeExitWith{}
			if node = lexExitWithChaos(ts); node == nil {
				os.Exit(1)
			}
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokNewline) {
			ts.consume()
			continue
		}

		fmt.Fprintf(os.Stderr, "[ERROR] The token '%s' is not expected\n", ts.current().value)
		panic("Unexpected token found")
	}

	assert(tokEndOfFile == ts.current().tokType, "Expected eof")
	ts.consume()
	lexerState.count = len(lexerState.data)
	return &lexerState
}
