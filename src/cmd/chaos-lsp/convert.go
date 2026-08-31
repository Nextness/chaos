package main

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"chaos_compiler/compiler"
)

// severityToLSP maps a compiler severity to the LSP DiagnosticSeverity enum
// (error=1, warning=2, note=3).
func severityToLSP(s compiler.Severity) int {
	switch s {
	case compiler.SeverityError:
		return 1
	case compiler.SeverityWarning:
		return 2
	case compiler.SeverityNote:
		return 3
	default:
		return 3
	}
}

// convertDiagnostics converts compiler diagnostics to LSP diagnostics. The
// suggestion is appended to the message. Invalid internal spans are clamped
// by the compiler's shared reporting helper rather than crashing the server.
func convertDiagnostics(diags compiler.DiagnosticList, sf *compiler.SourceFile) []Diagnostic {
	out := []Diagnostic{}
	for _, d := range diags {
		// Imported modules are analyzed with the root document, but diagnostics
		// for their FileID are published when that document is itself open.
		if d.Span.File != sf.ID {
			continue
		}
		r := compiler.SpanToRange(d.Span, sf)
		msg := d.Message
		if d.Suggestion != "" {
			msg += " -> " + d.Suggestion
		}
		out = append(out, Diagnostic{
			Range: Range{
				Start: Position{Line: r.Start.Line, Character: r.Start.Character},
				End:   Position{Line: r.End.Line, Character: r.End.Character},
			},
			Severity: severityToLSP(d.Severity),
			Source:   "chaos",
			Message:  msg,
		})
	}
	return out
}

// applyEdits applies a sequence of content changes to text. A change without a
// range is a full-document replacement. Each edit is applied against the text
// as it is after the previous edit.
func applyEdits(text string, changes []TextDocumentContentChangeEvent) string {
	updated, ok := applyEditsChecked(text, changes)
	if !ok {
		return text
	}
	return updated
}

func applyEditsChecked(text string, changes []TextDocumentContentChangeEvent) (string, bool) {
	for _, change := range changes {
		if change.Range == nil {
			text = change.Text
			continue
		}
		start, startOK := positionToByteOffsetChecked(text, change.Range.Start)
		end, endOK := positionToByteOffsetChecked(text, change.Range.End)
		if !startOK || !endOK || start > end {
			return "", false
		}
		text = text[:start] + change.Text + text[end:]
	}
	return text, true
}

// positionToByteOffset converts an LSP position (0-based line, UTF-16
// character) to a byte offset in text.
func positionToByteOffset(text string, pos Position) int {
	if offset, ok := positionToByteOffsetChecked(text, pos); ok {
		return offset
	}
	if pos.Line < 0 || pos.Character < 0 {
		return 0
	}
	return len(text)
}

func positionToByteOffsetChecked(text string, pos Position) (int, bool) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, false
	}
	offset := 0
	for line := 0; line < pos.Line; line++ {
		idx := strings.IndexByte(text[offset:], '\n')
		if idx < 0 {
			return 0, false
		}
		offset += idx + 1
	}
	lineStart := offset
	relativeEnd := strings.IndexByte(text[offset:], '\n')
	lineEnd := len(text)
	if relativeEnd >= 0 {
		lineEnd = offset + relativeEnd
	}
	contentEnd := lineEnd
	if contentEnd > lineStart && text[contentEnd-1] == '\r' {
		contentEnd--
	}
	units := 0
	for offset < contentEnd {
		if units == pos.Character {
			return offset, true
		}
		r, size := utf8.DecodeRuneInString(text[offset:contentEnd])
		if size == 0 {
			break
		}
		width := 1
		if utf16.RuneLen(r) == 2 {
			width = 2
		}
		if units+width > pos.Character {
			return 0, false
		}
		units += width
		offset += size
	}
	if units == pos.Character {
		return contentEnd, true
	} else {
		return 0, false
	}
}
