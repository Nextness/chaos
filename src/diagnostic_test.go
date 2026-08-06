package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSeverityString(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		text     string
	}{
		{name: "error", severity: SeverityError, text: "error"},
		{name: "warning", severity: SeverityWarning, text: "warning"},
		{name: "note", severity: SeverityNote, text: "note"},
		{name: "unknown", severity: Severity(255), text: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.text {
				t.Errorf("String() = %q, want %q", got, tt.text)
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
	diagnostics.Warn(warningSpan, "warning message", "fix the warning")
	if diagnostics.HasErrors() {
		t.Fatal("warning-only diagnostic list reports errors")
	}
	if got := diagnostics[0]; got.Severity != SeverityWarning || got.Span != warningSpan || got.Message != "warning message" || got.Suggestion != "fix the warning" {
		t.Errorf("warning diagnostic = %#v", got)
	}

	errorSpan := Span{File: 2, Start: 4, End: 5}
	diagnostics.Error(errorSpan, "error message", "fix the error")
	if !diagnostics.HasErrors() {
		t.Fatal("diagnostic list containing an error does not report errors")
	}
	if got := diagnostics[1]; got.Severity != SeverityError || got.Span != errorSpan || got.Message != "error message" || got.Suggestion != "fix the error" {
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

func TestDiagnosticRenderIncludesSourceContextAndSuggestion(t *testing.T) {
	source := []byte("first\n\talpha\n")
	sf := &SourceFile{
		ID:          3,
		Path:        "test.chaos",
		Source:      source,
		LineOffsets: BuildLineOffsets(source),
	}
	diagnostic := Diagnostic{
		Severity:   SeverityWarning,
		Span:       Span{File: 3, Start: 7, End: 9},
		Message:    "example warning",
		Suggestion: "fix it",
	}

	var output bytes.Buffer
	diagnostic.Render(&output, sf)

	want := "[WARNING] 2:2:test.chaos - example warning\n \talpha\n \t^^ -> fix it\n"
	if got := output.String(); got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestRenderEmitsSuggestionWithoutSourceContext(t *testing.T) {
	source := []byte("x :: 42\n")
	sf := &SourceFile{
		Path:        "test.chaos",
		Source:      source,
		LineOffsets: BuildLineOffsets(source),
	}
	diagnostic := Diagnostic{
		Severity:   SeverityError,
		Span:       Span{Start: len(source), End: len(source)},
		Message:    "expected ;, got EOF",
		Suggestion: "add ';' here",
	}

	var output bytes.Buffer
	diagnostic.Render(&output, sf)

	want := "[ERROR] 2:1:test.chaos - expected ;, got EOF\n -> add ';' here\n"
	if got := output.String(); got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestRenderAllHandlesClampedAndEmptySpans(t *testing.T) {
	source := []byte("abc\ndef")
	sf := &SourceFile{
		Path:        "test.chaos",
		Source:      source,
		LineOffsets: BuildLineOffsets(source),
	}
	diagnostics := DiagnosticList{
		{Severity: SeverityError, Span: Span{Start: 1, End: 6}, Message: "wide error", Suggestion: "fix wide"},
		{Severity: SeverityNote, Span: Span{Start: len(source), End: len(source)}, Message: "eof note"},
	}

	var output bytes.Buffer
	RenderAll(&output, diagnostics, sf)

	text := output.String()
	if !strings.Contains(text, "[ERROR] 1:2:test.chaos - wide error") {
		t.Errorf("missing error header in %q", text)
	}
	if !strings.Contains(text, " ^^ -> fix wide") {
		t.Errorf("missing caret and suggestion in %q", text)
	}
	if !strings.Contains(text, "[NOTE] 2:4:test.chaos - eof note") {
		t.Errorf("missing note header in %q", text)
	}
	if strings.Contains(text, "eof note\n ") {
		t.Errorf("empty EOF span unexpectedly includes source context in %q", text)
	}
}
