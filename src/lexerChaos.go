package main

import (
	"bytes"
	"fmt"
	"os"
)

const PADNUMBER int = 2

func nodeInfer() *TokenVarType {
	result := TokenVarType{symbol: "infer", tokType: tokInferType}
	return &result
}

type LexerState struct {
	data   []Node
	count  int
	cursor int
}

type NodeType int

const (
	nodeIdentifier NodeType = iota
	nodeExit
	nodeProc
)

type NodeState int

const (
	nodeUninitialized NodeState = iota
	nodeInitialized
	nodeReassigned
)

func (n NodeType) asString() string {
	if n == nodeIdentifier {
		return "NodeIdentifier"
	} else if n == nodeExit {
		return "NodeExit"
	} else if n == nodeProc {
		return "NodeProc"
	}
	panic(fmt.Sprintf("Unexpected NodeType '%d'", n))
}

type PtrAnyNode any

type Node interface {
	asString() string
	asBuffer(padSize int) bytes.Buffer
	getNodeType() NodeType
}

type NodeExit struct {
	nodeType NodeType
	value    *TokenLiteral
	message  *TokenLiteral
}

func (n *NodeExit) asString() string {
	return todo[string]()
}

func (n *NodeExit) asBuffer(padSize int) bytes.Buffer {
	pad := makePad(padSize)

	tmp := bytes.Buffer{}
	tmp.WriteString(fmt.Sprintf("%s%s", pad, n.nodeType.asString()))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%ssoruce [instrinsic]", pad))
	tmp.WriteByte('\n')
	tmp.WriteString(fmt.Sprintf("%sno-symbol", pad))
	tmp.WriteByte('\n')
	tmp.WriteString(fmt.Sprintf("%sno-state", pad))
	tmp.WriteByte('\n')
	tmp.WriteString(fmt.Sprintf("%sdefinition", pad))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	if n.message != nil {
		message := n.message.value.(string)
		tmp.WriteString(fmt.Sprintf("%smessage ['%s']", pad, message))
		tmp.WriteByte('\n')
	}
	tmp.WriteString(fmt.Sprintf("%svalue [%d]", pad, n.value.value.(int)))
	return tmp
}

func (n *NodeExit) getNodeType() NodeType {
	return n.nodeType
}

type NodeIdentifier struct {
	nodeType   NodeType
	state      NodeState
	identifier *TokenIdentifier
	varType    *TokenVarType
	value      PtrAnyNode
}

func (n *NodeIdentifier) asString() string {
	return todo[string]()
}

func (n *NodeIdentifier) asBuffer(padSize int) bytes.Buffer {
	pad := makePad(padSize)

	tmp := bytes.Buffer{}
	tmp.WriteString(fmt.Sprintf("%s%s", pad, n.nodeType.asString()))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%ssource [user-made]", pad))
	tmp.WriteByte('\n')
	tmp.WriteString(fmt.Sprintf("%ssymbol [%s]", pad, n.identifier.symbol))
	tmp.WriteByte('\n')

	if n.state == nodeInitialized {
		tmp.WriteString(fmt.Sprintf("%sstate [initialized]", pad))
	} else if n.state == nodeUninitialized {
		tmp.WriteString(fmt.Sprintf("%sstate [uninitialized]", pad))
	} else if n.state == nodeReassigned {
		tmp.WriteString(fmt.Sprintf("%sstate [reassigned]", pad))
	} else {
		panic(fmt.Sprintf("Unknown NodeState found: '%d'", n.state))
	}
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%sdefinition", pad))
	tmp.WriteByte('\n')

	if any(n.value) == nil {
		tmp.WriteString(fmt.Sprintf("%stype [%s]", pad, n.varType.symbol))
		return tmp
	} else {
		tmp.WriteString(fmt.Sprintf("%stype [%s]", pad, n.varType.symbol))
		tmp.WriteByte('\n')
	}

	value := n.value.(*TokenLiteral).value
	if val, ok := value.(string); ok {
		tmp.WriteString(fmt.Sprintf("%svalue ['%s']", pad, val))
	} else if val, ok := value.(int); ok {
		tmp.WriteString(fmt.Sprintf("%svalue [%d]", pad, val))
	}

	return tmp
}

func (n *NodeIdentifier) getNodeType() NodeType {
	return n.nodeType
}

type NodeProcDef struct {
	nodeType   NodeType
	state      NodeState
	identifier *TokenIdentifier
	args       []NodeIdentifier
	rets       []NodeIdentifier
	statements []Node
}

func (n *NodeProcDef) asString() string {
	return todo[string]()
}

func (n *NodeProcDef) asBuffer(padSize int) bytes.Buffer {
	pad := makePad(padSize)

	tmp := bytes.Buffer{}
	tmp.WriteString(fmt.Sprintf("%s%s", pad, n.nodeType.asString()))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%ssource [user-made]", pad))
	tmp.WriteByte('\n')
	tmp.WriteString(fmt.Sprintf("%ssymbol [%s]", pad, n.identifier.symbol))
	tmp.WriteByte('\n')

	if n.state == nodeInitialized {
		tmp.WriteString(fmt.Sprintf("%sstate [initialized]", pad))
	} else if n.state == nodeUninitialized {
		tmp.WriteString(fmt.Sprintf("%sstate [uninitialized]", pad))
	} else if n.state == nodeReassigned {
		tmp.WriteString(fmt.Sprintf("%sstate [reassigned]", pad))
	} else {
		panic(fmt.Sprintf("Unknown NodeState found: '%d'", n.state))
	}
	tmp.WriteByte('\n')

	tmp.WriteString(fmt.Sprintf("%sdefinition", pad))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%sinput", pad))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	for _, arg := range n.args {
		a := arg.asBuffer(padSize)
		tmp.Write(a.Bytes())
		tmp.WriteByte('\n')
	}

	padSize -= PADNUMBER
	pad = makePad(padSize)
	tmp.WriteString(fmt.Sprintf("%soutput", pad))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)
	for _, arg := range n.rets {
		a := arg.asBuffer(padSize)
		tmp.Write(a.Bytes())
		tmp.WriteByte('\n')
	}

	padSize -= PADNUMBER
	pad = makePad(padSize)
	tmp.WriteString(fmt.Sprintf("%sbody", pad))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)
	for _, arg := range n.statements {
		a := arg.asBuffer(padSize)
		tmp.Write(a.Bytes())
		tmp.WriteByte('\n')
	}

	return tmp
}

func (n *NodeProcDef) getNodeType() NodeType {
	return n.nodeType
}

var _ Node = &NodeIdentifier{}
var _ Node = &NodeExit{}
var _ Node = &NodeProcDef{}

func lexExit(ts *TokenizerState) *NodeExit {
	var token Token
	node := NodeExit{nodeType: nodeExit}

	ts.consumeAssert(tokExit)
	if !ts.matchAt(0, tokNumber) {
		// TODO: Better handle errors
		errMsg := fmt.Sprintf("Expected a number but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.printAndExitLexerError(ts)
	}

	token = ts.consume()
	node.value, _ = token.(*TokenLiteral)

	if ts.matchAt(0, tokComma) {
		ts.consumeAssert(tokComma)
		if !ts.matchAt(0, tokString) {
			errMsg := fmt.Sprintf("Expected a string but found %s\n", ts.currentTokenTypeAsString())
			config := newLexerErroConfig(errMsg)
			config.printAndExitLexerError(ts)
		}
		token = ts.consume()
		node.message = token.(*TokenLiteral)
	}

	return &node
}

func lexChaosReassignment(ts *TokenizerState) *NodeIdentifier {
	var token Token
	node := NodeIdentifier{
		state:    nodeReassigned,
		nodeType: nodeIdentifier,
	}

	token = ts.consumeAssert(tokIdentifier)
	node.identifier = token.(*TokenIdentifier)

	ts.consumeAssert(tokAssignment)

	if !ts.matchAt(0, tokNumber) {
		errMsg := fmt.Sprintf("Expected a number while reassigning but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.printAndExitLexerError(ts)
	}
	value := ts.consume()
	node.value = value.(*TokenLiteral)
	node.varType = nodeInfer()

	return &node
}

func lexChaosLet(ts *TokenizerState) *NodeIdentifier {
	var token Token
	node := NodeIdentifier{
		state:    nodeUninitialized,
		nodeType: nodeIdentifier,
	}
	ts.consumeAssert(tokLet)

	if !ts.matchAt(0, tokIdentifier) {
		// TODO: Improve error handling
		errMsg := fmt.Sprintf("Expected an identifier but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
		config.printAndExitLexerError(ts)
	}

	token = ts.consume()
	node.identifier, _ = token.(*TokenIdentifier)

	if !ts.matchAt(0, tokVarType, tokAssignment) {
		// TODO: Improve error handling
		errMsg := fmt.Sprintf("Expected a type or assignment but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
		config.printAndExitLexerError(ts)
	}

	if ts.matchAt(0, tokVarType) {
		token = ts.consume()
		node.varType = token.(*TokenVarType)
	} else {
		node.varType = nodeInfer()
	}

	if ts.matchAt(0, tokAssignment) {
		ts.consumeAssert(tokAssignment)
		if !ts.matchAt(0, tokString, tokNumber) {
			errMsg := fmt.Sprintf("Expected a string, or number but found %s\n", ts.currentTokenTypeAsString())
			config := newLexerErroConfig(errMsg)
			config.printAndExitLexerError(ts)
		}
		if ts.matchAt(0, tokString, tokNumber) {
			token = ts.consume()
			node.value, _ = token.(*TokenLiteral)
			node.state = nodeInitialized
		}
	}

	return &node
}

func lexProcDefinition(ts *TokenizerState) (*NodeProcDef, bool) {
	var token Token
	node := NodeProcDef{nodeType: nodeProc, state: nodeInitialized}
	ts.consumeAssert(tokProc)

	if !ts.matchAt(0, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected an identifier to name a proc but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.printAndExitLexerError(ts)
	}
	token = ts.consume()
	node.identifier, _ = token.(*TokenIdentifier)

	if ts.matchAt(0, tokExpects) {
		ts.consumeAssert(tokExpects)
		for ts.current().getTokenType() != tokReturns {
			n := NodeIdentifier{nodeType: nodeIdentifier}
			if !ts.matchAt(0, tokIdentifier) {
				errMsg := fmt.Sprintf("Expected an identifier as argument to proc but found %s\n", ts.currentTokenTypeAsString())
				config := newLexerErroConfig(errMsg)
				config.printAndExitLexerError(ts)
			}
			token = ts.consume()
			n.identifier, _ = token.(*TokenIdentifier)

			if !ts.matchAt(0, tokVarType) {
				errMsg := fmt.Sprintf("Expected a var type as part of argument to proc but found %s\n", ts.currentTokenTypeAsString())
				config := newLexerErroConfig(errMsg)
				config.printAndExitLexerError(ts)
			}
			token = ts.consume()
			n.varType, _ = token.(*TokenVarType)
			// TODO: For now we don't allow default values, so all the identifiers should be
			// uninitialized.
			n.state = nodeUninitialized

			if ts.matchAt(0, tokComma) {
				ts.consume()
			}
			node.args = append(node.args, n)
		}
	}

	if ts.matchAt(0, tokReturns) {
		ts.consumeAssert(tokReturns)
		for ts.current().getTokenType() != tokExecutes {
			// TODO: For now we only allow a single return type. In the future we should not only
			// allow multiple return types, but also named return types, and default values
			// for return types
			n := NodeIdentifier{nodeType: nodeIdentifier}
			if !ts.matchAt(0, tokVarType) {
				errMsg := fmt.Sprintf("Expected a return type as part of proc but found %s\n", ts.currentTokenTypeAsString())
				config := newLexerErroConfig(errMsg)
				config.printAndExitLexerError(ts)
			}
			token = ts.consume()
			n.identifier = &TokenIdentifier{}
			n.varType, _ = token.(*TokenVarType)
			n.state = nodeUninitialized
			chaosDebug("%+v", n)
			node.rets = append(node.rets, n)
		}
	}

	ts.consumeAssert(tokExecutes)
	for !ts.matchAllAt(0, []TokenType{tokEnd, tokProc}) {
		n, ok := lexChaosStatement(ts)
		if !ok {
			os.Exit(1)
		}
		node.statements = append(node.statements, n)
	}

	ts.consumeEndBlock(tokProc)

	return &node, true
}

func lexChaosStatement(ts *TokenizerState) (Node, bool) {
	if ts.matchAt(0, tokLet) {
		node := lexChaosLet(ts)
		if node == nil {
			os.Exit(1)
		}
		return node, true
	}
	if ts.matchAt(0, tokIdentifier) && ts.matchAt(1, tokAssignment) {
		node := lexChaosReassignment(ts)
		if node == nil {
			os.Exit(1)
		}
		return node, true
	}
	if ts.matchAt(0, tokExit) {
		node := lexExit(ts)
		if node == nil {
			os.Exit(1)
		}
		return node, true
	}
	return nil, false
}

func lexerChaos(ts *TokenizerState) *LexerState {
	assert(ts.cursor == 0, "Cursor is not 0")
	lexerState := LexerState{
		data:   []Node{},
		count:  0,
		cursor: 0,
	}

	for ts.cursor < ts.count {
		if ts.matchAt(0, tokEndOfFile) {
			break
		}

		if ts.matchAt(0, tokProc) {
			if node, ok := lexProcDefinition(ts); ok {
				lexerState.data = append(lexerState.data, node)
				continue
			}
			panic("Failed to lex proc definition")
		}

		if ts.matchAt(0, tokLet, tokExit, tokIdentifier) {
			if node, ok := lexChaosStatement(ts); ok {
				lexerState.data = append(lexerState.data, node)
				continue
			}
			panic("Failed to lex chaos statement")
		}

		panic("Unrecheable")
	}
	return &lexerState
}
