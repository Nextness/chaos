package main

import (
	"bufio"
	"bytes"
	"testing"
)

func FuzzJSONRPCFrameNoPanic(f *testing.F) {
	f.Add([]byte("Content-Length: 2\r\n\r\n{}"))
	f.Add([]byte("Content-Length: 999999999\r\n\r\n"))
	f.Fuzz(func(t *testing.T, frame []byte) {
		if len(frame) > 128*1024 {
			t.Skip()
		}
		_, _ = readMessage(bufio.NewReader(bytes.NewReader(frame)))
	})
}

func FuzzLSPEditNoPanic(f *testing.F) {
	f.Add("a😀b\n", 0, 1, 0, 3, "x")
	f.Add("", -1, -1, 999, 999, "replacement")
	f.Fuzz(func(t *testing.T, text string, startLine, startChar, endLine, endChar int, replacement string) {
		if len(text)+len(replacement) > 128*1024 {
			t.Skip()
		}
		change := TextDocumentContentChangeEvent{
			Range: &Range{
				Start: Position{Line: startLine, Character: startChar},
				End:   Position{Line: endLine, Character: endChar},
			},
			Text: replacement,
		}
		_, _ = applyEditsChecked(text, []TextDocumentContentChangeEvent{change})
	})
}
