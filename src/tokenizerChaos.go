package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"unicode"
)

type Symbol string

type TokenType int

const (
	tokInferType           = -1
	tokGlobal    TokenType = iota
	tokLet
	tokAssignment
	tokPlus
	tokMinus
	tokVarType
	tokIdentifier
	tokNewline
	tokEndOfFile
	tokExit
	tokComma
	tokLessThan
	tokGreaterThan
	tokEquals
	tokExecutes
	tokIf
	tokElif
	tokElse
	tokProc
	tokReturns
	tokExpects
	tokEnd
	tokOpenParen
	tokCloseParen
	tokEllipsis
	tokSemicolon
	tokColon
	tokRun
	tokWith
	tokLiteral
	tokInferAssign
	tokCount
)

func (tokTyp TokenType) asString() string {
	if tokTyp == tokInferType {
		return "tokInferType"
	} else if tokTyp == tokGlobal {
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
	} else if tokTyp == tokEllipsis {
		return "tokEllipsis"
	} else if tokTyp == tokEquals {
		return "tokEquals"
	} else if tokTyp == tokElif {
		return "tokElif"
	} else if tokTyp == tokElse {
		return "tokElse"
	} else if tokTyp == tokSemicolon {
		return "tokSemicolon"
	} else if tokTyp == tokColon {
		return "tokColon"
	} else if tokTyp == tokRun {
		return "tokRun"
	} else if tokTyp == tokWith {
		return "tokWith"
	} else if tokTyp == tokLiteral {
		return "tokLiteral"
	} else if tokTyp == tokInferAssign {
		return "tokInferAssign"
	}

	panic(fmt.Sprintf("Unknown keyword '%v'", tokTyp))
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

func (t TokenKind) asString() string {
	if t == kindAnyError {
		return "AnyError"
	} else if t == kindAnyType {
		return "AnyType"
	} else if t == kindString {
		return "String"
	} else if t == kindChar {
		return "Char"
	} else if t == kindBool {
		return "Bool"
	} else if t == kindU8 {
		return "U8"
	} else if t == kindU16 {
		return "U16"
	} else if t == kindU32 {
		return "U32"
	} else if t == kindU64 {
		return "U64"
	} else if t == kindU128 {
		return "U128"
	} else if t == kindI8 {
		return "I8"
	} else if t == kindI16 {
		return "I16"
	} else if t == kindI32 {
		return "I32"
	} else if t == kindI64 {
		return "I64"
	} else if t == kindI128 {
		return "I128"
	} else if t == kindF16 {
		return "F16"
	} else if t == kindF32 {
		return "F32"
	} else if t == kindF64 {
		return "F64"
	} else if t == kindF128 {
		return "F128"
	} else if t == kindC64 {
		return "C64"
	} else if t == kindC128 {
		return "C128"
	} else if t == kindQ128 {
		return "Q128"
	} else if t == kindQ256 {
		return "Q256"
	} else if t == kindArray {
		return "Array"
	} else if t == kindVoid {
		return "Void"
	} else if t == kindMap {
		return "Map"
	} else if t == kindNone {
		return "None"
	}

	panic(fmt.Sprintf("Unknown keyword '%v'", t))
}

type PtrAnyToken any

type Token interface {
	asString() string
	getTokenType() TokenType
	getPosition() Position
}

type Position struct {
	line   int
	column int
}

type TokenKeyword struct {
	tokType  TokenType
	position Position
}

func (t *TokenKeyword) asString() string {
	return fmt.Sprintf("%03d:%03d [%s]", t.position.line, t.position.column, t.tokType.asString())
}

func (t *TokenKeyword) getTokenType() TokenType {
	return t.tokType
}

func (t *TokenKeyword) getPosition() Position {
	return t.position
}

type TokenVarType struct {
	symbol   Symbol
	variadic bool
	tokType  TokenType
	tokKind  TokenKind
	position Position
}

func (t *TokenVarType) asString() string {
	return fmt.Sprintf("%03d:%03d [%s] %s", t.position.line, t.position.column, t.tokType.asString(), t.symbol)
}

func (t *TokenVarType) getTokenType() TokenType {
	return t.tokType
}

func (t *TokenVarType) getPosition() Position {
	return t.position
}

type TokenOperator struct {
	tokType  TokenType
	position Position
}

func (t *TokenOperator) asString() string {
	return fmt.Sprintf("%03d:%03d [%s]", t.position.line, t.position.column, t.tokType.asString())
}

func (t *TokenOperator) getTokenType() TokenType {
	return t.tokType
}

func (t *TokenOperator) getPosition() Position {
	return t.position
}

type TokenLiteral struct {
	value    PtrAnyToken
	tokType  TokenType
	tokKind  TokenKind
	position Position
}

func (t *TokenLiteral) asString() string {
	curType := any(t.value)
	if val, ok := cast[string](curType); ok {
		if val != "" {
			return fmt.Sprintf("%03d:%03d [%s] \"%s\"", t.position.line, t.position.column, t.tokType.asString(), val)
		}
		return fmt.Sprintf("%03d:%03d [%s]", t.position.line, t.position.column, t.tokType.asString())
	} else if val, ok := cast[int](curType); ok {
		return fmt.Sprintf("%03d:%03d [%s] %d", t.position.line, t.position.column, t.tokType.asString(), val)
	} else if val, ok := cast[bool](curType); ok {
		return fmt.Sprintf("%03d:%03d [%s] %t", t.position.line, t.position.column, t.tokType.asString(), val)
	}
	panic(fmt.Sprintf("Unexpected token literal found: %v", t.value))
}

func (t *TokenLiteral) getTokenType() TokenType {
	return t.tokType
}

func (t *TokenLiteral) getPosition() Position {
	return t.position
}

type TokenIdentifier struct {
	symbol   Symbol
	tokType  TokenType
	position Position
}

func (t *TokenIdentifier) asString() string {
	return fmt.Sprintf("%03d:%03d [%s] %s", t.position.line, t.position.column, t.tokType.asString(), t.symbol)
}

func (t *TokenIdentifier) getTokenType() TokenType {
	return t.tokType
}

func (t *TokenIdentifier) getPosition() Position {
	return t.position
}

var _ Token = &TokenKeyword{}
var _ Token = &TokenVarType{}
var _ Token = &TokenOperator{}
var _ Token = &TokenLiteral{}
var _ Token = &TokenIdentifier{}

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

func (cs *ContentState) handleSingleCharacters() (bool, *TokenOperator) {
	// TODO: Technically speaking, this is not only operators, but all single character tokens
	// that are allowed in the language. Probably need to rename it, or handle it differently.
	tokenOperator := TokenOperator{}
	curByte := cs.currentChar()
	if curByte == '=' {
		tokenOperator.tokType = tokAssignment
	} else if curByte == '+' {
		tokenOperator.tokType = tokPlus
	} else if curByte == '-' {
		tokenOperator.tokType = tokMinus
	} else if curByte == ',' {
		tokenOperator.tokType = tokComma
	} else if curByte == '<' {
		tokenOperator.tokType = tokLessThan
	} else if curByte == '>' {
		tokenOperator.tokType = tokGreaterThan
	} else if curByte == ';' {
		tokenOperator.tokType = tokSemicolon
	} else if curByte == ':' {
		tokenOperator.tokType = tokColon
	} else {
		return false, nil
	}
	tokenOperator.position.line = cs.line
	tokenOperator.position.column = cs.column
	cs.column++
	cs.cursor++
	return true, &tokenOperator
}

func (cs *ContentState) handleNumberLiterals() (bool, *TokenLiteral) {
	if !isNum(cs.currentChar()) {
		return false, nil
	}

	tmp := bytes.Buffer{}
	defer tmp.Reset()

	for isNum(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	val, err := strconv.Atoi(tmp.String())
	assert(err == nil, "Failed to convert string to number")

	token := TokenLiteral{
		value:   val,
		tokType: tokLiteral,
		tokKind: kindI64,
		position: Position{
			line:   cs.line,
			column: cs.column,
		},
	}

	return true, &token
}

func (cs *ContentState) handleVarTypes() (bool, *TokenVarType) {
	if !isAlpha(cs.currentChar()) || !isUppercase(cs.currentChar()) || cs.currentChar() == "«"[0] {
		return false, nil
	}

	tmp := bytes.Buffer{}
	defer tmp.Reset()
	for isAlphanum(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	sym := Symbol(tmp.String())
	token := TokenVarType{
		symbol:  sym,
		tokType: tokVarType,
		position: Position{
			cs.line,
			cs.column,
		},
	}

	if sym == "AnyError" {
		token.tokKind = kindAnyError
	} else if sym == "AnyType" {
		token.tokKind = kindAnyType
	} else if sym == "String" {
		token.tokKind = kindString
	} else if sym == "Char" {
		token.tokKind = kindChar
	} else if sym == "Bool" {
		token.tokKind = kindBool
	} else if sym == "U8" {
		token.tokKind = kindU8
	} else if sym == "U16" {
		token.tokKind = kindU16
	} else if sym == "U32" {
		token.tokKind = kindU32
	} else if sym == "U64" {
		token.tokKind = kindU64
	} else if sym == "U128" {
		token.tokKind = kindU128
	} else if sym == "I8" {
		token.tokKind = kindI8
	} else if sym == "I16" {
		token.tokKind = kindI16
	} else if sym == "I32" {
		token.tokKind = kindI32
	} else if sym == "I64" {
		token.tokKind = kindI64
	} else if sym == "I128" {
		token.tokKind = kindI128
	} else if sym == "F16" {
		token.tokKind = kindF16
	} else if sym == "F32" {
		token.tokKind = kindF32
	} else if sym == "F64" {
		token.tokKind = kindF64
	} else if sym == "F128" {
		token.tokKind = kindF128
	} else if sym == "C64" {
		token.tokKind = kindC64
	} else if sym == "C128" {
		token.tokKind = kindC128
	} else if sym == "Q128" {
		token.tokKind = kindQ128
	} else if sym == "Q256" {
		token.tokKind = kindQ256
	} else if sym == "Array" {
		token.tokKind = kindArray
	} else if sym == "Void" {
		token.tokKind = kindVoid
	} else if sym == "Map" {
		token.tokKind = kindMap
	} else {
		return false, nil
	}
	return true, &token
}

func (cs *ContentState) handleBoolean() (bool, *TokenLiteral) {
	if !isAlpha(cs.currentChar()) || cs.currentChar() == "«"[0] {
		return false, nil
	}

	token := TokenLiteral{}
	saveCursorPos := cs.cursor
	saveColumPos := cs.column
	tmp := bytes.Buffer{}
	defer tmp.Reset()

	for isAlpha(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	boolean := tmp.String()
	if boolean == "true" {
		token.value = true
	} else if boolean == "false" {
		token.value = false
	} else {
		cs.cursor = saveCursorPos
		cs.column = saveColumPos
		return false, nil
	}

	token.tokType = tokLiteral
	token.position = Position{cs.line, cs.column}
	return true, &token
}

func (cs *ContentState) handleKeywords() (bool, *TokenKeyword) {
	if !isAlpha(cs.currentChar()) || cs.currentChar() == "«"[0] {
		return false, nil
	}

	token := TokenKeyword{}
	saveCursorPos := cs.cursor
	saveColumPos := cs.column
	tmp := bytes.Buffer{}
	defer tmp.Reset()
	for isAlphanum(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	keyword := tmp.String()
	if keyword == "global" {
		token.tokType = tokGlobal
	} else if keyword == "let" {
		token.tokType = tokLet
	} else if keyword == "exit" {
		token.tokType = tokExit
	} else if keyword == "executes" {
		token.tokType = tokExecutes
	} else if keyword == "if" {
		token.tokType = tokIf
	} else if keyword == "elif" {
		token.tokType = tokElif
	} else if keyword == "else" {
		token.tokType = tokElse
	} else if keyword == "proc" {
		token.tokType = tokProc
	} else if keyword == "end" {
		token.tokType = tokEnd
	} else if keyword == "returns" {
		token.tokType = tokReturns
	} else if keyword == "expects" {
		token.tokType = tokExpects
	} else if keyword == "run" {
		token.tokType = tokRun
	} else if keyword == "with" {
		token.tokType = tokWith
	} else {
		cs.cursor = saveCursorPos
		cs.column = saveColumPos
		return false, nil
	}

	token.position = Position{cs.line, cs.column}
	return true, &token
}

func (cs *ContentState) handleIdentifiers() (bool, *TokenIdentifier) {
	if !isAlpha(cs.currentChar()) || cs.currentChar() == "«"[0] {
		return false, nil
	}

	tmp := bytes.Buffer{}
	defer tmp.Reset()

	for isAlphanum(cs.currentChar()) {
		tmp.WriteByte(cs.currentChar())
		cs.column++
		cs.cursor++
	}

	token := TokenIdentifier{
		symbol:  Symbol(tmp.String()),
		tokType: tokIdentifier,
		position: Position{
			line:   cs.line,
			column: cs.column,
		},
	}

	return true, &token
}

func (cs *ContentState) handleStringLiterals() (bool, *TokenLiteral) {
	if cs.currentChar() != "«"[0] {
		return false, nil
	}

	tmp := bytes.Buffer{}
	defer tmp.Reset()
	cs.cursor += 2

	for cs.peakCharAsString(1) != "»" {
		tmp.WriteString(cs.currentCharAsString())
		cs.cursor++
	}

	token := TokenLiteral{
		value:   tmp.String(),
		tokType: tokLiteral,
		tokKind: kindString,
		position: Position{
			line:   cs.line,
			column: cs.column,
		},
	}

	cs.cursor += 2
	return true, &token
}

// TODO: Improve how characters are handled. Probably move to a byte or uint32 type of char
// I want to avoid this kind of shit '"«"[0]', which is super annoying - also it will probaly
// make it easier to handle other things in the lexer
// TODO: Handle how we use tokens. A lot of duplication using 'value' for all tokens, when
// in reality most of the tokens don't have values.
func tokenizeChaos(fileName string, fileContent *bytes.Buffer) *TokenizerState {
	cs := &ContentState{
		data:   fileContent.String(),
		count:  fileContent.Len(),
		cursor: 0,
		line:   1,
		column: 1,
	}

	ts := &TokenizerState{
		data:   []Token{},
		count:  0,
		cursor: 0,
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

		// TODO: Improve how to handle two character tokens
		if cs.matchStrAt(0, "==") {
			tok := &TokenOperator{
				tokType:  tokEquals,
				position: Position{cs.line, cs.column},
			}
			cs.column += 2
			cs.cursor += 2
			ts.data = append(ts.data, tok)
			continue
		}

		if cs.matchStrAt(0, ":=") {
			tok := &TokenKeyword{
				tokType: tokInferAssign,
				position: Position{
					line: cs.line,
					column: cs.column,
				},
			}
			cs.column += 2
			cs.cursor += 2
			ts.data = append(ts.data, tok)
			continue
		}

		// TODO: Improve how to handle three character tokens
		if cs.matchStrAt(0, "...") {
			tok := &TokenKeyword{
				tokType: tokEllipsis,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column += 3
			cs.cursor += 3
			ts.data = append(ts.data, tok)
			continue
		}

		if ok, token := cs.handleSingleCharacters(); ok {
			ts.data = append(ts.data, token)
			continue
		}

		if ok, token := cs.handleStringLiterals(); ok {
			ts.data = append(ts.data, token)
			continue
		}

		if ok, token := cs.handleNumberLiterals(); ok {
			ts.data = append(ts.data, token)
			continue
		}

		if ok, token := cs.handleBoolean(); ok {
			ts.data = append(ts.data, token)
			continue
		}

		if ok, token := cs.handleVarTypes(); ok {
			ts.data = append(ts.data, token)
			continue
		}

		if ok, token := cs.handleKeywords(); ok {
			ts.data = append(ts.data, token)
			continue
		}

		if ok, token := cs.handleIdentifiers(); ok {
			ts.data = append(ts.data, token)
			continue
		}

		fmt.Fprintf(os.Stderr, "[ERROR] Failed while tokenizing - unknown character '%s'\n", string(cs.currentChar()))
		panic("unrecheable")
	}

	eof := &TokenLiteral{
		value:   "eof",
		tokType: tokEndOfFile,
		position: Position{
			line:   cs.line,
			column: cs.column,
		},
	}
	ts.data = append(ts.data, eof)
	ts.count = len(ts.data)
	return ts
}

type TokenizerState struct {
	data   []Token
	count  int
	cursor int
}

func (tokens TokenizerState) print() {
	fmt.Print("Token List:\n")
	for _, tok := range tokens.data {
		if tok.getTokenType() == tokEndOfFile {
			return
		}
		if tok.getTokenType() == tokNewline {
			continue
		}
		fmt.Printf("%s\n", tok.asString())
	}
}

func (ts *TokenizerState) current() Token {
	assert(
		ts.cursor < ts.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", ts.cursor, ts.count),
	)
	return ts.data[ts.cursor]
}

func (ts *TokenizerState) currentTokenType() TokenType {
	return ts.current().getTokenType()
}

func (ts *TokenizerState) currentTokenTypeAsString() string {
	return ts.currentTokenType().asString()
}

func (ts *TokenizerState) matchAllAt(offset int, tokTypes []TokenType) (result bool) {
	result = false
	for i, tok := range tokTypes {
		pos := ts.cursor + i + offset
		if pos > ts.count {
			panic(fmt.Sprintf("out of bounds operation while checking for %v", tokTypes))
		}
		curTok := ts.data[pos]
		if curTok.getTokenType() == tok {
			result = true
		}
	}
	return result
}

func (tok *TokenKind) matchAt(offset int, tokKinds ...TokenKind) (result bool) {
	result = false
	cursor := 0
	length := len(tokKinds)
	for _, t := range tokKinds {
		if cursor+offset < length {
			if *tok == t {
				result = true
				break
			}
		}
	}
	return
}

func (ts *TokenizerState) matchAt(offset int, tokTypes ...TokenType) (result bool) {
	result = false
	for _, tok := range tokTypes {
		if ts.cursor+offset < ts.count {
			curTok := ts.data[ts.cursor+offset]
			if curTok.getTokenType() == tok {
				result = true
				break
			}
		}
	}
	return
}

func (ts *TokenizerState) peak(offset int) (result Token) {
	assert(offset != 0, "cannot peak with 0")
	result = nil
	if ts.cursor+offset < ts.count {
		result = ts.data[ts.cursor+offset]
	}
	return
}

func (ts *TokenizerState) consumeAssert(tokType TokenType) Token {
	assert(ts.currentTokenType() == tokType, fmt.Sprintf("Expected the token '%s' but found '%s'", tokType.asString(), ts.currentTokenTypeAsString()))
	result := ts.current()
	if ts.cursor < ts.count {
		ts.cursor++
	}
	return result
}

func (ts *TokenizerState) consume(count ...int) (result Token) {
	assert(len(count) <= 1, "Count can only be 1 or empty")
	c := 0
	if len(count) == 1 {
		c = count[0]
	}
	result = nil
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
	if !ts.matchAt(0, tokEnd) && !ts.matchAt(1, tokType) {
		erroMsg := fmt.Sprintf("Expected 'end %s' but found 'end %s'", tokType.asString(), ts.current().getTokenType().asString())
		panic(erroMsg)
	}
	ts.consume(2)
}
