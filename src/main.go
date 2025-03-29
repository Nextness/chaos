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

type TokenKind struct {
	unsignedInt64 bool
}

type Token struct {
	value   string
	tokType TokenType
	// tokKind TokenKind
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
	fmt.Fprintf(os.Stderr, "[ERROR] We did not expect this tokType '%d' - please include a new case or fix you shitty code :)\n", tokType2)
	os.Exit(1)
	return "unrecheable"
}

func printTokens(listTok []Token) {

	fmt.Printf("Token List:\n")
	for _, theToken := range listTok {
		fmt.Printf("  -> Token { value: '%s', type: '%s' }\n", theToken.value, tokTypeToString(theToken.tokType))
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

func chaosTokenizer(file *[]byte) []Token {
	tokens := bytes.Buffer{}
	listTok := []Token{}
	pos := 0

	content := string(*file)
	singleTokens := []byte{'=', '+', ';'}

	for pos < len(content) {
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
			os.Exit(1)
		}
	}
	listTok = append(listTok, Token{value: "eof", tokType: tokEndOfFile})
	return listTok
}

type Operation string

type NodeBinOp struct {
	lhs, rhs  Token
	operation Operation
}

type NodeExpression struct {
	nodeExpressionType string
	literal            Token
	binaryExpression   NodeBinOp
}

type NodeLet struct {
	identifier     Token
	identifierType Token
	expression     NodeExpression
}

type Statements struct {
	nodeLet NodeLet
}

type NodeStatements struct {
	statements []Statements
}

func increment(value *int) {
	(*value)++
}

func peekToken(listTok *[]Token, length int, pos int, offset int) *Token {
	if pos+offset < length {
		return &(*listTok)[pos+offset]
	}
	return nil
}

func lexNodeExpression(listTok *[]Token, length int, currentPos *int) (*NodeExpression, error) {
	lt := *listTok
	pos := *currentPos
	nodeExpression := NodeExpression{}
	lexError := errors.New("failed to lex expression node")

	if lt[pos].tokType == tokNumber &&
		peekToken(&lt, length, pos, 1) != nil && peekToken(&lt, length, pos, 1).tokType == tokPlus &&
		peekToken(&lt, length, pos, 2) != nil && peekToken(&lt, length, pos, 2).tokType == tokNumber {
		nodeExpression.binaryExpression.lhs = lt[pos]
		increment(&pos)
		nodeExpression.binaryExpression.operation = "sum"
		increment(&pos)
		nodeExpression.binaryExpression.rhs = lt[pos]
		increment(&pos)

		if lt[pos].tokType == tokSemiColon {
			increment(&pos)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Expected semicolon but found %s with value %s\n", tokTypeToString(lt[pos].tokType), lt[pos].value)
			return nil, lexError
		}

	} else if lt[pos].tokType == tokNumber {
		nodeExpression.literal = lt[pos]
		increment(&pos)

		if lt[pos].tokType == tokSemiColon {
			increment(&pos)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Expected semicolon but found %s with value %s\n", tokTypeToString(lt[pos].tokType), lt[pos].value)
			return nil, lexError
		}
	} else {
		return nil, lexError
	}
	return &nodeExpression, nil
}

func lexNodeLet(listTok *[]Token, length int, currentPos *int) (*NodeLet, error) {
	var nodeExpression *NodeExpression
	var err error
	lt := (*listTok)
	pos := *currentPos
	nodeLet := NodeLet{}
	lexError := errors.New("failed to lex let node")

	if lt[pos].tokType == tokIdentifier {
		nodeLet.identifier = lt[pos]
		increment(&pos)
	} else {
		fmt.Fprintf(os.Stderr, "[ERROR] Expected identifier but found %s with value %s\n", tokTypeToString(lt[pos].tokType), lt[pos].value)
		return nil, lexError
	}

	if lt[pos].tokType == tokPrimitiveType {
		nodeLet.identifierType = lt[pos]
		increment(&pos)
	} else {
		fmt.Fprintf(os.Stderr, "[ERROR] Expected type but found %s with value %s\n", tokTypeToString(lt[pos].tokType), lt[pos].value)
		return nil, lexError
	}

	if lt[pos].tokType == tokSemiColon {
		increment(&pos)
		goto end
	} else if lt[pos].tokType == tokAssignment {
		increment(&pos)
	} else {
		fmt.Fprintf(os.Stderr, "[ERROR] Expected assignment but found %s with value %s\n", tokTypeToString(lt[pos].tokType), lt[pos].value)
		return nil, lexError
	}

	nodeExpression, err = lexNodeExpression(&lt, length, &pos)
	if err != nil {
		return nil, lexError
	}
	nodeLet.expression = *nodeExpression

end:
	return &nodeLet, nil
}

func chaosLexer(listTok []Token) NodeLet {
	var nodeLet *NodeLet
	var err error

	pos := 0
	listTokSize := len(listTok)

	if listTok[pos].tokType == tokLet {
		increment(&pos)
		nodeLet, err = lexNodeLet(&listTok, listTokSize, &pos)
		if err != nil {
			fmt.Print("Failed to parse node let\n")
			os.Exit(1)
		}
	}

	if peekToken(&listTok, listTokSize, pos, 1) == nil && listTok[pos].tokType == tokEndOfFile {
		fmt.Printf("Finished lexing\n")
	}

	return *nodeLet
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
			listTok := chaosTokenizer(&file)
			result := chaosLexer(listTok)

			statements := Statements{nodeLet: result}
			listStatements := append([]Statements{}, statements)
			nodeStatements := NodeStatements{
				statements: listStatements,
			}

			fmt.Printf("%+v\n", nodeStatements)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
