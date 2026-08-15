package main

import (
	"testing"

	"chaos_new/compiler"
)

// parseDoc tokenizes and parses source in tolerant mode, returning the program
// and its SourceFile.
func parseDoc(t *testing.T, source string) (*compiler.Program, *compiler.SourceFile) {
	t.Helper()
	sm := &compiler.SourceManager{}
	fileID := sm.Register("test.chaos", []byte(source))
	sf := sm.Lookup(fileID)
	tokens, _ := compiler.Tokenize(sf.Source, fileID)
	result := compiler.ParseProgramTolerant(tokens)
	return result.Program, sf
}

func TestDocumentSymbols(t *testing.T) {
	source := "main :: proc -> S64 { return 0; }\nversion :: 1;\ncount := 2;"
	program, sf := parseDoc(t, source)
	symbols := documentSymbols(program, sf)
	if len(symbols) != 3 {
		t.Fatalf("got %d symbols, want 3", len(symbols))
	}
	if symbols[0].Name != "main" || symbols[0].Kind != symbolKindFunction {
		t.Errorf("symbol[0] = %+v, want main/Function", symbols[0])
	}
	if symbols[1].Name != "version" || symbols[1].Kind != symbolKindConstant {
		t.Errorf("symbol[1] = %+v, want version/Constant", symbols[1])
	}
	if symbols[2].Name != "count" || symbols[2].Kind != symbolKindVariable {
		t.Errorf("symbol[2] = %+v, want count/Variable", symbols[2])
	}
	// selectionRange should cover just the name "main" (chars 0-4).
	if symbols[0].SelectionRange.Start.Character != 0 || symbols[0].SelectionRange.End.Character != 4 {
		t.Errorf("main selectionRange = %+v, want chars 0-4", symbols[0].SelectionRange)
	}
}

func TestDocumentSymbolsEntryProc(t *testing.T) {
	program, sf := parseDoc(t, "main :: #entry proc { return 0; }")
	symbols := documentSymbols(program, sf)
	if len(symbols) != 1 {
		t.Fatalf("got %d symbols, want 1", len(symbols))
	}
	if symbols[0].Name != "main" || symbols[0].Kind != symbolKindFunction {
		t.Errorf("symbol = %+v, want main/Function", symbols[0])
	}
}

func TestDocumentSymbolsMultiResultAndUnicode(t *testing.T) {
	program, sf := parseDoc(t, "divmod :: proc (a: S64, b: S64) -> S64, S64 { return 0; }\nαβ :: 42;")
	symbols := documentSymbols(program, sf)
	if len(symbols) != 2 {
		t.Fatalf("got %d symbols, want 2", len(symbols))
	}
	if symbols[0].Name != "divmod" {
		t.Errorf("symbol[0].Name = %q, want divmod", symbols[0].Name)
	}
	if symbols[1].Name != "αβ" {
		t.Errorf("symbol[1].Name = %q, want αβ", symbols[1].Name)
	}
	// Unicode name: 2 UTF-8 bytes = 2 UTF-16 units.
	if symbols[1].SelectionRange.End.Character != 2 {
		t.Errorf("αβ selectionRange end char = %d, want 2", symbols[1].SelectionRange.End.Character)
	}
}

func TestDocumentSymbolsStruct(t *testing.T) {
	source := "Something_New :: struct {\n\tfield1: String;\n\tfield2: U64;\n}"
	program, sf := parseDoc(t, source)
	symbols := documentSymbols(program, sf)
	if len(symbols) != 1 {
		t.Fatalf("got %d symbols, want 1", len(symbols))
	}
	if symbols[0].Name != "Something_New" || symbols[0].Kind != symbolKindStruct {
		t.Errorf("symbol[0] = %+v, want Something_New/Struct", symbols[0])
	}
	if len(symbols[0].Children) != 2 {
		t.Fatalf("children = %d, want 2", len(symbols[0].Children))
	}
	if symbols[0].Children[0].Name != "field1" || symbols[0].Children[0].Kind != symbolKindVariable {
		t.Errorf("child[0] = %+v, want field1/Variable", symbols[0].Children[0])
	}
	if symbols[0].Children[1].Name != "field2" {
		t.Errorf("child[1].Name = %q, want field2", symbols[0].Children[1].Name)
	}
}

func TestDocumentSymbolsError(t *testing.T) {
	source := "Hash_Table_Error :: error {\n\tGENERIC;\n\tOUT_OF_MEMORY;\n\tNOT_FOUND;\n}"
	program, sf := parseDoc(t, source)
	symbols := documentSymbols(program, sf)
	if len(symbols) != 1 {
		t.Fatalf("got %d symbols, want 1", len(symbols))
	}
	if symbols[0].Name != "Hash_Table_Error" || symbols[0].Kind != symbolKindEnum {
		t.Errorf("symbol[0] = %+v, want Hash_Table_Error/Enum", symbols[0])
	}
	if len(symbols[0].Children) != 3 {
		t.Fatalf("children = %d, want 3", len(symbols[0].Children))
	}
	if symbols[0].Children[0].Name != "GENERIC" || symbols[0].Children[0].Kind != symbolKindEnumMember {
		t.Errorf("child[0] = %+v, want GENERIC/EnumMember", symbols[0].Children[0])
	}
	if symbols[0].Children[1].Name != "OUT_OF_MEMORY" {
		t.Errorf("child[1].Name = %q, want OUT_OF_MEMORY", symbols[0].Children[1].Name)
	}
}

func TestDocumentSymbolsEnum(t *testing.T) {
	program, sf := parseDoc(t, "Color :: enum { RED: U8 = 1; GREEN; }")
	symbols := documentSymbols(program, sf)
	if len(symbols) != 1 || symbols[0].Name != "Color" || symbols[0].Kind != symbolKindEnum {
		t.Fatalf("enum symbols = %+v", symbols)
	}
	if len(symbols[0].Children) != 2 || symbols[0].Children[0].Name != "RED" || symbols[0].Children[1].Name != "GREEN" {
		t.Fatalf("enum children = %+v", symbols[0].Children)
	}
}
