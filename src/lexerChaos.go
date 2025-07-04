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
	tokAs
	tokCount
)

func (tokTyp TokenType) HumanReadableString() string {
	if tokTyp == tokInferType {
		return "infered-type"
	}
	if tokTyp == tokGlobal {
		return "global"
	}
	if tokTyp == tokDef {
		return "def"
	}
	if tokTyp == tokAssignment {
		return "="
	}
	if tokTyp == tokPlus {
		return "+"
	}
	if tokTyp == tokMinus {
		return "-"
	}
	if tokTyp == tokVarType {
		return "type"
	}
	if tokTyp == tokIdentifier {
		return "identifier"
	}
	if tokTyp == tokNewline {
		return "newline"
	}
	if tokTyp == tokExit {
		return "exit"
	}
	if tokTyp == tokComma {
		return ","
	}
	if tokTyp == tokLessThan {
		return "<"
	}
	if tokTyp == tokGreaterThan {
		return ">"
	}
	if tokTyp == tokExecutes {
		return "executes"
	}
	if tokTyp == tokIf {
		return "if"
	}
	if tokTyp == tokEndOfFile {
		return "eof"
	}
	if tokTyp == tokProc {
		return "proc"
	}
	if tokTyp == tokEndProc {
		return "end-proc"
	}
	if tokTyp == tokReturns {
		return "returns"
	}
	if tokTyp == tokExpects {
		return "expects"
	}
	if tokTyp == tokOpenParen {
		return "("
	}
	if tokTyp == tokCloseParen {
		return ")"
	}
	if tokTyp == tokEllipsis {
		return "..."
	}
	if tokTyp == tokEquals {
		return "=="
	}
	if tokTyp == tokElif {
		return "elif"
	}
	if tokTyp == tokElse {
		return "else"
	}
	if tokTyp == tokSemicolon {
		return ";"
	}
	if tokTyp == tokColon {
		return ":"
	}
	if tokTyp == tokRun {
		return "run"
	}
	if tokTyp == tokWith {
		return "with"
	}
	if tokTyp == tokLiteral {
		return "literal"
	}
	if tokTyp == tokAs {
		return "as"
	}
	return "unknown-token"
}

func (tokTyp TokenType) String() string {
	if tokTyp == tokInferType {
		return "tokInferType"
	}
	if tokTyp == tokGlobal {
		return "tokGlobal"
	}
	if tokTyp == tokDef {
		return "tokDef"
	}
	if tokTyp == tokAssignment {
		return "tokAssignment"
	}
	if tokTyp == tokPlus {
		return "tokPlus"
	}
	if tokTyp == tokMinus {
		return "tokMinus"
	}
	if tokTyp == tokVarType {
		return "tokVarType"
	}
	if tokTyp == tokIdentifier {
		return "tokIdentifier"
	}
	if tokTyp == tokNewline {
		return "tokNewline"
	}
	if tokTyp == tokExit {
		return "tokExit"
	}
	if tokTyp == tokComma {
		return "tokComma"
	}
	if tokTyp == tokLessThan {
		return "tokLessThan"
	}
	if tokTyp == tokGreaterThan {
		return "tokGreaterThan"
	}
	if tokTyp == tokExecutes {
		return "tokExecutes"
	}
	if tokTyp == tokIf {
		return "tokIf"
	}
	if tokTyp == tokEndOfFile {
		return "tokEndOfFile"
	}
	if tokTyp == tokProc {
		return "tokProc"
	}
	if tokTyp == tokEndProc {
		return "tokEndProc"
	}
	if tokTyp == tokReturns {
		return "tokReturns"
	}
	if tokTyp == tokExpects {
		return "tokExpects"
	}
	if tokTyp == tokOpenParen {
		return "tokOpenParen"
	}
	if tokTyp == tokCloseParen {
		return "tokCloseParen"
	}
	if tokTyp == tokEllipsis {
		return "tokEllipsis"
	}
	if tokTyp == tokEquals {
		return "tokEquals"
	}
	if tokTyp == tokElif {
		return "tokElif"
	}
	if tokTyp == tokElse {
		return "tokElse"
	}
	if tokTyp == tokSemicolon {
		return "tokSemicolon"
	}
	if tokTyp == tokColon {
		return "tokColon"
	}
	if tokTyp == tokRun {
		return "tokRun"
	}
	if tokTyp == tokWith {
		return "tokWith"
	}
	if tokTyp == tokLiteral {
		return "tokLiteral"
	}
	if tokTyp == tokAs {
		return "tokAs"
	}
	return "unknown-tokType"
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

func (t TokenKind) String() string {
	if t == kindAnyError {
		return "Any-Error"
	}
	if t == kindAnyType {
		return "Any-Type"
	}
	if t == kindString {
		return "String"
	}
	if t == kindChar {
		return "Char"
	}
	if t == kindBool {
		return "Bool"
	}
	if t == kindU8 {
		return "U8"
	}
	if t == kindU16 {
		return "U16"
	}
	if t == kindU32 {
		return "U32"
	}
	if t == kindU64 {
		return "U64"
	}
	if t == kindU128 {
		return "U128"
	}
	if t == kindI8 {
		return "I8"
	}
	if t == kindI16 {
		return "I16"
	}
	if t == kindI32 {
		return "I32"
	}
	if t == kindI64 {
		return "I64"
	}
	if t == kindI128 {
		return "I128"
	}
	if t == kindF16 {
		return "F16"
	}
	if t == kindF32 {
		return "F32"
	}
	if t == kindF64 {
		return "F64"
	}
	if t == kindF128 {
		return "F128"
	}
	if t == kindC64 {
		return "C64"
	}
	if t == kindC128 {
		return "C128"
	}
	if t == kindQ128 {
		return "Q128"
	}
	if t == kindQ256 {
		return "Q256"
	}
	if t == kindArray {
		return "Array"
	}
	if t == kindVoid {
		return "Void"
	}
	if t == kindMap {
		return "Map"
	}
	if t == kindNone {
		return "None"
	}
	return "unknown-kind"
}

type PtrAnyToken any

type Token interface {
	String() string
	TokenType() TokenType
	Position() Position
}

type Position struct {
	line   int
	column int
}

type TokenKeyword struct {
	tokType  TokenType
	position Position
}

func (t *TokenKeyword) String() string {
	preffix := fmt.Sprintf("%03d:%03d", t.position.line, t.position.column)
	return fmt.Sprintf("%s [%s]", preffix, t.tokType.String())
}

func (t *TokenKeyword) TokenType() TokenType {
	return t.tokType
}

func (t *TokenKeyword) Position() Position {
	return t.position
}

type TokenVarType struct {
	symbol   Symbol
	variadic bool
	tokType  TokenType
	tokKind  TokenKind
	position Position
}

func (t *TokenVarType) String() string {
	preffix := fmt.Sprintf("%03d:%03d", t.position.line, t.position.column)
	return fmt.Sprintf("%s [%s] %s", preffix, t.tokType.String(), t.symbol)
}

func (t *TokenVarType) TokenType() TokenType {
	return t.tokType
}

func (t *TokenVarType) Position() Position {
	return t.position
}

type TokenOperator struct {
	tokType  TokenType
	position Position
}

func (t *TokenOperator) String() string {
	preffix := fmt.Sprintf("%03d:%03d", t.position.line, t.position.column)
	return fmt.Sprintf("%s [%s]", preffix, t.tokType.String())
}

func (t *TokenOperator) TokenType() TokenType {
	return t.tokType
}

func (t *TokenOperator) Position() Position {
	return t.position
}

type TokenLiteral struct {
	value    PtrAnyToken
	tokType  TokenType
	tokKind  TokenKind
	position Position
}

func (t *TokenLiteral) String() string {
	preffix := fmt.Sprintf("%03d:%03d", t.position.line, t.position.column)

	curType := any(t.value)
	if val, ok := cast[string](curType); ok {
		if val != "" {
			return fmt.Sprintf("%s [%s] \"%s\"", preffix, t.tokType.String(), val)
		}
		return fmt.Sprintf("%s [%s]", preffix, t.tokType.String())
	}

	if val, ok := cast[int](curType); ok {
		return fmt.Sprintf("%s [%s] %d", preffix, t.tokType.String(), val)
	}

	if val, ok := cast[bool](curType); ok {
		return fmt.Sprintf("%s [%s] %t", preffix, t.tokType.String(), val)
	}

	if val, ok := cast[float64](curType); ok {
		return fmt.Sprintf("%s [%s] %f", preffix, t.tokType.String(), val)
	}

	panic(fmt.Sprintf("Unexpected token literal found: %v", t.value))
}

func (t *TokenLiteral) TokenType() TokenType {
	return t.tokType
}

func (t *TokenLiteral) Position() Position {
	return t.position
}

type TokenIdentifier struct {
	symbol   Symbol
	tokType  TokenType
	position Position
}

func (t *TokenIdentifier) String() string {
	preffix := fmt.Sprintf("%03d:%03d", t.position.line, t.position.column)
	return fmt.Sprintf("%s [%s] %s", preffix, t.tokType.String(), t.symbol)
}

func (t *TokenIdentifier) TokenType() TokenType {
	return t.tokType
}

func (t *TokenIdentifier) Position() Position {
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

func (cs *ContentState) CurrentString() string {
	return string(cs.data[cs.cursor])
}

func (cs *ContentState) MatchStrAt(offset int, str string) bool {
	if cs.cursor+offset > cs.count {
		panic("Out of bounds while matchStrAt")
	}
	length := len(str)
	result := true
	for i := range length {
		result = result && (cs.PeekByte(i+offset) == str[i])
	}
	return result
}

func (cs *ContentState) CurrentByte() byte {
	return cs.data[cs.cursor]
}

func (cs *ContentState) PeekByte(offset int) (result byte) {
	result = byte(0)
	if cs.cursor+offset < cs.count {
		result = cs.data[cs.cursor+offset]
	}
	return
}

func (cs *ContentState) MatchByteAt(offset int, b byte) bool {
	if cs.cursor+offset > cs.count {
		panic("Out of bounds while matchByteAt")
	}
	return cs.PeekByte(offset) == b

}

func TokenizeChaos(fileContent *bytes.Buffer) []Token {
	cs := &ContentState{
		data:   fileContent.String(),
		count:  fileContent.Len(),
		cursor: 0,
		line:   1,
		column: 1,
	}

	tokens := []Token{}
	for cs.cursor < cs.count {
		// Single-line comment
		if cs.MatchStrAt(0, "//") {
			for !cs.MatchByteAt(0, '\n') {
				cs.cursor++
			}
			continue
		}

		// Newline
		if cs.MatchByteAt(0, '\n') {
			cs.line++
			cs.cursor++
			cs.column = 0
			continue
		}

		// Multi-line comment
		if cs.MatchStrAt(0, "/**") {
			nestedComment := 0
			cs.cursor += 3
			for true {
				if cs.MatchByteAt(0, '\n') {
					cs.line++
					cs.cursor++
					cs.column = 0
					continue
				}
				if cs.MatchStrAt(0, "**/") && nestedComment > 0 {
					nestedComment--
					cs.cursor += 3
					cs.column += 3
					continue
				}
				if cs.MatchStrAt(0, "**/") {
					cs.cursor += 3
					cs.column += 3
					break
				}
				cs.cursor++
				if cs.MatchStrAt(0, "/**") {
					nestedComment++
					cs.cursor += 3
					cs.column += 3
					continue
				}
			}
			continue
		}

		// Empty characters
		if unicode.IsSpace(rune(cs.CurrentByte())) {
			cs.column++
			cs.cursor++
			continue
		}

		// Strings
		if cs.MatchStrAt(0, "«") {

			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   cs.line,
				column: cs.column,
			}
			cs.cursor += 2

			// TO-DO: Handle nested '«»'
			// TO-DO: Handle interpolated strings like «hello {some-printable-variable}»
			for !cs.MatchStrAt(0, "»") {
				tmp.WriteString(cs.CurrentString())
				cs.cursor++
			}

			token := &TokenLiteral{
				value:    tmp.String(),
				tokType:  tokLiteral,
				tokKind:  kindString,
				position: position,
			}

			cs.cursor += 2
			tokens = append(tokens, token)
			continue
		}

		if cs.MatchStrAt(0, "==") {
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

		if cs.MatchStrAt(0, "...") {
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

		if cs.MatchStrAt(0, "=") {
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

		if cs.MatchStrAt(0, "+") {
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

		if cs.MatchStrAt(0, "-") {
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

		if cs.MatchStrAt(0, ",") {
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

		if cs.MatchStrAt(0, "<") {
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

		if cs.MatchStrAt(0, ">") {
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

		if cs.MatchStrAt(0, ";") {
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

		if cs.MatchStrAt(0, ":") {
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

		if cs.MatchStrAt(0, "(") {
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

		if cs.MatchStrAt(0, ")") {
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
		if isNum(cs.CurrentByte()) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   cs.line,
				column: cs.column,
			}

			isFloat := false
			for isNum(cs.CurrentByte()) || cs.MatchByteAt(0, '.') || cs.MatchByteAt(0, '_') {
				if cs.MatchByteAt(0, '.') {
					isFloat = true
				}
				if !cs.MatchByteAt(0, '_') {
					tmp.WriteByte(cs.CurrentByte())
				}
				cs.column++
				cs.cursor++
			}

			number := tmp.String()
			if isFloat {
				val, err := strconv.ParseFloat(number, 64)
				assert(err == nil, "Failed to convert string to float")

				token := &TokenLiteral{
					value:    val,
					tokType:  tokLiteral,
					tokKind:  kindF64,
					position: position,
				}
				tokens = append(tokens, token)
				continue
			} else {
				val, err := strconv.Atoi(tmp.String())
				assert(err == nil, "Failed to convert string to number")

				token := &TokenLiteral{
					value:    val,
					tokType:  tokLiteral,
					tokKind:  kindI64,
					position: position,
				}
				tokens = append(tokens, token)
				continue
			}
		}

		if isAlpha(cs.CurrentByte()) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   cs.line,
				column: cs.column,
			}

			if isUppercase(cs.CurrentByte()) {

				for isAlphanum(cs.CurrentByte()) || cs.MatchByteAt(0, '-') {
					tmp.WriteByte(cs.CurrentByte())
					cs.column++
					cs.cursor++
				}

				varString := tmp.String()

				if varString == "Any-Error" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindAnyError,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Any-Type" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindAnyType,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "String" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindString,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Char" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindChar,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Bool" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindBool,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U8" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindU8,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U16" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindU16,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U32" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindU32,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U64" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindU64,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "U128" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindU128,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I8" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindI8,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I16" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindI16,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I32" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindI32,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I64" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindI64,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "I128" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindI128,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F16" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindF16,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F32" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindF32,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F64" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindF64,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "F128" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindF128,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "C64" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindC64,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "C128" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindC128,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Q128" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindQ128,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Q256" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindQ256,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Array" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindArray,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Void" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindVoid,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if varString == "Map" {
					token := &TokenVarType{
						symbol:   Symbol(varString),
						tokType:  tokVarType,
						tokKind:  kindMap,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}
			} else {
				for isAlphanum(cs.CurrentByte()) || cs.MatchByteAt(0, '-') {
					tmp.WriteByte(cs.CurrentByte())
					cs.column++
					cs.cursor++
				}

				string := tmp.String()

				// Keywords
				if string == "true" {
					token := &TokenLiteral{
						value:    true,
						tokType:  tokLiteral,
						tokKind:  kindBool,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "false" {
					token := &TokenLiteral{
						value:    false,
						tokType:  tokLiteral,
						tokKind:  kindBool,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "global" {
					token := &TokenKeyword{
						tokType:  tokGlobal,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "def" {
					token := &TokenKeyword{
						tokType:  tokDef,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "exit" {
					token := &TokenKeyword{
						tokType:  tokExit,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "executes" {
					token := &TokenKeyword{
						tokType:  tokExecutes,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "if" {
					token := &TokenKeyword{
						tokType:  tokIf,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "elif" {
					token := &TokenKeyword{
						tokType:  tokElif,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "else" {
					token := &TokenKeyword{
						tokType:  tokElse,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "proc" {
					token := &TokenKeyword{
						tokType:  tokProc,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "end-proc" {
					token := &TokenKeyword{
						tokType:  tokEndProc,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "returns" {
					token := &TokenKeyword{
						tokType:  tokReturns,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "expects" {
					token := &TokenKeyword{
						tokType:  tokExpects,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "run" {
					token := &TokenKeyword{
						tokType:  tokRun,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "with" {
					token := &TokenKeyword{
						tokType:  tokWith,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				if string == "as" {
					token := &TokenKeyword{
						tokType:  tokAs,
						position: position,
					}
					tokens = append(tokens, token)
					continue
				}

				// Identifiers
				token := &TokenIdentifier{
					symbol:   Symbol(string),
					tokType:  tokIdentifier,
					position: position,
				}
				tokens = append(tokens, token)
				continue
			}
		}

		fmt.Fprintf(
			os.Stderr,
			"[ERROR] Failed while tokenizing - unknown character '%s' at position %03d:%03d\n",
			cs.CurrentString(), cs.line, cs.column,
		)
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

type LexerState struct {
	data     []Token
	filepath string
	count    int
	cursor   int
}

func (tokens LexerState) print() {
	fmt.Print("Token List:\n")
	for _, tok := range tokens.data {
		if tok.TokenType() == tokEndOfFile {
			return
		}
		if tok.TokenType() == tokNewline {
			continue
		}
		fmt.Printf("%s\n", tok.String())
	}
}

func (ts *LexerState) Current() Token {
	assert(
		ts.cursor < ts.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", ts.cursor, ts.count),
	)
	return ts.data[ts.cursor]
}

func (ts *LexerState) currentTokenTypeAsString() string {
	return ts.Current().TokenType().String()
}

func (tok *TokenKind) MatchAt(offset int, tokKinds ...TokenKind) (result bool) {
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

func (ts *LexerState) MatchAt(offset int, tokTypes ...TokenType) (result bool) {
	result = false
	for _, tok := range tokTypes {
		if ts.cursor+offset < ts.count {
			curTok := ts.data[ts.cursor+offset]
			if curTok.TokenType() == tok {
				result = true
				break
			}
		}
	}
	return
}

func (ts *LexerState) Peek(offset int) (result Token) {
	assert(offset != 0, "cannot peak with 0")
	result = nil
	if ts.cursor+offset < ts.count {
		result = ts.data[ts.cursor+offset]
	}
	return
}

func (ts *LexerState) ConsumeAssert(tokType TokenType) Token {
	assert(
		ts.Current().TokenType() == tokType,
		fmt.Sprintf("Expected the token '%s' but found '%s'", tokType.String(), ts.currentTokenTypeAsString()),
	)
	result := ts.Current()
	if ts.cursor < ts.count {
		ts.cursor++
	}
	return result
}

func (ts *LexerState) Consume(count ...int) (result Token) {
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
		result = ts.Current()
		ts.cursor++
	}
	return
}

func (ts *LexerState) PrintError(format string, a ...any) {
	t := ts.Current()
	pos := t.Position()
	preffix := fmt.Sprintf("[ERROR] %s:%02d:%02d", ts.filepath, pos.line, pos.column)
	errorMsg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "%s %s", preffix, errorMsg)
}
