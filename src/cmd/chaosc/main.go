package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"chaos_new/compiler"
)

func main() {
	os.Exit(run(filepath.Base(os.Args[0]), os.Args[1:], os.Stderr, os.Stdout))
}

// run executes the compiler CLI. Diagnostics are written to diagOut (stderr)
// and requested textual output to out (stdout). With no mode flag, the source
// is compiled through fasm into an executable.
func run(programName string, args []string, diagOut, out io.Writer) int {
	flags := flag.NewFlagSet(programName, flag.ContinueOnError)
	flags.SetOutput(diagOut)
	dump := flags.Bool("dump", false, "print the token stream and parsed AST")
	ir := flags.Bool("ir", false, "print the lowered HIR and MIR")
	asm := flags.Bool("asm", false, "emit backend assembly instead of an executable")
	check := flags.Bool("check", false, "type-check without requiring an entry point or emitting an artifact")
	outputPath := flags.String("o", "", "write the selected artifact to this path (default executable: a.out)")
	backendName := flags.String("backend", "fasm", "code-generation backend (supported: fasm)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if flags.NArg() != 1 {
		fmt.Fprintf(diagOut, "[ERROR] exactly one Chaos source file is required (got %d arguments; usage: %s [mode] [-o path] <file.chaos>)\n", flags.NArg(), programName)
		return 2
	}
	modeCount := 0
	for _, selected := range []bool{*dump, *ir, *asm, *check} {
		if selected {
			modeCount++
		}
	}
	if modeCount > 1 {
		fmt.Fprintln(diagOut, "[ERROR] choose exactly one of -check, -dump, -ir, or -asm")
		return 2
	}
	if *check && *outputPath != "" {
		fmt.Fprintln(diagOut, "[ERROR] -o cannot be used with -check because check mode produces no artifact")
		return 2
	}
	requiresBackend := !*check && !*dump && !*ir
	var backend compiler.Backend
	if requiresBackend {
		backend = compiler.NewBackend(*backendName)
		if backend == nil {
			fmt.Fprintf(diagOut, "[ERROR] unknown backend %q (supported: %s)\n", *backendName, strings.Join(compiler.SupportedBackends(), ", "))
			return 2
		}
	}

	path := flags.Arg(0)
	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to read Chaos source %q: %v\n", path, err)
		return 1
	}

	sm := &compiler.SourceManager{}
	fileID := sm.Register(path, source)
	sf := sm.Lookup(fileID)
	tokens, diags := compiler.Tokenize(source, fileID)
	if len(diags) > 0 {
		compiler.RenderAll(diagOut, diags, sf)
		if diags.HasErrors() {
			return 1
		}
	}

	result := compiler.ParseProgram(tokens)
	if result.Program != nil {
		result.Program.Sources = map[compiler.FileID]compiler.SourceFile{fileID: *sf}
	}
	if len(result.Diags) > 0 {
		compiler.RenderAll(diagOut, result.Diags, sf)
		if result.Diags.HasErrors() {
			return 1
		}
	}

	analysis, typeDiags := compiler.AnalyzeProgram(result.Program)
	if len(typeDiags) > 0 {
		compiler.RenderAll(diagOut, typeDiags, sf)
		if typeDiags.HasErrors() {
			return 1
		}
	}
	if *check {
		return 0
	}
	if *dump {
		var text strings.Builder
		fmt.Fprintf(&text, "Tokens:\n%s", compiler.DumpTokens(tokens))
		fmt.Fprintf(&text, "Parse result:\n%s", compiler.DumpParseResult(result))
		return writeSelectedOutput(*outputPath, text.String(), out, diagOut)
	}
	if requiresBackend {
		if targetDiags := compiler.ValidateTarget(result.Program, analysis, *backendName); len(targetDiags) > 0 {
			compiler.RenderAll(diagOut, targetDiags, sf)
			if targetDiags.HasErrors() {
				return 1
			}
		}
	}

	hir, hirDiags := compiler.LowerAnalyzedProgram(result.Program, analysis)
	if len(hirDiags) > 0 {
		compiler.RenderAll(diagOut, hirDiags, sf)
		if hirDiags.HasErrors() {
			return 1
		}
	}
	mir, mirDiags := compiler.LowerToMIR(hir)
	if len(mirDiags) > 0 {
		compiler.RenderAll(diagOut, mirDiags, sf)
		if mirDiags.HasErrors() {
			return 1
		}
	}
	if verifyDiags := compiler.VerifyMIR(mir); len(verifyDiags) > 0 {
		compiler.RenderAll(diagOut, verifyDiags, sf)
		if verifyDiags.HasErrors() {
			return 1
		}
	}
	if *ir {
		var text strings.Builder
		fmt.Fprintf(&text, "HIR:\n%s", compiler.DumpHIR(hir))
		fmt.Fprintf(&text, "MIR:\n%s", compiler.DumpMIR(mir))
		return writeSelectedOutput(*outputPath, text.String(), out, diagOut)
	}

	assembly, backendDiags := backend.Emit(mir)
	if len(backendDiags) > 0 {
		compiler.RenderAll(diagOut, backendDiags, sf)
		if backendDiags.HasErrors() {
			return 1
		}
	}
	if *asm {
		return writeSelectedOutput(*outputPath, assembly, out, diagOut)
	}

	executablePath := *outputPath
	if executablePath == "" {
		executablePath = "a.out"
	}
	return assembleFasm(assembly, executablePath, diagOut)
}

func writeSelectedOutput(path, contents string, out, diagOut io.Writer) int {
	if path == "" || path == "-" {
		if _, err := io.WriteString(out, contents); err != nil {
			fmt.Fprintf(diagOut, "[ERROR] failed to write output: %v\n", err)
			return 1
		}
		return 0
	}
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".chaosc-output-*")
	if err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to create output beside %q: %v\n", path, err)
		return 1
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := io.WriteString(temporary, contents); err != nil {
		temporary.Close()
		fmt.Fprintf(diagOut, "[ERROR] failed to write output %q: %v\n", path, err)
		return 1
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		fmt.Fprintf(diagOut, "[ERROR] failed to set output permissions for %q: %v\n", path, err)
		return 1
	}
	if err := temporary.Close(); err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to close output %q: %v\n", path, err)
		return 1
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to install output %q: %v\n", path, err)
		return 1
	}
	return 0
}

func assembleFasm(assembly, outputPath string, diagOut io.Writer) int {
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		fmt.Fprintln(diagOut, "[ERROR] fasm is required to produce an executable; install fasm or use -asm")
		return 1
	}
	asmFile, err := os.CreateTemp("", "chaosc-*.asm")
	if err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to create temporary assembly file: %v\n", err)
		return 1
	}
	asmPath := asmFile.Name()
	defer os.Remove(asmPath)
	if _, err := io.WriteString(asmFile, assembly); err != nil {
		asmFile.Close()
		fmt.Fprintf(diagOut, "[ERROR] failed to write temporary assembly file: %v\n", err)
		return 1
	}
	if err := asmFile.Close(); err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to close temporary assembly file: %v\n", err)
		return 1
	}

	dir := filepath.Dir(outputPath)
	temporaryOutput, err := os.CreateTemp(dir, ".chaosc-output-*")
	if err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to create output beside %q: %v\n", outputPath, err)
		return 1
	}
	temporaryPath := temporaryOutput.Name()
	if err := temporaryOutput.Close(); err != nil {
		os.Remove(temporaryPath)
		fmt.Fprintf(diagOut, "[ERROR] failed to prepare output %q: %v\n", outputPath, err)
		return 1
	}
	defer os.Remove(temporaryPath)
	command := exec.Command(fasmPath, asmPath, temporaryPath)
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(diagOut, "[ERROR] fasm failed: %v\n%s", err, output)
		return 1
	}
	if err := os.Chmod(temporaryPath, 0o755); err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to mark executable %q: %v\n", outputPath, err)
		return 1
	}
	if err := os.Rename(temporaryPath, outputPath); err != nil {
		fmt.Fprintf(diagOut, "[ERROR] failed to install executable %q: %v\n", outputPath, err)
		return 1
	}
	return 0
}
