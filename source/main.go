package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
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

type TokenizerMetadataT struct {
	runeBuffer               []rune
	currentIndex, bufferSize int
	tokenBuffer              bytes.Buffer
	tokens                   []Token
	lineCount                int
}

func (p *TokenizerMetadataT) consumeRune() {
	(*p).currentIndex++
}

func readFileAsRune(filePath string) ([]rune, int, error) {
	buffer, err := os.ReadFile(filePath)
	if err != nil {
		return []rune{}, 0, err
	}
	newBuffer := bytes.NewBuffer(buffer)
	return bytes.Runes(newBuffer.Bytes()), newBuffer.Len(), nil
}

type ExitFunctionNode struct {
	Value   int8
	Message string
}

func parseTokenExit(tokens []Token) ExitFunctionNode {
	result, _ := strconv.ParseInt(tokens[1].Value, 10, 8)
	a := ExitFunctionNode{}
	a.Value = int8(result)
	a.Message = "Some message"
	return a
}

func generateExitSyscal(exitFunctionNode ExitFunctionNode) string {
	s := []string{
		"    mov rax, 60\n",
		fmt.Sprintf("    mov rdi, %d\n", exitFunctionNode.Value),
		"    syscall\n",
	}
	return strings.Join(s, "")
}

func main() {
	filePath := "./example/exit_with.chaos"
	bufferAsRune, bufferSize, err := readFileAsRune(filePath)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return
	}

	tokenizer := TokenizerMetadataT{
		runeBuffer:   bufferAsRune,
		currentIndex: 0,
		bufferSize:   bufferSize,
		lineCount:    0,
	}

	for tokenizer.bufferSize > tokenizer.currentIndex {
		if tokenizer.runeBuffer[tokenizer.currentIndex] == '\n' {
			tokenizer.lineCount++
			tokenizer.consumeRune()
		}
		if unicode.IsSpace(tokenizer.runeBuffer[tokenizer.currentIndex]) {
			tokenizer.consumeRune()
		}
		if tokenizer.runeBuffer[tokenizer.currentIndex] == ';' {
			tokenizer.tokenBuffer.WriteRune(tokenizer.runeBuffer[tokenizer.currentIndex])
			startingIndex := tokenizer.currentIndex + 1
			identifier := Token{
				Type:   semiColon,
				Value:  tokenizer.tokenBuffer.String(),
				Line:   tokenizer.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("")
			tokenizer.tokens = append(tokenizer.tokens, identifier)
			tokenizer.tokenBuffer.Reset()
			tokenizer.consumeRune()
			continue
		} else if unicode.IsLetter(tokenizer.runeBuffer[tokenizer.currentIndex]) {
			startingIndex := tokenizer.currentIndex + 1
			for unicode.IsLetter(tokenizer.runeBuffer[tokenizer.currentIndex]) ||
				unicode.IsNumber(tokenizer.runeBuffer[tokenizer.currentIndex]) ||
				tokenizer.runeBuffer[tokenizer.currentIndex] == '_' {
				if _, err := tokenizer.tokenBuffer.WriteRune(tokenizer.runeBuffer[tokenizer.currentIndex]); err != nil {
					fmt.Printf("ERROR: %v", err.Error())
					return
				}
				tokenizer.consumeRune()
			}
			if tokenizer.tokenBuffer.String() == "exit_with" {
				identifier := Token{
					Type:   exit,
					Value:  tokenizer.tokenBuffer.String(),
					Line:   tokenizer.lineCount,
					Column: startingIndex,
				}
				identifier.logToken("")
				tokenizer.tokens = append(tokenizer.tokens, identifier)
				tokenizer.tokenBuffer.Reset()
			}
			continue
		} else if unicode.IsNumber(tokenizer.runeBuffer[tokenizer.currentIndex]) {
			startingIndex := tokenizer.currentIndex + 1
			for unicode.IsNumber(tokenizer.runeBuffer[tokenizer.currentIndex]) {
				if _, err := tokenizer.tokenBuffer.WriteRune(tokenizer.runeBuffer[tokenizer.currentIndex]); err != nil {
					fmt.Printf("ERROR: %v", err.Error())
					return
				}
				tokenizer.consumeRune()
			}
			identifier := Token{
				Type:   intLiteral,
				Value:  tokenizer.tokenBuffer.String(),
				Line:   tokenizer.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("")
			tokenizer.tokens = append(tokenizer.tokens, identifier)
			tokenizer.tokenBuffer.Reset()
			continue
		} else {
			startingIndex := tokenizer.currentIndex + 1
			tokenizer.tokenBuffer.WriteRune(tokenizer.runeBuffer[tokenizer.currentIndex])
			identifier := Token{
				Type:   invalidToken,
				Value:  tokenizer.tokenBuffer.String(),
				Line:   tokenizer.lineCount,
				Column: startingIndex,
			}
			identifier.logToken("Invalid token while parsing")
			tokenizer.tokenBuffer.Reset()
		}
		tokenizer.consumeRune()
	}
	destination, err := os.Create("./chaos_compiler.asm")
	if err != nil {
		return
	}
	defer destination.Close()
	exit := parseTokenExit(tokenizer.tokens)
	str := generateExitSyscal(exit)
	header := []string{
		"section .text\n",
		"    global _start\n\n",
		"_start:\n",
	}
	asd := strings.Join(header, "")
	asd = asd + str
	destination.Write([]byte(asd))
}
