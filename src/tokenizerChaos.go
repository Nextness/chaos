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
	tokMinus
	tokVarType
	tokIdentifier
	tokNumber
	tokString
	tokNewline
	tokEndOfFile
	tokExitWith
	tokComma
	tokLessThan
	tokGreaterThan
	tokDo
	tokIf
	tokEndIf
	tokProc
	tokEndProc
	tokOpenParen
	tokCloseParen
	tokVariadic
	tokCount
)

type TokenToStringMap map[TokenType]string

var mapping = TokenToStringMap{
	tokGlobal:      "tokGlobal",
	tokLet:         "tokLet",
	tokAssignment:  "tokAssignment",
	tokPlus:        "tokPlus",
	tokMinus:       "tokMinus",
	tokVarType:     "tokVarType",
	tokIdentifier:  "tokIdentifier",
	tokNumber:      "tokNumber",
	tokString:      "tokString",
	tokNewline:     "tokNewline",
	tokExitWith:    "tokExitWith",
	tokComma:       "tokComma",
	tokLessThan:    "tokLessThan",
	tokGreaterThan: "tokGreaterThan",
	tokDo:          "tokDo",
	tokIf:          "tokIf",
	tokEndIf:       "tokEndIf",
	tokEndOfFile:   "tokEndOfFile",
	tokProc:        "tokProc",
	tokEndProc:     "tokEndProc",
	tokOpenParen:   "tokOpenParen",
	tokCloseParen:  "tokCloseParen",
	tokVariadic:    "tokVariadic",
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

func (cs *ContentState) handleSingleLineComments() (result bool) {
	result = false
	if cs.currentChar() == '/' && cs.peakChar(1) == '/' {
		result = true
		for cs.currentChar() != '\n' {
			cs.cursor++
		}
	}
	return
}

func (cs *ContentState) handleMultiLineComments() (result bool) {
	result = false
	if cs.currentChar() == '/' && cs.peakChar(1) == '*' && cs.peakChar(2) == '*' {
		result = true
		nestedComment := 0
		cs.cursor += 3
		for {
			if cs.currentChar() == '*' && cs.peakChar(1) == '*' && cs.peakChar(2) == '/' && nestedComment > 0 {
				nestedComment--
				cs.cursor += 3
				continue
			}

			if cs.currentChar() == '*' && cs.peakChar(1) == '*' && cs.peakChar(2) == '/' {
				cs.cursor += 3
				break
			}

			cs.cursor++
			if cs.currentChar() == '/' && cs.peakChar(1) == '*' && cs.peakChar(2) == '*' {
				nestedComment++
				cs.cursor += 3
				continue
			}
		}
	}
	return
}

func (cs *ContentState) handleNewline(ts *TokenizerState) (result bool) {
	result = false
	if cs.currentChar() == '\n' {
		result = true
		token := Token{}
		cs.token.WriteString("newline")
		val := cs.token.String()
		token.value = val
		token.tokType = tokNewline
		token.line = cs.line
		token.column = cs.column
		token.tokKind = kindNone
		cs.line++
		cs.cursor++
		cs.column = 0
		ts.data = append(ts.data, token)
		cs.token.Reset()
	}
	return
}

func (cs *ContentState) handleEmptyCharacters() (result bool) {
	result = false
	if unicode.IsSpace(rune(cs.currentChar())) ||
		rune(cs.currentChar()) == rune("\x00"[0]) {
		result = true
		cs.column++
		cs.cursor++
	}
	return
}

func (cs *ContentState) handleSingleCharacters(ts *TokenizerState) (result bool) {
	result = false
	singleTokenMap := map[byte]Token{
		'=': {value: "=", tokType: tokAssignment},
		'+': {value: "+", tokType: tokPlus},
		'-': {value: "-", tokType: tokMinus},
		',': {value: ",", tokType: tokComma},
		'<': {value: "<", tokType: tokLessThan},
		'>': {value: ">", tokType: tokGreaterThan},
		'(': {value: "(", tokType: tokOpenParen},
		')': {value: ")", tokType: tokCloseParen},
	}
	token, ok := singleTokenMap[cs.currentChar()]
	if !ok {
		return
	}
	result = true
	token.line = cs.line
	token.column = cs.column
	token.tokKind = kindNone
	cs.column++
	cs.cursor++
	ts.data = append(ts.data, token)
	return
}

func (cs *ContentState) handleNumbers(ts *TokenizerState) (result bool) {
	result = false
	if isNum(cs.currentChar()) {
		result = true
		token := Token{}
		token.line = cs.line
		token.column = cs.column
		token.tokKind = kindNone
		for isNum(cs.currentChar()) {
			cs.token.WriteByte(cs.currentChar())
			cs.column++
			cs.cursor++
		}
		token.value = cs.token.String()
		token.tokType = tokNumber
		ts.data = append(ts.data, token)
		cs.token.Reset()
	}
	return
}

func (cs *ContentState) handleVariadics(ts *TokenizerState) (result bool) {
	result = false
	if cs.currentChar() == '.' && cs.peakChar(1) == '.' && cs.peakChar(2) == '.' {
		result = true
		token := Token{}
		token.column = cs.column
		token.line = cs.line
		token.value = "..."
		token.tokType = tokVariadic
		token.tokKind = kindNone
		ts.data = append(ts.data, token)
		cs.column += 3
		cs.cursor += 3
	}
	return
}

func (cs *ContentState) handleVarTypes(ts *TokenizerState) (result bool) {
	result = false
	if isAlpha(cs.currentChar()) && isUppercase(cs.currentChar()) && cs.currentChar() != "«"[0] {
		result = true
		token := Token{line: cs.line, column: cs.column, tokType: tokVarType}
		for isAlphanum(cs.currentChar()) {
			cs.token.WriteByte(cs.currentChar())
			cs.column++
			cs.cursor++
		}
		value := cs.token.String()
		token.value = value
		varTypeMap := map[string]TokenKind{
			"AnyError": kindAnyError,
			"AnyType":  kindAnyType,
			"String":   kindString,
			"Char":     kindChar,
			"Bool":     kindBool,
			"U8":       kindU8,
			"U16":      kindU16,
			"U32":      kindU32,
			"U64":      kindU64,
			"U128":     kindU128,
			"I8":       kindI8,
			"I16":      kindI16,
			"I32":      kindI32,
			"I64":      kindI64,
			"I128":     kindI128,
			"F16":      kindF16,
			"F32":      kindF32,
			"F64":      kindF64,
			"F128":     kindF128,
			"C64":      kindC64,
			"C128":     kindC128,
			"Q128":     kindQ128,
			"Q256":     kindQ256,
			"Array":    kindArray,
			"Void":     kindVoid,
			"Map":      kindMap,
		}
		kind, ok := varTypeMap[value]
		if !ok {
			panic("This will eventually fail because of structs. Needs to be implemented")
		}
		token.tokKind = kind

		ts.data = append(ts.data, token)
		cs.token.Reset()
	}
	return
}

func (cs *ContentState) handleKeywordsAndIdentifiers(ts *TokenizerState) (result bool) {
	result = false
	if isAlpha(cs.currentChar()) && cs.currentChar() != "«"[0] {
		result = true
		token := Token{line: cs.line, column: cs.column, tokKind: kindNone}
		for isAlphanum(cs.currentChar()) {
			cs.token.WriteByte(cs.currentChar())
			cs.column++
			cs.cursor++
		}
		value := cs.token.String()
		keyworkMap := map[string]Token{
			"global":   {value: "global", tokType: tokGlobal},
			"let":      {value: "let", tokType: tokLet},
			"exitWith": {value: "exitWith", tokType: tokExitWith},
			"do":       {value: "do", tokType: tokDo},
			"if":       {value: "if", tokType: tokIf},
			"endif":    {value: "endif", tokType: tokEndIf},
			"proc":     {value: "proc", tokType: tokProc},
			"endproc":  {value: "endproc", tokType: tokEndProc},
		}
		keyword, ok := keyworkMap[value]
		if !ok {
			token.value = value
			token.tokType = tokIdentifier
		} else {
			token.value = keyword.value
			token.tokType = keyword.tokType
		}
		ts.data = append(ts.data, token)
		cs.token.Reset()
	}
	return
}

func (cs *ContentState) handleStringLiterals(ts *TokenizerState) (result bool) {
	result = false
	if cs.currentChar() == "«"[0] {
		result = true
		token := Token{}
		token.line = cs.line
		token.column = cs.column
		token.tokKind = kindNone
		cs.cursor += 2
		for cs.peakCharAsString(1) != "»" {
			cs.token.WriteString(cs.currentCharAsString())
			cs.cursor++
		}
		cs.cursor += 2
		token.tokType = tokString
		token.value = cs.token.String()
		ts.data = append(ts.data, token)
		cs.token.Reset()
	}
	return
}

// TODO: Improve how characters are handled. Probably move to a byte or uint32 type of char
// I want to avoid this kind of shit '"«"[0]', which is super annoying - also it will probaly
// make it easier to handle other things in the lexer
func tokenizeChaos(fileName string, fileContent *bytes.Buffer) *TokenizerState {
	cs := &ContentState{
		token:  bytes.Buffer{},
		data:   fileContent.String(),
		count:  fileContent.Len(),
		cursor: 0,
		line:   1,
		column: 1,
	}

	ts := &TokenizerState{
		fileName:  fileName,
		data:      []Token{},
		variables: map[string]string{},
		count:     0,
		cursor:    0,
	}

	for cs.cursor < cs.count {
		if cs.handleSingleLineComments() {
			continue
		}

		if cs.handleMultiLineComments() {
			continue
		}

		if cs.handleNewline(ts) {
			continue
		}

		if cs.handleEmptyCharacters() {
			continue
		}

		if cs.handleSingleCharacters(ts) {
			continue
		}

		if cs.handleNumbers(ts) {
			continue
		}

		if cs.handleVariadics(ts) {
			continue
		}

		if cs.handleVarTypes(ts) {
			continue
		}

		if cs.handleKeywordsAndIdentifiers(ts) {
			continue
		}

		if cs.handleStringLiterals(ts) {
			continue
		}

		fmt.Fprintf(os.Stderr, "[ERROR] Failed while tokenizing - unknown character '%s'\n", string(cs.currentChar()))
		panic("unrecheable")
	}

	eof := Token{value: "eof", tokType: tokEndOfFile, column: cs.column, line: cs.line}
	ts.data = append(ts.data, eof)
	ts.count = len(ts.data)
	return ts
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
		if tok.tokType == tokNewline {
			continue
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
	errorMsg     string
	examples     []string
	exampleCount int
}

func newLexerErroConfig(errorMsg string) LexerErrorConfig {
	return LexerErrorConfig{
		errorMsg:     errorMsg,
		examples:     []string{},
		exampleCount: 0,
	}
}

func (lec *LexerErrorConfig) newExample(examples ...string) {
	examplesLen := len(examples)
	assert(examplesLen > 0, "Expected at least one example")
	pad := "    "
	lec.exampleCount++
	prefix := fmt.Sprintf("(%d)", lec.exampleCount)
	for idx, example := range examples {
		if idx == 0 {
			tmp := fmt.Sprintf("%s%s %s\n", pad, prefix, example)
			lec.examples = append(lec.examples, tmp)
			continue
		}
		tmp := fmt.Sprintf("%s    %s\n", pad, example)
		lec.examples = append(lec.examples, tmp)
	}
}

func (lec LexerErrorConfig) printAndExitLexerError(ts *TokenizerState) {
	buffer := bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("%s:%d:%d [ERROR] ", ts.fileName, ts.current().line, ts.current().column))
	buffer.WriteString(lec.errorMsg)
	if len(lec.examples) > 0 {
		buffer.WriteString("Example:\n")
		for _, ex := range lec.examples {
			buffer.WriteString(ex)
		}
	}
	fmt.Print(buffer.String())
	os.Exit(1)
}
