package main

import (
	"bytes"
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
)

type TokenKind struct {
	unsignedInt64 bool
}

type Token struct {
	value   string
	tokType TokenType
	tokKind TokenKind
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
			} else {
				fmt.Fprintf(os.Stderr, "[ERROR] Reached unrecheable location with '%s'\n", val)
				os.Exit(1)
			}

			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else if isNum(content[pos]) {
			for isNum(content[pos]) {
				tokens.WriteByte(content[pos])
				pos++
			}

			token := Token{
				value: tokens.String(),
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
				value: tokens.String(),
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
	return listTok
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
			printTokens(listTok)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}

	os.Exit(0)
}

