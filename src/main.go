package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type TokenType int

type Token struct {
	value   string
	tokType TokenType
}

func isNum(b byte) bool {
	s := rune(b)
	return unicode.IsNumber(s)
}

func isAlpha(b byte) bool {
	s := rune(b)
	return unicode.IsLetter(s)
}

func isAlphanum(b byte) bool {
	s := rune(b)
	return unicode.IsNumber(s) || unicode.IsLetter(s)
}

func isInside(b byte, listChar []byte) bool {
	result := false
	for _, item := range listChar {
		if result = (b == item); result {
			break
		}
	}
	return result
}

func chaosTokenizer(file *[]byte) error {
	tokens := bytes.Buffer{}
	listTok := []Token{}
	pos := 0

	content := string(*file)
	singleTokens := []byte{'=', '+', ';'}

	for pos < len(content) {
		if unicode.IsSpace(rune(content[pos])) {
			pos++
			continue
		} else if isInside(content[pos], singleTokens) {
			tokens.WriteByte(content[pos])
			pos++
			token := Token{
				value: tokens.String(),
			}
			fmt.Printf("Found token: %s\n", token.value)
			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else if isNum(content[pos]) {
			for isNum(content[pos]) {
				tokens.WriteByte(content[pos])
				pos++
			}
			token := Token{
				value: tokens.String(),
			}
			fmt.Printf("Found token: %s\n", token.value)
			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else if isAlpha(content[pos]) {
			for isAlphanum(content[pos]) {
				tokens.WriteByte(content[pos])
				pos++
			}
			token := Token{
				value: tokens.String(),
			}
			fmt.Printf("Found token: %s\n", token.value)
			listTok = append(listTok, token)
			tokens.Reset()
			continue
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed while tokenizing - unknown character '%s'\n", string(content[pos]))
			os.Exit(1)
		}
	}
	return nil
}

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
			chaosTokenizer(&file)
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to the program %s\n", programName)
			fmt.Printf("Usage: %s <file.chaos>\n", programName)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
