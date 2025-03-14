package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
	"strconv"
)

type TokenType int

const (
	tokGlobal TokenType = iota
	tokLet
	tokAssignment
	tokPlus
	tokType
	tokVariable
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
	case tokType:
		return "tokType"
	case tokVariable:
		return "tokVariable"
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
		if unicode.IsSpace(rune(content[pos])) {
			pos++
			continue
		} else if isInside(content[pos], singleTokens) {
			tokens.WriteByte(content[pos])
			pos++
			token := Token{
				value: tokens.String(),
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
			}
			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else if isAlpha(content[pos]) {
			for isAlphanum(content[pos]) {
				tokens.WriteByte(content[pos])
				pos++
			}
			token := Token{
				value: tokens.String(),
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

func chaosParser(listTok []Token) []Token {
	for id := range listTok {
		_, err := strconv.Atoi(listTok[id].value)
		if err == nil {
			listTok[id].tokType = tokNumber
			continue
		}
		if listTok[id].value == "global" {
			listTok[id].tokType = tokGlobal
			continue
		} else if listTok[id].value == "let" {
			listTok[id].tokType = tokLet
			continue
		} else if isUppercase(listTok[id].value[0]) {
			listTok[id].tokType = tokType
			continue
		} else if listTok[id].value == "+" {
			listTok[id].tokType = tokPlus
			continue
		} else if listTok[id].value == ";" {
			listTok[id].tokType = tokSemiColon
			continue
		} else if listTok[id].value == "=" {
			listTok[id].tokType = tokAssignment
			continue
		} else {
			listTok[id].tokType = tokVariable
			continue
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
			listTok = chaosParser(listTok)
			printTokens(listTok)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
