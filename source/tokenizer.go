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

func (p *chaosFileData) ConsumeRune() {
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

func (chaosData *chaosFileData) Tokenizer() ([]Token, error) {
	var tokens []Token
	for chaosData.bufferSize > chaosData.currentIndex {
		if chaosData.runeBuffer[chaosData.currentIndex] == '\n' {
			chaosData.lineCount++
			chaosData.ConsumeRune()
			continue
		} else if unicode.IsSpace(chaosData.runeBuffer[chaosData.currentIndex]) {
			chaosData.ConsumeRune()
			continue
		} else if chaosData.runeBuffer[chaosData.currentIndex] == ';' {
			startingIndex := chaosData.currentIndex + 1
			if _, err := chaosData.tokenBuffer.WriteRune(chaosData.runeBuffer[chaosData.currentIndex]); err != nil {
				fmt.Printf("ERROR: %v\n", err.Error())
				return []Token{}, err
			}
			chaosData.ConsumeRune()
			identifier := Token{
				Type:   semiColon,
				Value:  chaosData.tokenBuffer.String(),
				Line:   chaosData.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("")
			tokens = append(tokens, identifier)
			chaosData.tokenBuffer.Reset()
			continue
		} else if unicode.IsLetter(chaosData.runeBuffer[chaosData.currentIndex]) {
			startingIndex := chaosData.currentIndex + 1
			for unicode.IsLetter(chaosData.runeBuffer[chaosData.currentIndex]) ||
				chaosData.runeBuffer[chaosData.currentIndex] == '_' {
				if _, err := chaosData.tokenBuffer.WriteRune(chaosData.runeBuffer[chaosData.currentIndex]); err != nil {
					fmt.Printf("ERROR: %v\n", err.Error())
					return []Token{}, err
				}
				chaosData.ConsumeRune()
			}
			if chaosData.tokenBuffer.String() == "exit_with" {
				identifier := Token{
					Type:   exit,
					Value:  chaosData.tokenBuffer.String(),
					Line:   chaosData.lineCount,
					Column: startingIndex,
				}
				identifier.logToken("")
				tokens = append(tokens, identifier)
				chaosData.tokenBuffer.Reset()
				continue
			} else {
				identifier := Token{
					Type:   invalidToken,
					Value:  chaosData.tokenBuffer.String(),
					Line:   chaosData.lineCount,
					Column: startingIndex,
				}
				identifier.logToken("Invalid token while parsing")
				chaosData.tokenBuffer.Reset()
				continue
			}
		} else if unicode.IsDigit(chaosData.runeBuffer[chaosData.currentIndex]) {
			startingIndex := chaosData.currentIndex + 1
			for unicode.IsDigit(chaosData.runeBuffer[chaosData.currentIndex]) {
				if _, err := chaosData.tokenBuffer.WriteRune(chaosData.runeBuffer[chaosData.currentIndex]); err != nil {
					fmt.Printf("ERROR: %v\n", err.Error())
					return []Token{}, err
				}
				chaosData.ConsumeRune()
			}
			identifier := Token{
				Type:   intLiteral,
				Value:  chaosData.tokenBuffer.String(),
				Line:   chaosData.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("")
			tokens = append(tokens, identifier)
			chaosData.tokenBuffer.Reset()
			continue
		} else {
			startingIndex := chaosData.currentIndex + 1
			chaosData.tokenBuffer.WriteRune(chaosData.runeBuffer[chaosData.currentIndex])
			chaosData.ConsumeRune()
			identifier := Token{
				Type:   invalidToken,
				Value:  chaosData.tokenBuffer.String(),
				Line:   chaosData.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("Invalid token while parsing")
			chaosData.tokenBuffer.Reset()
			continue
		}
	}
	return tokens, nil
}
