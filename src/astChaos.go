package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
)

const PADNUMBER int = 2

func NodeInfer(tokKind TokenKind) *TokenVarType {
	result := &TokenVarType{symbol: "infer", tokType: tokInferType, tokKind: tokKind}
	return result
}

type Scope struct {
	nodes              []Node
	allocatedVariables []Symbol
	allocatedProcs     []Symbol
}

func (p *Scope) AllocateVariable(symbol Symbol) error {
	if slices.Contains(p.allocatedVariables, symbol) {
		return errors.New(fmt.Sprintf("symbol %s is already defined", symbol))
	}
	p.allocatedVariables = append(p.allocatedVariables, symbol)
	return nil
}

func (p *Scope) AllocateProc(symbol Symbol) error {
	if slices.Contains(p.allocatedProcs, symbol) {
		return errors.New(fmt.Sprintf("symbol %s is already defined", symbol))
	}
	p.allocatedProcs = append(p.allocatedProcs, symbol)
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

func (n NodeType) String() string {
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

type PrettyPrint struct {
	padCount       int
	includeNewline bool
}

// In this context, this is basically either a Node or a Token, depending on
// what is on the rhs. If it is a literal for instance, it is a *TokenLiteral.
// If it is a binary operation of function call, then it should be a *NodeBinOp.
type PtrAny any

type Node interface {
	Print(pp PrettyPrint)
	NodeType() NodeType
}

type NodeExit struct {
	nodeType NodeType
	status   PtrAny
	message  *TokenLiteral
}

func (node *NodeExit) Print(pp PrettyPrint) {
	pad := makePad(pp.padCount)

	pad = makePad(pp.padCount)
	fmt.Printf("%stype [%s]\n", pad, node.NodeType().String())
	fmt.Printf("%ssymbol [exit]\n", pad)
	fmt.Printf("%sdefinition\n", pad)

	pp.padCount++
	pad = makePad(pp.padCount)
	if node.message != nil {
		message := node.message.value.(string)
		fmt.Printf("%smessage ['%s']\n", pad, message)
	}

	if t, ok := cast[*TokenLiteral](node.status); ok {
		fmt.Printf("%svalue [%d]", pad, t.value.(int))
	}

	if pp.includeNewline {
		fmt.Print("\n")
	}
}

func (n *NodeExit) NodeType() NodeType {
	return n.nodeType
}

type NodeIdentifier struct {
	nodeType   NodeType
	identifier *TokenIdentifier
	varType    *TokenVarType
	value      PtrAny
}

func (node *NodeIdentifier) Print(pp PrettyPrint) {
	pad := makePad(pp.padCount)

	pad = makePad(pp.padCount)
	fmt.Printf("%stype [%s]\n", pad, node.NodeType().String())
	fmt.Printf("%ssymbol [%s]\n", pad, node.identifier.symbol)

	if node.value != nil {
		fmt.Printf("%sstate [initialized]\n", pad)
	} else {
		fmt.Printf("%sstate [unitialized]\n", pad)
	}

	fmt.Printf("%sdefinition\n", pad)

	pp.padCount++
	pad = makePad(pp.padCount)
	if node.value == nil {
		fmt.Printf("%stype [%s]", pad, node.varType.symbol)
		if pp.includeNewline {
			fmt.Print("\n")
		}
		return
	} else {
		if node.varType.TokenType() == tokInferType {
			fmt.Printf("%stype [infer]\n", pad)
		} else {
			fmt.Printf("%stype [%s]\n", pad, node.varType.tokKind.String())
		}
	}

	if value, ok := cast[*TokenLiteral](node.value); ok {
		fmt.Printf("%stoken [TokenLiteral]\n", pad)
		if val, ok := cast[string](value.value); ok {
			fmt.Printf("%svalue ['%s']", pad, val)
		} else if val, ok := cast[int](value.value); ok {
			fmt.Printf("%svalue [%d]", pad, val)
		} else if val, ok := cast[bool](value.value); ok {
			fmt.Printf("%svalue [%t]", pad, val)
		} else if val, ok := cast[float64](value.value); ok {
			fmt.Printf("%svalue [%.02f]", pad, val)
		} else {
			fmt.Printf("%svalue [Unsuported type %T]", pad, val)
		}
	} else if value, ok := cast[*TokenIdentifier](node.value); ok {
		fmt.Printf("%sTokenIdentifier value [%s]", pad, value.symbol)
	} else if value, ok := cast[*NodeBinOp](node.value); ok {
		pp.padCount++
		pad = makePad(pp.padCount)
		value.Print(pp)
	} else {
		fmt.Printf("%sUnknownToken value [Unsuported type %T]", pad, value)
	}
	if pp.includeNewline {
		fmt.Print("\n")
	}
}

func (n *NodeIdentifier) NodeType() NodeType {
	return n.nodeType
}

type NodeProcDef struct {
	nodeType   NodeType
	state      NodeState
	identifier *TokenIdentifier
	args       []NodeIdentifier
	rets       []NodeIdentifier
	scope      Scope
}

func (node *NodeProcDef) Print(pp PrettyPrint) {
	pad := makePad(pp.padCount)

	pad = makePad(pp.padCount)
	fmt.Printf("%stype [%s]\n", pad, node.NodeType().String())
	fmt.Printf("%ssymbol [%s]\n", pad, node.identifier.symbol)
	fmt.Printf("%sdefinition\n", pad)

	pp.padCount++
	pad = makePad(pp.padCount)
	fmt.Printf("%sinput\n", pad)

	pp.padCount++
	pad = makePad(pp.padCount)
	for _, arg := range node.args {
		arg.Print(pp)
	}

	pp.padCount--
	pad = makePad(pp.padCount)
	fmt.Printf("%soutput\n", pad)

	pp.padCount++
	pad = makePad(pp.padCount)
	for _, ret := range node.rets {
		ret.Print(pp)
	}

	pp.padCount--
	pad = makePad(pp.padCount)
	fmt.Printf("%sbody\n", pad)

	pp.padCount++
	saveState := pp.includeNewline
	length := len(node.scope.nodes)
	for idx, stmt := range node.scope.nodes {
		if length == idx+1 {
			pp.includeNewline = false
		}
		stmt.Print(pp)
	}
	pp.includeNewline = saveState

	if pp.includeNewline {
		fmt.Print("\n")
	}
}

func (n *NodeProcDef) NodeType() NodeType {
	return n.nodeType
}

type NodeBinOp struct {
	nodeType  NodeType
	state     NodeState
	lhs       PtrAny
	rhs       PtrAny
	operation *TokenOperator
}

func (node *NodeBinOp) Print(pp PrettyPrint) {
	pad := makePad(pp.padCount)

	pad = makePad(pp.padCount)
	fmt.Printf("%sdefinition\n", pad)

	pp.padCount++
	pad = makePad(pp.padCount)
	op := node.operation.tokType.String()
	fmt.Printf("%soperation [%s]\n", pad, op)

	token := castAssert[*TokenLiteral](node.lhs)
	lhs := castAssert[int](token.value)
	fmt.Printf("%slhs [%d]\n", pad, lhs)

	token = castAssert[*TokenLiteral](node.rhs)
	rhs := castAssert[int](token.value)
	fmt.Printf("%srhs [%d]\n", pad, rhs)

	if pp.includeNewline {
		fmt.Print("\n")
	}
}

func (n *NodeBinOp) NodeType() NodeType {
	return n.nodeType
}

type NodeProcCall struct {
	nodeType   NodeType
	state      NodeState
	identifier *TokenIdentifier
	args       []NodeIdentifier
}

func (node *NodeProcCall) Print(pp PrettyPrint) {
	pad := makePad(pp.padCount)

	pad = makePad(pp.padCount)
	fmt.Printf("%ssymbol [%s]\n", pad, node.identifier.symbol)
	fmt.Printf("%sinput\n", pad)

	pp.padCount++
	for _, arg := range node.args {
		arg.Print(pp)
	}
	if pp.includeNewline {
		fmt.Print("\n")
	}
}

func (n *NodeProcCall) NodeType() NodeType {
	return n.nodeType
}

var _ Node = &NodeIdentifier{}
var _ Node = &NodeExit{}
var _ Node = &NodeProcDef{}
var _ Node = &NodeProcCall{}
var _ Node = &NodeBinOp{}

func ASTCreateChaosExit(ls *LexerState, program *Scope) bool {
	var token Token
	node := NodeExit{nodeType: nodeExit}

	ls.ConsumeAssert(tokExit)
	if !ls.MatchAt(0, tokLiteral, tokIdentifier) {
		ls.PrintError("Expected a number or identifier but found %s\n", ls.currentTokenTypeAsString())
		os.Exit(1)
	}

	token = ls.Consume()
	if value, ok := cast[*TokenLiteral](token); ok {
		node.status = value
	} else {
		node.status = 420
	}

	if ls.MatchAt(0, tokComma) {
		ls.ConsumeAssert(tokComma)
		if !ls.MatchAt(0, tokLiteral) {
			ls.PrintError("Expected a string but found %s\n", ls.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ls.Consume()
		node.message, _ = cast[*TokenLiteral](token)
	}

	ls.ConsumeAssert(tokSemicolon)

	program.nodes = append(program.nodes, &node)
	return true
}

func ASTCreateChaosGlobalDef(ls *LexerState, program *Scope) bool {
	var token Token
	node := NodeIdentifier{nodeType: nodeIdentifier}

	ls.ConsumeAssert(tokGlobal)
	ls.ConsumeAssert(tokDef)

	if !ls.MatchAt(0, tokIdentifier) {
		ls.PrintError("Expected an identifier but found %s.\n", ls.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ls.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ls.Current().TokenType() == tokAs {

		ls.ConsumeAssert(tokAs)

		if !ls.MatchAt(0, tokVarType) {
			ls.PrintError("Expected a type or assignment but found %s.\n", ls.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ls.ConsumeAssert(tokVarType)
		node.varType = castAssert[*TokenVarType](token)
	} else {
		node.varType = NodeInfer(kindNone)
	}

	if ls.MatchAt(0, tokAssignment) {

		ls.ConsumeAssert(tokAssignment)

		if ls.MatchAt(0, tokLiteral) {
			token = ls.Consume()
			node.value = castAssert[*TokenLiteral](token)
		} else if ls.MatchAt(0, tokIdentifier) {
			token = ls.Consume()
			node.value = castAssert[*TokenIdentifier](token)
		}
	}

	ls.ConsumeAssert(tokSemicolon)

	if node.varType.tokType == tokInferType && node.value == nil {
		ls.PrintError("Expected uninitialized variable or assignment for '%s'.\n", node.identifier.symbol)
		os.Exit(1)
	}

	if err := program.AllocateVariable(node.identifier.symbol); err != nil {
		ls.PrintError("The variable `%s` already exists and cannot be defined twice.\n", node.identifier.symbol)
		return false
	}
	program.nodes = append(program.nodes, &node)
	return true
}

func ASTCreateChaosDef(ls *LexerState, program *Scope) bool {
	var token Token
	node := NodeIdentifier{nodeType: nodeIdentifier}

	ls.ConsumeAssert(tokDef)

	if !ls.MatchAt(0, tokIdentifier) {
		ls.PrintError("Expected an identifier but found %s.\n", ls.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ls.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ls.Current().TokenType() == tokAs {

		ls.ConsumeAssert(tokAs)

		if !ls.MatchAt(0, tokVarType) {
			ls.PrintError("Expected a type or assignment but found %s.\n", ls.currentTokenTypeAsString())
			os.Exit(1)
		}
		token = ls.ConsumeAssert(tokVarType)
		node.varType = castAssert[*TokenVarType](token)
	} else {
		node.varType = NodeInfer(kindNone)
	}

	if ls.MatchAt(0, tokAssignment) {

		ls.ConsumeAssert(tokAssignment)

		if ls.MatchAt(0, tokLiteral) {
			token = ls.Consume()
			node.value = castAssert[*TokenLiteral](token)
		} else if ls.MatchAt(0, tokIdentifier) {
			token = ls.Consume()
			node.value = castAssert[*TokenIdentifier](token)
		}
	}

	ls.ConsumeAssert(tokSemicolon)

	if node.varType.tokType == tokInferType && node.value == nil {
		ls.PrintError("Expected uninitialized variable or assignment for '%s'.\n", node.identifier.symbol)
		os.Exit(1)
	}

	program.nodes = append(program.nodes, &node)
	return true
}

func ASTCreateChaosProcDefinition(ls *LexerState, program *Scope) bool {

	var token Token
	node := NodeProcDef{nodeType: nodeProc}

	ls.ConsumeAssert(tokProc)

	if !ls.MatchAt(0, tokIdentifier) {
		ls.PrintError("Expected an identifier to name a proc but found '%s'\n", ls.currentTokenTypeAsString())
		os.Exit(1)
	}
	token = ls.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if ls.MatchAt(0, tokExpects) {
		ls.ConsumeAssert(tokExpects)
		for ls.Current().TokenType() != tokReturns {
			nIdent := NodeIdentifier{nodeType: nodeIdentifier}
			if !ls.MatchAt(0, tokIdentifier) {
				ls.PrintError("Expected identifier for 'proc %s' but found '%s'\n", node.identifier.symbol, ls.currentTokenTypeAsString())
				os.Exit(1)
			}
			token = ls.Consume()
			nIdent.identifier = castAssert[*TokenIdentifier](token)

			ls.ConsumeAssert(tokAs)

			isVariadic := false
			if ls.MatchAt(0, tokEllipsis) {
				ls.ConsumeAssert(tokEllipsis)
				isVariadic = true
			}

			// TO-DO: For now we don't allow default values, so all the identifiers should be
			// uninitialized.
			if !ls.MatchAt(0, tokVarType) {
				ls.PrintError("Expected identifier type for 'proc %s' but found '%s'\n", node.identifier.symbol, ls.currentTokenTypeAsString())
				os.Exit(1)
			}
			token = ls.Consume()
			nIdent.varType = castAssert[*TokenVarType](token)
			nIdent.varType.variadic = isVariadic

			if ls.MatchAt(0, tokComma) {
				ls.Consume()
			}

			node.args = append(node.args, nIdent)
		}

		if len(node.args) == 0 {
			ls.PrintError("Arguments are expected for %s, but no arguments were provided.\n", node.identifier.symbol)
			os.Exit(1)
		}
	}

	if ls.MatchAt(0, tokReturns) {
		ls.ConsumeAssert(tokReturns)
		for ls.Current().TokenType() != tokExecutes {
			// TO-DO: For now we only allow a single return type. In the future we should not only
			// allow multiple return types, but also named return types, and default values
			// for return types
			n := NodeIdentifier{nodeType: nodeIdentifier}
			if !ls.MatchAt(0, tokVarType) {
				ls.PrintError("Expected a return type as part of proc but found %s.\n", ls.currentTokenTypeAsString())
				return false
			}
			token = ls.Consume()
			n.identifier = &TokenIdentifier{}
			n.varType = castAssert[*TokenVarType](token)

			node.rets = append(node.rets, n)
		}

		if len(node.rets) == 0 {
			ls.PrintError("Returns are expected for %s, but no return arguments were provided.\n", node.identifier.symbol)
			os.Exit(1)
		}
	}

	ls.ConsumeAssert(tokExecutes)
	for !ls.MatchAt(0, tokEndProc) {
		// TO-DO: Handle allocating variables to a local scope.
		// Variables are basically skipped in allocation since they are not considered
		// global variables.
		if ok := ASTCreateChaosStatement(ls, &node.scope); !ok {
			return false
		}
	}

	ls.ConsumeAssert(tokEndProc)

	// TO-DO: This should take into account overloading and mangling, but I'm too lazy to do it now.
	if err := program.AllocateProc(node.identifier.symbol); err != nil {
		ls.PrintError("The proc `%s` already exists and cannot be defined twice.\n", node.identifier.symbol)
		return false
	}

	program.nodes = append(program.nodes, &node)
	return true
}

func ASTCreateChaosProcCall(ls *LexerState, program *Scope) bool {

	var token Token
	node := NodeProcCall{nodeType: nodeProcCall}

	ls.ConsumeAssert(tokRun)

	if !ls.MatchAt(0, tokIdentifier) {
		ls.PrintError("Expected an identifier, but found `%s`\n", ls.currentTokenTypeAsString())
		return false
	}
	token = ls.Consume()
	node.identifier = castAssert[*TokenIdentifier](token)

	if !ls.MatchAt(0, tokWith) {
		ls.PrintError("Expected `with`, but found `%s`\n", ls.currentTokenTypeAsString())
		return false
	}
	ls.ConsumeAssert(tokWith)

	for ls.Current().TokenType() != tokSemicolon {
		// Skipping for now
		ls.Consume()
	}

	ls.ConsumeAssert(tokSemicolon)
	program.nodes = append(program.nodes, &node)
	return true
}

func ASTCreateChaosStatement(ls *LexerState, program *Scope) bool {
	if ls.MatchAt(0, tokDef) {
		if ok := ASTCreateChaosDef(ls, program); ok {
			return true
		}
	}
	if ls.MatchAt(0, tokGlobal) {
		if ok := ASTCreateChaosGlobalDef(ls, program); ok {
			return true
		}
	}
	if ls.MatchAt(0, tokRun) {
		if ok := ASTCreateChaosProcCall(ls, program); ok {
			return true
		}
	}
	if ls.MatchAt(0, tokExit) {
		if ok := ASTCreateChaosExit(ls, program); ok {
			return true
		}
	}
	return false
}

func ASTCreateChaosProgram(ls *LexerState) Scope {
	assert(ls.cursor == 0, "Cursor is not 0")
	program := Scope{
		nodes:              []Node{},
		allocatedVariables: []Symbol{},
		allocatedProcs:     []Symbol{},
	}

	for ls.cursor < ls.count {
		if ls.MatchAt(0, tokEndOfFile) {
			break
		}

		if ls.MatchAt(0, tokProc) {
			if ok := ASTCreateChaosProcDefinition(ls, &program); ok {
				continue
			}
			panic("Failed to lex proc definition")
		}

		// TO-DO: handle def and global def separately
		if ls.MatchAt(0, tokGlobal, tokDef, tokExit, tokIdentifier, tokRun) {
			if ok := ASTCreateChaosStatement(ls, &program); ok {
				continue
			}
			panic("Failed to lex chaos statement")
		}

		panic("Unrecheable")
	}
	return program
}
