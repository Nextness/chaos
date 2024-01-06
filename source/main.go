package main

import (
	"fmt"
)

func main() {
	filePath := "./example/exit_with.chaos"

	chaosDataBuffer, err := ReadChaosFile(filePath)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		return
	}

	tokens, err := chaosDataBuffer.Tokenizer()
	if err != nil {
		return
	}

	fmt.Printf("tokens: %v\n", tokens)
}
