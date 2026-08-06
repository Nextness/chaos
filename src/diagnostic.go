package main

import (
	"fmt"
	"io"
	"strings"
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
	line, col := offsetToLineCol(d.Span.Start, sf.LineOffsets)
	fmt.Fprintf(w, "[%s] %d:%d:%s - %s\n", strings.ToUpper(d.Severity.String()), line, col, sf.Path, d.Message)

	// Span is half-open [Start, End); End may equal len(source).
	if d.Span.Start < len(sf.Source) && d.Span.End <= len(sf.Source) && d.Span.Start < d.Span.End {
		// Find the line boundaries around the span start.
		lineStart := d.Span.Start
		for lineStart > 0 && sf.Source[lineStart-1] != '\n' {
			lineStart--
		}
		lineEnd := d.Span.Start
		for lineEnd < len(sf.Source) && sf.Source[lineEnd] != '\n' {
			lineEnd++
		}
		if lineStart < lineEnd {
			fmt.Fprintf(w, " %s\n", string(sf.Source[lineStart:lineEnd]))

			// Caret underline: half-open [caretStart, caretEnd)
			caretStart := d.Span.Start - lineStart
			caretEnd := d.Span.End - lineStart
			if caretEnd > lineEnd-lineStart {
				caretEnd = lineEnd - lineStart
			}
			if caretEnd <= caretStart {
				caretEnd = caretStart + 1
			}

			var underline strings.Builder
			underline.WriteByte(' ')
			for i := 0; i < caretStart; i++ {
				if sf.Source[lineStart+i] == '\t' {
					underline.WriteByte('\t')
				} else {
					underline.WriteByte(' ')
				}
			}
			for i := caretStart; i < caretEnd; i++ {
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
// lineOffsets[0] is always 0. The table is used by offsetToLineCol.
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

// offsetToLineCol converts a byte offset to 1-based line/column.
func offsetToLineCol(offset int, lineOffsets []int) (int, int) {
	if len(lineOffsets) == 0 {
		return 1, offset + 1
	}
	// Binary search for the last line offset <= offset.
	lo, hi := 0, len(lineOffsets)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineOffsets[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	line := lo + 1
	col := offset - lineOffsets[lo] + 1
	return line, col
}
