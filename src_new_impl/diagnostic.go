package main

import (
	"fmt"
	"os"
)

// ─── Severity ────────────────────────────────────────────────────────────────

type Severity uint8

const (
	SeverityError   Severity = iota
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

// ─── Diagnostic ──────────────────────────────────────────────────────────────

// Diagnostic is a single compiler message associated with a source location.
type Diagnostic struct {
	Severity Severity
	Span     Span
	Message  string
}

// ─── DiagnosticList ──────────────────────────────────────────────────────────

// DiagnosticList is a growing list of diagnostics. The tokenizer appends to it
// instead of panicking. The driver checks whether any errors were reported.
type DiagnosticList []Diagnostic

func (d *DiagnosticList) Error(span Span, msg string) {
	*d = append(*d, Diagnostic{Severity: SeverityError, Span: span, Message: msg})
}

func (d *DiagnosticList) Errorf(span Span, format string, args ...any) {
	d.Error(span, fmt.Sprintf(format, args...))
}

func (d *DiagnosticList) Warn(span Span, msg string) {
	*d = append(*d, Diagnostic{Severity: SeverityWarning, Span: span, Message: msg})
}

func (d *DiagnosticList) HasErrors() bool {
	for _, diag := range *d {
		if diag.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ─── Rendering ───────────────────────────────────────────────────────────────

const colorRed = "\033[1;31m"
const colorYellow = "\033[1;33m"
const colorCyan = "\033[1;36m"
const colorReset = "\033[0m"

// Render writes a human-readable diagnostic to stderr. It requires a
// line-index table to compute line:column from byte offsets.
func (d Diagnostic) Render(source []byte, lineOffsets []int) {
	line, col := offsetToLineCol(d.Span.Start, lineOffsets)
	var color string
	switch d.Severity {
	case SeverityError:
		color = colorRed
	case SeverityWarning:
		color = colorYellow
	default:
		color = colorCyan
	}
	fmt.Fprintf(os.Stderr, "%s[%s]%s %d:%d:%d: %s\n",
		color, d.Severity, colorReset,
		int(d.Span.File), line, col, d.Message,
	)

	// Print a source line with a caret underline.
	if d.Span.Start < len(source) && d.Span.End <= len(source) && d.Span.Start <= d.Span.End {
		// Find the line start
		lineStart := d.Span.Start
		for lineStart > 0 && source[lineStart-1] != '\n' {
			lineStart--
		}
		lineEnd := d.Span.Start
		for lineEnd < len(source) && source[lineEnd] != '\n' {
			lineEnd++
		}
		if lineStart < lineEnd {
			fmt.Fprintf(os.Stderr, "  %s\n", string(source[lineStart:lineEnd]))
			// Caret underline
			caretStart := d.Span.Start - lineStart
			caretEnd := d.Span.End - lineStart
			if caretEnd > lineEnd-lineStart {
				caretEnd = lineEnd - lineStart
			}
			if caretEnd < caretStart {
				caretEnd = caretStart + 1
			}
			fmt.Fprintf(os.Stderr, "  ")
			for i := 0; i < caretStart; i++ {
				if source[lineStart+i] == '\t' {
					fmt.Fprintf(os.Stderr, "\t")
				} else {
					fmt.Fprintf(os.Stderr, " ")
				}
			}
			fmt.Fprintf(os.Stderr, "%s", colorRed)
			for i := caretStart; i < caretEnd; i++ {
				fmt.Fprintf(os.Stderr, "^")
			}
			fmt.Fprintf(os.Stderr, "%s\n", colorReset)
		}
	}
}

// RenderAll writes all diagnostics to stderr.
func RenderAll(diags DiagnosticList, source []byte, lineOffsets []int) {
	for _, d := range diags {
		d.Render(source, lineOffsets)
	}
}

// ─── Line offset table ───────────────────────────────────────────────────────

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