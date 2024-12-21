package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

type TokenType int

const (
	invalidToken TokenType = iota
	semiColon
	let
	valueAssignment
	typeAssignment
	intLiteral
)

type ChaosToken struct {
	Type         TokenType
	Value        string
	Line, Column int
	FilePath     string
}

func isAlphaNumeric(char byte) bool {
	return ((char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9'))
}

func main() {
	filePath, _ := filepath.Abs("./../example/assingment.chaos")

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Failed to open file - reason: %v", err.Error())
		return
	}

	index := 0
	var currentBuffer bytes.Buffer

	newBuffer := bytes.Runes(bytes.NewBuffer(fileContent).Bytes())
	bufferSize := len(newBuffer)
	for {
		if bufferSize <= index {
			break
		}

		char := newBuffer[index]
		currentBuffer.WriteRune(char)
		if currentBuffer.String() == " " || currentBuffer.String() == "\n" {
			currentBuffer.Reset()
			index++
			continue
		}

		if curByte := currentBuffer.Bytes(); isAlphaNumeric(curByte[0]) {
			for curByte = currentBuffer.Bytes(); isAlphaNumeric(curByte[0]); {
				char = newBuffer[index]
				currentBuffer.WriteRune(char)
				index++
			}
			currentBuffer.Reset()
			fmt.Printf("Found '%s'\n", string(char))
			continue
		}

		// if currentBuffer.String() == "let" {
		// 	fmt.Printf("Found 'let'\n")
		// 	currentBuffer.Reset()
		// 	index++
		// 	continue
		// }

		// if isAlphaNumeric(currentBuffer.Bytes()[0]) {
		// 	for isAlphaNumeric(currentBuffer.Bytes()[0]) {
		// 		fmt.Printf("Found '%s'", currentBuffer.String())
		// 		index++
		// 	}
		// 	currentBuffer.Reset()
		// 	continue
		// }
		index++
	}
	return
}
