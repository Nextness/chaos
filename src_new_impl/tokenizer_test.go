package main

import (
	"fmt"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   []TokenKind
		values []any // expected Value fields; nil means don't check
		errs   int   // expected number of error diagnostics
	}{
		// ─── Empty / trivial ──────────────────────────────────────────────
		{
			name:  "empty",
			input: "",
			want:  []TokenKind{TkEOF},
		},
		{
			name:  "whitespace only",
			input: "   \t\n  ",
			want:  []TokenKind{TkEOF},
		},
		{
			name:  "newlines only",
			input: "\n\n\n",
			want:  []TokenKind{TkEOF},
		},

		// ─── Single-character tokens ──────────────────────────────────────
		{
			name:  "single char tokens",
			input: "=+-*/<>,;:(){}.#?!@|",
			want: []TokenKind{
				TkAssign, TkPlus, TkMinus, TkStar, TkSlash,
				TkLt, TkGt, TkComma, TkSemicolon, TkColon,
				TkLParen, TkRParen, TkLBrace, TkRBrace, TkDot,
				TkHash, TkQuestion, TkNot, TkAt, TkPipe,
				TkEOF,
			},
		},

		// ─── Multi-character operators ────────────────────────────────────
		{
			name:  "multi-char operators",
			input: ":: := == != <= >= && || -> ...",
			want: []TokenKind{
				TkCompTimeAssign, TkInfer,
				TkEq, TkNeq, TkLe, TkGe,
				TkAnd, TkOr, TkArrow, TkEllipsis,
				TkEOF,
			},
		},

		// ─── Keywords ─────────────────────────────────────────────────────
		{
			name:  "keywords",
			input: "true false exit if elif else proc then return as",
			want: []TokenKind{
				TkTrue, TkFalse, TkExit, TkIf, TkElif, TkElse,
				TkProc, TkThen, TkReturn, TkAs,
				TkEOF,
			},
			values: []any{true, false, nil, nil, nil, nil, nil, nil, nil, nil, nil},
		},

		// ─── Identifiers ──────────────────────────────────────────────────
		{
			name:  "identifiers",
			input: "foo bar123 _baz _",
			want:  []TokenKind{TkIdent, TkIdent, TkIdent, TkIdent, TkEOF},
		},

		// ─── Integer literals ─────────────────────────────────────────────
		{
			name:  "integers",
			input: "0 42 1_000_000",
			want:  []TokenKind{TkInt, TkInt, TkInt, TkEOF},
			values: []any{"0", "42", "1_000_000", nil},
		},
		{
			name:  "integer with bad underscore",
			input: "1__2",
			want:  []TokenKind{TkInt, TkEOF},
			errs:  1, // misplaced underscore
		},
		{
			name:  "integer trailing underscore",
			input: "1_",
			want:  []TokenKind{TkInt, TkEOF},
			errs:  1, // trailing underscore
		},

		// ─── Float literals ───────────────────────────────────────────────
		{
			name:  "floats",
			input: "3.14 .5 1.0 0.0",
			want:  []TokenKind{TkFloat, TkFloat, TkFloat, TkFloat, TkEOF},
			values: []any{"3.14", ".5", "1.0", "0.0", nil},
		},
		{
			name:  "float with underscore",
			input: "1_000.5",
			want:  []TokenKind{TkFloat, TkEOF},
			values: []any{"1_000.5", nil},
		},

		// ─── String literals ──────────────────────────────────────────────
		{
			name:  "strings",
			input: `«hello» «» «a b c»`,
			want:  []TokenKind{TkString, TkString, TkString, TkEOF},
			values: []any{"hello", "", "a b c", nil},
		},
		{
			name:  "string with newline",
			input: "«hello\nworld»",
			want:  []TokenKind{TkString, TkEOF},
			values: []any{"hello\nworld", nil},
		},

		// ─── Comments ─────────────────────────────────────────────────────
		{
			name:  "line comment",
			input: "// hello\n42",
			want:  []TokenKind{TkInt, TkEOF},
		},
		{
			name:  "line comment at EOF",
			input: "// hello",
			want:  []TokenKind{TkEOF},
		},
		{
			name:  "block comment",
			input: "/** comment **/42",
			want:  []TokenKind{TkInt, TkEOF},
		},
		{
			name:  "nested block comment",
			input: "/** outer /** inner **/ still outer **/42",
			want:  []TokenKind{TkInt, TkEOF},
		},
		{
			name:  "consecutive comments",
			input: "// line1\n// line2\n/** block **/42",
			want:  []TokenKind{TkInt, TkEOF},
		},

		// ─── Statements (integration) ─────────────────────────────────────
		{
			name:  "simple declaration",
			input: "x :: 42;",
			want:  []TokenKind{TkIdent, TkCompTimeAssign, TkInt, TkSemicolon, TkEOF},
		},
		{
			name:  "inferred assignment",
			input: "y := x;",
			want:  []TokenKind{TkIdent, TkInfer, TkIdent, TkSemicolon, TkEOF},
		},
		{
			name:  "typed declaration",
			input: "z : S64 = 1;",
			want:  []TokenKind{TkIdent, TkColon, TkIdent, TkAssign, TkInt, TkSemicolon, TkEOF},
		},
		{
			name:  "procedure",
			input: "main :: proc -> S64 { return 0; }",
			want: []TokenKind{
				TkIdent, TkCompTimeAssign, TkProc, TkArrow, TkIdent,
				TkLBrace, TkReturn, TkInt, TkSemicolon, TkRBrace,
				TkEOF,
			},
		},
		{
			name:  "conditionals",
			input: "if a < b { exit 1; } elif c == d { exit 2; } else { exit 3; }",
			want: []TokenKind{
				TkIf, TkIdent, TkLt, TkIdent, TkLBrace,
				TkExit, TkInt, TkSemicolon, TkRBrace,
				TkElif, TkIdent, TkEq, TkIdent, TkLBrace,
				TkExit, TkInt, TkSemicolon, TkRBrace,
				TkElse, TkLBrace,
				TkExit, TkInt, TkSemicolon, TkRBrace,
				TkEOF,
			},
		},
		{
			name:  "logical operators",
			input: "a && b || !c",
			want:  []TokenKind{TkIdent, TkAnd, TkIdent, TkOr, TkNot, TkIdent, TkEOF},
		},
		{
			name:  "comparisons",
			input: "a == b != c < d > e <= f >= g",
			want: []TokenKind{
				TkIdent, TkEq, TkIdent, TkNeq, TkIdent,
				TkLt, TkIdent, TkGt, TkIdent,
				TkLe, TkIdent, TkGe, TkIdent,
				TkEOF,
			},
		},

		// ─── Error recovery ───────────────────────────────────────────────
		{
			name:  "unknown char recovers",
			input: "% &",
			want:  []TokenKind{TkError, TkError, TkEOF},
			errs:  2,
		},
		{
			name:  "unknown char between tokens",
			input: "x @ y",
			want:  []TokenKind{TkIdent, TkAt, TkIdent, TkEOF},
		},

		// ─── Unicode identifiers ──────────────────────────────────────────
		{
			name:  "unicode identifiers",
			input: "αβγ :: 42;",
			want:  []TokenKind{TkIdent, TkCompTimeAssign, TkInt, TkSemicolon, TkEOF},
		},

		// ─── Span consistency ─────────────────────────────────────────────
		{
			name:  "abc",
			input: "abc",
			want:  []TokenKind{TkIdent, TkEOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, diags := Tokenize([]byte(tt.input), 0)

			// Check error count
			gotErrs := 0
			for _, d := range diags {
				if d.Severity == SeverityError {
					gotErrs++
				}
			}
			if gotErrs != tt.errs {
				t.Errorf("got %d errors, want %d", gotErrs, tt.errs)
				for _, d := range diags {
					t.Logf("  %s: %s", d.Severity, d.Message)
				}
			}

			// Check token kinds
			if len(tokens) != len(tt.want) {
				t.Fatalf("got %d tokens, want %d\n%s", len(tokens), len(tt.want), dumpTokens(tokens))
			}
			for i, want := range tt.want {
				if tokens[i].Kind != want {
					t.Errorf("token[%d] = %s, want %s", i, tokens[i].Kind, want)
				}
			}

			// Check values
			if tt.values != nil {
				for i, v := range tt.values {
					if i >= len(tokens) {
						break
					}
					if v != nil {
						if tokens[i].Value != v {
							t.Errorf("token[%d].Value = %v (type %T), want %v (type %T)",
								i, tokens[i].Value, tokens[i].Value, v, v)
						}
					}
				}
			}

			// Verify basic span invariants
			for i, tok := range tokens {
				if tok.Span.Start < 0 {
					t.Errorf("token[%d] %s has negative Start %d", i, tok.Kind, tok.Span.Start)
				}
				if tok.Span.End < tok.Span.Start {
					t.Errorf("token[%d] %s has End < Start (%d < %d)", i, tok.Kind, tok.Span.End, tok.Span.Start)
				}
				if tok.Span.End > len(tt.input) {
					t.Errorf("token[%d] %s has End > len(input) (%d > %d)", i, tok.Kind, tok.Span.End, len(tt.input))
				}
				// Raw must match the span
				if tok.Span.End <= len(tt.input) {
					expected := tt.input[tok.Span.Start:tok.Span.End]
					if string(tok.Raw) != expected {
						t.Errorf("token[%d] %s Raw = %q, want %q (span [%d,%d))",
							i, tok.Kind, string(tok.Raw), expected, tok.Span.Start, tok.Span.End)
					}
				}
			}
		})
	}
}

func dumpTokens(tokens []Token) string {
	var s string
	for i, tok := range tokens {
		s += fmt.Sprintf("  [%d] %s", i, tok.Kind)
		if tok.Value != nil {
			s += fmt.Sprintf("(%v)", tok.Value)
		}
		s += "\n"
	}
	return s
}

func TestUnterminatedString(t *testing.T) {
	tokens, diags := Tokenize([]byte("x := «hello"), 0)
	if len(tokens) < 3 {
		t.Fatalf("expected at least 3 tokens (ident, infer, error), got %d", len(tokens))
	}
	if tokens[0].Kind != TkIdent {
		t.Errorf("token[0] = %s, want identifier", tokens[0].Kind)
	}
	if tokens[1].Kind != TkInfer {
		t.Errorf("token[1] = %s, want :=", tokens[1].Kind)
	}
	if tokens[2].Kind != TkError {
		t.Errorf("token[2] = %s, want Error", tokens[2].Kind)
	}
	hasErr := false
	for _, d := range diags {
		if d.Severity == SeverityError {
			hasErr = true
			break
		}
	}
	if !hasErr {
		t.Error("expected an error diagnostic for unterminated string")
	}
	// Check that EOF is still produced after the error
	last := tokens[len(tokens)-1]
	if last.Kind != TkEOF {
		t.Errorf("last token = %s, want EOF", last.Kind)
	}
}

func TestUnterminatedBlockComment(t *testing.T) {
	tokens, diags := Tokenize([]byte("x :: 42; /** oops"), 0)
	hasErr := false
	for _, d := range diags {
		if d.Severity == SeverityError {
			hasErr = true
		}
	}
	if !hasErr {
		t.Error("expected an error diagnostic for unterminated block comment")
	}
	// Check that tokens before the comment are still valid
	if len(tokens) < 4 {
		t.Fatalf("expected at least 4 tokens, got %d", len(tokens))
	}
	if tokens[0].Kind != TkIdent {
		t.Errorf("token[0] = %s, want identifier", tokens[0].Kind)
	}
	last := tokens[len(tokens)-1]
	if last.Kind != TkEOF {
		t.Errorf("last token = %s, want EOF", last.Kind)
	}
}

func TestMultipleFiles(t *testing.T) {
	sm := &SourceManager{}
	fid1 := sm.Register("a.chaos", []byte("x :: 1;"))
	fid2 := sm.Register("b.chaos", []byte("y := 2;"))

	toks1, _ := Tokenize(sm.Lookup(fid1).Source, fid1)
	toks2, _ := Tokenize(sm.Lookup(fid2).Source, fid2)

	if toks1[0].Kind != TkIdent || string(toks1[0].Raw) != "x" {
		t.Errorf("file 1 first token = %s %q, want identifier x", toks1[0].Kind, toks1[0].Raw)
	}
	if toks2[0].Kind != TkIdent || string(toks2[0].Raw) != "y" {
		t.Errorf("file 2 first token = %s %q, want identifier y", toks2[0].Kind, toks2[0].Raw)
	}
	if toks1[0].Span.File != fid1 {
		t.Errorf("file 1 token has FileID %d, want %d", toks1[0].Span.File, fid1)
	}
	if toks2[0].Span.File != fid2 {
		t.Errorf("file 2 token has FileID %d, want %d", toks2[0].Span.File, fid2)
	}
}

func TestSpanRanges(t *testing.T) {
	// Verify that spans cover the correct byte ranges for various token types.
	source := []byte("abc :: 42;")
	tokens, _ := Tokenize(source, 0)

	expect := []struct {
		start, end int
	}{
		{0, 3},   // "abc"
		{4, 6},   // "::"
		{7, 9},   // "42"
		{9, 10},  // ";"
		{10, 10}, // EOF
	}
	if len(tokens) != len(expect) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(expect))
	}
	for i, e := range expect {
		if tokens[i].Span.Start != e.start {
			t.Errorf("token[%d] Span.Start = %d, want %d", i, tokens[i].Span.Start, e.start)
		}
		if tokens[i].Span.End != e.end {
			t.Errorf("token[%d] Span.End = %d, want %d (kind=%s, raw=%q)", i, tokens[i].Span.End, e.end, tokens[i].Kind, string(tokens[i].Raw))
		}
	}

	// Verify that the raw bytes match the span.
	for i, tok := range tokens {
		if tok.Kind == TkEOF {
			continue
		}
		expected := string(source[tok.Span.Start:tok.Span.End])
		if string(tok.Raw) != expected {
			t.Errorf("token[%d] Raw = %q, want %q", i, string(tok.Raw), expected)
		}
	}
}