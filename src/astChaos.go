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
		}
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokArrow) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokArrow)
		identifier := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		node.Proc.Outputs = append(node.Proc.Outputs, Node{
			NodeType: nodeIdentifier,
			VarDecl: &VarDecl{
				Type: identifier,
			},
		})
	}

	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenBraket)
	for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseBraket) {
		statement := ChaosContentASTParseStatement(chaosSlice)
		node.Proc.Scope = append(node.Proc.Scope, statement)
	}
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseBraket)

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

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokNumberLiteral) {
		expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokNumberLiteral)
		lhs.NodeType = nodeIntLiteral
		lhs.Literal = &Literal{
			Int: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokStringLiteral) {
		expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokStringLiteral)
		lhs.NodeType = nodeStringLiteral
		lhs.Literal = &Literal{
			String: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier) {
		expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)

		if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenParen) {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)
			if _, ok := AllocatedProcs[expr.Symbol]; !ok {
				assert[any](false, "Proc doesn't exist")
			}
			lhs.NodeType = nodeCall
			lhs.Call = &Call{
				Name:   expr,
				Inputs: []Node{},
			}
			for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseParen) {
				if CSMatch(chaosSlice, CSTokenTypeComparison, tokNumberLiteral) {
					expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokNumberLiteral)
					lhs.Call.Inputs = append(lhs.Call.Inputs, Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							Int: expr,
						},
					})
				}

				if CSMatch(chaosSlice, CSTokenTypeComparison, tokStringLiteral) {
					expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokStringLiteral)
					lhs.Call.Inputs = append(lhs.Call.Inputs, Node{
						NodeType: nodeIntLiteral,
						Literal: &Literal{
							String: expr,
						},
					})
				}

				if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
					CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
				}
			}
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
		}

		// TO-DO: This is a crappy solution - I need to rethink how I'm handling shit
		if val, ok := AllocatedVars[expr.Symbol]; ok {
			lhs = val
		}
	}

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

func ChaosContentParseExit(chaosSlice *ChaosSlice[Token]) Node {
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokExit)

	var message Node
	status := ChaosContentParseExpression(chaosSlice, 0.0)

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
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokAssignment)
		node.NodeType = nodeIdentifier
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
		node.VarDecl.Assignment = ChaosContentParseExpression(chaosSlice, 0.0)
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenParen) {
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
			if CSMatch(chaosSlice, CSTokenTypeComparison, tokNumberLiteral) {
				expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokNumberLiteral)
				node.Call.Inputs = append(node.Call.Inputs, Node{
					NodeType: nodeIntLiteral,
					Literal: &Literal{
						Int: expr,
					},
				})
			}

			if CSMatch(chaosSlice, CSTokenTypeComparison, tokStringLiteral) {
				expr := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokStringLiteral)
				node.Call.Inputs = append(node.Call.Inputs, Node{
					NodeType: nodeIntLiteral,
					Literal: &Literal{
						String: expr,
					},
				})
			}

			if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
				CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
			}
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

func ChaosContentAST(tokens *ChaosSlice[Token]) Program {
	assert[any](tokens.cursor == 0, "Cursor is not 0")

	program := Program{
		Nodes: []Node{},
	}

	for tokens.cursor < tokens.count {
		node := ChaosContentASTParseStatement(tokens)
		if node.NodeType == nodeNoOp {
			CSConsume(tokens)
			continue
		}
		chaosDebug(node)
		program.Nodes = append(program.Nodes, node)
	}

	return program
}
