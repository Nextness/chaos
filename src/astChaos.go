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

func (node *Node) printtt(idx int) {

	if node.NodeType == nodeNoOp {
		return
	}

	if node.NodeType == nodeCall {
		name := node.Call.Name.Symbol
		fmt.Printf("%6d. call(%s, Void, Void)\n", idx, name)
		return
	}

	if node.NodeType == nodeIdentifier {
		reassinableType := "const"
		if node.Reassignable {
			reassinableType = "var"
		}

		name := node.VarDecl.Name.Symbol
		t := "infer"
		if node.VarDecl.Type.Symbol != "" {
			t = node.VarDecl.Type.Symbol
		}

		if node.VarDecl.Assignment.NodeType == nodeProcDef {
			var inputs string = ""
			if node.VarDecl.Assignment.Proc.Inputs != nil {
				size := len(node.VarDecl.Assignment.Proc.Inputs)
				for id, n := range node.VarDecl.Assignment.Proc.Inputs {
					if id == size-1 {
						inputs = fmt.Sprintf("%s%s: %s", inputs, n.VarDecl.Name.Symbol, n.VarDecl.Type.Symbol)
						continue
					}
					inputs = fmt.Sprintf("%s%s: %s, ", inputs, n.VarDecl.Name.Symbol, n.VarDecl.Type.Symbol)
				}
			}
			fmt.Printf("%6d. %s(%s) -> Void {\n", idx, name, inputs)
			for _, nod := range node.VarDecl.Assignment.Proc.Scope {
				nod.printtt(idx)
			}
			fmt.Printf("%6d. }\n", idx)
			return
		}

		if node.VarDecl.Assignment.NodeType == nodeBinOp {
			var lhs, rhs Node
			var okLhs, okRhs bool = false, false
			if node.VarDecl.Assignment.BinOp.Lhs.NodeType != nodeNull {
				okLhs = true
				lhs = node.VarDecl.Assignment.BinOp.Lhs
			}

			if node.VarDecl.Assignment.BinOp.Rhs.NodeType != nodeNull {
				okRhs = true
				rhs = node.VarDecl.Assignment.BinOp.Rhs
			}

			if node.VarDecl.Assignment.BinOp.Operation == opPlus {
				if okLhs && okRhs {
					if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIntLiteral {
						l := castAssert[int](lhs.Literal.Int.Value)
						r := castAssert[int](rhs.Literal.Int.Value)
						fmt.Printf("%6d. %s(%s, %s) = %d + %d\n", idx, reassinableType, name, t, l, r)
						return
					}
					if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIdentifier {
						l := castAssert[int](lhs.Literal.Int.Value)
						r := rhs.VarDecl.Name.Symbol
						fmt.Printf("%6d. %s(%s, %s) = %d + %s\n", idx, reassinableType, name, t, l, r)
						return
					}
					if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIntLiteral {
						l := lhs.VarDecl.Name.Symbol
						r := castAssert[int](rhs.Literal.Int.Value)
						fmt.Printf("%6d. %s(%s, %s) = %s + %d\n", idx, reassinableType, name, t, l, r)
						return
					}
					if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIdentifier {
						l := lhs.VarDecl.Name.Symbol
						r := rhs.VarDecl.Name.Symbol
						fmt.Printf("%6d. %s(%s, %s) = %s + %s\n", idx, reassinableType, name, t, l, r)
						return
					}
				}
				return
			}
			if node.VarDecl.Assignment.BinOp.Operation == opLessThan {
				if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIntLiteral {
					l := castAssert[int](lhs.Literal.Int.Value)
					r := castAssert[int](rhs.Literal.Int.Value)
					fmt.Printf("%6d. %s(%s, %s) = %d < %d\n", idx, reassinableType, name, t, l, r)
					return
				}
				if lhs.NodeType == nodeIntLiteral && rhs.NodeType == nodeIdentifier {
					l := castAssert[int](lhs.Literal.Int.Value)
					r := rhs.VarDecl.Name.Symbol
					fmt.Printf("%6d. %s(%s, %s) = %d < %s\n", idx, reassinableType, name, t, l, r)
					return
				}
				if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIntLiteral {
					l := lhs.VarDecl.Name.Symbol
					r := castAssert[int](rhs.Literal.Int.Value)
					fmt.Printf("%6d. %s(%s, %s) = %s < %d\n", idx, reassinableType, name, t, l, r)
					return
				}
				if lhs.NodeType == nodeIdentifier && rhs.NodeType == nodeIdentifier {
					l := lhs.VarDecl.Name.Symbol
					r := rhs.VarDecl.Name.Symbol
					fmt.Printf("%6d. %s(%s, %s) = %s < %s\n", idx, reassinableType, name, t, l, r)
					return
				}
				return
			}

		}

		if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
			result := castAssert[int](node.VarDecl.Assignment.Literal.Int.Value)
			fmt.Printf("%6d. %s(%s, %s) = %d\n", idx, reassinableType, name, t, result)
			return
		}

		if node.VarDecl.Assignment.NodeType == nodeFloatLiteral {
			result := castAssert[float64](node.VarDecl.Assignment.Literal.Int.Value)
			fmt.Printf("%6d. %s(%s, %s) = %.2f\n", idx, reassinableType, name, t, result)
			return
		}

		if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
			result := castAssert[string](node.VarDecl.Assignment.Literal.String.Value)
			fmt.Printf("%6d. %s(%s, %s) = \"%s\"\n", idx, reassinableType, name, t, result)
			return
		}

		if !node.VarDecl.Initialized {
			reassinableType = "var"
			name := node.VarDecl.Name.Symbol
			fmt.Printf("%6d. %s(%s, %s)\n", idx, reassinableType, name, t)
			return
		}
		if node.VarDecl.Assignment.NodeType == nodeIdentifier {
			name := node.VarDecl.Name.Symbol
			fmt.Printf("%6d. %s(%s, %s) = %s\n", idx, reassinableType, name, t, node.VarDecl.Assignment.VarDecl.Name.Symbol)
			return
		}

		fmt.Printf("[ERROR] Not implemented :: %+v\n", node.VarDecl.Assignment)
		return
	}

	if node.NodeType == nodeExit {
		msg := ""
		if node.Exit.Message.NodeType == nodeStringLiteral {
			msg = castAssert[string](node.Exit.Message.Literal.String.Value)
		}

		if node.Exit.Status.NodeType == nodeIdentifier {
			status := node.Exit.Status.VarDecl.Name.Symbol
			fmt.Printf("%6d. exit(%s, \"%s\")\n", idx, status, msg)
		}
		if node.Exit.Status.NodeType == nodeIntLiteral {
			status := castAssert[int](node.Exit.Status.Literal.Int.Value)
			fmt.Printf("%6d. exit(%d, \"%s\")\n", idx, status, msg)
		}
		return
	}

	assert[any](false, fmt.Sprintf("Unknown node '%v'", node))
}

func ASTParseProc(lex *Lexer) Node {
	node := Node{
		NodeType: nodeProcDef,
		Proc: &Proc{
			Scope:   []Node{},
			Inputs:  []Node{},
			Outputs: []Node{},
		},
	}

	if lex.MatchTokenSequence(tokOpenParen) {
		lex.ConsumeAssert(tokOpenParen)
		for lex.GetToken(0).TokenType != tokCloseParen {
			ident := lex.ConsumeAssert(tokIdentifier)
			lex.ConsumeAssert(tokColon)
			varType := lex.ConsumeAssert(tokIdentifier)
			lex.MatchTokenSequenceAndConsumeAssert(tokComma)

			node.Proc.Inputs = append(node.Proc.Inputs, Node{
				NodeType: nodeIdentifier,
				VarDecl: &VarDecl{
					Name:        ident,
					Type:        varType,
					Initialized: false,
				},
			})
		}
		lex.ConsumeAssert(tokCloseParen)
	}

	if lex.MatchTokenSequence(tokArrow) {
		lex.ConsumeAssert(tokArrow)
		ident := lex.ConsumeAssert(tokIdentifier)
		node.Proc.Outputs = append(node.Proc.Outputs, Node{
			NodeType: nodeIdentifier,
			VarDecl: &VarDecl{
				Type: ident,
			},
		})
	}

	lex.ConsumeAssert(tokOpenBraket)
	for lex.GetToken(0).TokenType != tokCloseBraket {
		assert[any](false, "must be fixed later")
		// stmt := ASTParseStatement(lex)
		// node.Proc.Scope = append(node.Proc.Scope, stmt)
	}
	lex.ConsumeAssert(tokCloseBraket)
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

func ASTParseIdentifier(lex *Lexer) Node {
	var node Node
	if lex.MatchTokenSequence(tokIdentifier, tokAssignment) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssert(tokAssignment)
		node.NodeType = nodeIdentifier
		node.Reassignable = true
		node.VarDecl = &VarDecl{
			Name:        ident,
			Assignment:  ASTParseExpression(lex, 0.0),
			Initialized: true,
		}
		AllocatedVars[ident.Symbol] = node
		lex.ConsumeAssert(tokSemicolon)
		return node
	}

	if lex.MatchTokenSequence(tokIdentifier, tokColon) {
		ident := lex.ConsumeAssert(tokIdentifier)
		lex.ConsumeAssert(tokColon)
		node.NodeType = nodeIdentifier
		node.Reassignable = true
		node.VarDecl = &VarDecl{
			Name:        ident,
			Initialized: false,
		}

		if lex.MatchTokenSequence(tokIdentifier) {
			varType := lex.Consume()
			node.VarDecl.Type = varType
			AllocatedVars[ident.Symbol] = node
		}

		if lex.GetToken(0).TokenType == tokSemicolon {
			lex.ConsumeAssert(tokSemicolon)
			return node
		}

		if lex.MatchTokenSequence(tokAssignment) {
			lex.ConsumeAssert(tokAssignment)
			node.VarDecl.Initialized = true
			if node.VarDecl.Type == (Token{}) {
				node.VarDecl.Type = Token{
					TokenType: tokInfer,
				}
			}
		} else if lex.MatchTokenSequence(tokColon) {
			lex.ConsumeAssert(tokColon)
			node.VarDecl.Initialized = true
			node.Reassignable = false
			if node.VarDecl.Type == (Token{}) {
				node.VarDecl.Type = Token{
					TokenType: tokInfer,
				}
			}
			if lex.MatchTokenSequence(tokProc) {
				lex.ConsumeAssert(tokProc)
				node.VarDecl.Assignment = ASTParseProc(lex)
				AllocatedProcs[ident.Symbol] = node
				return node
			}
		} else {
			assert[any](false, fmt.Sprintf("Unexpected token %s", TokenTypeToString(lex.GetToken(0).TokenType)))
		}

		AllocatedVars[ident.Symbol] = node
		node.VarDecl.Assignment = ASTParseExpression(lex, 0.0)
		lex.ConsumeAssert(tokSemicolon)
		return node
	}

	return Node{NodeType: nodeNoOp}
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

func ASTParseExit(lex *Lexer) Node {
	if lex.MatchTokenSequence(tokExit) {
		lex.ConsumeAssert(tokExit)

		var message Node
		status := ASTParseExpression(lex, 0.0)
		if lex.MatchTokenSequence(tokComma) {
			lex.ConsumeAssert(tokComma)
			message = ASTParseExpression(lex, 0.0)
		}

		node := Node{
			NodeType: nodeExit,
			Exit: &Exit{
				Status:  status,
				Message: message,
			},
		}

		lex.ConsumeAssert(tokSemicolon)
		return node
	}
	return Node{NodeType: nodeNoOp}
}

func ASTParsePrimaryExpression(lex *Lexer) Node {
	if node := ASTParseIdentifier(lex); node.NodeType != nodeNoOp {
		return node
	}

	// TO-DO: generalize this so that it can also be used in the ASTParseExpression
	if lex.GetToken(0).TokenType == tokIdentifier {
		node := Node{
			NodeType: nodeCall,
		}
		expr := lex.ConsumeAssert(tokIdentifier)
		if lex.GetToken(0).TokenType == tokOpenParen {
			if _, ok := AllocatedProcs[expr.Symbol]; !ok {
				assert[any](false, "Proc doesn't exist")
			}
			node.NodeType = nodeCall
			node.Call = &Call{
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
				node.Call.Inputs = append(node.Call.Inputs, n)
			}
			lex.ConsumeAssert(tokCloseParen)
			lex.ConsumeAssert(tokSemicolon)
			return node
		} else {
			var ok bool
			node, ok = AllocatedVars[expr.Symbol]
			assert[any](ok, fmt.Sprintf("Variable %s not allocated", expr.Symbol))
		}
	}

	if lex.GetToken(0).TokenType == tokEndOfFile {
		return Node{NodeType: nodeNoOp}
	}

	return assert[Node](
		false,
		fmt.Sprintf(
			"[ERROR] %s - Unexpected token \"%s\" at %02d:%02d",
			lex.filepath, TokenTypeToString(lex.GetToken(0).TokenType), lex.GetToken(0).Position.line, lex.GetToken(0).Position.column,
		),
	)
}

func ASTParseStatement(tokens *ChaosSlice[Token]) Node {
	var search []TokenType

	search = ChaosSliceSearchValue[TokenType, TokenType](tokIdentifier)
	if ChaosSliceMatchOp(tokens, ChaosSliceTokenTypeComparison, search) {
		// return ASTParsePrimaryExpression(lex)
	}

	search = ChaosSliceSearchValue[TokenType, TokenType](tokExit)
	if ChaosSliceMatchOp(tokens, ChaosSliceTokenTypeComparison, search) {
		// return ASTParseExit(lex)
	}

	return Node{NodeType: nodeNoOp}
}

func ASTCreateChaosProgram(tokens *ChaosSlice[Token]) Program {
	assert[any](tokens.cursor == 0, "Cursor is not 0")

	program := Program{
		Nodes: []Node{},
	}

	for tokens.cursor < tokens.count {
		node := ASTParseStatement(tokens)
		program.Nodes = append(program.Nodes, node)
		ChaosSliceConsume(tokens)
	}

	return program
}
