package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"reflect"
	"testing"
)

func TestSeverityStringAndLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		text     string
		level    slog.Level
	}{
		{name: "error", severity: SeverityError, text: "error", level: slog.LevelError},
		{name: "warning", severity: SeverityWarning, text: "warning", level: slog.LevelWarn},
		{name: "note", severity: SeverityNote, text: "note", level: slog.LevelInfo},
		{name: "unknown", severity: Severity(255), text: "unknown", level: slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.text {
				t.Errorf("String() = %q, want %q", got, tt.text)
			}
			if got := tt.severity.LogLevel(); got != tt.level {
				t.Errorf("LogLevel() = %s, want %s", got, tt.level)
			}
		})
	}
}

func TestDiagnosticList(t *testing.T) {
	var diagnostics DiagnosticList
	if diagnostics.HasErrors() {
		t.Fatal("empty diagnostic list reports errors")
	}

	warningSpan := Span{File: 1, Start: 2, End: 3}
	diagnostics.Warn(warningSpan, "warning message")
	if diagnostics.HasErrors() {
		t.Fatal("warning-only diagnostic list reports errors")
	}
	if got := diagnostics[0]; got.Severity != SeverityWarning || got.Span != warningSpan || got.Message != "warning message" {
		t.Errorf("warning diagnostic = %#v", got)
	}

	errorSpan := Span{File: 2, Start: 4, End: 5}
	diagnostics.Error(errorSpan, "error message")
	if !diagnostics.HasErrors() {
		t.Fatal("diagnostic list containing an error does not report errors")
	}
	if got := diagnostics[1]; got.Severity != SeverityError || got.Span != errorSpan || got.Message != "error message" {
		t.Errorf("error diagnostic = %#v", got)
	}
}

func TestLineOffsetsAndCoordinates(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []int
	}{
		{name: "empty", source: "", want: []int{0}},
		{name: "single line", source: "abc", want: []int{0}},
		{name: "multiple lines", source: "ab\ncd\n", want: []int{0, 3, 6}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildLineOffsets([]byte(tt.source)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildLineOffsets() = %v, want %v", got, tt.want)
			}
		})
	}

	if line, column := offsetToLineCol(4, nil); line != 1 || column != 5 {
		t.Errorf("offsetToLineCol(4, nil) = %d:%d, want 1:5", line, column)
	}

	offsets := BuildLineOffsets([]byte("ab\ncd\nef"))
	coordinates := []struct {
		offset       int
		line, column int
	}{
		{offset: 0, line: 1, column: 1},
		{offset: 2, line: 1, column: 3},
		{offset: 3, line: 2, column: 1},
		{offset: 5, line: 2, column: 3},
		{offset: 6, line: 3, column: 1},
		{offset: 8, line: 3, column: 3},
	}
	for _, coordinate := range coordinates {
		line, column := offsetToLineCol(coordinate.offset, offsets)
		if line != coordinate.line || column != coordinate.column {
			t.Errorf("offsetToLineCol(%d) = %d:%d, want %d:%d", coordinate.offset, line, column, coordinate.line, coordinate.column)
		}
	}
}

func TestDiagnosticRenderIncludesSourceContext(t *testing.T) {
	source := []byte("first\n\talpha\n")
	diagnostic := Diagnostic{
		Severity: SeverityWarning,
		Span:     Span{File: 3, Start: 7, End: 9},
		Message:  "example warning",
	}

	var output bytes.Buffer
	logger := newLogger(&output, logConfig{Level: slog.LevelInfo, Format: logFormatJSON})
	diagnostic.Render(logger, source, BuildLineOffsets(source))

	records := decodeDiagnosticRecords(t, output.Bytes())
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	assertDiagnosticField(t, record, "level", "WARN")
	assertDiagnosticField(t, record, "msg", "example warning")
	assertDiagnosticField(t, record, "diagnostic", "warning")
	assertDiagnosticField(t, record, "file", float64(3))
	assertDiagnosticField(t, record, "line", float64(2))
	assertDiagnosticField(t, record, "column", float64(2))
	assertDiagnosticField(t, record, "span_start", float64(7))
	assertDiagnosticField(t, record, "span_end", float64(9))
	assertDiagnosticField(t, record, "source_line", "\talpha")
	assertDiagnosticField(t, record, "underline", "\t^^")
}

func TestRenderAllHandlesClampedAndEmptySpans(t *testing.T) {
	source := []byte("abc\ndef")
	diagnostics := DiagnosticList{
		{Severity: SeverityError, Span: Span{Start: 1, End: 6}, Message: "wide error"},
		{Severity: SeverityNote, Span: Span{Start: len(source), End: len(source)}, Message: "eof note"},
	}

	var output bytes.Buffer
	logger := newLogger(&output, logConfig{Level: slog.LevelInfo, Format: logFormatJSON})
	RenderAll(logger, diagnostics, source, BuildLineOffsets(source))

	records := decodeDiagnosticRecords(t, output.Bytes())
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	assertDiagnosticField(t, records[0], "source_line", "abc")
	assertDiagnosticField(t, records[0], "underline", " ^^")
	if _, ok := records[1]["source_line"]; ok {
		t.Error("empty EOF span unexpectedly includes source_line")
	}
	assertDiagnosticField(t, records[1], "level", "INFO")
}

func decodeDiagnosticRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()

	var records []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("decode diagnostic record: %v", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan diagnostic records: %v", err)
	}
	return records
}

func assertDiagnosticField(t *testing.T, record map[string]any, key string, want any) {
	t.Helper()
	if got := record[key]; got != want {
		t.Errorf("%s = %v, want %v", key, got, want)
	}
}
