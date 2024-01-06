package main

import (
	"fmt"
	"os"
)

func main() {
	compilationConfig, err := RunCmdArguments()
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return
	}

	chaosDataBuffer, err := ReadChaosFile(compilationConfig.filePath)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return
	}

	tokens, err := chaosDataBuffer.Tokenizer()
	if err != nil {
		return
	}
	generate := GenerateChaosParse(tokens)
	parsed := Parser(generate)
	result := GenerateAssembly(parsed)
	file, err := os.OpenFile("chaos_compiler.asm", os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return
	}
	defer file.Close()

	file.Write([]byte(result))
}
