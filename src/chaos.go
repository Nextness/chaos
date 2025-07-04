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
			tokens := TokenizeChaos(fileContent)
			ts := &LexerState{
				data:     tokens,
				filepath: arg,
				count:    len(tokens),
				cursor:   0,
			}
			ts.print()
			fmt.Print("\n")

			program := ASTCreateChaosProgram(ts)

			pp := PrettyPrint{
				padCount:       0,
				includeNewline: true,
			}
			size := len(program.nodes)
			fmt.Print("----------------------------------------------------------------------------------\n")
			for idx, program := range program.nodes {
				program.Print(pp)
				if idx+1 != size {
					fmt.Print("----------------------------------------------------------------------------------\n")
				}
			}
			fmt.Print("----------------------------------------------------------------------------------\n")

			for _, proc := range program.allocatedProcs {
				fmt.Printf("proc %s\n", proc)
			}

			for _, vars := range program.allocatedVariables {
				fmt.Printf("global def %s\n", vars)
			}

			// es := globalExecutionOrderChaos(program)
			// typeCheckingChaos(es)
			// irChaos(es)
			// generateProgram(instructions)

		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}
}
