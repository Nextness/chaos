package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI invokes run with separate buffers for diagnostics and program
// output, returning the exit code and both buffers.
func runCLI(args []string) (int, *bytes.Buffer, *bytes.Buffer) {
	var diagOut, out bytes.Buffer
	code := run("chaosc", args, &diagOut, &out)
	return code, &diagOut, &out
}

func TestRunSuccessfulParse(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "input.chaos")
	if err := os.WriteFile(sourcePath, []byte("x :: 42;"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	code, diagOut, out := runCLI([]string{"-check", sourcePath})
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; diagnostics: %s", code, diagOut.String())
	}
	if diagOut.Len() != 0 || out.Len() != 0 {
		t.Errorf("expected no output on success, got diagnostics %q output %q", diagOut.String(), out.String())
	}
}

func TestRunRendersDiagnosticWithSuggestion(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "bad.chaos")
	if err := os.WriteFile(sourcePath, []byte("x :: $;"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	code, diagOut, _ := runCLI([]string{sourcePath})
	if code != 1 {
		t.Fatalf("run exit code = %d, want 1; diagnostics: %s", code, diagOut.String())
	}

	text := diagOut.String()
	if !strings.Contains(text, "[ERROR] 1:6:") {
		t.Errorf("output missing error header with location in %q", text)
	}
	if !strings.Contains(text, "unexpected character") {
		t.Errorf("output missing error reason in %q", text)
	}
	if !strings.Contains(text, "-> remove the character or replace it with a valid token") {
		t.Errorf("output missing suggestion in %q", text)
	}
}

func TestRunIRFlag(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "input.chaos")
	if err := os.WriteFile(sourcePath, []byte("main :: proc -> S64 {\n    x := 1 + 2;\n    return x;\n}"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	code, diagOut, out := runCLI([]string{"-ir", sourcePath})
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; diagnostics: %s", code, diagOut.String())
	}
	text := out.String()
	for _, want := range []string{"HIR:", "MIR:", "Proc main() -> S64", "Function main() -> S64", "bb0:"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
}

func TestRunAsmFlag(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "input.chaos")
	if err := os.WriteFile(sourcePath, []byte("#entry main :: proc -> S64 {\n    return 42;\n}"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	code, diagOut, out := runCLI([]string{"-asm", sourcePath})
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; diagnostics: %s", code, diagOut.String())
	}
	text := out.String()
	for _, want := range []string{"format ELF64 executable 3", "entry _start", "call chaos_fn_0", "mov rdi, rax", "syscall"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
}

func TestRunDefaultBuildsExecutable(t *testing.T) {
	temporaryDirectory := t.TempDir()
	sourcePath := filepath.Join(temporaryDirectory, "input.chaos")
	outputPath := filepath.Join(temporaryDirectory, "program")
	if err := os.WriteFile(sourcePath, []byte("#entry main :: proc -> S64 { return 37; }"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}
	code, diagOut, out := runCLI([]string{"-o", outputPath, sourcePath})
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; diagnostics: %s", code, diagOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("default compile wrote stdout: %q", out.String())
	}
	command := exec.Command(outputPath)
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 37 {
		t.Fatalf("executable result = %v, want exit status 37", err)
	}
}

func TestRunRejectsUnknownBackendBeforeCreatingArtifact(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "input.chaos")
	outputPath := filepath.Join(dir, "program")
	if err := os.WriteFile(sourcePath, []byte("#entry main :: proc -> S64 { return 0; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, diagOut, _ := runCLI([]string{"-backend", "missing", "-o", outputPath, sourcePath})
	if code != 2 || !strings.Contains(diagOut.String(), "unknown backend") || !strings.Contains(diagOut.String(), "fasm") {
		t.Fatalf("unknown backend: code=%d diagnostics=%q", code, diagOut.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("unknown backend left an artifact: %v", err)
	}
}

func TestRunTargetFailurePreservesExistingArtifact(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "input.chaos")
	outputPath := filepath.Join(dir, "output.asm")
	if err := os.WriteFile(sourcePath, []byte("#entry main :: proc -> S64 { x: F16 = 1.5; return 0; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, []byte("previous artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, diagOut, _ := runCLI([]string{"-asm", "-o", outputPath, sourcePath})
	if code != 1 || !strings.Contains(diagOut.String(), "F16 is not yet supported") {
		t.Fatalf("target failure: code=%d diagnostics=%q", code, diagOut.String())
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "previous artifact" {
		t.Fatalf("failed compile replaced artifact with %q", contents)
	}
}

func TestWriteSelectedOutputAtomicallyReplacesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dump.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := writeSelectedOutput(path, "new", &out, &diagnostics); code != 0 {
		t.Fatalf("writeSelectedOutput = %d: %s", code, diagnostics.String())
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "new" {
		t.Fatalf("output = %q, want new", contents)
	}
}

func TestRunControlAndFailurePaths(t *testing.T) {
	temporaryDirectory := t.TempDir()
	invalidSourcePath := filepath.Join(temporaryDirectory, "invalid.chaos")
	if err := os.WriteFile(invalidSourcePath, []byte("$"), 0o600); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{name: "help", args: []string{"-h"}, wantCode: 0, wantOutput: "Usage of"},
		{name: "unknown flag", args: []string{"-unknown"}, wantCode: 2, wantOutput: "flag provided but not defined"},
		{name: "missing source argument", args: nil, wantCode: 2, wantOutput: "exactly one Chaos source file is required"},
		{name: "too many source arguments", args: []string{"one.chaos", "two.chaos"}, wantCode: 2, wantOutput: "got 2 arguments"},
		{name: "missing source file", args: []string{filepath.Join(temporaryDirectory, "missing.chaos")}, wantCode: 1, wantOutput: "failed to read Chaos source"},
		{name: "tokenizer diagnostic", args: []string{invalidSourcePath}, wantCode: 1, wantOutput: "unexpected character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, diagOut, _ := runCLI(tt.args)
			if code != tt.wantCode {
				t.Errorf("run() = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(diagOut.String(), tt.wantOutput) {
				t.Errorf("output %q does not contain %q", diagOut.String(), tt.wantOutput)
			}
		})
	}
}
