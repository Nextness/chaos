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

func (tokTyp TokenType) String() string {
	assert(
		tokCount == 29,
		fmt.Sprintf("Expected 29 token count but found %d", tokCount),
	)
	if tokTyp == tokAssignment {
		return "tokAssignment"
	}
	if tokTyp == tokPlus {
		return "tokPlus"
	}
	if tokTyp == tokMinus {
		return "tokMinus"
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
	if tokTyp == tokReturns {
		return "tokReturns"
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
	if tokTyp == tokNumberLiteral {
		return "tokNumberLiteral"
	}
	if tokTyp == tokStringLiteral {
		return "tokStringLiteral"
	}
	if tokTyp == tokBoolLiteral {
		return "tokBoolLiteral"
	}
	if tokTyp == tokAs {
		return "tokAs"
	}
	if tokTyp == tokOpenBraket {
		return "tokOpenBraket"
	}
	if tokTyp == tokCloseBraket {
		return "tokCloseBraket"
	}
	if tokTyp == tokHash {
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

		if cs.MatchByteAt(0, '_') {
			fmt.Fprint(os.Stderr, "[ERROR] If you want to use underscore for identifiers, please go use another peasant fucking language :)\n")
			os.Exit(1)
		}

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

			token := Token{
				Symbol:    "None",
				TokenType: tokStringLiteral,
				Value:     tmp.String(),
				Position:  position,
			}

			cs.cursor += 2
			tokens = append(tokens, token)
			continue
		}

		if sym := "=="; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokEquals,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column += 2
			cs.cursor += 2
			tokens = append(tokens, token)
			continue
		}

		if sym := "..."; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokEllipsis,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column += 3
			cs.cursor += 3
			tokens = append(tokens, token)
			continue
		}

		if sym := "="; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokAssignment,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "+"; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokPlus,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "-"; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokMinus,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ","; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokComma,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "<"; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokLessThan,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ">"; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokGreaterThan,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ";"; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokSemicolon,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := ":"; cs.MatchStrAt(0, sym) {
			token := Token{
				Symbol:    sym,
				TokenType: tokColon,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, token)
			continue
		}

		if sym := "("; cs.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokOpenParen,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := ")"; cs.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokCloseParen,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := "{"; cs.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokOpenBraket,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := "}"; cs.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokCloseBraket,
				Position: Position{
					line:   cs.line,
					column: cs.column,
				},
			}
			cs.column++
			cs.cursor++
			tokens = append(tokens, tok)
			continue
		}

		if sym := "#"; cs.MatchStrAt(0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokHash,
				Position: Position{
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

				token := Token{
					TokenType: tokNumberLiteral,
					Value:     val,
					Position:  position,
				}
				tokens = append(tokens, token)
				continue
			} else {
				val, err := strconv.Atoi(tmp.String())
				assert(err == nil, "Failed to convert string to number")

				token := Token{
					TokenType: tokNumberLiteral,
					Value:     val,
					Position:  position,
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

			for isAlphanum(cs.CurrentByte()) || cs.MatchByteAt(0, '_') {
				tmp.WriteByte(cs.CurrentByte())
				cs.column++
				cs.cursor++
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
			cs.CurrentString(), cs.line, cs.column,
		)
		panic("unrecheable")
	}

	eof := Token{
		Symbol:    "eof",
		TokenType: tokEndOfFile,
		Position: Position{
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

func (ts *LexerState) Current() Token {
	assert(
		ts.cursor < ts.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", ts.cursor, ts.count),
	)
	return ts.data[ts.cursor]
}

func (ts *LexerState) currentTokenTypeAsString() string {
	return ts.Current().TokenType.String()
}

func (ts *LexerState) MatchAt(offset int, tokTypes ...TokenType) (result bool) {
	result = false
	for _, token := range tokTypes {
		if ts.cursor+offset < ts.count {
			curTok := ts.data[ts.cursor+offset]
			if curTok.TokenType == token {
				result = true
				break
			}
		}
	}
	return
}

func (ls *LexerState) MatchTokenSequence(tokenTypes ...TokenType) (result bool) {
	result = true
	for idx, tokenType := range tokenTypes {
		if ls.cursor+idx < ls.count {
			currentToken := ls.data[ls.cursor+idx]
			if currentToken.TokenType != tokenType {
				result = false
				break
			}
		}
	}
	return
}

func (ts *LexerState) Peek(offset int) (result Token) {
	assert(offset != 0, "cannot peak with 0")
	if ts.cursor+offset < ts.count {
		result = ts.data[ts.cursor+offset]
	}
	return
}

func (ts *LexerState) ConsumeAssert(tokType TokenType) Token {
	assert(
		ts.Current().TokenType == tokType,
		fmt.Sprintf("Expected the token '%s' but found '%s'", tokType.String(), ts.currentTokenTypeAsString()),
	)
	result := ts.Current()
	if ts.cursor < ts.count {
		ts.cursor++
	}
	return result
}

func (ls *LexerState) ConsumeAssertMany(tokenType ...TokenType) {
	for _, tt := range tokenType {
		ls.ConsumeAssert(tt)
	}
}

func (ts *LexerState) Consume(count ...int) (result Token) {
	assert(len(count) <= 1, "Count can only be 1 or empty")
	c := 0
	if len(count) == 1 {
		c = count[0]
	}
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
	token := ts.Current()
	pos := token.Position
	preffix := fmt.Sprintf("[ERROR] %s:%02d:%02d", ts.filepath, pos.line, pos.column)
	errorMsg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "%s %s", preffix, errorMsg)
}
