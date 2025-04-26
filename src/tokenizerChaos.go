package main

import (
	"bytes"
	"fmt"
	"os"
	"unicode"
)

type TokenType int

const (
	tokGlobal TokenType = iota
	tokLet
	tokAssignment
	tokPlus
	tokVarType
	tokIdentifier
	tokNumber
	tokSemiColon
	tokCount
	tokEndOfFile
)

type TokenToStringMap map[TokenType]string

var mapping = TokenToStringMap{
	tokGlobal:     "tokGlobal",
	tokLet:        "tokLet",
	tokAssignment: "tokAssignment",
	tokPlus:       "tokPlus",
	tokVarType:    "tokVarPlus",
	tokIdentifier: "tokIdentifier",
	tokNumber:     "tokNumber",
	tokEndOfFile:  "tokEndOfFile",
}

func (tok TokenType) asString() string {
	if tokType, ok := mapping[tok]; ok {
		return tokType
	}
	errorMsg := fmt.Sprintf("[ERROR] Unexpected toke '%d' - fix this shitty code :)\n", tok)
	panic(errorMsg)
}

type Token struct {
	value   string
	tokType TokenType
	row     int
	column  int
}

type ContentState struct {
	token       bytes.Buffer
	data        string
	count       int
	cursor      int
	row, column int
}

func (cs *ContentState) currentChar() byte {
	return cs.data[cs.cursor]
}

func tokenizeChaos(fileContent *bytes.Buffer) *TokenizerState {
	contentState := ContentState{
		token:  bytes.Buffer{},
		data:   fileContent.String(),
		count:  fileContent.Len(),
		cursor: 0,
		row: 0,
		column: 0,
	}

	tokenizerState := TokenizerState{
		data:   []Token{},
		count:  0,
		cursor: 0,
	}

	singleTokens := []byte{'=', '+', ';'}

	for contentState.cursor < contentState.count {
		if contentState.currentChar() == '\n' {
			contentState.row++
			contentState.column = 0
		}

		token := Token{}
		if unicode.IsSpace(rune(contentState.currentChar())) {
			contentState.column++
			contentState.cursor++
			continue
		}

		if isInside(contentState.currentChar(), singleTokens) {
			contentState.token.WriteByte(contentState.currentChar())
			val := contentState.token.String()
			if val == "=" {
				token.value = val
				token.tokType = tokAssignment
				token.row = contentState.row
				token.column = contentState.column
			} else if val == "+" {
				token.value = val
				token.tokType = tokPlus
				token.row = contentState.row
				token.column = contentState.column
			} else if val == ";" {
				token.value = val
				token.tokType = tokSemiColon
				token.row = contentState.row
				token.column = contentState.column
			}
			contentState.column++
			contentState.cursor++
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		if isNum(contentState.currentChar()) {
			for isNum(contentState.currentChar()) {
				contentState.token.WriteByte(contentState.currentChar())
				contentState.column++
				contentState.cursor++
			}
			token := Token{
				value:   contentState.token.String(),
				tokType: tokNumber,
				column:  contentState.column,
				row:     contentState.row,
			}
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		if isAlpha(contentState.currentChar()) && isUppercase(contentState.currentChar()) {
			for isAlphanum(contentState.currentChar()) {
				contentState.token.WriteByte(contentState.currentChar())
				contentState.column++
				contentState.cursor++
			}
			token := Token{
				value:   contentState.token.String(),
				tokType: tokVarType,
				column:  contentState.column,
				row:     contentState.row,
			}
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		if isAlpha(contentState.currentChar()) {
			for isAlphanum(contentState.currentChar()) {
				contentState.token.WriteByte(contentState.currentChar())
				contentState.column++
				contentState.cursor++
			}
			val := contentState.token.String()
			if val == "global" {
				token.value = val
				token.tokType = tokGlobal
				token.row = contentState.row
				token.column = contentState.column
			} else if val == "let" {
				token.value = val
				token.tokType = tokLet
				token.row = contentState.row
				token.column = contentState.column
			} else {
				token.value = val
				token.tokType = tokIdentifier
				token.row = contentState.row
				token.column = contentState.column
			}
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}
		fmt.Fprintf(os.Stderr, "[ERROR] Failed while tokenizing - unknown character '%s'\n", string(contentState.currentChar()))
		panic("unrecheable")
	}

	eof := Token{value: "eof", tokType: tokEndOfFile, column: contentState.column, row: contentState.row}
	tokenizerState.data = append(tokenizerState.data, eof)
	tokenizerState.count = len(tokenizerState.data)
	return &tokenizerState
}


type TokenizerState struct {
	data   []Token
	count  int
	cursor int
}

func (tokens TokenizerState) print() {
	fmt.Print("Token List:\n")
	for idx, tok := range tokens.data {
		fmt.Printf(
			"  [%03d:%03d] token %05d: '%s' (%s)\n",
			tok.row, tok.column, idx, tok.value, tok.tokType.asString(),
		)
	}
}

func (ts *TokenizerState) current() Token {
	return ts.data[ts.cursor]
}

func (ts *TokenizerState) peak(offset int) (result Token) {
	if offset == 0 {
		panic("cannot peak with 0")
	}
	result = Token{}
	if ts.cursor+offset < ts.count {
		result = ts.data[ts.cursor+offset]
	}
	return
}

func (ts *TokenizerState) consume(offset int) (result Token) {
	if offset == 0 {
		panic("cannot peak with 0")
	}
	result = Token{}
	if ts.cursor+offset < ts.count {
		result = ts.current()
		ts.count++
	}
	return
}

