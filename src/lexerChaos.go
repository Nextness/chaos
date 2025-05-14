package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Node interface {
	print(int)
}

type LexerState struct {
	data   []Node
	count  int
	cursor int
}

// TODO: Include fields to overload procedures
// TODO: Include fields to generics in procedures
type NodeProc struct {
	name         string
	argCount     int
	argTypeCount int
	retsCount    int
	args         []any
	argsType     []any
	argDefault   []any
	rets         []any
	retsType     []string
	retsDefault  []any
	procBody     []Node
}

type NodeCondition struct {
	conditionCount int
	conditions     []NodeBinOp
	scope          map[int][]Node
}

type NodeBinOp struct {
	lhs string
	rhs string
	op  string
}

type NodeExit struct {
	sym string
	msg string
	val int
}

type NodeIdentifier struct {
	sym string
	stt string
	tpe string
	val any
}

var _ Node = &NodeExit{}
var _ Node = &NodeIdentifier{}
var _ Node = &NodeBinOp{}
var _ Node = &NodeProc{}

func makePad(padSize int) string {
	return strings.Repeat(" ", padSize)
}

func (n *NodeProc) print(padSize int) {
	pad := makePad(padSize)
	fmt.Printf("%sNodeProc [%s]\n", pad, n.name)
	fmt.Printf("%s├─ Inputs [%d]: ", pad, n.argCount)
	for _, arg := range n.args {
		fmt.Printf("%s ", arg.(Token).value)
	}
	fmt.Print("\n")

	fmt.Printf("%s├─ Types [%d]: ", pad, n.argTypeCount)
	for _, arg := range n.argsType {
		fmt.Printf("%s ", arg.(Token).value)
	}
	fmt.Print("\n")

	fmt.Printf("%s├─ Returns [%d]: ", pad, n.retsCount)
	for _, arg := range n.rets {
		fmt.Printf("%s ", arg.(Token).value)
	}
	fmt.Print("\n")

	fmt.Printf("%s└─ Body\n", pad)
	for _, arg := range n.procBody {
		arg.print(padSize + 4)
	}
}

func (n *NodeCondition) print(padSize int) {
	pad := makePad(padSize)
	fmt.Printf("%sNodeCondition [chaosIntrinsic]\n", pad)
	for _, binOp := range n.conditions {
		binOp.print(padSize + 4)
	}
	for i := 0; i <= n.conditionCount; i++ {
		for _, expr := range n.scope[i] {
			expr.print(padSize + 4)
		}
	}
}

func (n *NodeBinOp) print(padSize int) {
	pad := makePad(padSize)
	fmt.Printf("%sNodeBinOp [chaosIntrinsic]\n", pad)
	fmt.Printf("%s├─ lhs: '%s'\n", pad, n.lhs)
	fmt.Printf("%s├─ rhs: '%s'\n", pad, n.rhs)
	fmt.Printf("%s└─ op:  '%s'\n", pad, n.op)
}

func (n *NodeExit) print(padSize int) {
	pad := makePad(padSize)
	fmt.Printf("%sNodeExit [chaosIntrinsic]\n", pad)
	if n.msg != "" {
		fmt.Printf("%s├─ Value: '%d'\n", pad, n.val)
		fmt.Printf("%s└─ Msg: '%s'\n", pad, n.msg)
		return
	}
	fmt.Printf("%s└─➜ Value:'%d'\n", pad, n.val)
}

func (n *NodeIdentifier) print(padSize int) {
	pad := makePad(padSize)
	fmt.Printf("%sNodeIdentifier [%s]\n", pad, n.sym)
	switch v := n.val.(type) {
	case string:
		fmt.Printf("%s├─ State: %s\n", pad, n.stt)
		fmt.Printf("%s├─ Type: %s\n", pad, n.tpe)
		if n.tpe == kindString.asString() {
			fmt.Printf("%s└─ Value: «%s»\n", pad, n.val)
			break
		}
		fmt.Printf("%s└─ Value: %s\n", pad, n.val)
	case NodeBinOp:
		fmt.Printf("%s├─ State: %s\n", pad, n.stt)
		fmt.Printf("%s├─ Type: %s\n", pad, n.tpe)
		fmt.Printf("%s└─ Value:\n", pad)
		v.print(padSize + 4)
		break
	}
}

func lexChaosProc(ts *TokenizerState) *NodeProc {
	newNode := NodeProc{
		argCount:  0,
		retsCount: 0,
	}

	ts.consume() // consume the 'proc'
	if !ts.matchAt(0, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected an identifier but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("proc someName with arg1 String returns Void executes ... endproc")
		config.printAndExitLexerError(ts)
	}

	newNode.name = ts.consume().value

	if !ts.matchAt(0, tokExpects) {
		errMsg := fmt.Sprintf("Expected an '(' but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("proc someName with arg1 String returns Void executes ... endproc")
		config.printAndExitLexerError(ts)
	}

	ts.consume() // consume the 'expects'

	for ts.current().tokType != tokReturns {
		if !ts.matchAt(0, tokIdentifier) {
			panic("Expected identifier - handle errors correctly")
		}
		newNode.args = append(newNode.args, ts.consume())

		if !ts.matchAt(0, tokVariadic, tokVarType) {
			panic("Expected Type - handle errors correctly")
		}

		// TODO: Handle variadic types
		if ts.matchAt(0, tokVariadic) && ts.matchAt(1, tokVarType) {
			ts.consume()
			newNode.argsType = append(newNode.argsType, ts.consume())
		} else if ts.matchAt(0, tokVarType) {
			newNode.argsType = append(newNode.argsType, ts.consume())
		}

		if ts.matchAt(0, tokComma) {
			ts.consume()
		}
	}
	newNode.argCount = len(newNode.args)
	newNode.argTypeCount = len(newNode.argsType)

	ts.consume() // consume the 'returns'

	// TODO: Handle parenthesis in return type
	for ts.current().tokType != tokExecutes {
		newNode.rets = append(newNode.rets, ts.consume())
	}
	newNode.retsCount = len(newNode.rets)

	ts.consume() // consume the 'executes'

	for ts.current().tokType != tokEnd {
		if ts.matchAt(0, tokLet) {
			node := &NodeIdentifier{}
			// TODO: better hanadle lexing errors
			if node = lexChaosLet(ts); node == nil {
				os.Exit(1)
			}
			newNode.procBody = append(newNode.procBody, node)
			continue
		}

		if ts.matchAt(0, tokIf) {
			node := &NodeCondition{}
			if node = lexChaosConditions(ts); node == nil {
				os.Exit(1)
			}
			newNode.procBody = append(newNode.procBody, node)
			continue
		}

		if ts.matchAt(0, tokExit) {
			node := &NodeExit{}
			if node = lexExitChaos(ts); node == nil {
				os.Exit(1)
			}
			newNode.procBody = append(newNode.procBody, node)
			continue
		}
		ts.consume()
	}

	ts.consumeEndBlock(tokProc)
	return &newNode
}

func lexChaosConditions(ts *TokenizerState) *NodeCondition {
	newNode := NodeCondition{
		conditionCount: 0,
		conditions:     []NodeBinOp{},
		scope:          map[int][]Node{},
	}

	ts.consume() // consume the 'if'

	if !ts.matchAt(0, tokNumber, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected a Number but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("if 1 < 2 do ... endif")
		config.printAndExitLexerError(ts)
	}

	if ts.matchAt(1, tokLessThan, tokGreaterThan) {
		lhs := ts.consume()
		op := ts.consume()
		if !ts.matchAt(0, tokNumber) {
			errMsg := fmt.Sprintf("Expected a number but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg)
			config.printAndExitLexerError(ts)
		}
		rhs := ts.consume()
		binOp := NodeBinOp{
			lhs: lhs.value,
			rhs: rhs.value,
			op:  op.value,
		}
		newNode.conditions = append(newNode.conditions, binOp)
		newNode.conditionCount++
	}

	if !ts.matchAt(0, tokExecutes) {
		errMsg := fmt.Sprintf("Expected 'executes' but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("if 1 < 2 do ... endif")
		config.printAndExitLexerError(ts)
	}

	ts.consume() // consume the 'executes'

	if ts.matchAt(0, tokLet) {
		node := &NodeIdentifier{}
		// TODO: better hanadle lexing errors
		if node = lexChaosLet(ts); node == nil {
			os.Exit(1)
		}
		newNode.scope[newNode.conditionCount] = append(newNode.scope[newNode.conditionCount], node)
	}

	if ts.matchAt(0, tokExit) {
		node := &NodeExit{}
		if node = lexExitChaos(ts); node == nil {
			os.Exit(1)
		}
		newNode.scope[newNode.conditionCount] = append(newNode.scope[newNode.conditionCount], node)
	}

	if !ts.matchAt(0, tokEnd) && !ts.matchAt(1, tokIf) {
		errMsg := fmt.Sprintf("Expected 'end if' but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("if 1 < 2 executes ... end if")
		config.printAndExitLexerError(ts)
	}

	ts.consumeEndBlock(tokIf)
	return &newNode
}

func lexChaosLet(ts *TokenizerState) *NodeIdentifier {
	newNode := NodeIdentifier{}
	newNode.val = ""
	newNode.stt = "not-initialized"

	ts.consume() // consume the 'let'
	newNode.sym = ts.current().value

	if !ts.matchAt(0, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected an identifier but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
		config.printAndExitLexerError(ts)
	}
	varName := ts.consume().value
	if _, ok := ts.variables[varName]; !ok {
		ts.variables[varName] = ""
	}

	if !ts.matchAt(0, tokVarType, tokAssignment) {
		errMsg := fmt.Sprintf("Expected a type or assignment but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("let something U64")
		config.newExample("let something U64 = 1")
		config.printAndExitLexerError(ts)
	}

	if ts.matchAt(0, tokVarType) {
		token := ts.consume()
		newNode.tpe = token.tokKind.asString()
	}

	if ts.matchAt(0, tokAssignment) {
		if newNode.tpe == "" {
			newNode.tpe = "must-infer"
		}
		ts.consume() // Consume '='
		if !ts.matchAt(0, tokNumber, tokIdentifier, tokString) {
			errMsg := fmt.Sprintf("Expected a number, or identifier or string but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg)
			config.newExample("let something U64")
			config.newExample("let something U64 = 1")
			config.printAndExitLexerError(ts)
		}
		if ts.matchAt(0, tokNumber) {
			newNode.stt = "initialized"
			if ts.matchAt(1, tokPlus, tokMinus) {
				lhs := ts.consume()
				op := ts.consume()
				if !ts.matchAt(0, tokNumber) {
					errMsg := fmt.Sprintf("Expected a number but found %s\n", ts.current().tokType.asString())
					config := newLexerErroConfig(errMsg)
					config.printAndExitLexerError(ts)
				}
				rhs := ts.consume()
				binOp := NodeBinOp{
					lhs: lhs.value,
					rhs: rhs.value,
					op:  op.value,
				}
				newNode.val = binOp
			} else {
				value := ts.consume().value
				newNode.val = value
				ts.variables[varName] = value
			}
		} else if ts.matchAt(0, tokString) {
			value := ts.consume().value
			newNode.val = value
			newNode.stt = "initialized"
			ts.variables[varName] = value
		} else if ts.matchAt(0, tokIdentifier) {
			if val, ok := ts.variables[ts.current().value]; ok {
				name := ts.consume().value
				ts.variables[name] = val
				newNode.val = name
				newNode.stt = "initialized"
			} else {
				errMsg := fmt.Sprintf("Undefined identifier %s\n", ts.current().value)

				config := newLexerErroConfig(errMsg)
				config.newExample("let something U64")
				config.newExample("let something U64 = 1")
				config.newExample("let something U64 = 1", "let somethingElse = something")
				config.newExample("let something String = «string literal»")
				config.newExample("let something String = «string literal»", "let somethingElse = something")
				config.printAndExitLexerError(ts)
			}
		}
	}

	return &newNode
}

func lexExitChaos(ts *TokenizerState) *NodeExit {
	newNode := NodeExit{}

	ts.consume()
	if !ts.matchAt(0, tokNumber, tokIdentifier) {
		errMsg := fmt.Sprintf("Expected a number or identifier but found %s\n", ts.current().tokType.asString())
		config := newLexerErroConfig(errMsg)
		config.newExample("exitWith 1")
		config.newExample("exitWith 1, «reason for exiting early»")
		config.newExample("let code U64 = 1", "exitWith code")
		config.newExample("let code U64 = 1", "exitWith code, «reason for exitin early»")
		config.printAndExitLexerError(ts)
	}

	if ts.matchAt(0, tokNumber) {
		token := ts.consume()
		asInt, err := strconv.Atoi(token.value)
		if err != nil {
			panic("failed to convert str to number")
		}
		newNode.val = asInt
	}
	if ts.matchAt(0, tokIdentifier) {
		token := ts.consume()
		val := ts.variables[token.value]
		asInt, err := strconv.Atoi(val)
		if err != nil {
			panic("failed to convert str to number")
		}
		newNode.sym = token.value
		newNode.val = asInt
	}

	if ts.matchAt(0, tokComma) {
		if ts.matchAt(1, tokString) {
			ts.consume()
			newNode.msg = ts.consume().value
		} else {
			if ts.matchAt(0, tokComma) {
				ts.consume()
			}
			errMsg := fmt.Sprintf("Expected a string but found %s\n", ts.current().tokType.asString())
			config := newLexerErroConfig(errMsg)
			config.newExample("exitWith 1")
			config.newExample("exitWith 1, «reason for exiting early»")
			config.newExample("let code U64 = 1", "exitWith code")
			config.newExample("let code U64 = 1", "exitWith code, «reason for exitin early»")
			config.printAndExitLexerError(ts)
		}
	}

	return &newNode
}

func lexChaosExpression(ts *TokenizerState) (bool, Node) {
	// TODO: tokLet should allow not only variable definition and assignment, but also anonymous
	// structs and functions
	if ts.matchAt(0, tokLet) {
		node := &NodeIdentifier{}
		if node = lexChaosLet(ts); node == nil {
			os.Exit(1)
		}
		return true, node
	}
	return false, nil
}

// TODO: better handle how 'end <block>' is parsed in the lexer
// since we are always doing the same operation once a block is closed
func lexerChaos(ts *TokenizerState) *LexerState {
	assert(ts.cursor == 0, "Cursor is not 0")
	lexerState := LexerState{data: []Node{}, count: 0, cursor: 0}

	for ts.cursor < ts.count {
		if ts.matchAt(0, tokEndOfFile) {
			break
		}

		if ts.matchAt(0, tokProc) {
			node := &NodeProc{}
			if node = lexChaosProc(ts); node == nil {
				os.Exit(1)
			}
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ok, node := lexChaosExpression(ts); ok {
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokIf) {
			node := &NodeCondition{}
			if node = lexChaosConditions(ts); node == nil {
				os.Exit(1)
			}
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokExit) {
			node := &NodeExit{}
			if node = lexExitChaos(ts); node == nil {
				os.Exit(1)
			}
			lexerState.data = append(lexerState.data, node)
			continue
		}

		if ts.matchAt(0, tokNewline) {
			ts.consume()
			continue
		}

		fmt.Fprintf(os.Stderr, "[ERROR] The token '%s' is not expected\n", ts.current().value)
		panic("Unexpected token found")
	}

	assert(tokEndOfFile == ts.current().tokType, "Expected eof")
	ts.consume()
	lexerState.count = len(lexerState.data)
	return &lexerState
}
