package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"chaos_new/compiler"
)

func main() {
	os.Exit(run(filepath.Base(os.Args[0]), os.Args[1:], os.Stderr))
}

func run(programName string, args []string, output io.Writer) int {
	flags := flag.NewFlagSet(programName, flag.ContinueOnError)
	flags.SetOutput(output)
	dump := flags.Bool("dump", false, "print the token stream and parsed AST")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if flags.NArg() != 1 {
		fmt.Fprintf(output, "[ERROR] exactly one Chaos source file is required (got %d arguments; usage: %s <file.chaos>)\n", flags.NArg(), programName)
		return 2
	}

	path := flags.Arg(0)
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(output, "[ERROR] failed to read Chaos source %q: %v\n", path, err)
		return 1
	}

	sm := &compiler.SourceManager{}
	fileID := sm.Register(path, source)
	sf := sm.Lookup(fileID)

	tokens, diags := compiler.Tokenize(source, fileID)

	if len(diags) > 0 {
		compiler.RenderAll(output, diags, sf)
		if diags.HasErrors() {
			return 1
		}
	}

	// Parse the tokens into an AST.
	result := compiler.ParseProgram(tokens)
	if len(result.Diags) > 0 {
		compiler.RenderAll(output, result.Diags, sf)
		if result.Diags.HasErrors() {
			return 1
		}
	}

	if *dump {
		fmt.Fprintf(output, "Tokens:\n%s", compiler.DumpTokens(tokens))
		fmt.Fprintf(output, "Parse result:\n%s", compiler.DumpParseResult(result))
	}

	return 0
}
