package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"unicode"
)

type TokenType int

const (
	tokAssignment TokenType = iota
	tokPlus
	tokMinus
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
	tokOpenParen
	tokCloseParen
	tokEllipsis
	tokSemicolon
	tokColon
	tokStringLiteral
	tokNumberLiteral
	tokBoolLiteral
	tokAs
	tokOpenBraket
	tokCloseBraket
	tokHash
	tokCount
)

func (tokenType TokenType) String() string {
	assert[any](
		tokCount == 29,
		fmt.Sprintf("Expected 29 token count but found %d", tokCount),
	)
	if tokenType == tokAssignment {
		return "tokAssignment"
	}
	if tokenType == tokPlus {
		return "tokPlus"
	}
	if tokenType == tokMinus {
		return "tokMinus"
	}
	if tokenType == tokIdentifier {
		return "tokIdentifier"
	}
	if tokenType == tokNewline {
		return "tokNewline"
	}
	if tokenType == tokExit {
		return "tokExit"
	}
	if tokenType == tokComma {
		return "tokComma"
	}
	if tokenType == tokLessThan {
		return "tokLessThan"
	}
	if tokenType == tokGreaterThan {
		return "tokGreaterThan"
	}
	if tokenType == tokExecutes {
		return "tokExecutes"
	}
	if tokenType == tokIf {
		return "tokIf"
	}
	if tokenType == tokEndOfFile {
		return "tokEndOfFile"
	}
	if tokenType == tokProc {
		return "tokProc"
	}
	if tokenType == tokReturns {
		return "tokReturns"
	}
	if tokenType == tokOpenParen {
		return "tokOpenParen"
	}
	if tokenType == tokCloseParen {
		return "tokCloseParen"
	}
	if tokenType == tokEllipsis {
		return "tokEllipsis"
	}
	if tokenType == tokEquals {
		return "tokEquals"
	}
	if tokenType == tokElif {
		return "tokElif"
	}
	if tokenType == tokElse {
		return "tokElse"
	}
	if tokenType == tokSemicolon {
		return "tokSemicolon"
	}
	if tokenType == tokColon {
		return "tokColon"
	}
	if tokenType == tokNumberLiteral {
		return "tokNumberLiteral"
	}
	if tokenType == tokStringLiteral {
		return "tokStringLiteral"
	}
	if tokenType == tokBoolLiteral {
		return "tokBoolLiteral"
	}
	if tokenType == tokAs {
		return "tokAs"
	}
	if tokenType == tokOpenBraket {
		return "tokOpenBraket"
	}
	if tokenType == tokCloseBraket {
		return "tokCloseBraket"
	}
	if tokenType == tokHash {
		return "tokHash"
	}
	return "unrecheable"
}

type Position struct {
	line   int
	column int
}

type Token struct {
	Symbol    string
	TokenType TokenType
	Position  Position
	Value     any
}

type Source struct {
	data         string
	count        int
	cursor       int
	line, column int
}

func (src *Source) CurrentString() string {
	return string(src.data[src.cursor])
}

func (src *Source) MatchStrAt(offset int, str string) bool {
	if src.cursor+offset > src.count {
		panic("Out of bounds while matchStrAt")
	}
	length := len(str)
	result := true
	for i := range length {
		result = result && (src.PeekByte(i+offset) == str[i])
	}
	return result
}

func (src *Source) CurrentByte() byte {
	return src.data[src.cursor]
}

func (src *Source) PeekByte(offset int) (result byte) {
	result = byte(0)
	if src.cursor+offset < src.count {
		result = src.data[src.cursor+offset]
	}
	return
}

func (src *Source) MatchByteAt(offset int, b byte) bool {
	if src.cursor+offset > src.count {
		panic("Out of bounds while matchByteAt")
	}
	return src.PeekByte(offset) == b

}

func TokenizeChaos(fileContent *bytes.Buffer) []Token {
	src := &Source{
		data:   fileContent.String(),
		count:  fileContent.Len(),
		cursor: 0,
		line:   1,
		column: 1,
	}

	tokens := []Token{}
	for src.cursor < src.count {

		// Single-line comment
		if src.MatchStrAt(0, "//") {
			for !src.MatchByteAt(0, '\n') {
				src.cursor++
			}
			continue
		}

		// Newline
		if src.MatchByteAt(0, '\n') {
			src.line++
			src.cursor++
			src.column = 0
			continue
		}

		// Multi-line comment
		if src.MatchStrAt(0, "/**") {
			nestedComment := 0
			src.cursor += 3
			for true {
				if src.MatchByteAt(0, '\n') {
					src.line++
					src.cursor++
					src.column = 0
					continue
				}
				if src.MatchStrAt(0, "**/") && nestedComment > 0 {
					nestedComment--
					src.cursor += 3
					src.column += 3
					continue
				}
				if src.MatchStrAt(0, "**/") {
					src.cursor += 3
					src.column += 3
					break
				}
				src.cursor++
				if src.MatchStrAt(0, "/**") {
					nestedComment++
					src.cursor += 3
					src.column += 3
					continue
				}
			}
			continue
		}

		// Empty characters
		if unicode.IsSpace(rune(src.CurrentByte())) {
			src.column++
			src.cursor++
			continue
		}

		// Strings
		if src.MatchStrAt(0, "«") {

			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   src.line,
				column: src.column,
			}
			src.cursor += 2

			// TO-DO: Handle nested '«»'
			// TO-DO: Handle interpolated strings like «hello {some-printable-variable}»
			for !src.MatchStrAt(0, "»") {
				tmp.WriteString(src.CurrentString())
				src.cursor++
			}

			token := Token{
				Symbol:    "None",
				TokenType: tokStringLiteral,
				Value:     tmp.String(),
				Position:  position,
			}

			src.cursor += 2
			tokens = append(tokens, token)
			continue
		}

		if sym := "=="; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokEquals,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column += 2
			src.cursor += 2
			tokens = append(tokens, token)
			continue
		}

		if sym := "..."; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokEllipsis,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column += 3
			src.cursor += 3
			tokens = append(tokens, token)
			continue
		}

		if sym := "="; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokAssignment,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "+"; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokPlus,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "-"; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokMinus,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ","; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokComma,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "<"; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokLessThan,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ">"; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokGreaterThan,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ";"; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokSemicolon,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ":"; src.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokColon,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "("; src.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokOpenParen,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := ")"; src.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokCloseParen,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := "{"; src.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokOpenBraket,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := "}"; src.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokCloseBraket,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := "#"; src.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokHash,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			}
			src.column++
			src.cursor++
			tokens = append(tokens, tok)
			continue
		}

		// Number literals (float or int)
		if isNum(src.CurrentByte()) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   src.line,
				column: src.column,
			}

			isFloat := false
			for isNum(src.CurrentByte()) || src.MatchByteAt(0, '.') || src.MatchByteAt(0, '_') {
				if src.MatchByteAt(0, '.') {
					isFloat = true
				}
				if !src.MatchByteAt(0, '_') {
					tmp.WriteByte(src.CurrentByte())
				}
				src.column++
				src.cursor++
			}

			number := tmp.String()
			if isFloat {
				val, err := strconv.ParseFloat(number, 64)
				assert[any](err == nil, "Failed to convert string to float")

				token := Token{
					TokenType: tokNumberLiteral,
					Value:     val,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			} else {
				val, err := strconv.Atoi(tmp.String())
				assert[any](err == nil, "Failed to convert string to number")

				token := Token{
					TokenType: tokNumberLiteral,
					Value:     val,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}
		}

		if isAlpha(src.CurrentByte()) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   src.line,
				column: src.column,
			}

			for isAlphanum(src.CurrentByte()) || src.MatchByteAt(0, '_') {
				tmp.WriteByte(src.CurrentByte())
				src.column++
				src.cursor++
			}

			string := tmp.String()

			// Keywords
			if sym := "true"; string == sym {
				token := Token{
					Symbol:    sym,
					Value:     true,
					TokenType: tokBoolLiteral,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "false"; string == sym {
				token := Token{
					Symbol:    sym,
					Value:     false,
					TokenType: tokBoolLiteral,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "exit"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokExit,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "if"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokIf,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "elif"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokElif,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "else"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokElse,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "proc"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokProc,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "as"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokAs,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			}

			// Identifiers
			token := Token{
				Symbol:    string,
				TokenType: tokIdentifier,
				Value:     nil,
				Position:  position,
			}
			tokens = append(tokens, token)
			continue
		}

		fmt.Fprintf(
			os.Stderr,
			"[ERROR] Failed while tokenizing - unknown character '%s' at position %03d:%03d\n",
			src.CurrentString(), src.line, src.column,
		)
		panic("unrecheable")
	}

	eof := Token{
		Symbol:    "eof",
		TokenType: tokEndOfFile,
		Position: Position{
			line:   src.line,
			column: src.column,
		},
	}

	tokens = append(tokens, eof)
	return tokens
}

type Lexer struct {
	data     []Token
	filepath string
	count    int
	cursor   int
}

func (tokens Lexer) print() {
	fmt.Print("Token List:\n")
	for idx, token := range tokens.data {
		if token.TokenType == tokEndOfFile {
			return
		}
		if token.TokenType == tokNewline {
			continue
		}
		if token.TokenType == tokStringLiteral {
			fmt.Printf("%6d. [%s] `%s`\n", idx, token.TokenType.String(), token.Value)
		} else if token.TokenType == tokNumberLiteral {
			if val, ok := cast[int](token.Value); ok {
				fmt.Printf("%6d. [%s] `%d`\n", idx, token.TokenType.String(), val)
			} else if val, ok := cast[float64](token.Value); ok {
				fmt.Printf("%6d. [%s] `%f`\n", idx, token.TokenType.String(), val)
			}
		} else {
			fmt.Printf("%6d. [%s] `%s`\n", idx, token.TokenType.String(), token.Symbol)
		}
	}
}

func (lex *Lexer) checkBounds(offset int) {
	assert[any](
		lex.cursor+offset < lex.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", lex.cursor, lex.count),
	)
}

func (lex *Lexer) GetToken(offset int) Token {
	lex.checkBounds(offset)
	return lex.data[lex.cursor+offset]
}

func (lex *Lexer) MatchAt(offset int, tokTypes ...TokenType) bool {
	for _, token := range tokTypes {
		currentToken := lex.GetToken(offset)
		if currentToken.TokenType == token {
			return true
		}
	}
	return false
}

func (lex *Lexer) MatchTokenSequence(tokenTypes ...TokenType) bool {
	for offset, tokenType := range tokenTypes {
		currentToken := lex.GetToken(offset)
		if currentToken.TokenType != tokenType {
			return false
		}
	}
	return true
}

func (lex *Lexer) MatchTokenAndConsumeAssert(tokenType TokenType) (Token, bool) {
	if lex.MatchAt(0, tokenType) {
		token := lex.ConsumeAssert(tokenType)
		return token, true
	}
	return Token{}, false
}

func (lex *Lexer) Consume() Token {
	token := lex.GetToken(0)
	lex.cursor++
	return token
}

func (lex *Lexer) ConsumeMany(count int) {
	lex.checkBounds(count)
	lex.cursor += count
}

func (lex *Lexer) ConsumeAssert(tokType TokenType) Token {
	token := lex.GetToken(0)
	assert[any](
		token.TokenType == tokType,
		fmt.Sprintf(
			"%s:%d:%d Expected %s but got %s",
			lex.filepath, token.Position.line, token.Position.column, tokType.String(), token.TokenType.String(),
		),
	)
	lex.cursor++
	return token
}

func (lex *Lexer) ConsumeAssertSequence(tokenTypes ...TokenType) {
	for _, tokenType := range tokenTypes {
		lex.ConsumeAssert(tokenType)
	}
}
