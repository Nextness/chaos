package main

import (
	"bytes"
	"fmt"
	"os"
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

func main() {
	filePath := "./../example/assignment.chaos"

	buffer, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	newBuffer := bytes.NewBuffer(buffer)
	fileText := bytes.Runes(newBuffer.Bytes())

	fmt.Println(newBuffer)
	fmt.Println(fileText)

	return
}
