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
	tokExit
	tokComma
	tokLessThan
	tokGreaterThan
	tokExecutes
	tokIf
	tokProc
	tokReturns
	tokExpects
	tokEnd
	tokOpenParen
	tokCloseParen
	tokVariadic
	tokCount
)

type TokenToStringMap map[TokenType]string

func (tokTyp TokenType) toString() string {
	if tokTyp == tokGlobal {
		return "tokGlobal"
	} else if tokTyp == tokLet {
		return "tokLet"
	} else if tokTyp == tokAssignment {
		return "tokAssignment"
	} else if tokTyp == tokPlus {
		return "tokPlus"
	} else if tokTyp == tokMinus {
		return "tokMinus"
	} else if tokTyp == tokVarType {
		return "tokVarType"
	} else if tokTyp == tokIdentifier {
		return "tokIdentifier"
	} else if tokTyp == tokNumber {
		return "tokNumber"
	} else if tokTyp == tokString {
		return "tokString"
	} else if tokTyp == tokNewline {
		return "tokNewline"
	} else if tokTyp == tokExit {
		return "tokExit"
	} else if tokTyp == tokComma {
		return "tokComma"
	} else if tokTyp == tokLessThan {
		return "tokLessThan"
	} else if tokTyp == tokGreaterThan {
		return "tokGreaterThan"
	} else if tokTyp == tokExecutes {
		return "tokExecutes"
	} else if tokTyp == tokIf {
		return "tokIf"
	} else if tokTyp == tokEndOfFile {
		return "tokEndOfFile"
	} else if tokTyp == tokProc {
		return "tokProc"
	} else if tokTyp == tokReturns {
		return "tokReturns"
	} else if tokTyp == tokExpects {
		return "tokExpects"
	} else if tokTyp == tokEnd {
		return "tokEnd"
	} else if tokTyp == tokOpenParen {
		return "tokOpenParen"
	} else if tokTyp == tokCloseParen {
		return "tokCloseParen"
	} else if tokTyp == tokVariadic {
		return "tokVariadic"
	}
	panic(fmt.Sprintf("Unknown keyword '%d'", tokTyp))
}

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
	tokExit:        "tokExit",
	tokComma:       "tokComma",
	tokLessThan:    "tokLessThan",
	tokGreaterThan: "tokGreaterThan",
	tokExecutes:    "tokExecutes",
	tokIf:          "tokIf",
	tokEndOfFile:   "tokEndOfFile",
	tokProc:        "tokProc",
	tokReturns:     "tokReturns",
	tokExpects:     "tokExpects",
	tokEnd:         "tokEnd",
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

func (cs *ContentState) matchStrAt(offset int, str string) bool {
	if cs.cursor+offset > cs.count {
		panic("Out of bounds while matchStrAt")
	}
	length := len(str)
	result := true
	for i := 0; i < length; i++ {
		result = result && (cs.peakChar(i+offset) == str[i])
	}
	return result
}

func (cs *ContentState) handleSingleLineComments() bool {
	if !cs.matchStrAt(0, "//") {
		return false
	}
	for cs.currentChar() != '\n' {
		cs.cursor++
	}
	return true
}

func (cs *ContentState) handleMultiLineComments() bool {
	if !cs.matchStrAt(0, "/**") {
		return false
	}
	nestedComment := 0
	cs.cursor += 3
	for true {
		if cs.matchStrAt(0, "**/") && nestedComment > 0 {
			nestedComment--
			cs.cursor += 3
			continue
		}
		if cs.matchStrAt(0, "**/") {
			cs.cursor += 3
			break
		}
		cs.cursor++
		if cs.matchStrAt(0, "/**") {
			nestedComment++
			cs.cursor += 3
			continue
		}
	}
	return true
}

func (cs *ContentState) handleNewline() bool {
	if cs.currentChar() != '\n' {
		return false
	}

	// token := Token{
	// 	value:   "newline",
	// 	tokType: tokNewline,
	// 	line:    cs.line,
	// 	column:  cs.column,
	// 	tokKind: kindNone,
	// }

	cs.line++
	cs.cursor++
	cs.column = 0
	return true
}

func (cs *ContentState) handleEmptyCharacters() bool {
	if !unicode.IsSpace(rune(cs.currentChar())) && rune(cs.currentChar()) != rune("\x00"[0]) {
		return false
	}

	cs.column++
	cs.cursor++
	return true
}

func (cs *ContentState) handleSingleCharacters() (bool, *Token) {
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
		return false, nil
	}

	token.line = cs.line
	token.column = cs.column
	token.tokKind = kindNone
	cs.column++
	cs.cursor++
	return true, &token
}

func (cs *ContentState) handleNumbers() (bool, *Token) {
	if !isNum(cs.currentChar()) {
		return false, nil
	}

	token := Token{line: cs.line, column: cs.column, tokKind: kindNone}
	tmp := bytes.Buffer{}
	defer tmp.Reset()

	for isNum(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	token.value = tmp.String()
	token.tokType = tokNumber
	return true, &token
}

func (cs *ContentState) handleVariadics() (bool, *Token) {
	if !cs.matchStrAt(0, "...") {
		return false, nil
	}

	token := Token{
		value:   "...",
		column:  cs.column,
		line:    cs.line,
		tokType: tokVariadic,
		tokKind: kindNone,
	}

	cs.column += 3
	cs.cursor += 3
	return true, &token
}

func (cs *ContentState) handleVarTypes() (bool, *Token) {
	if !isAlpha(cs.currentChar()) || !isUppercase(cs.currentChar()) || cs.currentChar() == "«"[0] {
		return false, nil
	}

	token := Token{line: cs.line, column: cs.column, tokType: tokVarType}
	tmp := bytes.Buffer{}
	defer tmp.Reset()

	for isAlphanum(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	value := tmp.String()
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

	if kind, ok := varTypeMap[value]; ok {
		token.tokKind = kind
		return true, &token
	} else {
		panic("Unexpected type - probably a struct?")
	}
}

func (cs *ContentState) handleKeywordsAndIdentifiers() (bool, *Token) {
	if !isAlpha(cs.currentChar()) || cs.currentChar() == "«"[0] {
		return false, nil
	}
	token := Token{line: cs.line, column: cs.column, tokKind: kindNone}
	tmp := bytes.Buffer{}
	defer tmp.Reset()

	for isAlphanum(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	value := tmp.String()
	keyworkMap := map[string]Token{
		"global":   {value: "global", tokType: tokGlobal},
		"let":      {value: "let", tokType: tokLet},
		"exit":     {value: "exit", tokType: tokExit},
		"executes": {value: "executes", tokType: tokExecutes},
		"if":       {value: "if", tokType: tokIf},
		"proc":     {value: "proc", tokType: tokProc},
		"end":      {value: "end", tokType: tokEnd},
		"returns":  {value: "returns", tokType: tokReturns},
		"expects":  {value: "expects", tokType: tokExpects},
	}

	keyword, ok := keyworkMap[value]
	if !ok {
		token.value = value
		token.tokType = tokIdentifier
	} else {
		token.value = keyword.value
		token.tokType = keyword.tokType
	}

	return true, &token
}

func (cs *ContentState) handleStringLiterals() (bool, *Token) {
	if cs.currentChar() != "«"[0] {
		return false, nil
	}

	token := Token{line: cs.line, column: cs.column, tokKind: kindNone}
	tmp := bytes.Buffer{}
	defer tmp.Reset()
	cs.cursor += 2

	for cs.peakCharAsString(1) != "»" {
		tmp.WriteString(cs.currentCharAsString())
		cs.cursor++
	}

	cs.cursor += 2
	token.tokType = tokString
	token.value = tmp.String()
	return true, &token
}

// TODO: Improve how characters are handled. Probably move to a byte or uint32 type of char
// I want to avoid this kind of shit '"«"[0]', which is super annoying - also it will probaly
// make it easier to handle other things in the lexer
func tokenizeChaos(fileName string, fileContent *bytes.Buffer) *TokenizerState {
	cs := &ContentState{
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

		if cs.handleNewline() {
			continue
		}

		if cs.handleEmptyCharacters() {
			continue
		}

		if ok, token := cs.handleSingleCharacters(); ok {
			ts.data = append(ts.data, *token)
			continue
		}

		if ok, token := cs.handleNumbers(); ok {
			ts.data = append(ts.data, *token)
			continue
		}

		if ok, token := cs.handleVariadics(); ok {
			ts.data = append(ts.data, *token)
			continue
		}

		if ok, token := cs.handleVarTypes(); ok {
			ts.data = append(ts.data, *token)
			continue
		}

		if ok, token := cs.handleKeywordsAndIdentifiers(); ok {
			ts.data = append(ts.data, *token)
			continue
		}

		if ok, token := cs.handleStringLiterals(); ok {
			ts.data = append(ts.data, *token)
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
	assert(
		ts.cursor < ts.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", ts.cursor, ts.count),
	)
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

func (ts *TokenizerState) consumeEndBlock(tokType TokenType) {
	if !ts.matchAt(0, tokEnd) {
		erroMsg := fmt.Sprintf("Expected 'end %s' but found 'end %s'", tokType.asString(), ts.current().tokType.asString())
		panic(erroMsg)
	}
	ts.consume(2)
}

