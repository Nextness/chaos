package main

import "fmt"

type ArrayItem[T any] struct {
	value T
}

type LexerArray struct {
	data   []ArrayItem[any]
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

type LexerState struct {
	data   string
	count  int
	cursor int
}

func lexChaosLet(tokenizerState *TokenizerState) *Node {
	var newNode *Node = nil
	if tokenizerState.expects(0, tokLet) {
		tokenizerState.consume() // consume the 'let'
		newNode.nodeType = identifierNode
		newNode.identifier.sym = tokenizerState.current().value
	}
	return newNode
}

func lexerChaos(tokenizerState *TokenizerState) *LexerState {
	assert(tokenizerState.cursor == 0, "Cursor is not 0")
	lexerState := LexerState{count: 0, cursor: 0}
	for tokenizerState.cursor < tokenizerState.count {

		if tokenizerState.expects(0, tokLet) {
			tokenizerState.consume()
			newNode := Node{}
			newNode.nodeType = identifierNode
			newNode.identifier.sym = tokenizerState.current().value
			if tokenizerState.expects(0, tokIdentifier) &&
				tokenizerState.expects(1, tokVarType, tokAssignment) {
				tokenizerState.consume()
			}
			if tokenizerState.expects(0, tokVarType) {
				newNode.identifier.tpe = tokenizerState.current().value
				tokenizerState.consume()
			} else {
				newNode.identifier.tpe = "must-infer"
			}
			if tokenizerState.expects(0, tokAssignment) {
				tokenizerState.consume()
				newNode.identifier.val = tokenizerState.current().value
				newNode.identifier.stt = "initialized"
				tokenizerState.consume()
			} else {
				newNode.identifier.val = ""
				newNode.identifier.stt = "not-initialized"
				tokenizerState.consume()
			}
			if tokenizerState.expects(0, tokNewline) {
				tokenizerState.consume()
			}
			newNode.print()
			continue
		}
		if tokenizerState.expects(0, tokNewline) {
			tokenizerState.consume()
		}

		if tokenizerState.expects(0, tokEndOfFile) {
			break
		}
	}

	assert(tokEndOfFile == tokenizerState.current().tokType, "Expected eof")
	return &lexerState
}
