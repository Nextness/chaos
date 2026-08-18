package compiler

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type Severity uint8

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityNote
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityNote:
		return "note"
	default:
		return "unknown"
	}
}

// Diagnostic is a single compiler message associated with a source location.
// Message states the reason for the diagnostic; Suggestion is an optional
// hint describing how to fix the problem.
type Diagnostic struct {
	Severity   Severity
	Span       Span
	Message    string
	Suggestion string
}

// DiagnosticList is a growing list of diagnostics. The tokenizer and parser
// append to it instead of panicking. The driver checks whether any errors
// were reported.
type DiagnosticList []Diagnostic

func (d *DiagnosticList) Error(span Span, msg, suggestion string) {
	*d = append(*d, Diagnostic{Severity: SeverityError, Span: span, Message: msg, Suggestion: suggestion})
}

func (d *DiagnosticList) Warn(span Span, msg, suggestion string) {
	*d = append(*d, Diagnostic{Severity: SeverityWarning, Span: span, Message: msg, Suggestion: suggestion})
}

func (d *DiagnosticList) HasErrors() bool {
	for _, diag := range *d {
		if diag.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Render writes a compact, rust-like diagnostic to w:
//
//	[ERROR] 1:5:file.chaos - reason
//	 source line
//	 ^^^ -> suggestion
//
// The caret underline is aligned with the diagnostic span. The source context
// is omitted when the span is empty or falls outside the source buffer; the
// suggestion is still emitted in that case.
func (d Diagnostic) Render(w io.Writer, sf *SourceFile) {
	d.Span, sf = ClampSpan(d.Span, sf)
	line, _ := OffsetToLineCol(d.Span.Start, sf.LineOffsets)
	lineStart := sf.LineOffsets[line-1]
	col := utf8.RuneCount(sf.Source[lineStart:d.Span.Start]) + 1
	fmt.Fprintf(w, "[%s] %d:%d:%s - %s\n", strings.ToUpper(d.Severity.String()), line, col, sf.Path, d.Message)

	// Span is half-open [Start, End); End may equal len(source).
	if d.Span.Start < len(sf.Source) && d.Span.End <= len(sf.Source) && d.Span.Start < d.Span.End {
		// Find the line boundaries around the span start.
		contextStart := d.Span.Start
		for contextStart > 0 && sf.Source[contextStart-1] != '\n' {
			contextStart--
		}
		lineEnd := d.Span.Start
		for lineEnd < len(sf.Source) && sf.Source[lineEnd] != '\n' {
			lineEnd++
		}
		if contextStart < lineEnd {
			fmt.Fprintf(w, " %s\n", string(sf.Source[contextStart:lineEnd]))

			// Caret underline: half-open [caretStart, caretEnd)
			caretStart := d.Span.Start - contextStart
			caretEnd := d.Span.End - contextStart
			if caretEnd > lineEnd-contextStart {
				caretEnd = lineEnd - contextStart
			}
			if caretEnd <= caretStart {
				caretEnd = caretStart + 1
			}

			var underline strings.Builder
			underline.WriteByte(' ')
			for _, r := range string(sf.Source[contextStart : contextStart+caretStart]) {
				if r == '\t' {
					underline.WriteByte('\t')
				} else {
					underline.WriteByte(' ')
				}
			}
			width := utf8.RuneCount(sf.Source[contextStart+caretStart : contextStart+caretEnd])
			if width < 1 {
				width = 1
			}
			for i := 0; i < width; i++ {
				underline.WriteByte('^')
			}
			if d.Suggestion != "" {
				fmt.Fprintf(w, "%s -> %s\n", underline.String(), d.Suggestion)
			} else {
				fmt.Fprintf(w, "%s\n", underline.String())
			}
			return
		}
	}
	if d.Suggestion != "" {
		fmt.Fprintf(w, " -> %s\n", d.Suggestion)
	}
}

// RenderAll writes all diagnostics to w in order.
func RenderAll(w io.Writer, diags DiagnosticList, sf *SourceFile) {
	for _, d := range diags {
		d.Render(w, sf)
	}
}

// BuildLineOffsets builds a table of byte offsets for each line start (0-indexed).
// lineOffsets[0] is always 0. The table is used by OffsetToLineCol.
func BuildLineOffsets(source []byte) []int {
	var offsets []int
	offsets = append(offsets, 0)
	for i, b := range source {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}
