package main

import (
	"bytes"
	"fmt"
	"os"
)

type TokenType int

const (
	tokGlobal TokenType = iota
	tokLet
	tokVariable
	tokType
	tokAssign
	tokNum
)

type Token struct {
	value   string
	tokType TokenType
}

func parseFile(filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		fmt.Fprintf(os.Stderr, "The provided file path '%s' doesn't exist\n", filePath)
		return err
	}
	file, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open the file for whatever reason\n")
		return err
	}

	tokens := bytes.Buffer{}
	actualString := string(file)
	size := len(actualString)
	listTok := []Token{}
	id := 0
	for {
		if id == size {
			break
		}
		if actualString[id] == ' ' {
			if tokens.String() == "global" {
				id++
				token := Token{
					value:   tokens.String(),
					tokType: tokGlobal,
				}
				listTok = append(listTok, token)
				tokens.Reset()
			}
			if tokens.String() == "let" {
				id++
				token := Token{
					value:   tokens.String(),
					tokType: tokLet,
				}
				listTok = append(listTok, token)
				tokens.Reset()
			}
			if 'a' < actualString[id] || actualString[id] > 'Z' {
				variableBuf := bytes.Buffer{}
				for {
					fmt.Printf("%s\n", string(actualString[id]))
					variableBuf.WriteByte(actualString[id])
					if actualString[id] == ' ' {
						id++
						token := Token{
							value:   variableBuf.String(),
							tokType: tokVariable,
						}
						listTok = append(listTok, token)
						variableBuf.Reset()
						break
					}
					id++
				}
			}
			fmt.Printf("%v\n", listTok)
		}
		tokens.WriteByte(actualString[id])
		id++
	}
	return nil
}

func main() {
	programName := os.Args[0]
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "Failed to the program %s\n", programName)
		fmt.Printf("Usage: %s <file.chaos>\n", programName)
		os.Exit(1)
	}

	otherArgs := os.Args[1:]
	for _, args := range otherArgs {
		parseFile(args)
	}

	return
}
