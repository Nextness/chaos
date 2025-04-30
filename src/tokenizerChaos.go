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
	tokString
	tokNewline
	tokSemiColon
	tokEndOfFile
	tokExitWith
	tokComma
	tokCount
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
	tokString:     "tokString",
	tokNewline:    "tokNewline",
	tokExitWith:   "tokExitWith",
	tokComma:      "tokComma",
	tokEndOfFile:  "tokEndOfFile",
}

func (tok TokenType) asString() string {
	if tokType, ok := mapping[tok]; ok {
		return tokType
	}
	errorMsg := fmt.Sprintf("[ERROR] Unexpected toke '%d' - fix this shitty code :)\n", tok)
	panic(errorMsg)
}

type TokenKind int

const (
	kindAnyError TokenKind = iota
	kindAnyType
	kindString
	kindChar
	kindBool
	kindU8
	kindU16
	kindU32
	kindU64
	kindU128
	kindI8
	kindI16
	kindI32
	kindI64
	kindI128
	kindF16
	kindF32
	kindF64
	kindF128
	kindC64
	kindC128
	kindQ128
	kindQ256
	kindArray
	kindVoid
	kindMap
	kindNone
)

type IdentTypeToStringMap map[TokenKind]string

var identTypeMapping = IdentTypeToStringMap{
	kindAnyError: "AnyError",
	kindAnyType:  "AnyType",
	kindString:   "String",
	kindChar:     "Char",
	kindBool:     "Bool",
	kindU8:       "U8",
	kindU16:      "U16",
	kindU32:      "U32",
	kindU64:      "U64",
	kindU128:     "U128",
	kindI8:       "I8",
	kindI16:      "I16",
	kindI32:      "I32",
	kindI64:      "I64",
	kindI128:     "I128",
	kindF16:      "F16",
	kindF32:      "F32",
	kindF64:      "F64",
	kindF128:     "F128",
	kindC64:      "C64",
	kindC128:     "C128",
	kindQ128:     "Q128",
	kindQ256:     "Q256",
	kindArray:    "Array",
	kindVoid:     "Void",
	kindMap:      "Map",
	kindNone:     "noType",
}

func (tok TokenKind) asString() string {
	if tokType, ok := identTypeMapping[tok]; ok {
		return tokType
	}
	errorMsg := fmt.Sprintf("[ERROR] Unexpected type '%d' - fix this shitty code :)\n", tok)
	panic(errorMsg)
}

type Token struct {
	value   string
	tokType TokenType
	tokKind TokenKind
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

func (cs *ContentState) currentCharAsString() string {
	return string(cs.data[cs.cursor])
}

func (cs *ContentState) currentChar() byte {
	return cs.data[cs.cursor]
}

func (cs *ContentState) peakCharAsString(offset int) (result string) {
	result = ""
	if cs.cursor+offset < cs.count {
		result = string(cs.data[cs.cursor+offset])
	}
	return
}

func (cs *ContentState) peakChar(offset int) (result byte) {
	result = byte(0)
	if cs.cursor+offset < cs.count {
		result = cs.data[cs.cursor+offset]
	}
	return
}

// TODO: Improve how characters are handled. Probably move to a byte or uint32 type of char
// I want to avoid this kind of shit '"«"[0]', which is super annoying - also it will probaly
// make it easier to handle other things in the lexer
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

	singleTokens := []byte{'=', '+', ';', ','}

	for contentState.cursor < contentState.count {
		// Single line comment
		if contentState.currentChar() == '/' && contentState.peakChar(1) == '/' {
			for contentState.currentChar() != '\n' {
				contentState.cursor++
			}
			continue
		}

		// Multiline and inplace comment
		if contentState.currentChar() == '/' &&
			contentState.peakChar(1) == '*' &&
			contentState.peakChar(2) == '*' {
			nestedComment := 0
			contentState.cursor += 3
			for {
				if contentState.currentChar() == '*' &&
					contentState.peakChar(1) == '*' &&
					contentState.peakChar(2) == '/' &&
					nestedComment > 0 {
					nestedComment--
					contentState.cursor += 3
					continue
				}

				if contentState.currentChar() == '*' &&
					contentState.peakChar(1) == '*' &&
					contentState.peakChar(2) == '/' {
					contentState.cursor += 3
					break
				}
				contentState.cursor++
				if contentState.currentChar() == '/' &&
					contentState.peakChar(1) == '*' &&
					contentState.peakChar(2) == '*' {
					nestedComment++
					contentState.cursor += 3
					continue
				}
			}
			continue
		}

		// New line
		if contentState.currentChar() == '\n' {
			token := Token{}
			contentState.token.WriteString("newline")
			val := contentState.token.String()
			token.value = val
			token.tokType = tokNewline
			token.line = contentState.line
			token.column = contentState.column
			token.tokKind = kindNone
			contentState.line++
			contentState.cursor++
			contentState.column = 0
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		// Empty characters
		if unicode.IsSpace(rune(contentState.currentChar())) ||
			rune(contentState.currentChar()) == rune("\x00"[0]) {
			contentState.column++
			contentState.cursor++
			continue
		}

		// Single char tokens
		if isInside(contentState.currentChar(), singleTokens) {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			token.tokKind = kindNone
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
			} else if val == "," {
				token.value = val
				token.tokType = tokComma
			}
			contentState.column++
			contentState.cursor++
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		// Number tokens
		if isNum(contentState.currentChar()) {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			token.tokKind = kindNone
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

		// Variable Type tokens
		if isAlpha(contentState.currentChar()) && isUppercase(contentState.currentChar()) && contentState.currentChar() != "«"[0] {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			for isAlphanum(contentState.currentChar()) {
				contentState.token.WriteByte(contentState.currentChar())
				contentState.column++
				contentState.cursor++
			}
			token.tokType = tokVarType
			token.value = contentState.token.String()
			switch token.value {
			case "AnyError":
				token.tokKind = kindAnyError
			case "AnyType":
				token.tokKind = kindAnyType
			case "String":
				token.tokKind = kindString
			case "Char":
				token.tokKind = kindChar
			case "Bool":
				token.tokKind = kindBool
			case "U8":
				token.tokKind = kindU8
			case "U16":
				token.tokKind = kindU16
			case "U32":
				token.tokKind = kindU32
			case "U64":
				token.tokKind = kindU64
			case "U128":
				token.tokKind = kindU128
			case "I8":
				token.tokKind = kindI8
			case "I16":
				token.tokKind = kindI16
			case "I32":
				token.tokKind = kindI32
			case "I64":
				token.tokKind = kindI64
			case "I128":
				token.tokKind = kindI128
			case "F16":
				token.tokKind = kindF16
			case "F32":
				token.tokKind = kindF32
			case "F64":
				token.tokKind = kindF64
			case "F128":
				token.tokKind = kindF128
			case "C64":
				token.tokKind = kindC64
			case "C128":
				token.tokKind = kindC128
			case "Q128":
				token.tokKind = kindQ128
			case "Q256":
				token.tokKind = kindQ256
			case "Array":
				token.tokKind = kindArray
			case "Void":
				token.tokKind = kindVoid
			case "Map":
				token.tokKind = kindMap
			}

			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		// Identifier and keyword tokens
		if isAlpha(contentState.currentChar()) && contentState.currentChar() != "«"[0] {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			token.tokKind = kindNone
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
			} else if val == "exitWith" {
				token.value = val
				token.tokType = tokExitWith
			} else {
				token.value = val
				token.tokType = tokIdentifier
			}
			tokenizerState.data = append(tokenizerState.data, token)
			contentState.token.Reset()
			continue
		}

		// String literal tokens
		if contentState.currentChar() == "«"[0] {
			token := Token{}
			token.line = contentState.line
			token.column = contentState.column
			token.tokKind = kindNone
			contentState.cursor += 2
			for contentState.peakCharAsString(1) != "»" {
				contentState.token.WriteString(contentState.currentCharAsString())
				contentState.cursor++
			}
			contentState.cursor += 2
			token.tokType = tokString
			token.value = contentState.token.String()
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
		if tok.tokKind == kindNone {
			fmt.Printf(
				"    %s [%03d:%03d] token %05d: '%s' (%s)\n",
				tokens.fileName, tok.line, tok.column, idx, tok.value, tok.tokType.asString(),
			)
		} else {
			fmt.Printf(
				"    %s [%03d:%03d] token %05d: '%s' (%s[%s])\n",
				tokens.fileName, tok.line, tok.column, idx, tok.value, tok.tokType.asString(), tok.tokKind.asString(),
			)
		}
	}
}

func (ts *TokenizerState) current() Token {
	return ts.data[ts.cursor]
}

func (ts *TokenizerState) matchAt(offset int, tokTypes ...TokenType) (result bool) {
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
