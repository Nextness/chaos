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

	tokens := []Token{}
	currentLine := 0
	currentColumn := 0

	for sourceSlice.cursor < sourceSlice.count {
		if CSMatch(sourceSlice, CSStringUint8Comparison, "/", "/") {
			ChaosSliceConsume(sourceSlice, 2)
			for !CSMatch(sourceSlice, CSStringUint8Comparison, "\n") {
				ChaosSliceConsume(sourceSlice)
			}
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "\n") {
			ChaosSliceConsume(sourceSlice)
			currentLine++
			currentColumn = 0
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "/", "*", "*") {
			nestedComment := 0
			ChaosSliceConsume(sourceSlice, 3)
			for true {
				if CSMatch(sourceSlice, CSStringUint8Comparison, "\n") {
					ChaosSliceConsume(sourceSlice)
					currentLine++
					currentColumn = 0
					continue
				}
				if CSMatch(sourceSlice, CSStringUint8Comparison, "*", "*", "/") &&
					nestedComment > 0 {
					nestedComment--
					ChaosSliceConsume(sourceSlice, 3)
					currentColumn += 3
					continue
				}
				if CSMatch(sourceSlice, CSStringUint8Comparison, "*", "*", "/") {
					ChaosSliceConsume(sourceSlice, 3)
					currentColumn += 3
					break
				}
				sourceSlice.cursor++
				if CSMatch(sourceSlice, CSStringUint8Comparison, "/", "*", "*") {
					nestedComment++
					ChaosSliceConsume(sourceSlice, 3)
					currentColumn += 3
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
		if CSMatch(sourceSlice, CSUint8Uint8Comparison, strStart) {
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
			for !CSMatch(sourceSlice, CSUint8Uint8Comparison, strEnd) {
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

		if CSMatch(sourceSlice, CSStringUint8Comparison, "-", ">") {
			tokens = append(tokens, Token{
				Symbol:    "->",
				TokenType: tokArrow,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 2,
			})
			currentColumn += 2
			ChaosSliceConsume(sourceSlice, 2)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "=", "=") {
			tokens = append(tokens, Token{
				Symbol:    "==",
				TokenType: tokEquals,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 2,
			})
			currentColumn += 2
			ChaosSliceConsume(sourceSlice, 2)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, ".", ".", ".") {
			tokens = append(tokens, Token{
				Symbol:    "...",
				TokenType: tokEllipsis,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 3,
			})
			currentColumn += 3
			ChaosSliceConsume(sourceSlice, 3)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "=") {
			tokens = append(tokens, Token{
				Symbol:    "=",
				TokenType: tokAssignment,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "+") {
			tokens = append(tokens, Token{
				Symbol:    "+",
				TokenType: tokPlus,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "-") {
			tokens = append(tokens, Token{
				Symbol:    "-",
				TokenType: tokMinus,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, ",") {
			tokens = append(tokens, Token{
				Symbol:    ",",
				TokenType: tokComma,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "<") {
			tokens = append(tokens, Token{
				Symbol:    "<",
				TokenType: tokLessThan,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, ">") {
			tokens = append(tokens, Token{
				Symbol:    ">",
				TokenType: tokGreaterThan,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, ";") {
			tokens = append(tokens, Token{
				Symbol:    ";",
				TokenType: tokSemicolon,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, ":") {
			tokens = append(tokens, Token{
				Symbol:    ":",
				TokenType: tokColon,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "(") {
			tokens = append(tokens, Token{
				Symbol:    "(",
				TokenType: tokOpenParen,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, ")") {
			tokens = append(tokens, Token{
				Symbol:    ")",
				TokenType: tokCloseParen,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "{") {
			tokens = append(tokens, Token{
				Symbol:    "{",
				TokenType: tokOpenBraket,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "}") {
			tokens = append(tokens, Token{
				Symbol:    "}",
				TokenType: tokCloseBraket,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "#") {
			tokens = append(tokens, Token{
				Symbol:    "#",
				TokenType: tokHash,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
			ChaosSliceConsume(sourceSlice)
			continue
		}

		if CSMatch(sourceSlice, CSStringUint8Comparison, "*") {
			tokens = append(tokens, Token{
				Symbol:    "*",
				TokenType: tokStar,
				Position: Position{
					line:   currentLine,
					column: currentColumn,
				},
				Length: 1,
			})
			currentColumn += 1
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
			for isNum(ChaosSliceGet(sourceSlice)) || CSMatch(sourceSlice, CSStringUint8Comparison, ".") {
				if CSMatch(sourceSlice, CSStringUint8Comparison, ".") {
					isFloat = true
				}
				if !CSMatch(sourceSlice, CSStringUint8Comparison, "_") {
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

			for isAlphanum(ChaosSliceGet(sourceSlice)) || CSMatch(sourceSlice, CSStringUint8Comparison, "_") {
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
