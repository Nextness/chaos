package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func compileChaos(filepath string, src []byte) bool {
	fileContent := bytes.NewBuffer(src)
	tokens := TokenizeChaos(fileContent)

	lex := &Lexer{
		data:     tokens,
		filepath: filepath,
		count:    len(tokens),
		cursor:   0,
	}

	program := ASTCreateChaosProgram(lex)
	codeGen := GenerateCode(&program)

	os.WriteFile("testing.asm", codeGen.Bytes(), 0644)
	return true
}

func main() {
	debug := false
	// Handling panics so that the stacktrace from golang is not printed, only
	// the one for chaos compiler itself
	if !debug {
		defer func() {
			if r := recover(); r != nil {
				// debug.PrintStack() // Call this to print the stack trace
				os.Exit(1)
			}
		}()
	}

	programName := os.Args[0]
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
		fmt.Printf("Usage: %s <file.chaos>\n", programName)
		os.Exit(1)
	}

	otherArgs := os.Args[1:]
	for _, arg := range otherArgs {
		if strings.HasSuffix(arg, ".chaos") {
			if _, err := os.Stat(arg); err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] The provided file path '%s' doesn't exist\n", arg)
				os.Exit(1)
			}

			src, err := os.ReadFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to open the file for whatever reason\n")
				os.Exit(1)
			}
			compileChaos(arg, src)

		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to compile the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}
}
