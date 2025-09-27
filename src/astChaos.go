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

var _ = assert[any](
	nodeCount == 11,
	fmt.Sprintf("Expected 11 node types, but found %d", nodeCount),
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
	var value []TokenType

	node := Node{
		NodeType: nodeProcDef,
		Proc: &Proc{
			Scope:   []Node{},
			Inputs:  []Node{},
			Outputs: []Node{},
		},
	}

	value = ChaosSliceSearchValue[TokenType, TokenType](tokOpenParen)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokOpenParen)

		for !ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, ChaosSliceSearchValue[TokenType, TokenType](tokCloseParen)) {
			identifier := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokColon)
			varType := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)

			value = ChaosSliceSearchValue[TokenType, TokenType](tokComma)
			if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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

	value = ChaosSliceSearchValue[TokenType, TokenType](tokArrow)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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
	value = ChaosSliceSearchValue[TokenType, TokenType](tokCloseBraket)
	for !ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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
	var value []TokenType

	value = ChaosSliceSearchValue[TokenType, TokenType](tokNumberLiteral)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
		expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokNumberLiteral)
		lhs.NodeType = nodeIntLiteral
		lhs.Literal = &Literal{
			Int: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	value = ChaosSliceSearchValue[TokenType, TokenType](tokStringLiteral)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
		expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokStringLiteral)
		lhs.NodeType = nodeStringLiteral
		lhs.Literal = &Literal{
			String: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
		expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)

		// TO-DO: This is a crappy solution - I need to rethink how I'm handling shit
		if val, ok := AllocatedVars[expr.Symbol]; ok {
			lhs = val
		}
	}

	return lhs
}

func ASTParseExpression(lex *Lexer, minBp float64) Node {
	var lhs Node
	if lex.GetToken(0).TokenType == tokNumberLiteral {
		expr := lex.ConsumeAssert(tokNumberLiteral)
		lhs.NodeType = nodeIntLiteral
		lhs.Literal = &Literal{
			Int: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	if lex.GetToken(0).TokenType == tokStringLiteral {
		expr := lex.ConsumeAssert(tokStringLiteral)
		lhs.NodeType = nodeStringLiteral
		lhs.Literal = &Literal{
			String: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	if lex.GetToken(0).TokenType == tokBoolLiteral {
		expr := lex.ConsumeAssert(tokBoolLiteral)
		lhs.NodeType = nodeBoolLiteral
		lhs.Literal = &Literal{
			Boolean: expr,
		}
		AllocatedVars[expr.Symbol] = lhs
	}

	if lex.GetToken(0).TokenType == tokIdentifier {
		expr := lex.ConsumeAssert(tokIdentifier)
		if lex.GetToken(0).TokenType == tokOpenParen {
			if _, ok := AllocatedProcs[expr.Symbol]; !ok {
				assert[any](false, "Proc doesn't exist")
			}
			lhs.NodeType = nodeCall
			lhs.Call = &Call{
				Name:   expr,
				Inputs: []Node{},
			}
			lex.ConsumeAssert(tokOpenParen)
			for lex.GetToken(0).TokenType != tokCloseParen {
				n := Node{}
				if lex.GetToken(0).TokenType == tokNumberLiteral {
					expr := lex.ConsumeAssert(tokNumberLiteral)
					n.NodeType = nodeIntLiteral
					n.Literal = &Literal{
						Int: expr,
					}
					AllocatedVars[expr.Symbol] = n
				} else if lex.GetToken(0).TokenType == tokStringLiteral {
					expr := lex.ConsumeAssert(tokStringLiteral)
					n.NodeType = nodeStringLiteral
					n.Literal = &Literal{
						String: expr,
					}
					AllocatedVars[expr.Symbol] = n
				}
				if lex.GetToken(0).TokenType == tokComma {
					lex.ConsumeAssert(tokComma)
				}
				lhs.Call.Inputs = append(lhs.Call.Inputs, n)
			}
			lex.ConsumeAssert(tokCloseParen)
		}

		// TO-DO: This is a crappy solution - I need to rethink how I'm handling shit
		if val, ok := AllocatedVars[expr.Symbol]; ok {
			lhs = val
		}
	}

	if lex.GetToken(0).TokenType == tokOpenParen {
		lex.ConsumeAssert(tokOpenParen)
		lhs = ASTParseExpression(lex, 0.0)
		lex.ConsumeAssert(tokCloseParen)
	}

	for {
		expr := lex.GetToken(0)
		if expr.TokenType == tokCloseParen || expr.TokenType == tokSemicolon {
			break
		}

		var op BinOpOperation
		if expr.TokenType == tokPlus {
			op = opPlus
		} else if expr.TokenType == tokMinus {
			op = opMinus
		} else if expr.TokenType == tokStar {
			op = opMult
		} else if expr.TokenType == tokLessThan {
			op = opLessThan
		} else {
			return Node{NodeType: nodeNoOp}
		}

		lex.Consume()
		lbp, rbp := infixBindingPower(expr)
		if almostEqual(lbp, minBp) {
			break
		}

		rhs := ASTParseExpression(lex, rbp)
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

func ASTParseProcCall(lex *Lexer) (Node, bool) {
	if lex.MatchTokenSequence(tokIdentifier, tokOpenParen) {
		ident := lex.ConsumeAssert(tokIdentifier)
		if _, ok := AllocatedProcs[ident.Symbol]; !ok {
			assert[any](false, "Proc doesn't exist")
		}
		lex.ConsumeAssert(tokOpenParen)
		lex.ConsumeAssert(tokCloseParen)
		node := Node{
			NodeType: nodeCall,
			Call: &Call{
				Name: ident,
			},
		}
		return node, true
	}
	return Node{NodeType: nodeNoOp}, false
}

func ChaosContentParseExit(chaosSlice *ChaosSlice[Token]) Node {
	ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokExit)

	var message Node
	var value []TokenType
	status := ChaosContentParseExpression(chaosSlice, 0.0)

	value = ChaosSliceSearchValue[TokenType, TokenType](tokComma)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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
	var value []TokenType

	value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokEndOfFile)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
		ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokEndOfFile)
		node.NodeType = nodeNoOp
		return node
	}

	value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokAssignment)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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

	value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokColon)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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

		value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier)
		if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
			varType := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokIdentifier)
			node.VarDecl.Type = varType
			AllocatedVars[identifier.Symbol] = node
		}

		value = ChaosSliceSearchValue[TokenType, TokenType](tokSemicolon)
		if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokSemicolon)
			return node
		}

		if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, ChaosSliceSearchValue[TokenType, TokenType](tokAssignment)) {
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokAssignment)
			node.VarDecl.Initialized = true
		} else if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, ChaosSliceSearchValue[TokenType, TokenType](tokColon)) {
			ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokColon)
			node.VarDecl.Initialized = true
			node.Reassignable = false

			value = ChaosSliceSearchValue[TokenType, TokenType](tokProc)
			if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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

	value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokOpenParen)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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

		value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokCloseParen)
		for !ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
			value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokNumberLiteral)
			if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
				expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokNumberLiteral)
				node.Call.Inputs = append(node.Call.Inputs, Node{
					NodeType: nodeIntLiteral,
					Literal: &Literal{
						Int: expr,
					},
				})
			}

			value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokStringLiteral)
			if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
				expr := ChaosSliceConsumeAssert(chaosSlice, ChaosSliceTokenTypeAssert, tokStringLiteral)
				node.Call.Inputs = append(node.Call.Inputs, Node{
					NodeType: nodeIntLiteral,
					Literal: &Literal{
						String: expr,
					},
				})
			}

			value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier, tokComma)
			if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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
	var value []TokenType

	value = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
		return ChaosContentParsePrimaryExpression(chaosSlice)
	}

	value = ChaosSliceSearchValue[TokenType, TokenType](tokExit)
	if ChaosSliceMatchOp(chaosSlice, ChaosSliceTokenTypeComparison, value) {
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
