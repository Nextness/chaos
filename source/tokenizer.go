package main

import (
	"fmt"
	"unicode"
)

type TokenType int

var TokenTypeMap = map[TokenType]string{
	0: "INVALID_TOKEN",
	1: "EXIT_WITH",
	2: "INT_LITERAL",
	3: "SEMI_COLON",
}

const (
	invalidToken TokenType = iota
	exit
	intLiteral
	semiColon
)

type Token struct {
	Type         TokenType
	Value        string
	Line, Column int
}

func (p *TokenizerMetadataT) ConsumeRune() {
	(*p).currentIndex++
}

func (t *Token) logToken(errorMsg string) {
	token := *t
	if errorMsg == "" {
		fmt.Printf(
			"%04v:%03v:INFO { Type: '%v (%v)', Value: '%v' };\n",
			token.Line, token.Column,
			TokenTypeMap[token.Type], token.Type, token.Value,
		)
		return
	}
	fmt.Printf(
		"%04v:%03v:ERROR %s { Type: '%v (%v)', Value: '%v' };\n",
		token.Line, token.Column, errorMsg,
		TokenTypeMap[token.Type], token.Type, token.Value,
	)
}

func (tok *TokenizerMetadataT) Tokenizer() {
	for tok.bufferSize > tok.currentIndex {
		if tok.runeBuffer[tok.currentIndex] == '\n' {
			tok.lineCount++
			tok.ConsumeRune()
			continue
		} else if unicode.IsSpace(tok.runeBuffer[tok.currentIndex]) {
			tok.ConsumeRune()
			continue
		} else if unicode.IsLetter(tok.runeBuffer[tok.currentIndex]) {
			startingIndex := tok.currentIndex + 1
			for unicode.IsLetter(tok.runeBuffer[tok.currentIndex]) {
				if _, err := tok.tokenBuffer.WriteRune(tok.runeBuffer[tok.currentIndex]); err != nil {
					fmt.Printf("ERROR: %v", err.Error())
					return
				}
				tok.ConsumeRune()
			}
			if tok.tokenBuffer.String() == "exit_with" {
				identifier := Token{
					Type:   exit,
					Value:  tok.tokenBuffer.String(),
					Line:   tok.lineCount,
					Column: startingIndex,
				}
				identifier.logToken("")
				tok.tokens = append(tok.tokens, identifier)
				tok.tokenBuffer.Reset()
				continue
			}
		} else if unicode.IsDigit(tok.runeBuffer[tok.currentIndex]) {
			startingIndex := tok.currentIndex + 1
			for unicode.IsDigit(tok.runeBuffer[tok.currentIndex]) {
				if _, err := tok.tokenBuffer.WriteRune(tok.runeBuffer[tok.currentIndex]); err != nil {
					fmt.Printf("ERROR: %v", err.Error())
					return
				}
				tok.ConsumeRune()
			}
			identifier := Token{
				Type:   intLiteral,
				Value:  tok.tokenBuffer.String(),
				Line:   tok.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("")
			tok.tokens = append(tok.tokens, identifier)
			tok.tokenBuffer.Reset()
			continue
		} else {
			startingIndex := tok.currentIndex + 1
			tok.tokenBuffer.WriteRune(tok.runeBuffer[tok.currentIndex])
			identifier := Token{
				Type:   invalidToken,
				Value:  tok.tokenBuffer.String(),
				Line:   tok.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("Invalid token while parsing")
			tok.tokenBuffer.Reset()
		}
	}
}
