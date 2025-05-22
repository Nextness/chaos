package main

import (
	"bytes"
	"fmt"
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
	nodeBinOp
	nodeProcCall
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
	} else if n == nodeBinOp {
		return "NodeBinOp"
	} else if n == nodeProcCall {
		return "NodeProcCall"
	}
	panic(fmt.Sprintf("Unexpected NodeType '%d'", n))
}

// In this context, this is basically either a Node or a Token, depending on
// what is on the rhs. If it is a literal for instace, it is a *TokenLiteral.
// If it is a binary operation of function call, then it should be a *NodeBinOp.
type PtrAny any

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
	tmp.WriteByte('\n')
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
	value      PtrAny
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

	if any(n.value) == nil {
		tmp.WriteString(fmt.Sprintf("%stype [%s]", pad, n.varType.symbol))
		tmp.WriteByte('\n')
		return tmp
	} else {
		tmp.WriteString(fmt.Sprintf("%stype [%s]", pad, n.varType.symbol))
		tmp.WriteByte('\n')
	}

	if value, ok := n.value.(*TokenLiteral); ok {
		if val, ok := value.value.(string); ok {
			tmp.WriteString(fmt.Sprintf("%svalue ['%s']", pad, val))
		} else if val, ok := value.value.(int); ok {
			tmp.WriteString(fmt.Sprintf("%svalue [%d]", pad, val))
		} else if val, ok := value.value.(bool); ok {
			tmp.WriteString(fmt.Sprintf("%svalue [%t]", pad, val))
		}
	} else if value, ok := n.value.(*TokenIdentifier); ok {
		tmp.WriteString(fmt.Sprintf("%svalue [%s]", pad, value.symbol))
	} else if value, ok := n.value.(*NodeBinOp); ok {
		buf := value.asBuffer(padSize)
		tmp.Write(buf.Bytes())
	}
	tmp.WriteByte('\n')

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

	tmp.WriteString(fmt.Sprintf("%ssymbol [%s]", pad, n.identifier.symbol))
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

type NodeBinOp struct {
	nodeType  NodeType
	state     NodeState
	lhs       PtrAny
	rhs       PtrAny
	operation *TokenOperator
}

func (n *NodeBinOp) asString() string {
	return todo[string]()
}

func (n *NodeBinOp) asBuffer(padSize int) bytes.Buffer {
	pad := makePad(padSize)

	tmp := bytes.Buffer{}
	tmp.WriteString(fmt.Sprintf("%s%s", pad, n.nodeType.asString()))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%sdefinition", pad))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)

	op := n.operation.tokType.asString()
	tmp.WriteString(fmt.Sprintf("%soperation [%s]", pad, op))
	tmp.WriteByte('\n')

	lhs := n.lhs.(*TokenLiteral).value.(int)
	tmp.WriteString(fmt.Sprintf("%slhs [%d]", pad, lhs))
	tmp.WriteByte('\n')

	rhs := n.rhs.(*TokenLiteral).value.(int)
	tmp.WriteString(fmt.Sprintf("%srhs [%d]", pad, rhs))
	tmp.WriteByte('\n')

	return tmp
}

func (n *NodeBinOp) getNodeType() NodeType {
	return n.nodeType
}

type NodeProcCall struct {
	nodeType   NodeType
	state      NodeState
	identifier *TokenIdentifier
	args       []NodeIdentifier
}

func (n *NodeProcCall) asString() string {
	return todo[string]()
}

func (n *NodeProcCall) asBuffer(padSize int) bytes.Buffer {
	pad := makePad(padSize)

	tmp := bytes.Buffer{}
	tmp.WriteString(fmt.Sprintf("%s%s", pad, n.nodeType.asString()))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)
	tmp.WriteString(fmt.Sprintf("%ssymbol [%s]", pad, n.identifier.symbol))
	tmp.WriteByte('\n')

	tmp.WriteString(fmt.Sprintf("%sinput", pad))
	tmp.WriteByte('\n')

	padSize += PADNUMBER
	pad = makePad(padSize)
	for _, node := range n.args {
		n := node.asBuffer(padSize)
		tmp.Write(n.Bytes())
		tmp.WriteByte('\n')
	}

	return tmp
}

func (n *NodeProcCall) getNodeType() NodeType {
	return n.nodeType
}

var _ Node = &NodeIdentifier{}
var _ Node = &NodeExit{}
var _ Node = &NodeProcDef{}
var _ Node = &NodeProcCall{}
var _ Node = &NodeBinOp{}

func lexExit(ts *TokenizerState) *NodeExit {
	var token Token
	node := NodeExit{nodeType: nodeExit}

	ts.consumeAssert(tokExit)
	if !ts.matchAt(0, tokLiteral) {
		// TODO: Better handle errors
		errMsg := fmt.Sprintf("Expected a number but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.printAndExitLexerError(ts)
	}

	token = ts.consume()
	node.value, _ = token.(*TokenLiteral)

	if ts.matchAt(0, tokComma) {
		ts.consumeAssert(tokComma)
		if !ts.matchAt(0, tokLiteral) {
			errMsg := fmt.Sprintf("Expected a string but found %s\n", ts.currentTokenTypeAsString())
			config := newLexerErroConfig(errMsg)
			config.printAndExitLexerError(ts)
		}
		token = ts.consume()
		node.message = token.(*TokenLiteral)
	}

	ts.consumeAssert(tokSemicolon)

	return &node
}

func lexChaosReassignment(ts *TokenizerState) *NodeIdentifier {
	var token Token
	node := NodeIdentifier{state: nodeReassigned, nodeType: nodeIdentifier}

	token = ts.consumeAssert(tokIdentifier)
	node.identifier = token.(*TokenIdentifier)

	ts.consumeAssert(tokAssignment)

	if !ts.matchAt(0, tokLiteral) {
		errMsg := fmt.Sprintf("Expected a number while reassigning but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.printAndExitLexerError(ts)
	}
	value := ts.consume()
	node.value = value.(*TokenLiteral)
	node.varType = nodeInfer()

	ts.consumeAssert(tokSemicolon)

	return &node
}

func lexChaosLet(ts *TokenizerState) *NodeIdentifier {
	var token Token
	node := NodeIdentifier{state: nodeUninitialized, nodeType: nodeIdentifier}
	if ts.currentTokenType() == tokGlobal {
		// TODO: Handle differently the global variables depending on scope
		ts.consumeAssert(tokGlobal)
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

	ts.consumeAssert(tokColon)

	if !ts.matchAt(0, tokVarType, tokAssignment) {
		// TODO: Improve error handling
		errMsg := fmt.Sprintf("Expected a type or assignment but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
		config.printAndExitLexerError(ts)
	}

	// TODO: Fix case where we only have the identifier, but no type or assignment.
	// This behavior is not allowed. It should be either infer type by assignment or
	// identifier uninitialized with a type.
	node.varType = nodeInfer()
	if ts.matchAt(0, tokVarType) {
		token = ts.consume()
		node.varType = token.(*TokenVarType)
	}

	if ts.matchAt(0, tokAssignment) {
		ts.consumeAssert(tokAssignment)
		if !ts.matchAt(0, tokLiteral, tokLiteral, tokIdentifier) {
			errMsg := fmt.Sprintf("Expected a string, or number but found %s\n", ts.currentTokenTypeAsString())
			config := newLexerErroConfig(errMsg)
			config.printAndExitLexerError(ts)
		}

		node.state = nodeInitialized
		if ts.matchAt(0, tokLiteral, tokLiteral) && ts.matchAt(1, tokSemicolon) {
			token = ts.consume()
			node.value, _ = token.(*TokenLiteral)
		} else if ts.matchAt(0, tokIdentifier) && ts.matchAt(1, tokSemicolon) {
			token = ts.consume()
			node.value, _ = token.(*TokenIdentifier)
		}
		if ts.matchAt(0, tokLiteral) && ts.matchAt(1, tokEquals, tokGreaterThan, tokLessThan, tokPlus, tokMinus) {
			binOp := NodeBinOp{nodeType: nodeBinOp, state: nodeInitialized}
			token = ts.consumeAssert(tokLiteral)
			binOp.lhs = token.(*TokenLiteral)

			token = ts.consume()
			binOp.operation = token.(*TokenOperator)

			token = ts.consumeAssert(tokLiteral)
			binOp.rhs = token.(*TokenLiteral)

			node.value = &binOp
		}
	}

	ts.consumeAssert(tokSemicolon)

	return &node
}

func lexProcCall(ts *TokenizerState) *NodeProcCall {
	var token Token
	node := NodeProcCall{nodeType: nodeProcCall, state: nodeInitialized}
	ts.consumeAssert(tokRun)
	if !ts.matchAt(0, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected proc name a but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.printAndExitLexerError(ts)
	}

	token = ts.consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if !ts.matchAt(0, tokWith) {
		errMsg := fmt.Sprintf("Expected 'with' but found %s\n", ts.currentTokenTypeAsString())
		config := newLexerErroConfig(errMsg)
		config.printAndExitLexerError(ts)
	}

	token = ts.consumeAssert(tokWith)

	for ts.currentTokenType() != tokSemicolon {
		n := NodeIdentifier{
			nodeType:   nodeIdentifier,
			state:      nodeInitialized,
			identifier: &TokenIdentifier{},
			varType:    &TokenVarType{},
		}

		if !ts.matchAt(0, tokLiteral) {
			errMsg := fmt.Sprintf("Expected a literal as argument to proc but found %s\n", ts.currentTokenTypeAsString())
			config := newLexerErroConfig(errMsg)
			config.printAndExitLexerError(ts)
		}
		n.value = castAssert[*TokenLiteral](ts.consume())

		if ts.matchAt(0, tokComma) {
			ts.consume()
		}
		node.args = append(node.args, n)
	}
	ts.consumeAssert(tokSemicolon)

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

			ts.consumeAssert(tokColon)

			isVariadic := false
			if ts.matchAt(0, tokEllipsis) {
				ts.consumeAssert(tokEllipsis)
				isVariadic = true
			}

			if !ts.matchAt(0, tokVarType) {
				errMsg := fmt.Sprintf("Expected a var type as part of argument to proc but found %s\n", ts.currentTokenTypeAsString())
				config := newLexerErroConfig(errMsg)
				config.printAndExitLexerError(ts)
			}
			token = ts.consume()
			n.varType, _ = token.(*TokenVarType)
			n.varType.variadic = isVariadic
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

			node.rets = append(node.rets, n)
		}
	}

	ts.consumeAssert(tokExecutes)
	for !ts.matchAllAt(0, []TokenType{tokEnd, tokProc}) {
		n, ok := lexChaosStatement(ts)
		if !ok {
			return nil, false
		}
		node.statements = append(node.statements, n)
	}

	ts.consumeEndBlock(tokProc)

	return &node, true
}

func lexChaosStatement(ts *TokenizerState) (Node, bool) {
	if ts.matchAt(0, tokRun) {
		node := lexProcCall(ts)
		if node == nil {
			return nil, false
		}
		return node, true
	}
	if ts.matchAt(0, tokLet, tokGlobal) {
		node := lexChaosLet(ts)
		if node == nil {
			return nil, false
		}
		return node, true
	}
	if ts.matchAt(0, tokIdentifier) && ts.matchAt(1, tokAssignment) {
		node := lexChaosReassignment(ts)
		if node == nil {
			return nil, false
		}
		return node, true
	}
	if ts.matchAt(0, tokExit) {
		node := lexExit(ts)
		if node == nil {
			return nil, false
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

		// TODO: handle let and global let separetely
		if ts.matchAt(0, tokGlobal, tokLet, tokExit, tokIdentifier) {
			if node, ok := lexChaosStatement(ts); ok {
				lexerState.data = append(lexerState.data, node)
				continue
			}
			panic("Failed to lex chaos statement")
		}

		panic("Unrecheable")
	}
	lexerState.count = len(lexerState.data)
	return &lexerState
}
