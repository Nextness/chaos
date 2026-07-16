package main

import (
	"context"
	"log/slog"
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

func (s Severity) LogLevel() slog.Level {
	switch s {
	case SeverityError:
		return slog.LevelError
	case SeverityWarning:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// Diagnostic is a single compiler message associated with a source location.
type Diagnostic struct {
	Severity Severity
	Span     Span
	Message  string
}

// DiagnosticList is a growing list of diagnostics. The tokenizer appends to it
// instead of panicking. The driver checks whether any errors were reported.
type DiagnosticList []Diagnostic

func (d *DiagnosticList) Error(span Span, msg string) {
	*d = append(*d, Diagnostic{Severity: SeverityError, Span: span, Message: msg})
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

// Render emits a structured diagnostic log record. It requires a line-index
// table to compute line:column from byte offsets.
func (d Diagnostic) Render(logger *slog.Logger, source []byte, lineOffsets []int) {
	line, col := offsetToLineCol(d.Span.Start, lineOffsets)
	attributes := []slog.Attr{
		slog.String("diagnostic", d.Severity.String()),
		slog.Int("file", int(d.Span.File)),
		slog.Int("line", line),
		slog.Int("column", col),
		slog.Int("span_start", d.Span.Start),
		slog.Int("span_end", d.Span.End),
	}

	// Attach the source line and caret underline to the same record.
	// Span is half-open [Start, End); End may equal len(source).
	if d.Span.Start < len(source) && d.Span.End <= len(source) && d.Span.Start < d.Span.End {
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
			attributes = append(attributes, slog.String("source_line", string(source[lineStart:lineEnd])))

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
			for i := 0; i < caretStart; i++ {
				if source[lineStart+i] == '\t' {
					underline.WriteByte('\t')
				} else {
					underline.WriteByte(' ')
				}
			}
			for i := caretStart; i < caretEnd; i++ {
				underline.WriteByte('^')
			}
			attributes = append(attributes, slog.String("underline", underline.String()))
		}
	}

	logger.LogAttrs(context.Background(), d.Severity.LogLevel(), d.Message, attributes...)
}

// RenderAll emits all diagnostics through logger.
func RenderAll(logger *slog.Logger, diags DiagnosticList, source []byte, lineOffsets []int) {
	for _, d := range diags {
		d.Render(logger, source, lineOffsets)
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
