package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type TokenType int
type TokenKind int
type Operation string
type NodeType int

const (
	tokGlobal TokenType = iota
	tokLet
	tokAssignment
	tokPlus
	tokPrimitiveType
	tokIdentifier
	tokNumber
	tokSemiColon
	tokCount
	tokEndOfFile
)

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

type Token struct {
	value   string
	tokType TokenType
	tokKind *TokenKind
}

type Tokens struct {
	list   []Token
	count  int
	curPos int
}

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

func tokTypeToString(tokType2 TokenType) string {
	switch tokType2 {
	case tokGlobal:
		return "tokGlobal"
	case tokLet:
		return "tokLet"
	case tokAssignment:
		return "tokAssingment"
	case tokPlus:
		return "tokPlus"
	case tokPrimitiveType:
		return "tokPrimitiveType"
	case tokIdentifier:
		return "tokIdentifier"
	case tokNumber:
		return "tokNumber"
	case tokSemiColon:
		return "tokSemiColon"
	case tokEndOfFile:
		return "tokEndOfFile"
	}
	fmt.Fprintf(os.Stderr, "[ERROR] We did not expect this tokType '%d' - please include a new case or fix your shitty code :)\n", tokType2)
	panic("unrecheable")
}

func printTokens(tokens Tokens) {
	fmt.Print("Token List:\n")
	for _, theToken := range tokens.list {
		fmt.Printf("  Token { value: '%s', type: '%s', kind: '%v' }\n", theToken.value, tokTypeToString(theToken.tokType), theToken.tokKind)
	}
}

func printStatements(program []Statements) {
	// TODO: make this shit better. This looks awful...
	fmt.Print("Statements:\n")
	for _, statement := range program {
		if node := statement.node; node != nil {
			fmt.Printf("  - let (node)\n")
			fmt.Printf("    '- %s (%s)\n", node.identifier.value, tokTypeToString(node.identifier.tokType))
			fmt.Printf("    '- %s (%s)\n", node.identifierType.value, tokTypeToString(node.identifierType.tokType))
			if node.binaryExpression != nil {
				fmt.Printf("    '- %s (node)\n", node.binaryExpression.operation)
				fmt.Printf("      '- %s (%s) [lhs]\n", node.binaryExpression.lhs.value, tokTypeToString(node.binaryExpression.lhs.tokType))
				fmt.Printf("      '- %s (%s) [rhs]\n", node.binaryExpression.rhs.value, tokTypeToString(node.binaryExpression.rhs.tokType))
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

func chaosTokenizer(fileContent *bytes.Buffer) []Token {
	tokens := bytes.Buffer{}
	listTok := []Token{}
	pos := 0

	content := fileContent.String()
	singleTokens := []byte{'=', '+', ';'}

	for pos < fileContent.Len() {
		token := Token{}
		if unicode.IsSpace(rune(content[pos])) {
			pos++
			continue
		} else if isInside(content[pos], singleTokens) {
			tokens.WriteByte(content[pos])
			pos++
			val := tokens.String()

			if val == "=" {
				token.value = val
				token.tokType = tokAssignment
			} else if val == "+" {
				token.value = val
				token.tokType = tokPlus
			} else if val == ";" {
				token.value = val
				token.tokType = tokSemiColon
			}

			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else if isNum(content[pos]) {
			for isNum(content[pos]) {
				tokens.WriteByte(content[pos])
				pos++
			}

			// TODO: Improve details about numbers with tokenKind,
			// defining what type of number it is - int32, int64, etc...
			token := Token{
				value:   tokens.String(),
				tokType: tokNumber,
			}

			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else if isAlpha(content[pos]) && isUppercase(content[pos]) {
			for isAlphanum(content[pos]) {
				tokens.WriteByte(content[pos])
				pos++
			}

			token := Token{
				value:   tokens.String(),
				tokType: tokPrimitiveType,
			}

			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else if isAlpha(content[pos]) {
			for isAlphanum(content[pos]) {
				tokens.WriteByte(content[pos])
				pos++
			}
			val := tokens.String()

			if val == "global" {
				token.value = val
				token.tokType = tokGlobal
			} else if val == "let" {
				token.value = val
				token.tokType = tokLet
			} else {
				token.value = val
				token.tokType = tokIdentifier
			}

			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed while tokenizing - unknown character '%s'\n", string(content[pos]))
			panic("unrecheable")
		}
	}
	listTok = append(listTok, Token{value: "eof", tokType: tokEndOfFile})
	return listTok
}

func lexNodeExpression(tokens *Tokens) (*AtomNode, error) {
	nodeExpression := AtomNode{}
	lexError := errors.New("failed to lex expression node")

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

		fmt.Fprintf(os.Stderr, "[ERROR] Expected semicolon but found %s with value %s\n", tokTypeToString(tokens.current().tokType), tokens.current().value)
		return nil, lexError

	} else if tokens.expect(tokNumber) {
		nodeExpression.identifier = *tokens.current()
		tokens.consume()

		fmt.Fprintf(os.Stderr, "[ERROR] Expected semicolon but found %s with value %s\n", tokTypeToString(tokens.current().tokType), tokens.current().value)
		return nil, lexError
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
		fmt.Fprintf(os.Stderr, "[ERROR] Expected identifier but found %s with value %s\n", tokTypeToString(tokens.current().tokType), tokens.current().value)
		return nil, lexError
	}

	if tokens.expect(tokPrimitiveType) {
		nodeLet.identifierType = tokens.current()
		tokens.consume()
	} else {
		fmt.Fprintf(os.Stderr, "[ERROR] Expected type but found %s with value %s\n", tokTypeToString(tokens.current().tokType), tokens.current().value)
		return nil, lexError
	}

	if tokens.peak().tokType == tokAssignment {
		if tokens.expect(tokAssignment) {
			tokens.consume()
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Expected assignment but found %s with value %s\n", tokTypeToString(tokens.current().tokType), tokens.current().value)
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

	os.Exit(0)
}
