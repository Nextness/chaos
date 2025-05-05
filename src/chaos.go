package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func main() {
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
			file, err := os.ReadFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] Failed to open the file for whatever reason\n")
				os.Exit(1)
			}

			fileContent := bytes.NewBuffer(file)
			tokenState:= tokenizeChaos(arg, fileContent)
			tokenState.print()
			fmt.Print("\n")

			lexerState := lexerChaos(tokenState)
			for _, ls := range lexerState.data {
				ls.print(0)
			}
			// fmt.Print("\n")

			// irState := irChaos(lexerState)
			// irChaosToString(irState)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}
}

