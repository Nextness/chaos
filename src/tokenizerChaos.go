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
	tokNewline
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
	tokVarType:    "tokVarType",
	tokIdentifier: "tokIdentifier",
	tokNumber:     "tokNumber",
	tokNewline:    "tokNewline",
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
	line    int
	column  int
}

type ContentState struct {
	token        bytes.Buffer
	data         string
	count        int
	cursor       int
	line, column int
}

func (cs *ContentState) currentChar() byte {
	return cs.data[cs.cursor]
}

func tokenizeChaos(fileName string, fileContent *bytes.Buffer) *TokenizerState {
	contentState := ContentState{
		token:  bytes.Buffer{},
		data:   fileContent.String(),
		count:  fileContent.Len(),
		cursor: 0,
		line:   1,
		column: 1,
	}

	tokenizerState := TokenizerState{
		fileName:  fileName,
		data:      []Token{},
		variables: map[string]string{},
		count:     0,
		cursor:    0,
	}

	singleTokens := []byte{'=', '+', ';'}

	for contentState.cursor < contentState.count {

		if contentState.currentChar() == '\n' {
			token := Token{}
			contentState.token.WriteString("newline")
			val := contentState.token.String()
			token.value = val
			token.tokType = tokNewline
			token.line = contentState.line
			token.column = contentState.column
			contentState.line++
			contentState.column = 0
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
		}

		if unicode.IsSpace(rune(contentState.currentChar())) {
			contentState.column++
			contentState.cursor++
			continue
		}

		if isInside(contentState.currentChar(), singleTokens) {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			contentState.token.WriteByte(contentState.currentChar())
			val := contentState.token.String()
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
			contentState.column++
			contentState.cursor++
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		if isNum(contentState.currentChar()) {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			for isNum(contentState.currentChar()) {
				contentState.token.WriteByte(contentState.currentChar())
				contentState.column++
				contentState.cursor++
			}
			token.value = contentState.token.String()
			token.tokType = tokNumber
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		if isAlpha(contentState.currentChar()) && isUppercase(contentState.currentChar()) {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			for isAlphanum(contentState.currentChar()) {
				contentState.token.WriteByte(contentState.currentChar())
				contentState.column++
				contentState.cursor++
			}
			token.value = contentState.token.String()
			token.tokType = tokVarType
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		if isAlpha(contentState.currentChar()) {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			for isAlphanum(contentState.currentChar()) {
				contentState.token.WriteByte(contentState.currentChar())
				contentState.column++
				contentState.cursor++
			}
			val := contentState.token.String()
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
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}
		fmt.Fprintf(os.Stderr, "[ERROR] Failed while tokenizing - unknown character '%s'\n", string(contentState.currentChar()))
		panic("unrecheable")
	}

	eof := Token{value: "eof", tokType: tokEndOfFile, column: contentState.column, line: contentState.line}
	tokenizerState.data = append(tokenizerState.data, eof)
	tokenizerState.count = len(tokenizerState.data)
	return &tokenizerState
}

type TokenizerState struct {
	fileName  string
	data      []Token
	variables map[string]string
	count     int
	cursor    int
}

func (tokens TokenizerState) print() {
	fmt.Print("Token List:\n")
	for idx, tok := range tokens.data {
		if tok.tokType == tokEndOfFile {
			return
		}
		fmt.Printf(
			"    %s [%03d:%03d] token %05d: '%s' (%s)\n",
			tokens.fileName, tok.line, tok.column, idx, tok.value, tok.tokType.asString(),
		)
	}
}

func (ts *TokenizerState) current() Token {
	return ts.data[ts.cursor]
}

func (ts *TokenizerState) expects(offset int, tokTypes ...TokenType) (result bool) {
	result = false
	for _, tok := range tokTypes {
		if ts.cursor+offset < ts.count {
			if ts.data[ts.cursor+offset].tokType == tok {
				result = true
				break
			}
		}
	}
	return
}

func (ts *TokenizerState) peak(offset int) (result Token) {
	assert(offset == 0, "cannot peak with 0")
	result = Token{}
	if ts.cursor+offset < ts.count {
		result = ts.data[ts.cursor+offset]
	}
	return
}

func (ts *TokenizerState) consume(count ...int) (result Token) {
	assert(len(count) <= 1, "Count can only be 1 or empty")
	c := 0
	if len(count) == 1 {
		c = count[0]
	}
	result = Token{}
	for range c - 1 {
		ts.cursor++
	}
	if ts.cursor < ts.count {
		result = ts.current()
		ts.cursor++
	}
	return
}

type LexerErrorConfig struct {
	errorMsg string
	examples []string
}

func newLexerErroConfig(errorMsg string, examples ...string) LexerErrorConfig {
	return LexerErrorConfig{
		errorMsg: errorMsg,
		examples: examples,
	}
}

func (lec LexerErrorConfig) printAndExitLexerError(ts *TokenizerState) {
	buffer := bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("%s:%d:%d [ERROR] ", ts.fileName, ts.current().line, ts.current().column))
	buffer.WriteString(lec.errorMsg)
	if len(lec.examples) > 0 {
		buffer.WriteString("    ")
		buffer.WriteString("Example:\n")
		for idx, ex := range lec.examples {
			buffer.WriteString("        ")
			buffer.WriteString(fmt.Sprintf("(%d) ", idx+1))
			buffer.WriteString(ex)
		}
	}
	fmt.Print(buffer.String())
	os.Exit(1)
}
