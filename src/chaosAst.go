package main

import (
	"fmt"
)

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
	nodeConditions
	nodeAnonymousScope
	nodeReturn
	nodeCount
)

type BinOpOperation int
type Scope []Node

const (
	opNull BinOpOperation = iota
	opNoOp
	opPlus
	opMinus
	opMult
	opEquals
	opNotEquals
	opLessThan
	opGreaterThan
	opLessThanEquals
	opGreaterThanEquals
	opOr
	opAnd
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
	Scope   Scope
	Inputs  []Node
	Outputs []Node
	Arity   int
}

type Call struct {
	Name   Token
	Inputs []Node
	Arity  int
}

type Conditions struct {
	Count       int
	Evaluations []BinOp
	Scopes      [][]Node
}

type Return struct {
	Outputs []Node
}

type Node struct {
	NodeType       NodeType
	Reassignable   bool
	Reassigned     bool
	Infered        bool
	Literal        *Literal
	VarDecl        *VarDecl
	Exit           *Exit
	BinOp          *BinOp
	Proc           *Proc
	Call           *Call
	Conditions     *Conditions
	AnonymousScope *Scope
	Return         *Return
}

var _ = assert[any](
	nodeCount == 14,
	fmt.Sprintf("Expected 14 node types, but found %d", nodeCount),
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
	} else if nodeType == nodeConditions {
		return "nodeConditions"
	} else if nodeType == nodeAnonymousScope {
		return "nodeAnonymousScope"
	} else if nodeType == nodeReturn {
		return "nodeReturn"
	}
	return assert[string](
		nodeCount == 14,
		fmt.Sprintf("Expected 14 node types, but found %d", nodeCount),
	)
}

func checkIdentifierInScope(token Token, accessScopes []Scope, depth int) {
	scopeLength := len(accessScopes)
	if scopeLength-1 < depth {
		assert[any](false, fmt.Sprintf("Token: %s, depth (%d) cannot be larger than scopeLength (%d)", token.Symbol, depth, scopeLength))
	}
	for idx := range accessScopes {
		for _, node := range accessScopes[depth-idx] {
			if node.NodeType == nodeIdentifier && node.VarDecl.Name.Symbol == token.Symbol {
				return
			}
			if node.NodeType == nodeCall && node.Call.Name.Symbol == token.Symbol {
				return
			}
		}
	}
	assert[any](false, fmt.Sprintf("The symbol '%s' is not defined or out of scope", token.Symbol))
}

func infixBindingPower(token Token) (float64, float64) {
	// TO-DO: I think there is a bug in here somewhere, or maybe
	// in the ParseExperssion function, because the ast for the experssions
	// are not looking correct.
	switch token.TokenType {
	case tokPlus, tokMinus:
		return 1.0, 1.1
	case tokStar:
		return 2.0, 2.1
	case tokOr, tokAnd:
		return 4.1, 4.0
	case tokLessThan, tokNotEquals, tokEquals, tokGreaterThan, tokLessThanEquals, tokGreaterThanEquals:
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

func ChaosContentParseBaseNode(chaosSlice *ChaosSlice[Token], accessScopes []Scope, depth int) Node {
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
		checkIdentifierInScope(expr, accessScopes, depth)
		node.NodeType = nodeIdentifier
		node.VarDecl = &VarDecl{
			Name: expr,
		}

		if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenParen) {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)
			node.NodeType = nodeCall
			node.VarDecl = nil
			node.Call = &Call{
				Name:   expr,
				Inputs: []Node{},
			}
			for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseParen) {
				n := ChaosContentParseBaseNode(chaosSlice, accessScopes, depth)
				node.Call.Inputs = append(node.Call.Inputs, n)

				if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
					CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
				}

				node.Call.Arity++
			}
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
		}
	}

	return node
}

func ChaosContentParseExpression(chaosSlice *ChaosSlice[Token], minBp float64, accessScope []Scope, depth int) Node {
	var lhs Node = ChaosContentParseBaseNode(chaosSlice, accessScope, depth)

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenParen) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)
		lhs = ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth)
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseParen)
	}

	for true {
		var op BinOpOperation
		if CSMatch(chaosSlice, CSTokenTypeComparison, tokPlus) {
			op = opPlus
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokMinus) {
			op = opMinus
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokStar) {
			op = opMult
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokLessThan) {
			op = opLessThan
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokGreaterThan) {
			op = opGreaterThan
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokLessThanEquals) {
			op = opLessThanEquals
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokGreaterThanEquals) {
			op = opGreaterThanEquals
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokEquals) {
			op = opEquals
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokNotEquals) {
			op = opNotEquals
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokOr) {
			op = opOr
		} else if CSMatch(chaosSlice, CSTokenTypeComparison, tokAnd) {
			op = opAnd
		} else {
			break
		}

		expr := CSConsume(chaosSlice)
		lbp, rbp := infixBindingPower(expr)
		if almostEqual(lbp, minBp) {
			break
		}

		rhs := ChaosContentParseExpression(chaosSlice, rbp, accessScope, depth)
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

func ChaosContentParseScope(chaosSlice *ChaosSlice[Token], accessScopes []Scope, depth int, allowThen bool, isFunction bool) Scope {
	assert[any](isFunction, "Cannot create scopes outside of functions")
	var expectedThen bool = false
	var scope Scope
	depth++

	scopeLength := len(accessScopes)
	if scopeLength-1 != depth {
		accessScopes = append(accessScopes, Scope{})
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenBraket) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenBraket)
		for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseBraket) {
			node := ChaosContentASTParseStatement(chaosSlice, accessScopes, depth, isFunction)
			scope = append(scope, node)
		}
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokCloseBraket)
		return scope
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokThen) {
		if allowThen {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokThen)
			expectedThen = true
		} else {
			assert[any](false, "The token 'then' is not expected")
		}
	}

	n := ChaosContentASTParseStatement(chaosSlice, accessScopes, depth, isFunction)
	scope = append(scope, n)

	if allowThen && !expectedThen {
		// TO-DO: improve this warning message
		fmt.Printf("[Warning] Expected then but found nothing\n")
	}

	return scope
}

func ChaosContentParseConditionalIf(chaosSlice *ChaosSlice[Token], node *Node, accessScope []Scope, depth int, isFunction bool) {
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIf)

	cond := ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth)
	node.Conditions.Evaluations = append(node.Conditions.Evaluations, *cond.BinOp)

	currentBranch := ChaosContentParseScope(chaosSlice, accessScope, depth, true, isFunction)
	node.Conditions.Scopes = append(node.Conditions.Scopes, currentBranch)
	node.Conditions.Count += 1
}

func ChaosContentParseConditionalElif(chaosSlice *ChaosSlice[Token], node *Node, accessScope []Scope, depth int, isFunction bool) {
	if !CSMatch(chaosSlice, CSTokenTypeComparison, tokElif) {
		return
	}
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokElif)
	cond := ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth)
	node.Conditions.Evaluations = append(node.Conditions.Evaluations, *cond.BinOp)

	currentBranch := ChaosContentParseScope(chaosSlice, accessScope, depth, true, isFunction)
	node.Conditions.Scopes = append(node.Conditions.Scopes, currentBranch)

	node.Conditions.Count += 1
	ChaosContentParseConditionalElif(chaosSlice, node, accessScope, depth, isFunction)
}

func ChaosContentParseConditionalElse(chaosSlice *ChaosSlice[Token], node *Node, accessScope []Scope, depth int, isFunction bool) {
	if !CSMatch(chaosSlice, CSTokenTypeComparison, tokElse) {
		return
	}
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokElse)
	node.Conditions.Evaluations = append(node.Conditions.Evaluations, BinOp{Operation: opNoOp})

	currentBranch := ChaosContentParseScope(chaosSlice, accessScope, depth, false, isFunction)
	node.Conditions.Scopes = append(node.Conditions.Scopes, currentBranch)
	node.Conditions.Count += 1
}

func ChaosContentParseConditionalBranches(chaosSlice *ChaosSlice[Token], accessScope []Scope, depth int, isFunction bool) Node {
	node := Node{
		NodeType: nodeConditions,
		Conditions: &Conditions{
			Count:       0,
			Evaluations: []BinOp{},
			Scopes:      [][]Node{},
		},
	}

	ChaosContentParseConditionalIf(chaosSlice, &node, accessScope, depth, isFunction)
	ChaosContentParseConditionalElif(chaosSlice, &node, accessScope, depth, isFunction)
	ChaosContentParseConditionalElse(chaosSlice, &node, accessScope, depth, isFunction)

	return node
}

func ChaosContentParseProcDefinition(chaosSlice *ChaosSlice[Token], accessScope []Scope, depth int, isFunction bool) Node {
	node := Node{
		NodeType: nodeProcDef,
		Proc: &Proc{
			Scope:   Scope{},
			Inputs:  []Node{},
			Outputs: []Node{},
			Arity:   0,
		},
	}

	// TO-DO: Stop using hardcoded mechanism to acquire inputs for proc
	// definitions. This is being used for now just to make sure proc definition
	// work during development. We can't use ChaosContentParseBaseNode because
	// it expects identifiers to exist in scopes and we don't really have scopes for
	// input definitions.
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

	// TO-DO: Stop using hardcoded mechanism to handle function return type.
	// We can't use ChaosContentParseBaseNode because
	// it expects identifiers to exist in scopes and we don't really have scopes for
	// input definitions.
	if CSMatch(chaosSlice, CSTokenTypeComparison, tokArrow) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokArrow)
		retType := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		node.Proc.Outputs = append(node.Proc.Outputs, Node{
			NodeType: nodeIdentifier,
			VarDecl: &VarDecl{
				Type:        retType,
				Initialized: false,
			},
		})
	}

	depth++
	accessScope = append(accessScope, Scope{})
	accessScope[depth] = append(accessScope[depth], node.Proc.Inputs...)
	currentExecution := ChaosContentParseScope(chaosSlice, accessScope, depth, false, isFunction)
	node.Proc.Scope = append(node.Proc.Scope, currentExecution...)

	return node
}

func ChaosContentParseExit(chaosSlice *ChaosSlice[Token], accessScope []Scope, depth int) Node {
	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokExit)

	status := ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth)

	var message Node
	if CSMatch(chaosSlice, CSTokenTypeComparison, tokComma) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokComma)
		message = ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth)
	}

	CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)

	node := Node{
		NodeType: nodeExit,
		Exit: &Exit{
			Status:  status,
			Message: message,
		},
	}

	return node
}

func ChaosContentParsePrimaryExpression(chaosSlice *ChaosSlice[Token], accessScope []Scope, depth int) Node {
	var node Node

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokEndOfFile) {
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokEndOfFile)
		node.NodeType = nodeNoOp
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier, tokAssignment) {
		identifier := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		checkIdentifierInScope(identifier, accessScope, depth)

		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokAssignment)
		node.NodeType = nodeIdentifier
		node.Reassigned = true
		node.Reassignable = true
		node.VarDecl = &VarDecl{
			Name:        identifier,
			Assignment:  ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth),
			Initialized: true,
		}

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
			accessScope[depth] = append(accessScope[depth], node)
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
				node.VarDecl.Assignment = ChaosContentParseProcDefinition(chaosSlice, accessScope, depth, true)
				accessScope[depth] = append(accessScope[depth], node)
				return node
			}
		} else {
			token := CSGet(chaosSlice)
			assert[any](
				false,
				fmt.Sprintf("Unexpected token %s", TokenTypeToString(token.TokenType)),
			)
		}

		accessScope[depth] = append(accessScope[depth], node)

		expr := ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth)
		ChaosContentInferType(&node, expr)

		node.VarDecl.Assignment = expr
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)
		return node
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier, tokOpenParen) {
		identifier := CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokIdentifier)
		CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokOpenParen)
		checkIdentifierInScope(identifier, accessScope, depth)

		node.NodeType = nodeCall
		node.Call = &Call{
			Name:   identifier,
			Inputs: []Node{},
		}

		for !CSMatch(chaosSlice, CSTokenTypeComparison, tokCloseParen) {
			n := ChaosContentParseBaseNode(chaosSlice, accessScope, depth)
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

func ChaosContentASTParseStatement(chaosSlice *ChaosSlice[Token], accessScope []Scope, depth int, isFunction bool) Node {
	if CSMatch(chaosSlice, CSTokenTypeComparison, tokOpenBraket) {
		scp := ChaosContentParseScope(chaosSlice, accessScope, depth, true, isFunction)
		return Node{
			NodeType:       nodeAnonymousScope,
			AnonymousScope: &scp,
		}
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIdentifier) {
		return ChaosContentParsePrimaryExpression(chaosSlice, accessScope, depth)
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokExit) {
		return ChaosContentParseExit(chaosSlice, accessScope, depth)
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokIf) {
		if depth > 1 {
			return ChaosContentParseConditionalBranches(chaosSlice, accessScope, depth, isFunction)
		}
		assert[any](false, "Cannot have if statements at global scope.")
	}

	if CSMatch(chaosSlice, CSTokenTypeComparison, tokReturn) {
		if isFunction {
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokReturn)
			n := ChaosContentParseExpression(chaosSlice, 0.0, accessScope, depth)
			CSConsumeAssert(chaosSlice, CSTokenTypeAssert, tokSemicolon)

			return Node{
				NodeType: nodeReturn,
				Return: &Return{
					Outputs: []Node{n},
				},
			}
		}
		assert[any](false, "Cannot have return statement outside of procedures.")
	}

	return Node{NodeType: nodeNoOp}
}

func ChaosContentAST(tokens *ChaosSlice[Token]) (*ChaosSlice[Node], []Scope, int) {
	assert[any](tokens.cursor == 0, "Cursor is not 0")
	var accessScopes = []Scope{}
	var depth = 0

	scopeLength := len(accessScopes)
	if scopeLength-1 != depth {
		accessScopes = append(accessScopes, Scope{} /*global scope at 0th position*/)
	}

	nodes := []Node{}
	for tokens.cursor < tokens.count {
		node := ChaosContentASTParseStatement(tokens, accessScopes, depth, false)
		if node.NodeType == nodeNoOp {
			CSConsume(tokens)
			continue
		}
		accessScopes[depth] = append(accessScopes[depth], node)
		nodes = append(nodes, node)
	}

	program := &ChaosSlice[Node]{
		data:   nodes,
		count:  len(nodes),
		cursor: 0,
	}

	return program, accessScopes, depth
}

// TO-DO: While parsing scopes, we may find a problem where we don't close the scope.
//   This can result in a hang, which is not ideal. Probably I need to fix it later...
//   Example:
//   something :: proc {
//       a :: 10;
//       {
//           b := a;
//   }
