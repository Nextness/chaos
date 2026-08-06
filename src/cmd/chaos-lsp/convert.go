package main

import (
	"strings"

	"chaos_new/compiler"
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
// suggestion is appended to the message. Diagnostics whose span exceeds the
// current source length are dropped rather than panicking.
func convertDiagnostics(diags compiler.DiagnosticList, sf *compiler.SourceFile) []Diagnostic {
	out := []Diagnostic{}
	for _, d := range diags {
		if d.Span.Start > len(sf.Source) || d.Span.End > len(sf.Source) {
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
	for _, change := range changes {
		if change.Range == nil {
			text = change.Text
			continue
		}
		start := positionToByteOffset(text, change.Range.Start)
		end := positionToByteOffset(text, change.Range.End)
		if start > end {
			start, end = end, start
		}
		text = text[:start] + change.Text + text[end:]
	}
	return text
}

// positionToByteOffset converts an LSP position (0-based line, UTF-16
// character) to a byte offset in text.
func positionToByteOffset(text string, pos Position) int {
	offset := 0
	for line := 0; line < pos.Line; line++ {
		idx := strings.IndexByte(text[offset:], '\n')
		if idx < 0 {
			return len(text)
		}
		offset += idx + 1
	}
	lineStart := offset
	lineEnd := strings.IndexByte(text[offset:], '\n')
	if lineEnd < 0 {
		lineEnd = len(text)
	} else {
		lineEnd += offset
	}
	byteOff := compiler.UTF16ToByteOffset([]byte(text), lineStart, pos.Character)
	if byteOff > lineEnd {
		byteOff = lineEnd
	}
	return byteOff
}
