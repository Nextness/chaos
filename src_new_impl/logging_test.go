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
