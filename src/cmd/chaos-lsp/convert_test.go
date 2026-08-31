package main

import (
	"testing"

	"chaos_compiler/compiler"
)

func TestSeverityToLSP(t *testing.T) {
	tests := []struct {
		sev  compiler.Severity
		want int
	}{
		{compiler.SeverityError, 1},
		{compiler.SeverityWarning, 2},
		{compiler.SeverityNote, 3},
		{compiler.Severity(255), 3},
	}
	for _, tt := range tests {
		if got := severityToLSP(tt.sev); got != tt.want {
			t.Errorf("severityToLSP(%d) = %d, want %d", tt.sev, got, tt.want)
		}
	}
}

func TestConvertDiagnostics(t *testing.T) {
	source := []byte("x :: 42;\n")
	sf := &compiler.SourceFile{
		Path:        "test.chaos",
		Source:      source,
		LineOffsets: compiler.BuildLineOffsets(source),
	}
	diags := compiler.DiagnosticList{
		{Severity: compiler.SeverityError, Span: compiler.Span{Start: 0, End: 1}, Message: "boom", Suggestion: "fix it"},
	}
	out := convertDiagnostics(diags, sf)
	if len(out) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(out))
	}
	if out[0].Severity != 1 {
		t.Errorf("severity = %d, want 1", out[0].Severity)
	}
	if out[0].Message != "boom -> fix it" {
		t.Errorf("message = %q, want %q", out[0].Message, "boom -> fix it")
	}
	if out[0].Range.Start.Line != 0 || out[0].Range.Start.Character != 0 {
		t.Errorf("start = %+v, want line 0 char 0", out[0].Range.Start)
	}
	if out[0].Range.End.Line != 0 || out[0].Range.End.Character != 1 {
		t.Errorf("end = %+v, want line 0 char 1", out[0].Range.End)
	}
}

func TestConvertDiagnosticsClampsOutOfRange(t *testing.T) {
	source := []byte("x :: 42;")
	sf := &compiler.SourceFile{
		Path:        "test.chaos",
		Source:      source,
		LineOffsets: compiler.BuildLineOffsets(source),
	}
	diags := compiler.DiagnosticList{
		{Severity: compiler.SeverityError, Span: compiler.Span{Start: 100, End: 101}, Message: "out of range"},
	}
	out := convertDiagnostics(diags, sf)
	if len(out) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(out))
	}
	want := Position{Line: 0, Character: 8}
	if out[0].Range.Start != want || out[0].Range.End != want {
		t.Errorf("clamped range = %+v, want EOF %+v", out[0].Range, want)
	}
}

func TestConvertDiagnosticsExcludesImportedFiles(t *testing.T) {
	sm := &compiler.SourceManager{}
	rootID := sm.Register("root.chaos", []byte("value"))
	importID := sm.Register("module.chaos", []byte("bad"))
	sf := sm.Lookup(rootID)
	diags := compiler.DiagnosticList{
		{Severity: compiler.SeverityError, Span: compiler.Span{File: rootID, Start: 0, End: 5}, Message: "root"},
		{Severity: compiler.SeverityError, Span: compiler.Span{File: importID, Start: 0, End: 3}, Message: "import"},
	}
	out := convertDiagnostics(diags, sf)
	if len(out) != 1 || out[0].Message != "root" {
		t.Fatalf("convertDiagnostics() = %+v, want only the root diagnostic", out)
	}
}

func TestApplyEdits(t *testing.T) {
	// Full-document replacement.
	if got := applyEdits("old", []TextDocumentContentChangeEvent{{Text: "new"}}); got != "new" {
		t.Errorf("full replacement = %q, want %q", got, "new")
	}
	// Incremental edit: replace "42" with "43" in "x :: 42;".
	got := applyEdits("x :: 42;", []TextDocumentContentChangeEvent{
		{Range: &Range{Start: Position{Line: 0, Character: 5}, End: Position{Line: 0, Character: 7}}, Text: "43"},
	})
	if got != "x :: 43;" {
		t.Errorf("incremental edit = %q, want %q", got, "x :: 43;")
	}
}

func TestPositionToByteOffset(t *testing.T) {
	text := "ab\ncd\nef"
	tests := []struct {
		pos  Position
		want int
	}{
		{Position{Line: 0, Character: 0}, 0},
		{Position{Line: 0, Character: 2}, 2},
		{Position{Line: 1, Character: 0}, 3},
		{Position{Line: 1, Character: 2}, 5},
		{Position{Line: 2, Character: 2}, 8},
		{Position{Line: 5, Character: 0}, len(text)},
	}
	for _, tt := range tests {
		if got := positionToByteOffset(text, tt.pos); got != tt.want {
			t.Errorf("positionToByteOffset(%+v) = %d, want %d", tt.pos, got, tt.want)
		}
	}
}
