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
	tokDef
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
	tokEndProc
	tokReturns
	tokExpects
	tokOpenParen
	tokCloseParen
	tokEllipsis
	tokSemicolon
	tokColon
	tokRun
	tokWith
	tokLiteral
	tokInferAssign
	tokAs
	tokCount
)

func (tokTyp TokenType) asString() string {
	if tokTyp == tokInferType {
		return "tokInferType"
	} else if tokTyp == tokGlobal {
		return "tokGlobal"
	} else if tokTyp == tokDef {
		return "tokDef"
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
	} else if tokTyp == tokEndProc {
		return "tokEndProc"
	} else if tokTyp == tokReturns {
		return "tokReturns"
	} else if tokTyp == tokExpects {
		return "tokExpects"
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
	} else if tokTyp == tokAs {
		return "tokAs"
	}
	return "unknown tokType"
	// panic(fmt.Sprintf("Unknown keyword '%v'", tokTyp))
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
		return "Any-Error"
	} else if t == kindAnyType {
		return "Any-Type"
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
	} else if val, ok := cast[float64](curType); ok {
		return fmt.Sprintf("%03d:%03d [%s] %f", t.position.line, t.position.column, t.tokType.asString(), val)
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

func (cs *ContentState) peakChar(offset int) (result byte) {
	result = byte(0)
	if cs.cursor+offset < cs.count {
		result = cs.data[cs.cursor+offset]
	}
	return
}

func (cs *ContentState) matchByteAt(offset int, b byte) bool {
	if cs.cursor+offset > cs.count {
		panic("Out of bounds while matchByteAt")
	}
	return cs.peakChar(offset) == b

}

func (cs *ContentState) matchStrAt(offset int, str string) bool {
	if cs.cursor+offset > cs.count {
		panic("Out of bounds while matchStrAt")
	}
	length := len(str)
	result := true
	for i := range length {
		result = result && (cs.peakChar(i+offset) == str[i])
	}
	return result
}

func tokenizeChaos(fileName string, fileContent *bytes.Buffer) []Token {
	cs := &ContentState{
		data:   fileContent.String(),
		count:  fileContent.Len(),
		cursor: 0,
		line:   1,
		column: 1,
	}

	tokens := []Token{}
	for cs.cursor < cs.count {
		// Singleline comment
		if cs.matchStrAt(0, "//") {
			for !cs.matchByteAt(0, '\n') {
				cs.cursor++
			}
			continue
		}

		// Multiline comment
		if cs.matchStrAt(0, "/**") {
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
			continue
		}

		// Newline
		if cs.matchByteAt(0, '\n') {
			cs.line++
			cs.cursor++
			cs.column = 0
			continue
		}

		// Empty characters
		if unicode.IsSpace(rune(cs.currentChar())) {
			cs.column++
			cs.cursor++
			continue
		}

		// Strings
		if cs.matchStrAt(0, "«") {

			tmp := bytes.Buffer{}
			defer tmp.Reset()
			cs.cursor += 2

			// TODO: Handle nested '«»'
			// TODO: Handle multable strings «hello {some-printable-variable}»
			for !cs.matchStrAt(0, "»") {
				tmp.WriteString(cs.currentCharAsString())
				cs.cursor++
			}

			token := &TokenLiteral{
				value:   tmp.String(),
				tokType: tokLiteral,
				tokKind: kindString,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}

			cs.cursor += 2
			tokens = append(tokens, token)
			continue
		}

		if cs.matchStrAt(0, "==") {
			tok := &TokenOperator{
				tokType: tokEquals,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column += 2
			cs.cursor += 2
			tokens = append(tokens, tok)
			continue
		}

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
			tokens = append(tokens, tok)
			continue
		}

		if cs.matchByteAt(0, '=') {
			token := &TokenOperator{
				tokType: tokAssignment,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchByteAt(0, '+') {
			token := &TokenOperator{
				tokType: tokPlus,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchByteAt(0, '-') {
			token := &TokenOperator{
				tokType: tokMinus,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchByteAt(0, ',') {
			token := &TokenOperator{
				tokType: tokComma,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchByteAt(0, '<') {
			token := &TokenOperator{
				tokType: tokLessThan,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchByteAt(0, '>') {
			token := &TokenOperator{
				tokType: tokGreaterThan,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchByteAt(0, ';') {
			token := &TokenOperator{
				tokType: tokSemicolon,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchByteAt(0, ':') {
			token := &TokenOperator{
				tokType: tokColon,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if cs.matchStrAt(0, "(") {
			tok := &TokenKeyword{
				tokType: tokOpenParen,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if cs.matchStrAt(0, ")") {
			tok := &TokenKeyword{
				tokType: tokCloseParen,
				position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, tok)
			continue
		}

		// Number literals (float or int)
		if isNum(cs.currentChar()) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			isFloat := false
			for isNum(cs.currentChar()) || cs.matchByteAt(0, '.') || cs.matchByteAt(0, '_') {
				if cs.matchByteAt(0, '.') {
					isFloat = true
				}
				if !cs.matchByteAt(0, '_') {
					tmp.WriteByte(cs.currentChar())
				}
				cs.column++
				cs.cursor++
			}

			number := tmp.String()
			if isFloat {
				val, err := strconv.ParseFloat(number, 64)
				assert(err == nil, "Failed to convert string to float")

				token := &TokenLiteral{
					value:   val,
					tokType: tokLiteral,
					tokKind: kindF64,
					position: Position{
						line:   cs.line,
						column: cs.column,
					},
				}
				tokens = append(tokens, token)
				continue
			} else {
				val, err := strconv.Atoi(tmp.String())
				assert(err == nil, "Failed to convert string to number")

				token := &TokenLiteral{
					value:   val,
					tokType: tokLiteral,
					tokKind: kindI64,
					position: Position{
						line:   cs.line,
						column: cs.column,
					},
				}
				tokens = append(tokens, token)
				continue
			}
		}

		if isAlpha(cs.currentChar()) {
			// saveCursorPos := cs.cursor
			// saveColumPos := cs.column
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			if isUppercase(cs.currentChar()) {
				for isAlphanum(cs.currentChar()) || cs.matchByteAt(0, '-') {
					tmp.WriteByte(cs.currentChar())
					cs.column++
					cs.cursor++
				}

				varString := tmp.String()

				if varString == "Any-Error" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindAnyError,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Any-Type" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindAnyType,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "String" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindString,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Char" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindChar,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Bool" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindBool,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U8" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindU8,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U16" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindU16,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U32" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindU32,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U64" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindU64,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U128" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindU128,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I8" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindI8,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I16" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindI16,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I32" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindI32,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I64" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindI64,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I128" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindI128,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F16" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindF16,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F32" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindF32,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F64" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindF64,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F128" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindF128,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "C64" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindC64,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "C128" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindC128,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Q128" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindQ128,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Q256" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindQ256,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Array" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindArray,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Void" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindVoid,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Map" {
					token := &TokenVarType{
						symbol:  Symbol(varString),
						tokType: tokVarType,
						tokKind: kindMap,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}
			} else {
				for isAlphanum(cs.currentChar()) || cs.matchByteAt(0, '-') {
					tmp.WriteByte(cs.currentChar())
					cs.column++
					cs.cursor++
				}

				string := tmp.String()

				// Keywords
				if string == "true" {
					token := &TokenLiteral{
						value:   true,
						tokType: tokLiteral,
						tokKind: kindBool,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "false" {
					token := &TokenLiteral{
						value:   false,
						tokType: tokLiteral,
						tokKind: kindBool,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "global" {
					token := &TokenKeyword{
						tokType: tokGlobal,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "def" {
					token := &TokenKeyword{
						tokType: tokDef,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "exit" {
					token := &TokenKeyword{
						tokType: tokExit,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "executes" {
					token := &TokenKeyword{
						tokType: tokExecutes,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "if" {
					token := &TokenKeyword{
						tokType: tokIf,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "elif" {
					token := &TokenKeyword{
						tokType: tokElif,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "else" {
					token := &TokenKeyword{
						tokType: tokElse,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "proc" {
					token := &TokenKeyword{
						tokType: tokProc,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "end-proc" {
					token := &TokenKeyword{
						tokType: tokEndProc,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "returns" {
					token := &TokenKeyword{
						tokType: tokReturns,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "expects" {
					token := &TokenKeyword{
						tokType: tokExpects,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "run" {
					token := &TokenKeyword{
						tokType: tokRun,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "with" {
					token := &TokenKeyword{
						tokType: tokWith,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "as" {
					token := &TokenKeyword{
						tokType: tokAs,
						position: Position{
							line:   cs.line,
							column: cs.column,
						},
					}
					tokens = append(tokens, token)
					continue
				}

				// Identifiers
				token := &TokenIdentifier{
					symbol:  Symbol(string),
					tokType: tokIdentifier,
					position: Position{
						line:   cs.line,
						column: cs.column,
					},
				}
				tokens = append(tokens, token)
				continue
			}
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

	tokens = append(tokens, eof)
	return tokens
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
