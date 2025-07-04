package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
)

const PADNUMBER int = 2

func nodeInfer(tokKind TokenKind) *TokenVarType {
	result := &TokenVarType{symbol: "infer", tokType: tokInferType, tokKind: tokKind}
	return result
}

type Program struct {
	node                     []Node
	globalAllocatedVariables []Symbol
	globalAllocatedProcs     []Symbol
}

func (p *Program) allocateVariable(symbol Symbol) error {
	if slices.Contains(p.globalAllocatedVariables, symbol) {
		return errors.New(fmt.Sprintf("symbol %s is already defined", symbol))
	}
	p.globalAllocatedVariables = append(p.globalAllocatedVariables, symbol)
	return nil
}

func (p *Program) allocateProc(symbol Symbol) error {
	if slices.Contains(p.globalAllocatedProcs, symbol) {
		return errors.New(fmt.Sprintf("symbol %s is already defined", symbol))
	}
	p.globalAllocatedProcs = append(p.globalAllocatedProcs, symbol)
	return nil
}

type ASTState struct {
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

func (n NodeType) asString() string {
	if n == nodeIdentifier {
		return "NodeIdentifier"
	}
	if n == nodeExit {
		return "NodeExit"
	}
	if n == nodeProc {
		return "NodeProc"
	}
	if n == nodeBinOp {
		return "NodeBinOp"
	}
	if n == nodeProcCall {
		return "NodeProcCall"
	}
	return "UnknownNode"
}

// In this context, this is basically either a Node or a Token, depending on
// what is on the rhs. If it is a literal for instance, it is a *TokenLiteral.
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
	tmp.WriteString(fmt.Sprintf("%s%s\n", pad, n.nodeType.asString()))

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%sdefinition\n", pad))

	padSize += PADNUMBER
	pad = makePad(padSize)

	if n.message != nil {
		message := n.message.value.(string)
		tmp.WriteString(fmt.Sprintf("%smessage ['%s']\n", pad, message))
	}
	if t, ok := cast[*TokenLiteral](n.status); ok {
		tmp.WriteString(fmt.Sprintf("%svalue [%d]", pad, t.value.(int)))
	}
	return tmp
}

func (n *NodeExit) getNodeType() NodeType {
	return n.nodeType
}

type NodeIdentifier struct {
	nodeType   NodeType
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
	tmp.WriteString(fmt.Sprintf("%s%s\n", pad, n.nodeType.asString()))

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%ssymbol [%s]\n", pad, n.identifier.symbol))

	if n.value != nil {
		tmp.WriteString(fmt.Sprintf("%sstate [initialized]\n", pad))
	} else {
		tmp.WriteString(fmt.Sprintf("%sstate [uninitialized]\n", pad))
	}
	tmp.WriteString(fmt.Sprintf("%sdefinition\n", pad))

	padSize += PADNUMBER
	pad = makePad(padSize)

	if any(n.value) == nil {
		tmp.WriteString(fmt.Sprintf("%stype [%s]", pad, n.varType.symbol))
		return tmp
	} else {
		tmp.WriteString(fmt.Sprintf("%stype [%s]\n", pad, n.varType.tokKind.asString()))
	}

	if value, ok := cast[*TokenLiteral](n.value); ok {
		if val, ok := cast[string](value.value); ok {
			tmp.WriteString(fmt.Sprintf("%sTokenLiteral value ['%s']", pad, val))
		} else if val, ok := cast[int](value.value); ok {
			tmp.WriteString(fmt.Sprintf("%sTokenLiteral value [%d]", pad, val))
		} else if val, ok := cast[bool](value.value); ok {
			tmp.WriteString(fmt.Sprintf("%sTokenLiteral value [%t]", pad, val))
		} else if val, ok := cast[float64](value.value); ok {
			tmp.WriteString(fmt.Sprintf("%sTokenLiteral value [%.02f]", pad, val))
		} else {
			tmp.WriteString(fmt.Sprintf("%sTokenLiteral value [Unsuported type %T]", pad, val))
		}
	} else if value, ok := cast[*TokenIdentifier](n.value); ok {
		tmp.WriteString(fmt.Sprintf("%sTokenIdentifier value [%s]", pad, value.symbol))
	} else if value, ok := cast[*NodeBinOp](n.value); ok {
		buf := value.asBuffer(padSize)
		tmp.Write(buf.Bytes())
	} else {
		tmp.WriteString(fmt.Sprintf("%sUnknownToken value [Unsuported type %T]", pad, value))
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
	tmp.WriteString(fmt.Sprintf("%s%s\n", pad, n.nodeType.asString()))

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%ssymbol [%s]\n", pad, n.identifier.symbol))

	tmp.WriteString(fmt.Sprintf("%sdefinition\n", pad))

	padSize += PADNUMBER
	pad = makePad(padSize)

	tmp.WriteString(fmt.Sprintf("%sinput\n", pad))

	padSize += PADNUMBER
	pad = makePad(padSize)

	for _, arg := range n.args {
		a := arg.asBuffer(padSize)
		tmp.Write(a.Bytes())
		tmp.WriteByte('\n')
	}

	padSize -= PADNUMBER
	pad = makePad(padSize)
	tmp.WriteString(fmt.Sprintf("%soutput\n", pad))

	padSize += PADNUMBER
	pad = makePad(padSize)
	for _, arg := range n.rets {
		a := arg.asBuffer(padSize)
		tmp.Write(a.Bytes())
		tmp.WriteByte('\n')
	}

	padSize -= PADNUMBER
	pad = makePad(padSize)
	tmp.WriteString(fmt.Sprintf("%sbody\n", pad))

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

func ASTCreateExit(ts *LexerState, program *Program) bool {
	var token Token
	node := NodeExit{nodeType: nodeExit}

	ts.ConsumeAssert(tokExit)
	if !ts.MatchAt(0, tokLiteral, tokIdentifier) {
		ts.PrintError("Expected a number or identifier but found %s\n", ts.currentTokenTypeAsString())
		os.Exit(1)
	}

	token = ts.Consume()
	if value, ok := cast[*TokenLiteral](token); ok {
		node.status = value
	} else {
		node.status = 420
	}

	if ts.MatchAt(0, tokComma) {
		ts.ConsumeAssert(tokComma)
		if !ts.MatchAt(0, tokLiteral) {
			ts.PrintError("Expected a string but found %s\n", ts.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ts.Consume()
		node.message, _ = cast[*TokenLiteral](token)
	}

	ts.ConsumeAssert(tokSemicolon)

	program.node = append(program.node, &node)
	return true
}

func ASTCreateChaosGlobalDef(ts *LexerState, program *Program) bool {
	var token Token
	node := NodeIdentifier{nodeType: nodeIdentifier}

	ts.ConsumeAssert(tokGlobal)
	ts.ConsumeAssert(tokDef)

	if !ts.MatchAt(0, tokIdentifier) {
		ts.PrintError("Expected an identifier but found %s.\n", ts.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ts.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ts.Current().TokenType() == tokAs {

		ts.ConsumeAssert(tokAs)

		if !ts.MatchAt(0, tokVarType) {
			ts.PrintError("Expected a type or assignment but found %s.\n", ts.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ts.ConsumeAssert(tokVarType)
		node.varType = castAssert[*TokenVarType](token)
	} else {
		node.varType = nodeInfer(kindNone)
	}

	if ts.MatchAt(0, tokAssignment) {

		ts.ConsumeAssert(tokAssignment)

		if ts.MatchAt(0, tokLiteral) {
			token = ts.Consume()
			node.value = castAssert[*TokenLiteral](token)
		} else if ts.MatchAt(0, tokIdentifier) {
			token = ts.Consume()
			node.value = castAssert[*TokenIdentifier](token)
		}
	}

	ts.ConsumeAssert(tokSemicolon)

	if node.varType.tokType == tokInferType && node.value == nil {
		ts.PrintError("Expected uninitialized variable or assignment for '%s'.\n", node.identifier.symbol)
		os.Exit(1)
	}

	if err := program.allocateVariable(node.identifier.symbol); err != nil {
		ts.PrintError("The variable `%s` already exists and cannot be defined twice.\n", node.identifier.symbol)
		return false
	}
	program.node = append(program.node, &node)
	return true
}

func ASTCreateChaosDef(ts *LexerState, program *Program) bool {
	var token Token
	node := NodeIdentifier{nodeType: nodeIdentifier}

	ts.ConsumeAssert(tokDef)

	if !ts.MatchAt(0, tokIdentifier) {
		ts.PrintError("Expected an identifier but found %s.\n", ts.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ts.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ts.Current().TokenType() == tokAs {

		ts.ConsumeAssert(tokAs)

		if !ts.MatchAt(0, tokVarType) {
			ts.PrintError("Expected a type or assignment but found %s.\n", ts.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ts.ConsumeAssert(tokVarType)
		node.varType = castAssert[*TokenVarType](token)
	} else {
		node.varType = nodeInfer(kindNone)
	}

	if ts.MatchAt(0, tokAssignment) {

		ts.ConsumeAssert(tokAssignment)

		if ts.MatchAt(0, tokLiteral) {
			token = ts.Consume()
			node.value = castAssert[*TokenLiteral](token)
		} else if ts.MatchAt(0, tokIdentifier) {
			token = ts.Consume()
			node.value = castAssert[*TokenIdentifier](token)
		}
	}

	ts.ConsumeAssert(tokSemicolon)

	if node.varType.tokType == tokInferType && node.value == nil {
		ts.PrintError("Expected uninitialized variable or assignment for '%s'.\n", node.identifier.symbol)
		os.Exit(1)
	}

	program.node = append(program.node, &node)
	return true
}

func ASTCreateProcDefinition(ts *LexerState, program *Program) bool {

	var token Token
	node := NodeProcDef{nodeType: nodeProc}

	ts.ConsumeAssert(tokProc)

	if !ts.MatchAt(0, tokIdentifier) {
		ts.PrintError("Expected an identifier to name a proc but found '%s'\n", ts.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ts.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ts.MatchAt(0, tokExpects) {
		ts.ConsumeAssert(tokExpects)
		for ts.Current().TokenType() != tokReturns {
			nIdent := NodeIdentifier{nodeType: nodeIdentifier}
			if !ts.MatchAt(0, tokIdentifier) {
				ts.PrintError("Expected identifier for 'proc %s' but found '%s'\n", node.identifier.symbol, ts.currentTokenTypeAsString())
				os.Exit(1)
			}
			token = ts.Consume()
			nIdent.identifier = castAssert[*TokenIdentifier](token)

			ts.ConsumeAssert(tokAs)

			isVariadic := false
			if ts.MatchAt(0, tokEllipsis) {
				ts.ConsumeAssert(tokEllipsis)
				isVariadic = true
			}

			// TODO: For now we don't allow default values, so all the identifiers should be
			// uninitialized.
			if !ts.MatchAt(0, tokVarType) {
				ts.PrintError("Expected identifier type for 'proc %s' but found '%s'\n", node.identifier.symbol, ts.currentTokenTypeAsString())
				os.Exit(1)
			}
			token = ts.Consume()
			nIdent.varType = castAssert[*TokenVarType](token)
			nIdent.varType.variadic = isVariadic

			if ts.MatchAt(0, tokComma) {
				ts.Consume()
			}

			node.args = append(node.args, nIdent)
		}

		if len(node.args) == 0 {
			ts.PrintError("Arguments are expected for %s, but no arguments were provided.\n", node.identifier.symbol)
			os.Exit(1)
		}
	}

	if ts.MatchAt(0, tokReturns) {
		ts.ConsumeAssert(tokReturns)
		for ts.Current().TokenType() != tokExecutes {
			// TO-DO: For now we only allow a single return type. In the future we should not only
			// allow multiple return types, but also named return types, and default values
			// for return types
			n := NodeIdentifier{nodeType: nodeIdentifier}
			if !ts.MatchAt(0, tokVarType) {
				ts.PrintError("Expected a return type as part of proc but found %s.\n", ts.currentTokenTypeAsString())
				return false
			}
			token = ts.Consume()
			n.identifier = &TokenIdentifier{}
			n.varType = castAssert[*TokenVarType](token)

			node.rets = append(node.rets, n)
		}

		if len(node.rets) == 0 {
			ts.PrintError("Returns are expected for %s, but no return arguments were provided.\n", node.identifier.symbol)
			os.Exit(1)
		}
	}

	ts.ConsumeAssert(tokExecutes)
	for !ts.MatchAt(0, tokEndProc) {
		// TODO: Handle allocating variables to a local scope.
		// Variables are basically skipped in allocation since they are not considered
		// global variables.
		if ok := ASTCreateChaosStatement(ts, program); !ok {
			return false
		}
	}

	ts.ConsumeAssert(tokEndProc)

	// TODO: This should take into account overloading and mangling, but I'm too lazy to do it now.
	if err := program.allocateProc(node.identifier.symbol); err != nil {
		ts.PrintError("The proc `%s` already exists and cannot be defined twice.\n", node.identifier.symbol)
		return false
	}

	program.node = append(program.node, &node)
	return true
}

func ASTCreateProcCall(ts *LexerState, program *Program) bool {

	var token Token
	node := NodeProcCall{nodeType: nodeProcCall}

	ts.ConsumeAssert(tokRun)

	if !ts.MatchAt(0, tokIdentifier) {
		ts.PrintError("Expected an identifier, but found `%s`\n", ts.currentTokenTypeAsString())
		return false
	}
	token = ts.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if !ts.MatchAt(0, tokWith) {
		ts.PrintError("Expected `with`, but found `%s`\n", ts.currentTokenTypeAsString())
		return false
	}
	ts.ConsumeAssert(tokWith)

	for ts.Current().TokenType() != tokSemicolon {
		// Skipping for now
		ts.Consume()
	}

	ts.ConsumeAssert(tokSemicolon)
	program.node = append(program.node, &node)
	return true
}

func ASTCreateChaosStatement(ts *LexerState, program *Program) bool {
	if ts.MatchAt(0, tokDef) {
		if ok := ASTCreateChaosDef(ts, program); ok {
			return true
		}
	}
	if ts.MatchAt(0, tokGlobal) {
		if ok := ASTCreateChaosGlobalDef(ts, program); ok {
			return true
		}
	}
	if ts.MatchAt(0, tokRun) {
		if ok := ASTCreateProcCall(ts, program); ok {
			return true
		}
	}
	if ts.MatchAt(0, tokExit) {
		if ok := ASTCreateExit(ts, program); ok {
			return true
		}
	}
	return false
}

func ASTCreateChaosProgram(ts *LexerState) Program {
	assert(ts.cursor == 0, "Cursor is not 0")
	program := Program{
		node:                     []Node{},
		globalAllocatedVariables: []Symbol{},
		globalAllocatedProcs:     []Symbol{},
	}

	for ts.cursor < ts.count {
		if ts.MatchAt(0, tokEndOfFile) {
			break
		}

		if ts.MatchAt(0, tokProc) {
			if ok := ASTCreateProcDefinition(ts, &program); ok {
				continue
			}
			panic("Failed to lex proc definition")
		}

		// TODO: handle def and global def separately
		if ts.MatchAt(0, tokGlobal, tokDef, tokExit, tokIdentifier, tokRun) {
			if ok := ASTCreateChaosStatement(ts, &program); ok {
				continue
			}
			panic("Failed to lex chaos statement")
		}

		panic("Unrecheable")
	}
	return program
}
