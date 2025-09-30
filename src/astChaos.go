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
}

type Call struct {
	Name   Token
	Inputs []Node
}

type Node struct {
	NodeType     NodeType
	Reassignable bool
	Literal      *Literal
	VarDecl      *VarDecl
	Exit         *Exit
	BinOp        *BinOp
	Proc         *Proc
	Call         *Call
}

type Program struct {
	Nodes          []Node
	AllocatedVars  map[string]Node
	AllocatedProcs map[string]Node
}

var AllocatedVars map[string]Node = map[string]Node{}
var AllocatedProcs map[string]Node = map[string]Node{}

var _ = assert[any](
	nodeCount == 11,
	fmt.Sprintf("Expected 11 node types, but found %d", nodeCount),
)

func (node *Node) toString() string {
	if node.NodeType == nodeNull {
		return "nodeNull"
	} else if node.NodeType == nodeNull {
		return "nodeNoOp"
	} else if node.NodeType == nodeIntLiteral {
		return "nodeIntLiteral"
	} else if node.NodeType == nodeStringLiteral {
		return "nodeStringLiteral"
	} else if node.NodeType == nodeFloatLiteral {
		return "nodeFloatLiteral"
	} else if node.NodeType == nodeBoolLiteral {
		return "nodeBoolLiteral"
	} else if node.NodeType == nodeIdentifier {
		return "nodeIdentifier"
	} else if node.NodeType == nodeExit {
		return "nodeExit"
	} else if node.NodeType == nodeBinOp {
		return "nodeBinOp"
	} else if node.NodeType == nodeProcDef {
		return "nodeProcDef"
	} else if node.NodeType == nodeCall {
		return "nodeCall"
	}
	return assert[string](
		nodeCount == 11,
		fmt.Sprintf("Expected 11 node types, but found %d", nodeCount),
	)
}

func ChaosContentParseProcDefinition(chaosSlice *ChaosSlice[Token]) Node {
	node := Node{
		NodeType: nodeProcDef,
		Proc: &Proc{
			Scope:   []Node{},
			Inputs:  []Node{},
			Outputs: []Node{},
		},
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokOpenParen) {
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokOpenParen)

		for !CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokCloseParen) {
			identifier := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokColon)
			varType := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)

			if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokComma) {
				ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokComma)
			}

			node.Proc.Inputs = append(node.Proc.Inputs, Node{
				NodeType: nodeIdentifier,
				VarDecl: &VarDecl{
					Name:        identifier,
					Type:        varType,
					Initialized: false,
				},
			})
		}
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokCloseParen)
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokArrow) {
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokArrow)
		identifier := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
		node.Proc.Outputs = append(node.Proc.Outputs, Node{
			NodeType: nodeIdentifier,
			VarDecl: &VarDecl{
				Type: identifier,
			},
		})
	}

	ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokOpenBraket)
	for !CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokCloseBraket) {
		statement := ChaosContentASTParseStatement(chaosSlice)
		node.Proc.Scope = append(node.Proc.Scope, statement)
	}
	ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokCloseBraket)

	return node
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

func ChaosContentParseExpression(chaosSlice *ChaosSlice[Token], minBp float64) Node {
	var lhs Node

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokNumberLiteral) {
		expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokNumberLiteral)
		lhs.NodeType = nodeIntLiteral
		lhs.Literal = &Literal{
			Int: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokStringLiteral) {
		expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokStringLiteral)
		lhs.NodeType = nodeStringLiteral
		lhs.Literal = &Literal{
			String: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokIdentifier) {
		expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)

		if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokOpenParen) {
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokOpenParen)
			if _, ok := AllocatedProcs[expr.Symbol]; !ok {
				assert[any](false, "Proc doesn't exist")
			}
			lhs.NodeType = nodeCall
			lhs.Call = &Call{
				Name:   expr,
				Inputs: []Node{},
			}
			for !CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokCloseParen) {
				if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokNumberLiteral) {
					expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokNumberLiteral)
					lhs.Call.Inputs = append(lhs.Call.Inputs, Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							Int: expr,
						},
					})
				}

				if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokStringLiteral) {
					expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokStringLiteral)
					lhs.Call.Inputs = append(lhs.Call.Inputs, Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							String: expr,
						},
					})
				}

				if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokComma) {
					ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokComma)
				}
			}
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokCloseParen)
		}

		// TO-DO: This is a crappy solution - I need to rethink how I'm handling shit
		if val, ok := AllocatedVars[expr.Symbol]; ok {
			lhs = val
		}
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokOpenParen) {
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokOpenParen)
		lhs = ChaosContentParseExpression(chaosSlice, 0.0)
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokCloseParen)
	}

	for true {
		if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokCloseParen) ||
			CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokSemicolon) {
			break
		}

		var op BinOpOperation
		if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokPlus) {
			op = opPlus
		} else if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokMinus) {
			op = opMinus
		} else if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokStar) {
			op = opMult
		} else if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokLessThan) {
			op = opLessThan
		} else {
			return Node{NodeType: nodeNoOp}
		}
		expr := ChaosSliceConsume(chaosSlice)
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

func ChaosContentParseExit(chaosSlice *ChaosSlice[Token]) Node {
	ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokExit)

	var message Node
	status := ChaosContentParseExpression(chaosSlice, 0.0)

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokComma) {
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokComma)
		message = ChaosContentParseExpression(chaosSlice, 0.0)
	}

	node := Node{
		NodeType: nodeExit,
		Exit: &Exit{
			Status:  status,
			Message: message,
		},
	}

	ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokSemicolon)
	return node
}

func ChaosContentParsePrimaryExpression(chaosSlice *ChaosSlice[Token]) Node {
	var node Node

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokEndOfFile) {
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokEndOfFile)
		node.NodeType = nodeNoOp
		return node
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokIdentifier, tokAssignment) {
		identifier := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokAssignment)
		node.NodeType = nodeIdentifier
		node.Reassignable = true
		node.VarDecl = &VarDecl{
			Name:        identifier,
			Assignment:  ChaosContentParseExpression(chaosSlice, 0.0),
			Initialized: true,
		}
		AllocatedVars[identifier.Symbol] = node
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokSemicolon)
		return node
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokIdentifier, tokColon) {
		identifier := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokColon)
		node.NodeType = nodeIdentifier
		node.Reassignable = true
		node.VarDecl = &VarDecl{
			Name: identifier,
			Type: Token{
				TokenType: tokInfer,
			},
			Initialized: false,
		}

		if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokIdentifier) {
			varType := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
			node.VarDecl.Type = varType
			AllocatedVars[identifier.Symbol] = node
		}

		if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokSemicolon) {
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokSemicolon)
			return node
		}
		if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokAssignment) {
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokAssignment)
			node.VarDecl.Initialized = true
		} else if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokColon) {
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokColon)
			node.VarDecl.Initialized = true
			node.Reassignable = false

			if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokProc) {
				ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokProc)
				node.VarDecl.Assignment = ChaosContentParseProcDefinition(chaosSlice)
				AllocatedProcs[identifier.Symbol] = node
				return node
			}
		} else {
			token := ChaosSliceGet(chaosSlice)
			assert[any](
				false,
				fmt.Sprintf("Unexpected token %s", TokenTypeToString(token.TokenType)),
			)
		}

		AllocatedVars[identifier.Symbol] = node
		node.VarDecl.Assignment = ChaosContentParseExpression(chaosSlice, 0.0)
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokSemicolon)
		return node
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokOpenParen) {
		identifier := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokOpenParen)
		if _, ok := AllocatedProcs[identifier.Symbol]; !ok {
			assert[any](false, "Proc doesn't exist")
		}

		node.NodeType = nodeCall
		node.Call = &Call{
			Name:   identifier,
			Inputs: []Node{},
		}

		for !CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokCloseParen) {
			if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokNumberLiteral) {
				expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokNumberLiteral)
				node.Call.Inputs = append(node.Call.Inputs, Node{
					NodeType: nodeIntLiteral,
					Literal: &Literal{
						Int: expr,
					},
				})
			}

			if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokStringLiteral) {
				expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokStringLiteral)
				node.Call.Inputs = append(node.Call.Inputs, Node{
					NodeType: nodeIntLiteral,
					Literal: &Literal{
						String: expr,
					},
				})
			}

			if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokComma) {
				ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokComma)
			}
		}
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokCloseParen)
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokSemicolon)
		return node
	}

	token := ChaosSliceGet(chaosSlice)
	return assert[Node](
		false,
		fmt.Sprintf("Unexpected token %s", TokenTypeToString(token.TokenType)),
	)

}

func ChaosContentASTParseStatement(chaosSlice *ChaosSlice[Token]) Node {
	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokIdentifier) {
		return ChaosContentParsePrimaryExpression(chaosSlice)
	}

	if CSMatch(chaosSlice, ChaosSliceTokenTypeComparison, tokExit) {
		return ChaosContentParseExit(chaosSlice)
	}

	return Node{NodeType: nodeNoOp}
}

func ChaosContentAST(tokens *ChaosSlice[Token]) Program {
	assert[any](tokens.cursor == 0, "Cursor is not 0")

	program := Program{
		Nodes: []Node{},
	}

	for tokens.cursor < tokens.count {
		node := ChaosContentASTParseStatement(tokens)
		if node.NodeType == nodeNoOp {
			ChaosSliceConsume(tokens)
			continue
		}
		chaosDebug(node)
		program.Nodes = append(program.Nodes, node)
	}

	return program
}
