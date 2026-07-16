package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogFlagsConfig(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	logging := registerLogFlags(flags)

	if err := flags.Parse([]string{"-log-level=warn", "-log-format=json", "-log-source"}); err != nil {
		t.Fatalf("parse logging flags: %v", err)
	}

	config, err := logging.config()
	if err != nil {
		t.Fatalf("build logging config: %v", err)
	}
	if config.Level != slog.LevelWarn {
		t.Errorf("level = %s, want %s", config.Level, slog.LevelWarn)
	}
	if config.Format != logFormatJSON {
		t.Errorf("format = %q, want %q", config.Format, logFormatJSON)
	}
	if !config.AddSource {
		t.Error("AddSource = false, want true")
	}
}

func TestLogFlagsRejectInvalidFormat(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	logging := registerLogFlags(flags)

	if err := flags.Parse([]string{"-log-format=yaml"}); err != nil {
		t.Fatalf("parse logging flags: %v", err)
	}
	if _, err := logging.config(); err == nil {
		t.Fatal("expected invalid log format to return an error")
	}
}

func TestLogFlagsRejectInvalidLevel(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	logging := registerLogFlags(flags)

	if err := flags.Parse([]string{"-log-level=verbose"}); err != nil {
		t.Fatalf("parse logging flags: %v", err)
	}
	if _, err := logging.config(); err == nil {
		t.Fatal("expected invalid log level to return an error")
	}
}

func TestJSONLoggerEmitsStructuredAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := newLogger(&output, logConfig{Level: slog.LevelInfo, Format: logFormatJSON})
	logger.Info(
		"token",
		slog.String("kind", "identifier"),
		slog.Int("line", 3),
	)

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode JSON log record: %v", err)
	}
	if record["msg"] != "token" {
		t.Errorf("msg = %v, want token", record["msg"])
	}
	if record["kind"] != "identifier" {
		t.Errorf("kind = %v, want identifier", record["kind"])
	}
	if record["line"] != float64(3) {
		t.Errorf("line = %v, want 3", record["line"])
	}
}

func TestTextLoggerEmitsStructuredAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := newLogger(&output, logConfig{Level: slog.LevelInfo, Format: logFormatText})
	logger.Info("token", slog.String("kind", "identifier"))

	text := output.String()
	if !strings.Contains(text, "level=INFO") || !strings.Contains(text, "msg=token") || !strings.Contains(text, "kind=identifier") {
		t.Errorf("unexpected text log record: %s", text)
	}
}

func TestRunEmitsStructuredTokenLogs(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "input.chaos")
	if err := os.WriteFile(sourcePath, []byte("x :: 42;"), 0o600); err != nil {
		t.Fatalf("write test source: %v", err)
	}

	var output bytes.Buffer
	if exitCode := run("chaosc", []string{"-log-format=json", sourcePath}, &output); exitCode != 0 {
		t.Fatalf("run exit code = %d, want 0; output: %s", exitCode, output.String())
	}

	recordCount := 0
	scanner := bufio.NewScanner(&output)
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("decode JSON log record %d: %v", recordCount, err)
		}
		if record["msg"] != "token" {
			t.Errorf("record %d msg = %v, want token", recordCount, record["msg"])
		}
		recordCount++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan log output: %v", err)
	}
	if recordCount != 5 {
		t.Errorf("record count = %d, want 5", recordCount)
	}
}

func TestRunControlAndFailurePaths(t *testing.T) {
	temporaryDirectory := t.TempDir()
	invalidSourcePath := filepath.Join(temporaryDirectory, "invalid.chaos")
	if err := os.WriteFile(invalidSourcePath, []byte("%"), 0o600); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{name: "help", args: []string{"-h"}, wantCode: 0, wantOutput: "log-format"},
		{name: "unknown flag", args: []string{"-unknown"}, wantCode: 2, wantOutput: "flag provided but not defined"},
		{name: "invalid log format", args: []string{"-log-format=yaml", "source.chaos"}, wantCode: 2, wantOutput: "invalid logging configuration"},
		{name: "invalid log level", args: []string{"-log-level=verbose", "source.chaos"}, wantCode: 2, wantOutput: "invalid logging configuration"},
		{name: "missing source argument", args: nil, wantCode: 2, wantOutput: "exactly one Chaos source file is required"},
		{name: "too many source arguments", args: []string{"one.chaos", "two.chaos"}, wantCode: 2, wantOutput: "argument_count=2"},
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
