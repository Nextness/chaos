package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.chaos>\n", os.Args[0])
		os.Exit(1)
	}

	filepath := os.Args[1]
	source, err := os.ReadFile(filepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", filepath, err)
		os.Exit(1)
	}

	lineOffsets := BuildLineOffsets(source)

	tokens, diags := Tokenize(source, 0)

	if len(diags) > 0 {
		RenderAll(diags, source, lineOffsets)
		if diags.HasErrors() {
			os.Exit(1)
		}
	}

	// Print tokens
	for _, tok := range tokens {
		line, col := offsetToLineCol(tok.Span.Start, lineOffsets)
		if tok.Kind == TkEOF {
			fmt.Printf("%3d:%-3d  %-16s  EOF\n", line, col, tok.Kind.String())
			continue
		}
		if tok.Kind == TkInt || tok.Kind == TkFloat || tok.Kind == TkString {
			val := ""
			if tok.Value != nil {
				val = fmt.Sprintf(" %v", tok.Value)
			}
			fmt.Printf("%3d:%-3d  %-16s  %s%s\n", line, col, tok.Kind.String(), string(tok.Raw), val)
		} else if tok.Kind == TkTrue || tok.Kind == TkFalse {
			fmt.Printf("%3d:%-3d  %-16s  %v\n", line, col, tok.Kind.String(), tok.Value)
		} else {
			fmt.Printf("%3d:%-3d  %-16s  %s\n", line, col, tok.Kind.String(), string(tok.Raw))
		}
	}
}