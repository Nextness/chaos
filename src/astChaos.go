package main

import (
	"bytes"
	"fmt"
	"os"
)

const PADNUMBER int = 2

func nodeInfer(tokKind TokenKind) *TokenVarType {
	result := &TokenVarType{symbol: "infer", tokType: tokInferType, tokKind: tokKind}
	return result
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
	status   PtrAny
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
	if t, ok := cast[*TokenLiteral](n.status); ok {
		tmp.WriteString(fmt.Sprintf("%svalue [%d]", pad, t.value.(int)))
	}
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
		tmp.WriteString(fmt.Sprintf("%stype [%s]", pad, n.varType.tokKind.asString()))
		tmp.WriteByte('\n')
	}

	if value, ok := cast[*TokenLiteral](n.value); ok {
		if val, ok := cast[string](value.value); ok {
			tmp.WriteString(fmt.Sprintf("%svalue ['%s']", pad, val))
		} else if val, ok := cast[int](value.value); ok {
			tmp.WriteString(fmt.Sprintf("%svalue [%d]", pad, val))
		} else if val, ok := cast[bool](value.value); ok {
			tmp.WriteString(fmt.Sprintf("%svalue [%t]", pad, val))
		} else {
			panic(fmt.Sprintf("Unsuported type %T in asBuffer for NodeIdentifier", val))
		}
	} else if value, ok := cast[*TokenIdentifier](n.value); ok {
		tmp.WriteString(fmt.Sprintf("%svalue [%s]", pad, value.symbol))
	} else if value, ok := cast[*NodeBinOp](n.value); ok {
		buf := value.asBuffer(padSize)
		tmp.Write(buf.Bytes())
	} else {
		panic(fmt.Sprintf("Unsuported type %T in asBuffer for NodeIdentifier", value))
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

	token := castAssert[*TokenLiteral](n.lhs)
	lhs := castAssert[int](token.value)
	tmp.WriteString(fmt.Sprintf("%slhs [%d]", pad, lhs))
	tmp.WriteByte('\n')

	token = castAssert[*TokenLiteral](n.rhs)
	rhs := castAssert[int](token.value)
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
		ts.PrintError("Expected a number but found %s\n", ts.currentTokenTypeAsString())
		os.Exit(1)
	}

	token = ts.consume()
	node.status, _ = cast[*TokenLiteral](token)

	if ts.matchAt(0, tokComma) {
		ts.consumeAssert(tokComma)
		if !ts.matchAt(0, tokLiteral) {
			ts.PrintError("Expected a string but found %s\n", ts.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ts.consume()
		node.message, _ = cast[*TokenLiteral](token)
	}

	ts.consumeAssert(tokSemicolon)

	return &node
}

func lexChaosDef(ts *TokenizerState) *NodeIdentifier {
	var token Token
	node := NodeIdentifier{state: nodeUninitialized, nodeType: nodeIdentifier}

	ts.consumeAssert(tokDef)

	if !ts.matchAt(0, tokIdentifier) {
		ts.PrintError("Expected an identifier but found %s.\n", ts.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ts.consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ts.currentTokenType() == tokAs {

		ts.consumeAssert(tokAs)

		if !ts.matchAt(0, tokVarType) {
			ts.PrintError("Expected a type or assignment but found %s.\n", ts.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ts.consumeAssert(tokVarType)
		node.varType = castAssert[*TokenVarType](token)
	} else {
		node.varType = nodeInfer(kindNone)
	}

	if ts.matchAt(0, tokAssignment) {

		ts.consumeAssert(tokAssignment)

		node.state = nodeInitialized
		if ts.matchAt(0, tokLiteral) {
			token = ts.consume()
			node.value = castAssert[*TokenLiteral](token)
		} else if ts.matchAt(0, tokIdentifier) {
			token = ts.consume()
			node.value = castAssert[*TokenIdentifier](token)
		}
	}

	ts.consumeAssert(tokSemicolon)

	if node.varType.tokType == tokInferType && node.state == nodeUninitialized {
		ts.PrintError("Expected uninitialized variable or assignment for '%s'.\n", node.identifier.symbol)
		os.Exit(1)
	}

	ts.allocateVariable(node.identifier.symbol)
	return &node
}

func lexProcDefinition(ts *TokenizerState) (*NodeProcDef, bool) {
	var token Token
	node := NodeProcDef{nodeType: nodeProc, state: nodeInitialized}

	ts.consumeAssert(tokProc)

	if !ts.matchAt(0, tokIdentifier) {
		ts.PrintError("Expected an identifier to name a proc but found '%s'\n", ts.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ts.consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ts.matchAt(0, tokExpects) {
		ts.consumeAssert(tokExpects)
		for ts.currentTokenType() != tokReturns {
			nIdent := NodeIdentifier{nodeType: nodeIdentifier}
			if !ts.matchAt(0, tokIdentifier) {
				ts.PrintError("Expected identifier for 'proc %s' but found '%s'\n", node.identifier.symbol, ts.currentTokenTypeAsString())
				os.Exit(1)
			}
			token = ts.consume()
			nIdent.identifier = castAssert[*TokenIdentifier](token)

			ts.consumeAssert(tokAs)

			isVariadic := false
			if ts.matchAt(0, tokEllipsis) {
				ts.consumeAssert(tokEllipsis)
				isVariadic = true
			}

			// TODO: For now we don't allow default values, so all the identifiers should be
			// uninitialized.
			if !ts.matchAt(0, tokVarType) {
				ts.PrintError("Expected identifier type for 'proc %s' but found '%s'\n", node.identifier.symbol, ts.currentTokenTypeAsString())
				os.Exit(1)
			}
			token = ts.consume()
			nIdent.varType = castAssert[*TokenVarType](token)
			nIdent.varType.variadic = isVariadic
			nIdent.state = nodeUninitialized

			if ts.matchAt(0, tokComma) {
				ts.consume()
			}

			node.args = append(node.args, nIdent)
		}

		if len(node.args) == 0 {
			ts.PrintError("Arguments are expected for %s, but no arguments were provided.\n", node.identifier.symbol)
			os.Exit(1)
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
				errMsg := fmt.Sprintf("Expected a return type as part of proc but found %s.\n", ts.currentTokenTypeAsString())
				config := newLexerErroConfig(errMsg)
				config.printAndExitLexerError(ts)
			}
			token = ts.consume()
			n.identifier = &TokenIdentifier{}
			n.varType = castAssert[*TokenVarType](token)
			n.state = nodeUninitialized

			node.rets = append(node.rets, n)
		}

		if len(node.rets) == 0 {
			ts.PrintError("Returns are expected for %s, but no return arguments were provided.\n", node.identifier.symbol)
			os.Exit(1)
		}
	}

	ts.consumeAssert(tokExecutes)
	for !ts.matchAt(0, tokEndProc) {
		n, ok := lexChaosStatement(ts)
		if !ok {
			return nil, false
		}
		node.statements = append(node.statements, n)
	}

	ts.consumeAssert(tokEndProc)

	ts.allocateProc(node.identifier.symbol)
	return &node, true
}

func lexChaosStatement(ts *TokenizerState) (Node, bool) {
	if ts.matchAt(0, tokDef, tokGlobal) {
		node := lexChaosDef(ts)
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

func lexerChaos(ts *TokenizerState) []Node {
	assert(ts.cursor == 0, "Cursor is not 0")
	nodes := []Node{}

	for ts.cursor < ts.count {
		if ts.matchAt(0, tokEndOfFile) {
			break
		}

		if ts.matchAt(0, tokProc) {
			if node, ok := lexProcDefinition(ts); ok {
				nodes = append(nodes, node)
				continue
			}
			panic("Failed to lex proc definition")
		}

		// TODO: handle def and global def separetely
		if ts.matchAt(0, tokGlobal, tokDef, tokExit, tokIdentifier) {
			if node, ok := lexChaosStatement(ts); ok {
				nodes = append(nodes, node)
				continue
			}
			panic("Failed to lex chaos statement")
		}

		panic("Unrecheable")
	}
	return nodes
}
