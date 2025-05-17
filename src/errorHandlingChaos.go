package main

import (
	"bytes"
	"fmt"
	"os"
)

type LexerErrorConfig struct {
	errorMsg     string
	examples     []string
	exampleCount int
}

func newLexerErroConfig(errorMsg string) LexerErrorConfig {
	return LexerErrorConfig{
		errorMsg:     errorMsg,
		examples:     []string{},
		exampleCount: 0,
	}
}

func (lec *LexerErrorConfig) newExample(examples ...string) {
	examplesLen := len(examples)
	assert(examplesLen > 0, "Expected at least one example")
	pad := "    "
	lec.exampleCount++
	prefix := fmt.Sprintf("(%d)", lec.exampleCount)
	for idx, example := range examples {
		if idx == 0 {
			tmp := fmt.Sprintf("%s%s %s\n", pad, prefix, example)
			lec.examples = append(lec.examples, tmp)
			continue
		}
		tmp := fmt.Sprintf("%s    %s\n", pad, example)
		lec.examples = append(lec.examples, tmp)
	}
}

func (lec LexerErrorConfig) printAndExitLexerError(ts *TokenizerState) {
	buffer := bytes.Buffer{}
	buffer.WriteString(fmt.Sprintf("%d:%d [ERROR] ", ts.current().getPosition().line, ts.current().getPosition().column))
	buffer.WriteString(lec.errorMsg)
	if len(lec.examples) > 0 {
		buffer.WriteString("Example:\n")
		for _, ex := range lec.examples {
			buffer.WriteString(ex)
		}
	}
	fmt.Print(buffer.String())
	os.Exit(1)
}
