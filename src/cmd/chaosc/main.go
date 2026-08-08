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
	ir := flags.Bool("ir", false, "print the lowered HIR and MIR")
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

	// Type check the AST.
	typeDiags := compiler.CheckProgram(result.Program)
	if len(typeDiags) > 0 {
		compiler.RenderAll(output, typeDiags, sf)
		if typeDiags.HasErrors() {
			return 1
		}
	}

	if *dump {
		fmt.Fprintf(output, "Tokens:\n%s", compiler.DumpTokens(tokens))
		fmt.Fprintf(output, "Parse result:\n%s", compiler.DumpParseResult(result))
	}

	if *ir {
		hir, hirDiags := compiler.LowerProgram(result.Program)
		if len(hirDiags) > 0 {
			compiler.RenderAll(output, hirDiags, sf)
			if hirDiags.HasErrors() {
				return 1
			}
		}
		mir, mirDiags := compiler.LowerToMIR(hir)
		if len(mirDiags) > 0 {
			compiler.RenderAll(output, mirDiags, sf)
			if mirDiags.HasErrors() {
				return 1
			}
		}
		if verifyDiags := compiler.VerifyMIR(mir); len(verifyDiags) > 0 {
			compiler.RenderAll(output, verifyDiags, sf)
			if verifyDiags.HasErrors() {
				return 1
			}
		}
		fmt.Fprintf(output, "HIR:\n%s", compiler.DumpHIR(hir))
		fmt.Fprintf(output, "MIR:\n%s", compiler.DumpMIR(mir))
	}

	return 0
}
