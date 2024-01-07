package main

import (
	"fmt"
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
	fmt.Printf("%v\n", generate)
	// parsed := ParseProgram(generate)
	// stackData := StackData{
	// 	size:      0,
	// 	variables: map[string]VariableDetails{},
	// }
	// result := GenerateProgram(parsed, stackData)
	// file, err := os.OpenFile("chaos_compiler.asm", os.O_WRONLY|os.O_CREATE, 0666)
	// if err != nil {
	// 	fmt.Printf("ERROR: %s\n", err.Error())
	// 	return
	// }
	// defer file.Close()

	// file.Write([]byte(result))
}
