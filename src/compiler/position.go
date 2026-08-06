package compiler

import "unicode/utf8"

// OffsetToLineCol converts a byte offset to 1-based line/column. The column is
// a byte column (1-based), matching the CLI diagnostic renderer.
func OffsetToLineCol(offset int, lineOffsets []int) (int, int) {
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

// Position is a 0-based line/character position in UTF-16 code units, matching
// the LSP Position type.
type Position struct {
	Line      int
	Character int
}

// Range is a half-open range of Positions, matching the LSP Range type.
type Range struct {
	Start Position
	End   Position
}

// ByteOffsetToUTF16 returns the number of UTF-16 code units in
// source[lineStart:byteOffset]. Tabs and ASCII count as 1 unit; multi-byte
// runes count as 1 unit; astral-plane runes (4 UTF-8 bytes) count as 2 units.
func ByteOffsetToUTF16(source []byte, lineStart, byteOffset int) int {
	if byteOffset < lineStart {
		return 0
	}
	if byteOffset > len(source) {
		byteOffset = len(source)
	}
	units := 0
	for i := lineStart; i < byteOffset; {
		r, size := utf8.DecodeRune(source[i:byteOffset])
		if r == utf8.RuneError && size == 1 {
			// Invalid byte: count as a single code unit.
			units++
			i++
			continue
		}
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
		i += size
	}
	return units
}

// UTF16ToByteOffset returns the byte offset reached by advancing utf16Units
// UTF-16 code units from lineStart. It is the inverse of ByteOffsetToUTF16.
func UTF16ToByteOffset(source []byte, lineStart, utf16Units int) int {
	pos := lineStart
	units := 0
	for pos < len(source) && units < utf16Units {
		r, size := utf8.DecodeRune(source[pos:])
		if r == utf8.RuneError && size == 1 {
			// Invalid byte: count as a single code unit.
			units++
			pos++
			continue
		}
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
		pos += size
	}
	return pos
}

// lineIndexForOffset returns the 0-based line index containing the byte
// offset, using a binary search over the line-start offsets.
func lineIndexForOffset(offset int, lineOffsets []int) int {
	if len(lineOffsets) == 0 {
		return 0
	}
	lo, hi := 0, len(lineOffsets)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineOffsets[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// SpanToRange converts a byte span to a 0-based line/character range in UTF-16
// code units. The line is computed from LineOffsets; the character is computed
// from the line start in UTF-16 code units (not the byte column from
// OffsetToLineCol).
func SpanToRange(span Span, sf *SourceFile) Range {
	startLine := lineIndexForOffset(span.Start, sf.LineOffsets)
	startChar := ByteOffsetToUTF16(sf.Source, sf.LineOffsets[startLine], span.Start)
	endLine := lineIndexForOffset(span.End, sf.LineOffsets)
	endChar := ByteOffsetToUTF16(sf.Source, sf.LineOffsets[endLine], span.End)
	return Range{
		Start: Position{Line: startLine, Character: startChar},
		End:   Position{Line: endLine, Character: endChar},
	}
}
