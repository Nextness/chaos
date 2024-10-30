package main

import (
	"fmt"
	"runtime"
	"unicode"
)

type TokenType int

var TokenTypeMap = map[TokenType]string{
	0: "INVALID_TOKEN",
	1: "EXIT_WITH",
	2: "INT_LITERAL",
	3: "SEMI_COLON",
	4: "IDENTIFIER",
	5: "LET",
	6: "EQUALS",
}

const (
	invalidToken TokenType = iota
	exit
	intLiteral
	semiColon
	identifier
	let
	equals
)

type ChaosToken struct {
	Type         TokenType
	Value        string
	Line, Column int
	FilePath     string
}

func (t *ChaosToken) LogToken(errorMsg string) {
	token := *t
	_, fileName, _, _ := runtime.Caller(1)
	if errorMsg == "" {
		fmt.Printf(
			"INFO: %v:%04v:%03v - Type: '%v (%v)', Value: '%v'\n",
			fileName, token.Line, token.Column,
			TokenTypeMap[token.Type], token.Type, token.Value,
		)
		return
	}
	fmt.Printf(
		"ERROR: %v:%04v:%03v %s - Type: '%v (%v)', Value: '%v';\n",
		fileName, token.Line, token.Column, errorMsg,
		TokenTypeMap[token.Type], token.Type, token.Value,
	)
}

func (cfd *ChaosFileData) ConsumeRune() rune {
	if cfd.currentIndex < cfd.bufferSize {
		currentRune := cfd.runeBuffer[cfd.currentIndex]
		(*cfd).currentIndex++
		return currentRune
	}
	return rune(0)
}

func (cfd *ChaosFileData) Peek(position int) rune {
	if cfd.currentIndex+position < cfd.bufferSize {
		return cfd.runeBuffer[cfd.currentIndex+position]
	}
	return rune(0)
}

func (chaosData *ChaosFileData) Tokenizer() ([]ChaosToken, error) {
	var tokens []ChaosToken
	for chaosData.Peek(0) != rune(0) {
		startingIndex := chaosData.currentIndex + 1
		if chaosData.Peek(0) == '\n' {
			chaosData.lineCount++
			chaosData.ConsumeRune()
			continue
		} else if unicode.IsSpace(chaosData.Peek(0)) {
			chaosData.ConsumeRune()
			continue
		} else if chaosData.Peek(0) == ';' {
			if _, err := chaosData.tokenBuffer.WriteRune(chaosData.ConsumeRune()); err != nil {
				fmt.Printf("ERROR: %v\n", err.Error())
				return []ChaosToken{}, err
			}
			identifier := ChaosToken{
				Type:     semiColon,
				Value:    chaosData.tokenBuffer.String(),
				Line:     chaosData.lineCount,
				Column:   startingIndex,
				FilePath: chaosData.filePath,
			}
			identifier.LogToken("")
			tokens = append(tokens, identifier)
			chaosData.tokenBuffer.Reset()
			continue
		} else if chaosData.Peek(0) == '=' {
			if _, err := chaosData.tokenBuffer.WriteRune(chaosData.ConsumeRune()); err != nil {
				fmt.Printf("ERROR: %v\n", err.Error())
				return []ChaosToken{}, err
			}
			identifier := ChaosToken{
				Type:     equals,
				Value:    chaosData.tokenBuffer.String(),
				Line:     chaosData.lineCount,
				Column:   startingIndex,
				FilePath: chaosData.filePath,
			}
			identifier.LogToken("")
			tokens = append(tokens, identifier)
			chaosData.tokenBuffer.Reset()
			continue
		} else if unicode.IsLetter(chaosData.Peek(0)) {
			for unicode.IsLetter(chaosData.Peek(0)) || chaosData.Peek(0) == '_' {
				if _, err := chaosData.tokenBuffer.WriteRune(chaosData.ConsumeRune()); err != nil {
					fmt.Printf("ERROR: %v\n", err.Error())
					return []ChaosToken{}, err
				}
			}
			if chaosData.tokenBuffer.String() == "exit_with" {
				identifier := ChaosToken{
					Type:     exit,
					Value:    chaosData.tokenBuffer.String(),
					Line:     chaosData.lineCount,
					Column:   startingIndex,
					FilePath: chaosData.filePath,
				}
				identifier.LogToken("")
				tokens = append(tokens, identifier)
				chaosData.tokenBuffer.Reset()
				continue
			} else if chaosData.tokenBuffer.String() == "let" {
				identifier := ChaosToken{
					Type:     let,
					Value:    chaosData.tokenBuffer.String(),
					Line:     chaosData.lineCount,
					Column:   startingIndex,
					FilePath: chaosData.filePath,
				}
				identifier.LogToken("")
				tokens = append(tokens, identifier)
				chaosData.tokenBuffer.Reset()
				continue
			} else {
				identifier := ChaosToken{
					Type:     identifier,
					Value:    chaosData.tokenBuffer.String(),
					Line:     chaosData.lineCount,
					Column:   startingIndex,
					FilePath: chaosData.filePath,
				}
				identifier.LogToken("")
				tokens = append(tokens, identifier)
				chaosData.tokenBuffer.Reset()
				continue
			}
		} else if unicode.IsDigit(chaosData.Peek(0)) {
			for unicode.IsDigit(chaosData.Peek(0)) {
				if _, err := chaosData.tokenBuffer.WriteRune(chaosData.ConsumeRune()); err != nil {
					fmt.Printf("ERROR: %v\n", err.Error())
					return []ChaosToken{}, err
				}
			}
			identifier := ChaosToken{
				Type:     intLiteral,
				Value:    chaosData.tokenBuffer.String(),
				Line:     chaosData.lineCount,
				Column:   startingIndex,
				FilePath: chaosData.filePath,
			}
			identifier.LogToken("")
			tokens = append(tokens, identifier)
			chaosData.tokenBuffer.Reset()
			continue
		} else {
			chaosData.tokenBuffer.WriteRune(chaosData.ConsumeRune())
			identifier := ChaosToken{
				Type:     invalidToken,
				Value:    chaosData.tokenBuffer.String(),
				Line:     chaosData.lineCount,
				Column:   startingIndex,
				FilePath: chaosData.filePath,
			}
			identifier.LogToken("Invalid character while parsing")
			tokens = append(tokens, identifier)
			chaosData.tokenBuffer.Reset()
			continue
		}
	}
	return tokens, nil
}
