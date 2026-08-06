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
		0, 6, 1, semTypeParameter, 0, // n
		0, 3, 3, semTypeType, 0, // S64 (param type)
		0, 8, 3, semTypeType, 0, // S64 (result)
		1, 0, 1, semTypeVariable, 0, // x
		0, 5, 2, semTypeNumber, 0, // 42
		1, 0, 6, semTypeKeyword, 0, // return
		0, 7, 1, semTypeVariable, 0, // x
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
		1, 0, 7, semTypeComment, 0, // // note
		1, 0, 3, semTypeVariable, 0, // msg
		0, 7, 4, semTypeString, 0, // «hi» (4 UTF-16 units)
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensConstAndCallee(t *testing.T) {
	source := "main :: proc {\nconst_val :: 7;\nz := add(x);\n}"
	want := []int{
		0, 0, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		1, 0, 9, semTypeConstant, 0, // const_val
		0, 13, 1, semTypeNumber, 0, // 7
		1, 0, 1, semTypeVariable, 0, // z
		0, 5, 3, semTypeFunction, 0, // add (callee)
		0, 4, 1, semTypeVariable, 0, // x
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}
