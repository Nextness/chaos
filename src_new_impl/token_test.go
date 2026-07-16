package main

import (
	"reflect"
	"testing"
)

func TestSourceManagerRegisterAndLookup(t *testing.T) {
	manager := &SourceManager{}
	source := []byte("first\nsecond")
	fileID := manager.Register("example.chaos", source)

	if fileID != 0 {
		t.Errorf("first file ID = %d, want 0", fileID)
	}
	file := manager.Lookup(fileID)
	if file == nil {
		t.Fatal("registered file was not found")
	}
	if file.ID != fileID || file.Path != "example.chaos" || !reflect.DeepEqual(file.Source, source) {
		t.Errorf("registered file = %#v", file)
	}
	if want := []int{0, 6}; !reflect.DeepEqual(file.LineOffsets, want) {
		t.Errorf("line offsets = %v, want %v", file.LineOffsets, want)
	}
	if got := manager.Lookup(99); got != nil {
		t.Errorf("unknown file lookup = %#v, want nil", got)
	}
}

func TestTokenKindString(t *testing.T) {
	for i, want := range tokenKindNames {
		kind := TokenKind(i)
		if got := kind.String(); got != want {
			t.Errorf("TokenKind(%d).String() = %q, want %q", i, got, want)
		}
	}
}

func TestTokenKindStringPanicsForUnknownKind(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("TokenKind.String() did not panic for an unknown kind")
		}
	}()

	_ = TokenKind(len(tokenKindNames)).String()
}

func TestTokenText(t *testing.T) {
	token := Token{Raw: []byte("alpha")}
	if got := token.Text(); got != "alpha" {
		t.Errorf("Text() = %q, want %q", got, "alpha")
	}
}

func TestLookupKeyword(t *testing.T) {
	if kind, ok := LookupKeyword("return"); !ok || kind != TkReturn {
		t.Errorf("LookupKeyword(return) = %s, %t; want return, true", kind, ok)
	}
	if kind, ok := LookupKeyword("returns"); ok || kind != TkIdent {
		t.Errorf("LookupKeyword(returns) = %s, %t; want identifier, false", kind, ok)
	}
}
