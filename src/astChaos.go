package main

import "fmt"

type NodeType int

const (
	nodeNull NodeType = iota
	nodeNoOp
	nodeIntLiteral
	nodeStringLiteral
	nodeFloatLiteral
	nodeBoolLiteral
	nodeIdentifier
	nodeExit
	nodeBinOp
	nodeProcDef
	nodeCall
	nodeCount
)

type BinOpOperation int

const (
	opNull BinOpOperation = iota
	opPlus
	opMinus
	opMult
	opLessThan
)

type VarDecl struct {
	Name        Token
	Type        Token
	Assignment  Node
	Initialized bool
}

type Exit struct {
	Status  Node
	Message Node
}

type Literal struct {
	Int     Token
	String  Token
	Boolean Token
}

type BinOp struct {
	Operation BinOpOperation
	Lhs       Node
	Rhs       Node
}

type Proc struct {
	Scope   []Node
	Inputs  []Node
	Outputs []Node
	Arity   int
}

type Call struct {
	Name   Token
	Inputs []Node
	Arity  int
}

type Node struct {
	NodeType     NodeType
	Reassignable bool
	Reassigned   bool
	Infered      bool
	Literal      *Literal
	VarDecl      *VarDecl
	Exit         *Exit
	BinOp        *BinOp
	Proc         *Proc
	Call         *Call
}

var AllocatedVars map[string]Node = map[string]Node{}
var AllocatedProcs map[string]Node = map[string]Node{}

var _ = assert[any](
	nodeCount == 11,
	fmt.Sprintf("Expected 11 node types, but found %d", nodeCount),
)

func NodeTypeToString(nodeType NodeType) string {
	if nodeType == nodeNull {
		return "nodeNull"
	} else if nodeType == nodeNull {
		return "nodeNoOp"
	} else if nodeType == nodeIntLiteral {
		return "nodeIntLiteral"
	} else if nodeType == nodeStringLiteral {
		return "nodeStringLiteral"
	} else if nodeType == nodeFloatLiteral {
		return "nodeFloatLiteral"
	} else if nodeType == nodeBoolLiteral {
		return "nodeBoolLiteral"
	} else if nodeType == nodeIdentifier {
		return "nodeIdentifier"
	} else if nodeType == nodeExit {
		return "nodeExit"
	} else if nodeType == nodeBinOp {
		return "nodeBinOp"
	} else if nodeType == nodeProcDef {
		return "nodeProcDef"
	} else if nodeType == nodeCall {
		return "nodeCall"
	}
	return assert[string](
		nodeCount == 11,
		fmt.Sprintf("Expected 11 node types, but found %d", nodeCount),
	)
}

func infixBindingPower(token Token) (float64, float64) {
	switch token.TokenType {
	case tokPlus:
		fallthrough
	case tokMinus:
		return 1.0, 1.1
	case tokStar:
		return 2.0, 2.1
	case tokLessThan:
		return 5.1, 5.0
	default:
		assert[any](false, fmt.Sprintf("Unexpected token %s", TokenTypeToString(token.TokenType)))
	}
	// Unreachable because go sucks and doesn't understand control flow...
	return 0.0, 0.0
}

func ChaosContentInferType(node *Node, expr Node) {
	if node.VarDecl.Type.TokenType != tokInfer {
		return
	}
	if expr.NodeType == nodeIntLiteral {
		node.Infered = true
		node.VarDecl.Type = MakeType("S64")
	}
	if expr.NodeType == nodeStringLiteral {
		node.Infered = true
		node.VarDecl.Type = MakeType("String")
	}
	if expr.NodeType == nodeBoolLiteral {
		node.Infered = true
		node.VarDecl.Type = MakeType("Bool")
	}
}

func ChaosContentParseBaseNode(chaosSlice *ChaosSlice[Token]) Node {
	var node Node

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokNumberLiteral) {
		expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokNumberLiteral)
		node.NodeType = nodeIntLiteral
		node.Literal = &Literal{
			Int: expr,
		}
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokStringLiteral) {
		expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokStringLiteral)
		node.NodeType = nodeStringLiteral
		node.Literal = &Literal{
			String: expr,
		}
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokBoolLiteral) {
		expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokBoolLiteral)
		node.NodeType = nodeBoolLiteral
		node.Literal = &Literal{
			Boolean: expr,
		}
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier) {
		expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		node.NodeType = nodeIdentifier
		node.VarDecl = &VarDecl{
			Type: expr,
		}

		if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenParen) {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)
			if _, ok := AllocatedProcs[expr.Symbol]; !ok {
				assert[any](false, "Proc doesn't exist")
			}
			node.NodeType = nodeCall
			node.Call = &Call{
				Name:   expr,
				Inputs: []Node{},
			}
			for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseParen) {
				n := ChaosContentParseBaseNode(chaosSlice)
				node.Call.Inputs = append(node.Call.Inputs, n)

				if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
					CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
				}

				node.Call.Arity++
			}
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
		}

		// TO-DO: This is a crappy solution - I need to rethink how I'm handling shit
		if val, ok := AllocatedVars[expr.Symbol]; ok {
			node = val
		}
		return node
	}
	return assert[Node](
		false,
		fmt.Sprintf("Unexpected base node - %s", NodeTypeToString(node.NodeType)),
	)
}

func ChaosContentParseExpression(chaosSlice *ChaosSlice[Token], minBp float64) Node {
	var lhs Node = ChaosContentParseBaseNode(chaosSlice)

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenParen) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)
		lhs = ChaosContentParseExpression(chaosSlice, 0.0)
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
	}

	for true {
		if CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseParen) ||
			CSMatch(chaosSlice, CSTokenTypeComparison, tokSemicolon) {
			break
		}

		var op BinOpOperation
		if CSMatch(chaosSlice, CSTokenTypeComparison, tokPlus) {
			op = opPlus
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokMinus) {
			op = opMinus
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokStar) {
			op = opMult
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokLessThan) {
			op = opLessThan
		} else {
			return Node{NodeType: nodeNoOp}
		}

		expr := CSConsume(chaosSlice)
		lbp, rbp := infixBindingPower(expr)
		if almostEqual(lbp, minBp) {
			break
		}

		rhs := ChaosContentParseExpression(chaosSlice, rbp)
		lhs = Node{
			NodeType: nodeBinOp,
			BinOp: &BinOp{
				Operation: op,
				Lhs:       lhs,
				Rhs:       rhs,
			},
		}
	}

	return lhs
}

func ChaosContentParseProcDefinition(chaosSlice *ChaosSlice[Token]) Node {
	node := Node{
		NodeType: nodeProcDef,
		Proc: &Proc{
			Scope:   []Node{},
			Inputs:  []Node{},
			Outputs: []Node{},
			Arity:   0,
		},
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenParen) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)

		for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseParen) {
			identifier := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokColon)
			varType := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)

			if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
				CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
			}

			node.Proc.Inputs = append(node.Proc.Inputs, Node{
				NodeType: nodeIdentifier,
				VarDecl: &VarDecl{
					Name:        identifier,
					Type:        varType,
					Initialized: false,
				},
			})
			node.Proc.Arity++
		}
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokArrow) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokArrow)
		identifier := ChaosContentParseBaseNode(chaosSlice)
		node.Proc.Outputs = append(node.Proc.Outputs, identifier)
	}

	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenBraket)
	for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseBraket) {
		statement := ChaosContentASTParseStatement(chaosSlice)
		node.Proc.Scope = append(node.Proc.Scope, statement)
	}
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseBraket)

	return node
}

func ChaosContentParseExit(chaosSlice *ChaosSlice[Token]) Node {
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokExit)

	status := ChaosContentParseExpression(chaosSlice, 0.0)

	var message Node
	if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
		message = ChaosContentParseExpression(chaosSlice, 0.0)
	}

	node := Node{
		NodeType: nodeExit,
		Exit: &Exit{
			Status:  status,
			Message: message,
		},
	}

	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)
	return node
}

func ChaosContentParsePrimaryExpression(chaosSlice *ChaosSlice[Token]) Node {
	var node Node

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokEndOfFile) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokEndOfFile)
		node.NodeType = nodeNoOp
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier, tokAssignment) {
		identifier := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		if _, ok := AllocatedVars[identifier.Symbol]; !ok {
			assert[any](false, fmt.Sprintf("%s has not being defined", identifier.Symbol))
		}

		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokAssignment)
		node.NodeType = nodeIdentifier
		node.Reassigned = true
		node.Reassignable = true
		node.VarDecl = &VarDecl{
			Name:        identifier,
			Assignment:  ChaosContentParseExpression(chaosSlice, 0.0),
			Initialized: true,
		}
		AllocatedVars[identifier.Symbol] = node
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier, tokColon) {
		identifier := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokColon)
		node.NodeType = nodeIdentifier
		node.Reassignable = true
		node.VarDecl = &VarDecl{
			Name: identifier,
			Type: Token{
				TokenType: tokInfer,
			},
			Initialized: false,
		}

		if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier) {
			varType := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
			node.VarDecl.Type = varType
			AllocatedVars[identifier.Symbol] = node
		}

		if CSMatch(chaosSlice, CSTokenTypeComparison, tokSemicolon) {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)
			return node
		}
		if CSMatch(chaosSlice, CSTokenTypeComparison, tokAssignment) {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokAssignment)
			node.VarDecl.Initialized = true
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokColon) {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokColon)
			node.VarDecl.Initialized = true
			node.Reassignable = false

			if CSMatch(chaosSlice, CSTokenTypeComparison, tokProc) {
				CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokProc)
				node.VarDecl.Assignment = ChaosContentParseProcDefinition(chaosSlice)
				AllocatedProcs[identifier.Symbol] = node
				return node
			}
		} else {
			token := CSGet(chaosSlice)
			assert[any](
				false,
				fmt.Sprintf("Unexpected token %s", TokenTypeToString(token.TokenType)),
			)
		}

		AllocatedVars[identifier.Symbol] = node

		expr := ChaosContentParseExpression(chaosSlice, 0.0)
		ChaosContentInferType(&node, expr)

		node.VarDecl.Assignment = expr
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier, tokOpenParen) {
		identifier := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)
		if _, ok := AllocatedProcs[identifier.Symbol]; !ok {
			assert[any](false, "Proc doesn't exist")
		}

		node.NodeType = nodeCall
		node.Call = &Call{
			Name:   identifier,
			Inputs: []Node{},
		}

		for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseParen) {
			n := ChaosContentParseBaseNode(chaosSlice)
			node.Call.Inputs = append(node.Call.Inputs, n)

			if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
				CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
			}

			node.Call.Arity++
		}
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)
		return node
	}

	token := CSGet(chaosSlice)
	return assert[Node](
		false,
		fmt.Sprintf("Unexpected token %s", TokenTypeToString(token.TokenType)),
	)
}

func ChaosContentASTParseStatement(chaosSlice *ChaosSlice[Token]) Node {
	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier) {
		return ChaosContentParsePrimaryExpression(chaosSlice)
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokExit) {
		return ChaosContentParseExit(chaosSlice)
	}

	return Node{NodeType: nodeNoOp}
}

func ChaosContentAST(tokens *ChaosSlice[Token]) *ChaosSlice[Node] {
	assert[any](tokens.cursor == 0, "Cursor is not 0")

	nodes := []Node{}

	for tokens.cursor < tokens.count {
		node := ChaosContentASTParseStatement(tokens)
		if node.NodeType == nodeNoOp {
			CSConsume(tokens)
			continue
		}
		nodes = append(nodes, node)
	}

	return &ChaosSlice[Node]{
		data:   nodes,
		count:  len(nodes),
		cursor: 0,
	}
}
