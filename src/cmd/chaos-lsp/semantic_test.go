package main

import (
	"reflect"
	"testing"

	"chaos_new/compiler"
)

// semanticData tokenizes and parses source in tolerant mode and returns the
// delta-encoded semantic token data.
func semanticData(t *testing.T, source string) []int {
	t.Helper()
	sm := &compiler.SourceManager{}
	fileID := sm.Register("test.chaos", []byte(source))
	sf := sm.Lookup(fileID)
	tokens, _ := compiler.Tokenize(sf.Source, fileID)
	result := compiler.ParseProgramTolerant(tokens)
	all := append(tokenSemanticTokens(tokens, sf), astSemanticTokens(result.Program, sf)...)
	return encodeSemanticTokens(all)
}

func TestSemanticTokensBasic(t *testing.T) {
	source := "main :: proc (n: S64) -> S64 {\nx := 42;\nreturn x;\n}"
	want := []int{
		0, 0, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 5, 1, semTypeDelimiter, 0, // (
		0, 1, 1, semTypeParameter, 0, // n
		0, 3, 3, semTypeType, 0, // S64 (param type)
		0, 3, 1, semTypeDelimiter, 0, // )
		0, 5, 3, semTypeType, 0, // S64 (result)
		0, 4, 1, semTypeDelimiter, 0, // {
		1, 0, 1, semTypeVariable, 0, // x
		0, 5, 2, semTypeNumber, 0, // 42
		1, 0, 6, semTypeKeyword, 0, // return
		0, 7, 1, semTypeVariable, 0, // x
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensCommentAndString(t *testing.T) {
	source := "main :: proc {\n// note\nmsg := «hi»;\n}"
	want := []int{
		0, 0, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 5, 1, semTypeDelimiter, 0, // {
		1, 0, 7, semTypeComment, 0, // // note
		1, 0, 3, semTypeVariable, 0, // msg
		0, 7, 4, semTypeString, 0, // «hi» (4 UTF-16 units)
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensVarAndCallee(t *testing.T) {
	source := "main :: proc {\nconst_val :: 7;\nz := add(x);\n}"
	want := []int{
		0, 0, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 5, 1, semTypeDelimiter, 0, // {
		1, 0, 9, semTypeVariable, 0, // const_val (compile-time :: is a variable)
		0, 13, 1, semTypeNumber, 0, // 7
		1, 0, 1, semTypeVariable, 0, // z
		0, 5, 3, semTypeFunction, 0, // add (callee)
		0, 3, 1, semTypeDelimiter, 0, // (
		0, 1, 1, semTypeVariable, 0, // x
		0, 1, 1, semTypeDelimiter, 0, // )
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensEmpty(t *testing.T) {
	// An empty document must produce an empty (non-nil) data array so
	// Neovim's parser does not fail on a nil value.
	got := semanticData(t, "")
	if got == nil {
		t.Fatal("semantic data is nil, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("semantic data = %v, want empty", got)
	}
}

func TestSemanticTokensEntryProcDirectiveFirst(t *testing.T) {
	source := "#entry main :: proc () -> I64 {\n    return 10;\n}"
	want := []int{
		0, 0, 1, semTypeKeyword, 0, // #
		0, 1, 5, semTypeKeyword, 0, // entry
		0, 6, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 5, 1, semTypeDelimiter, 0, // (
		0, 1, 1, semTypeDelimiter, 0, // )
		0, 9, 1, semTypeDelimiter, 0, // { (I64 is not a builtin, so not highlighted)
		1, 4, 6, semTypeKeyword, 0, // return
		0, 7, 2, semTypeNumber, 0, // 10
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensCompileTimeVarIsVariable(t *testing.T) {
	source := "variable3 :: 10.1;"
	want := []int{
		0, 0, 9, semTypeVariable, 0, // variable3
		0, 13, 4, semTypeNumber, 0, // 10.1
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensStruct(t *testing.T) {
	source := "Something_New :: struct {\n    field1: String;\n    field2: U64;\n    field3: Bool;\n}"
	want := []int{
		0, 0, 13, semTypeType, 0, // Something_New
		0, 17, 6, semTypeKeyword, 0, // struct
		0, 7, 1, semTypeDelimiter, 0, // {
		1, 4, 6, semTypeVariable, 0, // field1
		0, 8, 6, semTypeType, 0, // String
		1, 4, 6, semTypeVariable, 0, // field2
		0, 8, 3, semTypeType, 0, // U64
		1, 4, 6, semTypeVariable, 0, // field3
		0, 8, 4, semTypeType, 0, // Bool
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensUndeclaredTypeNotHighlighted(t *testing.T) {
	// A type position referencing an undeclared name must not be highlighted
	// as a type; only the declared struct name and built-ins are.
	source := "Something_New :: struct {\n    a: Another;\n    b: S64;\n}"
	want := []int{
		0, 0, 13, semTypeType, 0, // Something_New
		0, 17, 6, semTypeKeyword, 0, // struct
		0, 7, 1, semTypeDelimiter, 0, // {
		1, 4, 1, semTypeVariable, 0, // a
		1, 4, 1, semTypeVariable, 0, // b
		0, 3, 3, semTypeType, 0, // S64 (built-in)
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}
