package main

import (
	"fmt"
	"os"
	"unicode"
	"bytes"
)

type TokenType int
type TokenKind int

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

func (tok TokenType) asString() string {
	switch tok {
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
	panic(
		fmt.Sprintf(
			"[ERROR] We did not expect this tokType '%d' "+
				"- please include a new case or fix your shitty code :)\n",
			tok),
	)
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
