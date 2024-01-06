package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type TokenizerMetadataT struct {
	runeBuffer               []rune
	currentIndex, bufferSize int
	tokenBuffer              bytes.Buffer
	tokens                   []Token
	lineCount                int
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
	tokenizer.Tokenizer()
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
