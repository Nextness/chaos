package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSuccessfulParse(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "input.chaos")
	if err := os.WriteFile(sourcePath, []byte("x :: 42;"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	var output bytes.Buffer
	if exitCode := run("chaosc", []string{sourcePath}, &output); exitCode != 0 {
		t.Fatalf("run exit code = %d, want 0; output: %s", exitCode, output.String())
	}
	if output.Len() != 0 {
		t.Errorf("expected no output on success, got %q", output.String())
	}
}

func TestRunRendersDiagnosticWithSuggestion(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "bad.chaos")
	if err := os.WriteFile(sourcePath, []byte("x :: $;"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	var output bytes.Buffer
	if exitCode := run("chaosc", []string{sourcePath}, &output); exitCode != 1 {
		t.Fatalf("run exit code = %d, want 1; output: %s", exitCode, output.String())
	}

	text := output.String()
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

	var output bytes.Buffer
	if exitCode := run("chaosc", []string{"-ir", sourcePath}, &output); exitCode != 0 {
		t.Fatalf("run exit code = %d, want 0; output: %s", exitCode, output.String())
	}
	text := output.String()
	for _, want := range []string{"HIR:", "MIR:", "Proc main() -> S64", "Function main() -> S64", "bb0:"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
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
			var output bytes.Buffer
			if got := run("chaosc", tt.args, &output); got != tt.wantCode {
				t.Errorf("run() = %d, want %d", got, tt.wantCode)
			}
			if !strings.Contains(output.String(), tt.wantOutput) {
				t.Errorf("output %q does not contain %q", output.String(), tt.wantOutput)
			}
		})
	}
}
