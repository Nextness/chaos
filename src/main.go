package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type Operation string
type NodeType int

const (
	rtNumIntU8 TokenKind = iota
	rtNumIntU16
	rtNumIntU32
	rtNumIntU64
	rtNumIntU128
	rtNumIntI8
	rtNumIntI16
	rtNumIntI32
	rtNumIntI64
	rtNumIntI128
	rtNumFloat16
	rtNumFloat32
	rtNumFloat64
	rtNumFloat128
	rtNumComplex64
	rtNumComplex128
	rtNumQuaternion128
	rtNumQuaternion256
	rtString
	rtChar
	rtBool
	rtVoid
	rtArray
	rtMap
	rtAnyError
	rtAnyType
)

const (
	nodeLet NodeType = iota
)

type NodeBinOp struct {
	lhs, rhs  Token
	operation Operation
}

type AtomNodeDetails struct {
}

type AtomNode struct {
	// Global fields
	identifier Token
	nodeType   NodeType

	// Let field
	identifierType *Token

	// Expression field
	nodeExpressionType *string
	binaryExpression   *NodeBinOp
}

type Statements struct {
	node *AtomNode
}

type NodeStatements struct {
	statements []Statements
}

func printTokens(tokens Tokens) {
	fmt.Print("Token List:\n")
	for _, theToken := range tokens.list {
		fmt.Printf("  Token { value: '%s', type: '%s', kind: '%v' }\n", theToken.value, theToken.tokType.asString(), theToken.tokKind)
	}
}

func printStatements(program []Statements) {
	// TODO: make this shit better. This looks awful...
	fmt.Print("Statements:\n")
	for _, statement := range program {
		if node := statement.node; node != nil {
			fmt.Printf("  - let (node)\n")
			fmt.Printf("    '- %s (%s)\n", node.identifier.value, node.identifier.tokType.asString())
			fmt.Printf("    '- %s (%s)\n", node.identifierType.value, node.identifierType.tokType.asString())
			if node.binaryExpression != nil {
				fmt.Printf("    '- %s (node)\n", node.binaryExpression.operation)
				fmt.Printf("      '- %s (%s) [lhs]\n", node.binaryExpression.lhs.value, node.binaryExpression.lhs.tokType.asString())
				fmt.Printf("      '- %s (%s) [rhs]\n", node.binaryExpression.rhs.value, node.binaryExpression.rhs.tokType.asString())
			}
		}
	}
}

func isUppercase(b byte) bool {
	s := rune(b)
	return unicode.IsUpper(s) && unicode.IsLetter(s)
}

func isNum(b byte) bool {
	s := rune(b)
	return unicode.IsNumber(s)
}

func isAlpha(b byte) bool {
	s := rune(b)
	return unicode.IsLetter(s)
}

func isAlphanum(b byte) bool {
	s := rune(b)
	return unicode.IsNumber(s) || unicode.IsLetter(s)
}

func isInside(b byte, listChar []byte) bool {
	result := false
	for _, item := range listChar {
		if result = (b == item); result {
			break
		}
	}
	return result
}

func (tk *Tokens) consume() {
	if tk.curPos < tk.count {
		tk.curPos++
	}
}

func (tk *Tokens) current() (result *Token) {
	result = nil
	if tk.curPos < tk.count {
		result = &tk.list[tk.curPos]
	}
	return
}

func (tk *Tokens) peak(offset ...int) (result *Token) {
	offsetLen := len(offset)
	if offsetLen > 1 {
		panic("Cannot pass more than one value. This is used to allow no variable")
	}

	off := 0
	result = nil
	if offsetLen == 1 {
		off = offset[0]
	}

	if tk.curPos+off < tk.count {
		result = &tk.list[tk.curPos+off]
	}
	return
}

func (tk *Tokens) expect(tokType TokenType, offset ...int) bool {
	return tk.peak(offset...).tokType == tokType
}

func lexNodeExpression(tokens *Tokens) (*AtomNode, error) {
	nodeExpression := AtomNode{}

	hasBinOp := (tokens.expect(tokNumber) &&
		tokens.peak(1) != nil && tokens.expect(tokPlus, 1) &&
		tokens.peak(2) != nil && tokens.expect(tokNumber, 2))
	if hasBinOp {
		nodeBinOp := &NodeBinOp{}
		nodeBinOp.lhs = *tokens.current()
		tokens.consume()
		nodeBinOp.operation = "sum"
		tokens.consume()
		nodeBinOp.rhs = *tokens.current()
		tokens.consume()
		nodeExpression.binaryExpression = nodeBinOp
	} else if tokens.expect(tokNumber) {
		nodeExpression.identifier = *tokens.current()
		tokens.consume()
	}
	return &nodeExpression, nil
}

func lexNodeLet(tokens *Tokens) (*AtomNode, error) {
	nodeLet := AtomNode{}
	lexError := errors.New("failed to lex let node")

	if tokens.expect(tokIdentifier) {
		nodeLet.identifier = *tokens.current()
		tokens.consume()
	} else {
		fmt.Fprintf(os.Stderr, "[ERROR] Expected identifier but found %s with value %s\n", tokens.current().tokType.asString(), tokens.current().value)
		return nil, lexError
	}

	if tokens.expect(tokPrimitiveType) {
		nodeLet.identifierType = tokens.current()
		tokens.consume()
	} else {
		fmt.Fprintf(os.Stderr, "[ERROR] Expected type but found %s with value %s\n", tokens.current().tokType.asString(), tokens.current().value)
		return nil, lexError
	}

	if tokens.peak().tokType == tokAssignment {
		if tokens.expect(tokAssignment) {
			tokens.consume()
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Expected assignment but found %s with value %s\n", tokens.current().tokType.asString(), tokens.current().value)
			return nil, lexError
		}
	}
	nodeExpression, err := lexNodeExpression(tokens)
	if err != nil {
		return nil, lexError
	}
	nodeLet.nodeExpressionType = nodeExpression.nodeExpressionType
	nodeLet.binaryExpression = nodeExpression.binaryExpression

	return &nodeLet, nil
}

func chaosLexer(tokens *Tokens) (*NodeStatements, error) {
	listStatements := []Statements{}

	statements := Statements{}
	for !tokens.expect(tokEndOfFile) {
		if tokens.expect(tokLet) {
			tokens.consume()
			nodeLet, err := lexNodeLet(tokens)
			if err != nil {
				fmt.Print("Failed to parse node let\n")
				return nil, errors.New("failed to parse statements")
			}
			statements.node = nodeLet
			listStatements = append(listStatements, statements)
		}
	}
	if tokens.expect(tokEndOfFile) {
		fmt.Printf("Finished lexing\n")
	}

	return &NodeStatements{listStatements}, nil

}

func main() {
	programName := os.Args[0]
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
		fmt.Printf("Usage: %s <file.chaos>\n", programName)
		os.Exit(1)
	}

	otherArgs := os.Args[1:]
	for _, arg := range otherArgs {
		if strings.HasSuffix(arg, ".chaos") {
			if _, err := os.Stat(arg); err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] The provided file path '%s' doesn't exist\n", arg)
				os.Exit(1)
			}
			file, err := os.ReadFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to open the file for whatever reason\n")
				os.Exit(1)
			}

			fileContent := bytes.NewBuffer(file)
			tokensList := chaosTokenizer(fileContent)
			tokens := Tokens{list: tokensList, count: len(tokensList), curPos: 0}
			printTokens(tokens)

			program, err := chaosLexer(&tokens)
			printStatements(program.statements)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}
}
