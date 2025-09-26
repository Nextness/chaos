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
	// TODO: Distinguish int literal from float literal at token level
	tokNumberLiteral
	tokBoolLiteral
	tokAs
	tokOpenBraket
	tokCloseBraket
	tokHash
	tokStar
	tokInfer
	tokArrow
	tokCount
)

func (tokenType TokenType) String() string {
	if tokenType == tokAssignment {
		return "tokAssignment"
	} else if tokenType == tokPlus {
		return "tokPlus"
	} else if tokenType == tokMinus {
		return "tokMinus"
	} else if tokenType == tokIdentifier {
		return "tokIdentifier"
	} else if tokenType == tokNewline {
		return "tokNewline"
	} else if tokenType == tokExit {
		return "tokExit"
	} else if tokenType == tokComma {
		return "tokComma"
	} else if tokenType == tokLessThan {
		return "tokLessThan"
	} else if tokenType == tokGreaterThan {
		return "tokGreaterThan"
	} else if tokenType == tokExecutes {
		return "tokExecutes"
	} else if tokenType == tokIf {
		return "tokIf"
	} else if tokenType == tokEndOfFile {
		return "tokEndOfFile"
	} else if tokenType == tokProc {
		return "tokProc"
	} else if tokenType == tokReturns {
		return "tokReturns"
	} else if tokenType == tokOpenParen {
		return "tokOpenParen"
	} else if tokenType == tokCloseParen {
		return "tokCloseParen"
	} else if tokenType == tokEllipsis {
		return "tokEllipsis"
	} else if tokenType == tokEquals {
		return "tokEquals"
	} else if tokenType == tokElif {
		return "tokElif"
	} else if tokenType == tokElse {
		return "tokElse"
	} else if tokenType == tokSemicolon {
		return "tokSemicolon"
	} else if tokenType == tokColon {
		return "tokColon"
	} else if tokenType == tokNumberLiteral {
		return "tokNumberLiteral"
	} else if tokenType == tokStringLiteral {
		return "tokStringLiteral"
	} else if tokenType == tokBoolLiteral {
		return "tokBoolLiteral"
	} else if tokenType == tokAs {
		return "tokAs"
	} else if tokenType == tokOpenBraket {
		return "tokOpenBraket"
	} else if tokenType == tokCloseBraket {
		return "tokCloseBraket"
	} else if tokenType == tokHash {
		return "tokHash"
	} else if tokenType == tokStar {
		return "tokStar"
	} else if tokenType == tokInfer {
		return "tokInfer"
	} else if tokenType == tokArrow {
		return "tokArrow"
	}
	return assert[string](
		tokCount == 32,
		fmt.Sprintf("Expected 32 token count but found %d", tokCount),
	)
}

type Position struct {
	line   int
	column int
}

type Token struct {
	Symbol    string
	TokenType TokenType
	Position  Position
	Length    int
	Value     any
}

type Source struct {
	data         string
	count        int
	cursor       int
	line, column int
}

func SourceCheckBounds(src *Source, offset int) {
	assert[any](
		src.cursor+offset < src.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", src.cursor, src.count),
	)
}

func SourceCurrentString(src *Source) string {
	return string(src.data[src.cursor])
}

func SourceMatchStringAt(src *Source, offset int, str string) bool {
	SourceCheckBounds(src, offset)
	length := len(str)
	result := true
	for i := range length {
		result = result && (SourcePeekByte(src, i+offset) == str[i])
	}
	return result
}

func SourceCurrentByte(src *Source) byte {
	return src.data[src.cursor]
}

func SourcePeekByte(src *Source, offset int) byte {
	SourceCheckBounds(src, offset)
	return src.data[src.cursor+offset]
}

func SourceMatchByteAt(src *Source, offset int, b byte) bool {
	SourceCheckBounds(src, offset)
	return SourcePeekByte(src, offset) == b
}

func TokenizeChaos(fileContent *bytes.Buffer) ChaosSlice[Token] {
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
		if sym := "//"; SourceMatchStringAt(src, 0, sym) {
			src.cursor += len(sym)
			for !SourceMatchByteAt(src, 0, '\n') {
				src.cursor++
			}
			continue
		}

		// Newline
		if SourceMatchByteAt(src, 0, '\n') {
			src.line++
			src.cursor++
			src.column = 0
			continue
		}

		// Multi-line comment
		if sym := "/**"; SourceMatchStringAt(src, 0, sym) {
			nestedComment := 0
			src.cursor += len(sym)
			for true {
				if SourceMatchByteAt(src, 0, '\n') {
					src.line++
					src.cursor++
					src.column = 0
					continue
				}
				if sym := "**/"; SourceMatchStringAt(src, 0, sym) && nestedComment > 0 {
					nestedComment--
					src.cursor += len(sym)
					src.column += len(sym)
					continue
				}
				if sym := "**/"; SourceMatchStringAt(src, 0, sym) {
					src.cursor += len(sym)
					src.column += len(sym)
					break
				}
				src.cursor++
				if sym := "/**"; SourceMatchStringAt(src, 0, sym) {
					nestedComment++
					src.cursor += len(sym)
					src.column += len(sym)
					continue
				}
			}
			continue
		}

		// Empty characters
		if unicode.IsSpace(rune(SourceCurrentByte(src))) {
			src.column++
			src.cursor++
			continue
		}

		// Strings
		if sym := "«"; SourceMatchStringAt(src, 0, sym) {

			tmp := bytes.Buffer{}
			defer tmp.Reset()

			length := 0
			position := Position{
				line:   src.line,
				column: src.column,
			}
			src.cursor += len(sym)

			// TO-DO: Handle nested '«»'
			// TO-DO: Handle interpolated strings like «hello {some-printable-variable}»
			endSym := "»"
			for !SourceMatchStringAt(src, 0, endSym) {
				tmp.WriteString(SourceCurrentString(src))
				length++
				src.cursor++
			}
			src.cursor += len(endSym)

			tokens = append(tokens, Token{
				TokenType: tokStringLiteral,
				Value:     tmp.String(),
				Position:  position,
				Length:    length,
			})
			continue
		}

		if sym := "->"; SourceMatchStringAt(src, 0, sym) {
			length := len(sym)
			tokens = append(tokens, Token{
				Symbol:    sym,
				TokenType: tokArrow,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
				Length: length,
			})
			src.column += length
			src.cursor += length
			continue
		}

		if sym := "=="; SourceMatchStringAt(src, 0, sym) {
			length := len(sym)
			tokens = append(tokens, Token{
				Symbol:    sym,
				TokenType: tokEquals,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
				Length: length,
			})
			src.column += length
			src.cursor += length
			continue
		}

		if sym := "..."; SourceMatchStringAt(src, 0, sym) {
			length := len(sym)
			tokens = append(tokens, Token{
				Symbol:    sym,
				TokenType: tokEllipsis,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
			})
			src.column += length
			src.cursor += length
			continue
		}

		if sym := "="; SourceMatchStringAt(src, 0, sym) {
			length := len(sym)
			tokens = append(tokens, Token{
				Symbol:    sym,
				TokenType: tokAssignment,
				Position: Position{
					line:   src.line,
					column: src.column,
				},
				Length: length,
			})
			src.column += length
			src.cursor += length
			continue
		}

		if sym := "+"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := "-"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := ","; SourceMatchStringAt(src, 0, sym) {
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

		if sym := "<"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := ">"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := ";"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := ":"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := "("; SourceMatchStringAt(src, 0, sym) {
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

		if sym := ")"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := "{"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := "}"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := "#"; SourceMatchStringAt(src, 0, sym) {
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

		if sym := "*"; SourceMatchStringAt(src, 0, sym) {
			tok := Token{
				Symbol:    sym,
				TokenType: tokStar,
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
		if isNum(SourceCurrentByte(src)) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   src.line,
				column: src.column,
			}

			isFloat := false
			for isNum(SourceCurrentByte(src)) || SourceMatchByteAt(src, 0, '.') || SourceMatchByteAt(src, 0, '_') {
				if SourceMatchByteAt(src, 0, '.') {
					isFloat = true
				}
				if !SourceMatchByteAt(src, 0, '_') {
					tmp.WriteByte(SourceCurrentByte(src))
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

		if isAlpha(SourceCurrentByte(src)) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   src.line,
				column: src.column,
			}

			for isAlphanum(SourceCurrentByte(src)) || SourceMatchByteAt(src, 0, '_') {
				tmp.WriteByte(SourceCurrentByte(src))
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
			SourceCurrentString(src), src.line, src.column,
		)
		panic("unrecheable")
	}

	tokens = append(tokens, Token{
		Symbol:    "eof",
		TokenType: tokEndOfFile,
		Position: Position{
			line:   src.line,
			column: src.column,
		},
		Length: 3,
	})

	chaosSlice := ChaosSlice[Token]{
		data:   tokens,
		count:  len(tokens),
		cursor: 0,
	}

	return chaosSlice
}

type Lexer struct {
	data     []Token
	filepath string
	count    int
	cursor   int
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

func (lex *Lexer) MatchTokenSequenceAndConsumeAssert(tokenTypes ...TokenType) bool {
	result := true
	for offset, tokenType := range tokenTypes {
		currentToken := lex.GetToken(offset)
		if currentToken.TokenType != tokenType {
			result = false
		}
	}
	if result {
		lex.ConsumeAssertSequence(tokenTypes...)
	}
	return result
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
