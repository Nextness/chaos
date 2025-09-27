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
	tokIf
	tokElif
	tokElse
	tokProc
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

var _ = assert[any](
	tokCount == 30,
	fmt.Sprintf("Expected 30 token count but found %d", tokCount),
)

func TokenTypeToString(tokenType TokenType) string {
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
	} else if tokenType == tokIf {
		return "tokIf"
	} else if tokenType == tokEndOfFile {
		return "tokEndOfFile"
	} else if tokenType == tokProc {
		return "tokProc"
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
		tokCount == 30,
		fmt.Sprintf("Expected 30 token count but found %d", tokCount),
	)

}

type Position struct {
	line, column int
}

type Token struct {
	Symbol    string
	TokenType TokenType
	Position  Position
	Length    int
	Value     any
}

func ChaosContentTokenize(fileContent *bytes.Buffer) ChaosSlice[Token] {
	sourceSlice := &ChaosSlice[byte]{
		data:   fileContent.Bytes(),
		count:  fileContent.Len(),
		cursor: 0,
	}

	var value []byte
	tokens := []Token{}
	currentLine := 0
	currentColumn := 0

	for sourceSlice.cursor < sourceSlice.count {
		value = ChaosSliceSearchValue[string, byte]("//")
		if ChaosSliceMatchOp(sourceSlice, ChaosSliceDefaultComparison, value) {
			ChaosSliceConsume(sourceSlice, len(value))
			value = ChaosSliceSearchValue[string, byte]("\n")
			for !ChaosSliceMatch(sourceSlice, value) {
				ChaosSliceConsume(sourceSlice)
			}
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("\n")
		if ChaosSliceMatch(sourceSlice, value) {
			ChaosSliceConsume(sourceSlice)
			currentLine++
			currentColumn = 0
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("/**")
		if ChaosSliceMatch(sourceSlice, value) {
			nestedComment := 0
			ChaosSliceConsume(sourceSlice, len(value))
			for true {
				value = ChaosSliceSearchValue[string, byte]("\n")
				if ChaosSliceMatch(sourceSlice, value) {
					ChaosSliceConsume(sourceSlice)
					currentLine++
					currentColumn = 0
					continue
				}
				value = ChaosSliceSearchValue[string, byte]("**/")
				if ChaosSliceMatch(sourceSlice, value) && nestedComment > 0 {
					nestedComment--
					ChaosSliceConsume(sourceSlice, len(value))
					currentColumn = len(value)
					continue
				}
				value = ChaosSliceSearchValue[string, byte]("**/")
				if ChaosSliceMatch(sourceSlice, value) {
					ChaosSliceConsume(sourceSlice, len(value))
					currentColumn = len(value)
					break
				}
				sourceSlice.cursor++
				value = ChaosSliceSearchValue[string, byte]("/**")
				if ChaosSliceMatch(sourceSlice, value) {
					nestedComment++
					ChaosSliceConsume(sourceSlice, len(value))
					currentColumn = len(value)
					continue
				}
			}
			continue
		}

		if unicode.IsSpace(rune(ChaosSliceGet(sourceSlice))) {
			ChaosSliceConsume(sourceSlice)
			currentColumn++
			continue
		}

		strStart := []byte("«")
		if ChaosSliceMatch(sourceSlice, strStart) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			length := 0
			position := Position{
				line:   currentLine,
				column: currentColumn,
			}
			ChaosSliceConsume(sourceSlice, len(strStart))

			// TO-DO: Handle nested '«»'
			// TO-DO: Handle interpolated strings like «hello {some-printable-variable}»
			strEnd := []byte("»")
			for !ChaosSliceMatch(sourceSlice, strEnd) {
				tmp.WriteByte(ChaosSliceGet(sourceSlice))
				length++
				sourceSlice.cursor++
			}
			ChaosSliceConsume(sourceSlice, len(strEnd))

			tokens = append(tokens, Token{
				TokenType: tokStringLiteral,
				Value:     tmp.String(),
				Position:  position,
				Length:    length,
			})
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("->")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokArrow,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice, length)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("==")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokEquals,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice, length)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("...")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokEllipsis,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice, length)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("=")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokAssignment,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("+")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokPlus,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("-")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokMinus,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte](",")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokComma,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("<")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokLessThan,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte](">")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokGreaterThan,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte](";")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokSemicolon,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte](":")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokColon,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("(")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokOpenParen,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte](")")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokCloseParen,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("{")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokOpenBraket,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("}")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokCloseBraket,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("#")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokHash,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		value = ChaosSliceSearchValue[string, byte]("*")
		if ChaosSliceMatch(sourceSlice, value) {
			length := len(value)
			tokens = append(tokens, Token{
				Symbol:    string(value),
				TokenType: tokStar,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: length,
			})
			currentColumn += length
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if isNum(ChaosSliceGet(sourceSlice)) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   currentLine,
				column: currentColumn,
			}

			isFloat := false
			for isNum(ChaosSliceGet(sourceSlice)) || ChaosSliceMatch(sourceSlice, ChaosSliceSearchValue[string, byte](".")) {
				if ChaosSliceMatch(sourceSlice, ChaosSliceSearchValue[string, byte](".")) {
					isFloat = true
				}
				if !ChaosSliceMatch(sourceSlice, ChaosSliceSearchValue[string, byte]("_")) {
					tmp.WriteByte(ChaosSliceGet(sourceSlice))
				}
				currentColumn++
				ChaosSliceConsume(sourceSlice)
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

		if isAlpha(ChaosSliceGet(sourceSlice)) {
			tmp := bytes.Buffer{}
			defer tmp.Reset()

			position := Position{
				line:   currentLine,
				column: currentColumn,
			}

			for isAlphanum(ChaosSliceGet(sourceSlice)) || ChaosSliceMatch(sourceSlice, ChaosSliceSearchValue[string, byte]("_")) {
				tmp.WriteByte(ChaosSliceGet(sourceSlice))
				currentColumn++
				ChaosSliceConsume(sourceSlice)
			}

			string := tmp.String()

			// Keywords
			if sym := "true"; string == sym {
				token := Token{
					Symbol:    sym,
					Value:     true,
					TokenType: tokBoolLiteral,
					Position:  position,
					Length:    len(sym),
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
					Length:    len(sym),
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "exit"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokExit,
					Position:  position,
					Length:    len(sym),
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "if"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokIf,
					Position:  position,
					Length:    len(sym),
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "elif"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokElif,
					Position:  position,
					Length:    len(sym),
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "else"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokElse,
					Position:  position,
					Length:    len(sym),
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "proc"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokProc,
					Position:  position,
					Length:    len(sym),
				}
				tokens = append(tokens, token)
				continue
			}

			if sym := "as"; string == sym {
				token := Token{
					Symbol:    sym,
					TokenType: tokAs,
					Position:  position,
					Length:    len(sym),
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
				Length:    len(string),
			}
			tokens = append(tokens, token)
			continue
		}

		fmt.Fprintf(
			os.Stderr,
			"[ERROR] Failed while tokenizing - unknown character '%s' at position %03d:%03d\n",
			string(ChaosSliceGet(sourceSlice)), currentLine, currentColumn,
		)
		panic("unrecheable")
	}

	tokens = append(tokens, Token{
		Symbol:    "eof",
		TokenType: tokEndOfFile,
		Position: Position{
			line:   currentLine,
			column: currentColumn,
		},
		Length: 3,
	})

	tokensSlice := ChaosSlice[Token]{
		data:   tokens,
		count:  len(tokens),
		cursor: 0,
	}

	return tokensSlice
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
			lex.filepath, token.Position.line, token.Position.column, TokenTypeToString(tokType), TokenTypeToString(token.TokenType),
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
