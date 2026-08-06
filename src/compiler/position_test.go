package compiler

import (
	"testing"
	"unicode/utf8"
)

func TestOffsetToLineColRoundTrip(t *testing.T) {
	source := []byte("ab\ncd\nef")
	offsets := BuildLineOffsets(source)
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
	for _, c := range coordinates {
		line, column := OffsetToLineCol(c.offset, offsets)
		if line != c.line || column != c.column {
			t.Errorf("OffsetToLineCol(%d) = %d:%d, want %d:%d", c.offset, line, column, c.line, c.column)
		}
	}
}

func TestByteOffsetToUTF16(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		lineStart  int
		byteOffset int
		want       int
	}{
		{name: "ascii", source: "abc", lineStart: 0, byteOffset: 3, want: 3},
		{name: "empty", source: "", lineStart: 0, byteOffset: 0, want: 0},
		// « = 2 UTF-8 bytes, 1 UTF-16 unit; » = 2 UTF-8 bytes, 1 UTF-16 unit.
		{name: "guillemets", source: "«hello»", lineStart: 0, byteOffset: 9, want: 7},
		// Multi-byte rune (é = 2 UTF-8 bytes, 1 UTF-16 unit).
		{name: "multi-byte rune", source: "aéb", lineStart: 0, byteOffset: 4, want: 3},
		// Astral-plane rune (4 UTF-8 bytes, 2 UTF-16 units).
		{name: "astral plane", source: "a\U0001F600b", lineStart: 0, byteOffset: 6, want: 4},
		// Tabs count as 1 unit.
		{name: "tab", source: "\t", lineStart: 0, byteOffset: 1, want: 1},
		// CRLF: \r and \n each count as 1 unit.
		{name: "crlf", source: "a\r\nb", lineStart: 0, byteOffset: 4, want: 4},
		// Offset before line start clamps to 0.
		{name: "before line start", source: "abc", lineStart: 2, byteOffset: 1, want: 0},
		// Offset past end clamps to end.
		{name: "past end", source: "abc", lineStart: 0, byteOffset: 10, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ByteOffsetToUTF16([]byte(tt.source), tt.lineStart, tt.byteOffset); got != tt.want {
				t.Errorf("ByteOffsetToUTF16(%q, %d, %d) = %d, want %d", tt.source, tt.lineStart, tt.byteOffset, got, tt.want)
			}
		})
	}
}

func TestUTF16ToByteOffsetInverse(t *testing.T) {
	sources := []string{
		"abc",
		"«hello»",
		"aéb",
		"a\U0001F600b",
		"\t x",
		"a\r\nb",
	}
	for _, src := range sources {
		source := []byte(src)
		// Only rune-boundary offsets round-trip; mid-rune offsets are not
		// valid byte positions.
		boundaries := []int{0}
		for i := 0; i < len(source); {
			_, size := utf8.DecodeRune(source[i:])
			i += size
			boundaries = append(boundaries, i)
		}
		for _, byteOffset := range boundaries {
			units := ByteOffsetToUTF16(source, 0, byteOffset)
			got := UTF16ToByteOffset(source, 0, units)
			if got != byteOffset {
				t.Errorf("round-trip %q offset %d: UTF16ToByteOffset(ByteOffsetToUTF16(%d)) = %d, want %d", src, byteOffset, byteOffset, got, byteOffset)
			}
		}
	}
}

func TestSpanToRange(t *testing.T) {
	source := []byte("ab\n«x»\ncd")
	sf := &SourceFile{
		Path:        "test.chaos",
		Source:      source,
		LineOffsets: BuildLineOffsets(source),
	}

	// Span covering "«x»" on line 2 (0-based line 1): bytes [3, 8).
	// « = 1 unit, x = 1 unit, » = 1 unit → character 0..3.
	r := SpanToRange(Span{Start: 3, End: 8}, sf)
	if r.Start.Line != 1 || r.Start.Character != 0 {
		t.Errorf("start = %+v, want line 1 char 0", r.Start)
	}
	if r.End.Line != 1 || r.End.Character != 3 {
		t.Errorf("end = %+v, want line 1 char 3", r.End)
	}

	// Multiline span: from "ab" (line 0) to "cd" (line 2).
	r = SpanToRange(Span{Start: 0, End: len(source)}, sf)
	if r.Start.Line != 0 || r.Start.Character != 0 {
		t.Errorf("multiline start = %+v, want line 0 char 0", r.Start)
	}
	if r.End.Line != 2 || r.End.Character != 2 {
		t.Errorf("multiline end = %+v, want line 2 char 2", r.End)
	}

	// Empty span at EOF.
	r = SpanToRange(Span{Start: len(source), End: len(source)}, sf)
	if r.Start != r.End {
		t.Errorf("empty span start/end differ: %+v vs %+v", r.Start, r.End)
	}
	if r.Start.Line != 2 || r.Start.Character != 2 {
		t.Errorf("EOF position = %+v, want line 2 char 2", r.Start)
	}
}
