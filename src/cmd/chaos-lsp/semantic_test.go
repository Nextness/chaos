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

func TestSemanticTokensStructInitTypeHighlighted(t *testing.T) {
	// The type name in an explicit struct literal (TypeName.{...}) must be
	// highlighted as a type. Field names are not highlighted; field values are.
	source := "Something_New :: struct {\n    field1: String;\n    field2: U64;\n    field3: Bool;\n}\nmain :: proc {\n    x := Something_New.{field1=«hello», field2=10, field3=true};\n}"
	want := []int{
		0, 0, 13, semTypeType, 0, // Something_New (struct decl)
		0, 17, 6, semTypeKeyword, 0, // struct
		0, 7, 1, semTypeDelimiter, 0, // {
		1, 4, 6, semTypeVariable, 0, // field1
		0, 8, 6, semTypeType, 0, // String
		1, 4, 6, semTypeVariable, 0, // field2
		0, 8, 3, semTypeType, 0, // U64
		1, 4, 6, semTypeVariable, 0, // field3
		0, 8, 4, semTypeType, 0, // Bool
		1, 0, 1, semTypeDelimiter, 0, // }
		1, 0, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 5, 1, semTypeDelimiter, 0, // {
		1, 4, 1, semTypeVariable, 0, // x
		0, 5, 13, semTypeType, 0, // Something_New (struct literal type)
		0, 14, 1, semTypeDelimiter, 0, // {
		0, 8, 7, semTypeString, 0, // «hello»
		0, 16, 2, semTypeNumber, 0, // 10
		0, 11, 4, semTypeKeyword, 0, // true
		0, 4, 1, semTypeDelimiter, 0, // }
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensProcWithIfReturnAndCalls(t *testing.T) {
	// A procedure with parameters, a return type, an if/return body, and
	// calls used both as an initializer and as a bare statement. The '>'
	// comparison operator is not highlighted as a delimiter.
	source := "some_procedure :: proc (param_1: String, param_2: S64) -> String {\n    if param_2 > 10 {\n        return «Not valid»;\n    }\n    return param_1;\n}\nmain :: proc {\n    calling := some_procedure(«Valid», 1);\n    some_procedure(«Valid», 20);\n}"
	want := []int{
		0, 0, 14, semTypeFunction, 0, // some_procedure
		0, 18, 4, semTypeKeyword, 0, // proc
		0, 5, 1, semTypeDelimiter, 0, // (
		0, 1, 7, semTypeParameter, 0, // param_1
		0, 9, 6, semTypeType, 0, // String
		0, 8, 7, semTypeParameter, 0, // param_2
		0, 9, 3, semTypeType, 0, // S64
		0, 3, 1, semTypeDelimiter, 0, // )
		0, 5, 6, semTypeType, 0, // String (result)
		0, 7, 1, semTypeDelimiter, 0, // {
		1, 4, 2, semTypeKeyword, 0, // if
		0, 3, 7, semTypeVariable, 0, // param_2
		0, 10, 2, semTypeNumber, 0, // 10
		0, 3, 1, semTypeDelimiter, 0, // {
		1, 8, 6, semTypeKeyword, 0, // return
		0, 7, 11, semTypeString, 0, // «Not valid»
		1, 4, 1, semTypeDelimiter, 0, // }
		1, 4, 6, semTypeKeyword, 0, // return
		0, 7, 7, semTypeVariable, 0, // param_1
		1, 0, 1, semTypeDelimiter, 0, // }
		1, 0, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 5, 1, semTypeDelimiter, 0, // {
		1, 4, 7, semTypeVariable, 0, // calling
		0, 11, 14, semTypeFunction, 0, // some_procedure (callee)
		0, 14, 1, semTypeDelimiter, 0, // (
		0, 1, 7, semTypeString, 0, // «Valid»
		0, 9, 1, semTypeNumber, 0, // 1
		0, 1, 1, semTypeDelimiter, 0, // )
		1, 4, 14, semTypeFunction, 0, // some_procedure (callee)
		0, 14, 1, semTypeDelimiter, 0, // (
		0, 1, 7, semTypeString, 0, // «Valid»
		0, 9, 2, semTypeNumber, 0, // 20
		0, 2, 1, semTypeDelimiter, 0, // )
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

// decodeSemanticData reverses the delta encoding, returning the absolute
// token positions.
func decodeSemanticData(data []int) []semanticToken {
	var out []semanticToken
	line, start := 0, 0
	for i := 0; i+4 < len(data); i += 5 {
		line += data[i]
		if data[i] == 0 {
			start += data[i+1]
		} else {
			start = data[i+1]
		}
		out = append(out, semanticToken{line: line, startChar: start, length: data[i+2], typeIndex: data[i+3]})
	}
	return out
}

// tokenAt returns the semantic token starting at the given line and column.
func tokenAt(tokens []semanticToken, line, start int) (semanticToken, bool) {
	for _, tok := range tokens {
		if tok.line == line && tok.startChar == start {
			return tok, true
		}
	}
	return semanticToken{}, false
}

func TestSemanticTokensGenericProcs(t *testing.T) {
	// Every procedure form must highlight the function name as a function,
	// including generic type parameters before '::'.
	source := "function1 :: proc (input1: String) { }\n" +
		"function2 :: proc (input1: String) -> String { }\n" +
		"function3 :: proc -> String { }\n" +
		"function4 :: proc { }\n" +
		"function5 <T: String | S64> :: proc (input1: T) { }\n" +
		"function6 <T: String | S64> :: proc (input: String) -> T { }\n" +
		"function7 <T: String | S64> :: proc -> T { }\n"
	tokens := decodeSemanticData(semanticData(t, source))
	for line := 0; line < 7; line++ {
		tok, ok := tokenAt(tokens, line, 0)
		if !ok {
			t.Errorf("no token at line %d col 0", line)
			continue
		}
		if tok.typeIndex != semTypeFunction {
			t.Errorf("token at line %d col 0 has type %d, want %d (function)", line, tok.typeIndex, semTypeFunction)
		}
		if tok.length != 9 {
			t.Errorf("token at line %d col 0 has length %d, want 9", line, tok.length)
		}
	}
}

func TestSemanticTokensGenericEntryProc(t *testing.T) {
	// '#entry name <T: ...> :: proc {...}' must highlight the name as a
	// function even though the directive precedes it.
	source := "#entry function5 <T: String | S64> :: proc (input1: T) { }"
	tokens := decodeSemanticData(semanticData(t, source))
	tok, ok := tokenAt(tokens, 0, 7)
	if !ok {
		t.Fatalf("no token at line 0 col 7 (function name)")
	}
	if tok.typeIndex != semTypeFunction {
		t.Errorf("token at line 0 col 7 has type %d, want %d (function)", tok.typeIndex, semTypeFunction)
	}
	if tok.length != 9 {
		t.Errorf("token at line 0 col 7 has length %d, want 9", tok.length)
	}
}

func TestSemanticTokensErrorDecl(t *testing.T) {
	// An error type name is highlighted as a type, its members as constants,
	// and the 'error' keyword as a keyword.
	source := "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n    NOT_FOUND;\n    OUT_OF_BOUNDS;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start int
		want        int
	}{
		{0, 0, semTypeType},     // Hash_Table_Error
		{0, 20, semTypeKeyword}, // error
		{1, 4, semTypeConstant}, // GENERIC
		{2, 4, semTypeConstant}, // OUT_OF_MEMORY
		{3, 4, semTypeConstant}, // NOT_FOUND
		{4, 4, semTypeConstant}, // OUT_OF_BOUNDS
	}
	for _, c := range checks {
		tok, ok := tokenAt(tokens, c.line, c.start)
		if !ok {
			t.Errorf("no token at line %d col %d", c.line, c.start)
			continue
		}
		if tok.typeIndex != c.want {
			t.Errorf("token at line %d col %d has type %d, want %d", c.line, c.start, tok.typeIndex, c.want)
		}
	}
}

func TestSemanticTokensErrorMemberUsage(t *testing.T) {
	// 'Type.MEMBER!' highlights the type name as a type and the member (with
	// its '!') as a constant; the bare '.MEMBER!' highlights the member as a
	// constant.
	source := "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n}\nmain :: proc {\n    err: Hash_Table_Error = .OUT_OF_MEMORY!;\n    if err == Hash_Table_Error.OUT_OF_MEMORY! { }\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, length int
		want                int
	}{
		{4, 0, 4, semTypeFunction},   // main
		{5, 4, 3, semTypeVariable},   // err
		{5, 9, 16, semTypeType},      // Hash_Table_Error (decl type)
		{5, 29, 14, semTypeConstant}, // OUT_OF_MEMORY! (bare member)
		{6, 14, 16, semTypeType},     // Hash_Table_Error (member type)
		{6, 31, 14, semTypeConstant}, // OUT_OF_MEMORY! (explicit member)
	}
	for _, c := range checks {
		tok, ok := tokenAt(tokens, c.line, c.start)
		if !ok {
			t.Errorf("no token at line %d col %d", c.line, c.start)
			continue
		}
		if tok.typeIndex != c.want {
			t.Errorf("token at line %d col %d has type %d, want %d", c.line, c.start, tok.typeIndex, c.want)
		}
		if tok.length != c.length {
			t.Errorf("token at line %d col %d has length %d, want %d", c.line, c.start, tok.length, c.length)
		}
	}
}

func TestSemanticTokensErrorReturnSpec(t *testing.T) {
	// '-> (String <> Some_Error)' highlights the value and error types as
	// types, '<>' as a delimiter, and the returned error literal as a
	// constant.
	source := "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (String <> Some_Error) {\n    return .GENERIC!;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start int
		want        int
	}{
		{3, 0, semTypeFunction},   // f
		{3, 14, semTypeType},      // String (value type)
		{3, 21, semTypeDelimiter}, // <>
		{3, 24, semTypeType},      // Some_Error (error type)
		{4, 12, semTypeConstant},  // GENERIC! (error literal)
	}
	for _, c := range checks {
		tok, ok := tokenAt(tokens, c.line, c.start)
		if !ok {
			t.Errorf("no token at line %d col %d", c.line, c.start)
			continue
		}
		if tok.typeIndex != c.want {
			t.Errorf("token at line %d col %d has type %d, want %d", c.line, c.start, tok.typeIndex, c.want)
		}
	}
}

func TestSemanticTokensErrorBlockWithoutKeyword(t *testing.T) {
	// 'Hash_Table_Error :: { ... }' without the 'error' keyword still
	// registers the name as a type and highlights the members and uses, so
	// the editor stays useful while the declaration is incomplete.
	source := "Hash_Table_Error :: {\n    GENERIC;\n}\nsomething :: proc -> (String <> Hash_Table_Error) {\n    return .GENERIC!;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start int
		want        int
	}{
		{0, 0, semTypeType},      // Hash_Table_Error (decl)
		{1, 4, semTypeConstant},  // GENERIC (member)
		{3, 32, semTypeType},     // Hash_Table_Error (error return spec)
		{4, 12, semTypeConstant}, // GENERIC! (error literal)
	}
	for _, c := range checks {
		tok, ok := tokenAt(tokens, c.line, c.start)
		if !ok {
			t.Errorf("no token at line %d col %d", c.line, c.start)
			continue
		}
		if tok.typeIndex != c.want {
			t.Errorf("token at line %d col %d has type %d, want %d", c.line, c.start, tok.typeIndex, c.want)
		}
	}
}

func TestSemanticTokensUnlessCatch(t *testing.T) {
	// 'unless catch err' highlights the keywords and the catch binding.
	source := "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\nmain :: proc -> S64 {\n    r := f(5) unless catch err {\n        return -1;\n    }\n    return r;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start int
		want        int
	}{
		{10, 14, semTypeKeyword},  // unless
		{10, 21, semTypeKeyword},  // catch
		{10, 27, semTypeVariable}, // err (catch binding)
	}
	for _, c := range checks {
		tok, ok := tokenAt(tokens, c.line, c.start)
		if !ok {
			t.Errorf("no token at line %d col %d", c.line, c.start)
			continue
		}
		if tok.typeIndex != c.want {
			t.Errorf("token at line %d col %d has type %d, want %d", c.line, c.start, tok.typeIndex, c.want)
		}
	}
}

func TestSemanticTokensLoopAndUnlessBindings(t *testing.T) {
	source := "main :: proc {\n    arr := []S64.{1};\n    for idx, elem: arr {\n        value := idx + elem;\n    }\n    result := work() unless catch err { exit 1; }\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, length int
	}{
		{2, 8, 3},  // idx
		{2, 13, 4}, // elem
		{5, 4, 6},  // result
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok {
			t.Errorf("no semantic token at line %d col %d", check.line, check.start)
			continue
		}
		if tok.typeIndex != semTypeVariable || tok.length != check.length {
			t.Errorf("token at line %d col %d = type %d length %d, want variable length %d", check.line, check.start, tok.typeIndex, tok.length, check.length)
		}
	}
}
