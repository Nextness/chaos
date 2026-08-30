package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"chaos_compiler/compiler"
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

func TestSemanticTokenEncodingInvariants(t *testing.T) {
	source := "Thing :: struct {\n\tlabel: String = «😀»;\n}\n#entry main :: proc -> S64 {\n\tvalue :: Thing.{}; // comment\n\treturn value.label == «😀»;\n}"
	data := semanticData(t, source)
	if len(data)%5 != 0 {
		t.Fatalf("semantic token data length = %d, want a multiple of 5", len(data))
	}
	lines := strings.Split(source, "\n")
	tokens := decodeSemanticData(data)
	for i, tok := range tokens {
		if tok.length <= 0 {
			t.Errorf("token %d has non-positive length: %+v", i, tok)
		}
		if tok.typeIndex < 0 || tok.typeIndex >= len(semanticTokenTypes) {
			t.Errorf("token %d has invalid type index: %+v", i, tok)
		}
		if tok.line < 0 || tok.line >= len(lines) {
			t.Errorf("token %d is outside source lines: %+v", i, tok)
			continue
		}
		lineLength := len(utf16.Encode([]rune(strings.TrimSuffix(lines[tok.line], "\r"))))
		if tok.startChar < 0 || tok.startChar+tok.length > lineLength {
			t.Errorf("token %d is outside UTF-16 line length %d: %+v", i, lineLength, tok)
		}
		if i > 0 {
			prev := tokens[i-1]
			if tok.line < prev.line || (tok.line == prev.line && tok.startChar < prev.startChar+prev.length) {
				t.Errorf("tokens are unsorted or overlapping: previous=%+v current=%+v", prev, tok)
			}
		}
	}
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

func TestSemanticTokensInitLaterEllipsis(t *testing.T) {
	// 'a: S64 = ...;' declares a variable initialized later; the '...'
	// marker reads as a keyword.
	source := "#entry main :: proc -> S64 {\n    a: S64 = ...;\n    a = 42;\n    return a;\n}"
	want := []int{
		0, 0, 1, semTypeKeyword, 0, // #
		0, 1, 5, semTypeKeyword, 0, // entry
		0, 6, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 8, 3, semTypeType, 0, // S64 (result)
		0, 4, 1, semTypeDelimiter, 0, // {
		1, 4, 1, semTypeVariable, 0, // a
		0, 3, 3, semTypeType, 0, // S64
		0, 6, 3, semTypeKeyword, 0, // ...
		1, 4, 1, semTypeVariable, 0, // a (reassign)
		0, 4, 2, semTypeNumber, 0, // 42
		1, 4, 6, semTypeKeyword, 0, // return
		0, 7, 1, semTypeVariable, 0, // a
		1, 0, 1, semTypeDelimiter, 0, // }
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
	// highlighted as a type. Field labels and field values are highlighted too.
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
		0, 1, 6, semTypeVariable, 0, // field1
		0, 7, 7, semTypeString, 0, // «hello»
		0, 9, 6, semTypeVariable, 0, // field2
		0, 7, 2, semTypeNumber, 0, // 10
		0, 4, 6, semTypeVariable, 0, // field3
		0, 7, 4, semTypeKeyword, 0, // true
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
		out = append(out, semanticToken{line: line, startChar: start, length: data[i+2], typeIndex: data[i+3], modifiers: data[i+4]})
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

func TestSemanticTokensGenericProcedures(t *testing.T) {
	source := "function1 :: proc (input1: String) { }\n" +
		"function2 :: proc (input1: String) -> String { }\n" +
		"function3 :: proc -> String { }\n" +
		"function4 :: proc { }\n" +
		"function5 <T: String | S64> :: proc (input1: T) { }\n"
	tokens := decodeSemanticData(semanticData(t, source))
	for line := 0; line < 5; line++ {
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
	source := "#entry function5 <T: String | S64> :: proc (input1: T) { }"
	tokens := decodeSemanticData(semanticData(t, source))
	if tok, ok := tokenAt(tokens, 0, 7); !ok || tok.typeIndex != semTypeFunction {
		t.Fatalf("generic entry was not highlighted as a function: %+v", tok)
	}
}

func TestSemanticTokensGenericTypeParamsHighlighted(t *testing.T) {
	// A generic type parameter must be highlighted as a type both in its
	// declaration '<T>' and everywhere it is used in the signature, including
	// array element positions and return types.
	source := "pop_last <T> :: proc (arr: *[dyn]T) -> T {\n    return arr[0];\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want int
	}{
		{0, 10, semTypeType}, // T (type parameter in <T>)
		{0, 33, semTypeType}, // T (element type of *[dyn]T)
		{0, 39, semTypeType}, // T (return type)
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want {
			t.Errorf("token at %d:%d = %+v, want type %d", check.line, check.start, tok, check.want)
		}
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

func TestSemanticTokensSizeOf(t *testing.T) {
	source := "main :: proc -> S64 {\n    if size_of(S64) != 8 { return 1; }\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	// size_of is a keyword at line 1, col 7.
	tok, ok := tokenAt(tokens, 1, 7)
	if !ok {
		t.Fatalf("no token at line 1 col 7 (size_of)")
	}
	if tok.typeIndex != semTypeKeyword {
		t.Errorf("size_of token type = %d, want %d (keyword)", tok.typeIndex, semTypeKeyword)
	}
}

func TestSemanticTokensErrorMemberUsage(t *testing.T) {
	// 'Type.MEMBER!' highlights the type name as a type and the member name as
	// a constant. The punctuation is deliberately outside the identifier span.
	source := "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n}\nmain :: proc {\n    err: Hash_Table_Error = .OUT_OF_MEMORY!;\n    if err == Hash_Table_Error.OUT_OF_MEMORY! { }\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, length int
		want                int
	}{
		{4, 0, 4, semTypeFunction},   // main
		{5, 4, 3, semTypeVariable},   // err
		{5, 9, 16, semTypeType},      // Hash_Table_Error (decl type)
		{5, 29, 13, semTypeConstant}, // OUT_OF_MEMORY (bare member)
		{6, 14, 16, semTypeType},     // Hash_Table_Error (member type)
		{6, 31, 13, semTypeConstant}, // OUT_OF_MEMORY (explicit member)
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
	// An incomplete 'Name :: { ... }' is skipped in tolerant mode, but it must
	// not fabricate an error type or error members. The following declaration
	// remains parseable and receives its normal tokens.
	source := "Hash_Table_Error :: {\n    GENERIC;\n}\nsomething :: proc -> (String <> Hash_Table_Error) {\n    return .GENERIC!;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	for _, pos := range [][2]int{{0, 0}, {1, 4}, {3, 32}} {
		if _, ok := tokenAt(tokens, pos[0], pos[1]); ok {
			t.Errorf("unexpected fabricated semantic token at line %d col %d", pos[0], pos[1])
		}
	}
	if tok, ok := tokenAt(tokens, 3, 0); !ok || tok.typeIndex != semTypeFunction {
		t.Errorf("following procedure was not highlighted: token=%+v, ok=%v", tok, ok)
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

func TestSemanticTokensEnum(t *testing.T) {
	source := "Color :: enum {\n    RED: U8 = 1;\n    GREEN;\n}\nmain :: proc {\n    c: Color = Color.GREEN;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want int
	}{
		{0, 0, semTypeType},
		{1, 4, semTypeConstant},
		{1, 9, semTypeType},
		{2, 4, semTypeConstant},
		{5, 7, semTypeType},
		{5, 15, semTypeType},
		{5, 21, semTypeConstant},
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want {
			t.Errorf("token at %d:%d = %+v, want type %d", check.line, check.start, tok, check.want)
		}
	}
}

func TestSemanticTokensIfx(t *testing.T) {
	// The ifx keyword and the then/else keywords are highlighted as
	// keywords; identifiers and literals inside the branches keep their
	// normal highlighting.
	source := "main :: proc -> S64 {\n\ta := ifx true then 1 else 2;\n\treturn a;\n}"
	want := []int{
		0, 0, 4, semTypeFunction, 0, // main
		0, 8, 4, semTypeKeyword, 0, // proc
		0, 8, 3, semTypeType, 0, // S64
		0, 4, 1, semTypeDelimiter, 0, // {
		1, 1, 1, semTypeVariable, 0, // a
		0, 5, 3, semTypeKeyword, 0, // ifx
		0, 4, 4, semTypeKeyword, 0, // true
		0, 5, 4, semTypeKeyword, 0, // then
		0, 5, 1, semTypeNumber, 0, // 1
		0, 2, 4, semTypeKeyword, 0, // else
		0, 5, 1, semTypeNumber, 0, // 2
		1, 1, 6, semTypeKeyword, 0, // return
		0, 7, 1, semTypeVariable, 0, // a
		1, 0, 1, semTypeDelimiter, 0, // }
	}
	got := semanticData(t, source)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semantic data = %v, want %v", got, want)
	}
}

func TestSemanticTokensPointers(t *testing.T) {
	// The null keyword is a keyword; pointer type elements keep their type
	// tokens; dereference operands render as variables.
	source := "main :: proc -> S64 {\n\tp: *S64? = null;\n\tx := p.*;\n\treturn x;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want int
	}{
		{0, 0, semTypeFunction},   // main
		{0, 8, semTypeKeyword},    // proc
		{0, 16, semTypeType},      // S64 (result type)
		{0, 20, semTypeDelimiter}, // {
		{1, 1, semTypeVariable},   // p
		{1, 4, semTypeType},       // * (pointer marker)
		{1, 5, semTypeType},       // S64 (inside *S64?)
		{1, 8, semTypeType},       // ? (nullable marker)
		{1, 12, semTypeKeyword},   // null
		{2, 1, semTypeVariable},   // x
		{2, 6, semTypeVariable},   // p (deref operand)
		{3, 1, semTypeKeyword},    // return
		{3, 8, semTypeVariable},   // x
		{4, 0, semTypeDelimiter},  // }
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want {
			t.Errorf("token at %d:%d = %+v, want type %d", check.line, check.start, tok, check.want)
		}
	}
}

func TestSemanticTokensStringFieldsAndInterpolation(t *testing.T) {
	// String field access, #allocate / #deallocate, and casts render their
	// identifiers as variables. In an interpolated string the literal segments
	// are strings and the inner expression is highlighted as its real type.
	source := "main :: proc -> S64 {\n\ts := «hello»;\n\tn := s.count;\n\tname := «world»;\n\tg := «hello {name}!»;\n\tbuf := #allocate 16;\n\tp: *S64 = buf.(*S64);\n\t#deallocate buf;\n\treturn n;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want int
	}{
		{0, 0, semTypeFunction},   // main
		{1, 1, semTypeVariable},   // s
		{2, 1, semTypeVariable},   // n
		{2, 6, semTypeVariable},   // s (field base)
		{2, 8, semTypeVariable},   // count (field)
		{3, 1, semTypeVariable},   // name
		{4, 1, semTypeVariable},   // g
		{4, 6, semTypeString},     // « (opening guillemet)
		{4, 7, semTypeString},     // hello  (literal before interpolation)
		{4, 13, semTypeDelimiter}, // { (interpolation open)
		{4, 14, semTypeVariable},  // name (interpolated)
		{4, 18, semTypeDelimiter}, // } (interpolation close)
		{4, 19, semTypeString},    // ! (literal after interpolation)
		{4, 20, semTypeString},    // » (closing guillemet)
		{5, 1, semTypeVariable},   // buf
		{6, 1, semTypeVariable},   // p
		{6, 11, semTypeVariable},  // buf (cast value)
		{6, 14, semTypeType},      // .(*S64) (whole cast)
		{7, 13, semTypeVariable},  // buf (deallocate operand)
		{8, 8, semTypeVariable},   // n (returned)
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want {
			t.Errorf("token at %d:%d = %+v, want type %d", check.line, check.start, tok, check.want)
		}
	}
}

func TestSemanticTokensDerefAndCastTypeColor(t *testing.T) {
	// The '.*' dereference operator reads as a keyword; the whole '.(*T)'
	// cast reads as a bold type token.
	source := "main :: proc -> S64 {\n\tx := 5;\n\tp: *S64 = *x;\n\ty := p.*;\n\tbuf := #allocate 16;\n\tq: *S64 = buf.(*S64);\n\treturn 0;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want, wantMod int
	}{
		{3, 6, semTypeVariable, 0},       // p (deref operand)
		{3, 7, semTypeKeyword, 0},        // .* (deref operator, keyword)
		{5, 11, semTypeVariable, 0},      // buf (cast value)
		{5, 14, semTypeType, semModBold}, // .(*S64) (whole cast, bold)
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want || tok.modifiers != check.wantMod {
			t.Errorf("token at %d:%d = %+v, want type %d modifiers %d", check.line, check.start, tok, check.want, check.wantMod)
		}
	}
}

func TestSemanticTokensDerefInAssignmentLvalue(t *testing.T) {
	// A complex assignment lvalue like 'p.*.node_pool[n].text' must be
	// walked so the '.*' operator reads as a keyword and the members and
	// index are highlighted.
	source := "main :: proc {\n    p.*.node_pool[n].text = op;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want int
	}{
		{1, 4, semTypeVariable},   // p (deref operand)
		{1, 5, semTypeKeyword},    // .* (deref operator)
		{1, 8, semTypeVariable},   // node_pool (field)
		{1, 17, semTypeDelimiter}, // [ (index open)
		{1, 18, semTypeVariable},  // n (index)
		{1, 19, semTypeDelimiter}, // ] (index close)
		{1, 21, semTypeVariable},  // text (field)
		{1, 28, semTypeVariable},  // op (value)
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want {
			t.Errorf("token at %d:%d = %+v, want type %d", check.line, check.start, tok, check.want)
		}
	}
}

func TestSemanticTokensImportedTypeHighlighted(t *testing.T) {
	// A type defined in an imported module must be recognized and highlighted
	// as a type in the current file. The LSP resolves imports through the
	// resolved program returned by AnalyzeProgram.
	dir := t.TempDir()
	modPath := filepath.Join(dir, "parser.chaos")
	if err := os.WriteFile(modPath, []byte("Parser :: struct {\n    pos: S64;\n}\n"), 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}
	mainPath := filepath.Join(dir, "main.chaos")
	source := "#import «parser»;\nmain :: proc {\n    p: Parser;\n    p.pos = 1;\n}"
	if err := os.WriteFile(mainPath, []byte(source), 0o600); err != nil {
		t.Fatalf("write main: %v", err)
	}

	sm := &compiler.SourceManager{}
	fileID := sm.Register(mainPath, []byte(source))
	sf := sm.Lookup(fileID)
	tokens, _ := compiler.Tokenize(sf.Source, fileID)
	result := compiler.ParseProgramTolerant(tokens)
	result.Program.Sources = map[compiler.FileID]compiler.SourceFile{fileID: *sf}
	analysis, _ := compiler.AnalyzeProgram(result.Program)
	prog := result.Program
	if analysis != nil && analysis.Program != nil {
		prog = analysis.Program
	}
	all := append(tokenSemanticTokens(tokens, sf), astSemanticTokens(prog, sf)...)
	decoded := decodeSemanticData(encodeSemanticTokens(all))

	// 'Parser' in the type position of the current file reads as a type.
	found := false
	for _, tok := range decoded {
		if tok.line == 2 && tok.startChar == 7 && tok.typeIndex == semTypeType {
			found = true
		}
	}
	if !found {
		t.Errorf("Parser not highlighted as a type in the current file")
	}
	// No tokens may be emitted for the imported module's declarations, which
	// live in another file and would otherwise render at garbage positions.
	for _, tok := range decoded {
		if tok.line > 4 {
			t.Errorf("unexpected token beyond the current file: %+v", tok)
		}
	}
}

func TestSemanticTokensArrayTypes(t *testing.T) {
	// Array type expressions in all their forms read as type tokens: the
	// brackets, the size/dyn marker, and the element type. Pointer prefixes
	// are type tokens too.
	source := "main :: proc {\n    a: *[dyn]S64;\n    b: [dyn]S64;\n    c: []S64;\n    d: [4]S64;\n    e: [N]S64;\n    f: **[dyn]S64;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want int
	}{
		{1, 7, semTypeType},  // * (pointer to array)
		{1, 8, semTypeType},  // [dyn]
		{1, 13, semTypeType}, // S64
		{2, 7, semTypeType},  // [dyn]
		{2, 12, semTypeType}, // S64
		{3, 7, semTypeType},  // []
		{3, 9, semTypeType},  // S64
		{4, 7, semTypeType},  // [4]
		{4, 10, semTypeType}, // S64
		{5, 7, semTypeType},  // [N]
		{5, 10, semTypeType}, // S64
		{6, 7, semTypeType},  // * (first pointer)
		{6, 8, semTypeType},  // * (second pointer)
		{6, 9, semTypeType},  // [dyn]
		{6, 14, semTypeType}, // S64
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want {
			t.Errorf("token at %d:%d = %+v, want type %d", check.line, check.start, tok, check.want)
		}
	}
}

func TestSemanticTokensPointerProcSignature(t *testing.T) {
	source := "another_proc :: proc (param1: *String) -> *String {\n    return param1;\n}"
	tokens := decodeSemanticData(semanticData(t, source))
	checks := []struct {
		line, start, want int
	}{
		{0, 0, semTypeFunction},   // another_proc
		{0, 16, semTypeKeyword},   // proc
		{0, 22, semTypeParameter}, // param1
		{0, 30, semTypeType},      // * (param pointer marker)
		{0, 31, semTypeType},      // String (param pointed-to type)
		{0, 42, semTypeType},      // * (result pointer marker)
		{0, 43, semTypeType},      // String (result pointed-to type)
		{1, 11, semTypeVariable},  // param1 (returned)
	}
	for _, check := range checks {
		tok, ok := tokenAt(tokens, check.line, check.start)
		if !ok || tok.typeIndex != check.want {
			t.Errorf("token at %d:%d = %+v, want type %d", check.line, check.start, tok, check.want)
		}
	}
}
