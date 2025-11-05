package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func compileChaos(filepath string, src []byte) bool {
	fileContent := bytes.NewBuffer(src)
	tokensSlice := ChaosContentTokenize(fileContent)
	program, scope := ChaosContentAST(&tokensSlice)
	ChaosInferAndCheckType(program, scope)
	// ChaosTypeCheck(program, scope)
	// chaosDebug(program.data)
	return true
}

func main() {
	defer panicHandler()

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
