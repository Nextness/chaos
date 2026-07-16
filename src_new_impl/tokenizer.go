package main

import (
	"unicode"
	"unicode/utf8"
)

// ─── Tokenizer ───────────────────────────────────────────────────────────────

// Tokenizer converts a source buffer into a TokenList. It is a stateful
// cursor-based scanner that reports errors via a DiagnosticList instead of
// panicking.
//
// Design decisions that differ from the old lexer:
//   - No custom ChaosSlice[T] wrapper; uses a plain struct with a `pos` int.
//   - No `any` boxing for comparisons; direct byte/string matching.
//   - Byte offsets (Span) instead of line/column pairs; line/col is derived
//     lazily via a line-offset table for diagnostics.
//   - Int and float literals are separate token kinds (TkInt, TkFloat).
//   - Numeric literals store their raw text; parsing to a concrete type is
//     deferred to the semantic phase (avoids host-int truncation).
//   - String literals use full guillemet matching (multibyte « »).
//   - Unterminated strings/comments produce an error token and recover.
//   - Comments and whitespace are skipped iteratively (no recursion).
//   - After an error token, scanning continues (no early EOF).
//   - EOF is always safe: every scan loop checks bounds before reading.
//   - Block comments handle nesting correctly.
type Tokenizer struct {
	source  []byte // the full source buffer
	pos     int    // current byte offset in source
	file    FileID // file identity for Span
	diags   DiagnosticList
	tokens  TokenList
	lastPos int // position before the current token, used for progress guard
}

// NewTokenizer creates a tokenizer for the given source buffer.
// The tokenizer is single-use: call Tokenize exactly once.
func NewTokenizer(source []byte, file FileID) *Tokenizer {
	return &Tokenizer{
		source: source,
		file:   file,
	}
}

// Tokenize runs the scanner and returns the token list and any diagnostics.
// After this call, the tokenizer should not be reused.
func (t *Tokenizer) Tokenize() (TokenList, DiagnosticList) {
	t.tokens = nil
	t.diags = nil
	t.pos = 0

	for {
		tok := t.next()
		t.tokens = append(t.tokens, tok)
		if tok.Kind == TkEOF {
			break
		}
		// Error recovery: continue scanning, but guarantee progress to
		// avoid infinite loops on unrecognizable input.
		if tok.Kind == TkError && t.pos == t.lastPos {
			t.advance()
		}
	}
	return t.tokens, t.diags
}

// ─── Core scanning ───────────────────────────────────────────────────────────

// peek returns the byte at the current position, or 0 if at EOF.
func (t *Tokenizer) peek() byte {
	if t.pos >= len(t.source) {
		return 0
	}
	return t.source[t.pos]
}

// peekN returns the byte at pos+n, or 0 if out of bounds.
func (t *Tokenizer) peekN(n int) byte {
	if t.pos+n >= len(t.source) || t.pos+n < 0 {
		return 0
	}
	return t.source[t.pos+n]
}

// advance consumes one byte.
func (t *Tokenizer) advance() {
	if t.pos < len(t.source) {
		t.pos++
	}
}

// ─── Trivia skipping (iterative — no recursion) ──────────────────────────────

// skipTrivia advances past whitespace, line comments (//), and block
// comments (/** ... **/). It returns the first non-trivia byte position.
// Unlike the old lexer, this is purely iterative — no recursive calls to
// next() — so a file full of consecutive comments cannot overflow the stack.
func (t *Tokenizer) skipTrivia() {
	for t.pos < len(t.source) {
		b := t.source[t.pos]

		// Whitespace
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			t.advance()
			continue
		}

		// Line comment //  — consume until newline or EOF
		if b == '/' && t.peekN(1) == '/' {
			t.advance()
			t.advance()
			for t.pos < len(t.source) && t.source[t.pos] != '\n' {
				t.advance()
			}
			continue
		}

		// Block comment /** ... **/  — handle nesting
		if b == '/' && t.peekN(1) == '*' && t.peekN(2) == '*' {
			t.advance()
			t.advance()
			t.advance()
			depth := 1
			for t.pos < len(t.source) && depth > 0 {
				// Check open before advance (fixes P0-16)
				if t.source[t.pos] == '/' && t.peekN(1) == '*' && t.peekN(2) == '*' {
					depth++
					t.advance()
					t.advance()
					t.advance()
					continue
				}
				if t.source[t.pos] == '*' && t.peekN(1) == '*' && t.peekN(2) == '/' {
					depth--
					t.advance()
					t.advance()
					t.advance()
					continue
				}
				t.advance()
			}
			if depth > 0 {
				t.diags.Error(Span{File: t.file, Start: t.pos, End: t.pos}, "unterminated block comment")
			}
			continue
		}

		// Not trivia
		break
	}
}

// ─── Token dispatch ──────────────────────────────────────────────────────────

// makeToken creates a token with the given kind spanning from start to current
// pos. Span is half-open [start, t.pos).
func (t *Tokenizer) makeToken(kind TokenKind, start int) Token {
	return Token{
		Kind: kind,
		Span: Span{File: t.file, Start: start, End: t.pos},
		Raw:  t.source[start:t.pos],
	}
}

// makeTokenValue creates a token with a parsed value.
func (t *Tokenizer) makeTokenValue(kind TokenKind, start int, value any) Token {
	tok := t.makeToken(kind, start)
	tok.Value = value
	return tok
}

func (t *Tokenizer) next() Token {
	t.lastPos = t.pos
	t.skipTrivia()

	start := t.pos
	if start >= len(t.source) {
		return Token{
			Kind: TkEOF,
			Span: Span{File: t.file, Start: start, End: start},
		}
	}

	b := t.source[start]

	// String literal: « ... »
	if b == 0xC2 && t.peekN(1) == 0xAB { // « = U+00AB = 0xC2 0xAB in UTF-8
		return t.scanString()
	}

	// Identifiers and keywords
	if t.isIdentStart() {
		return t.scanIdentOrKeyword()
	}

	// Numeric literals
	if isDigit(b) || (b == '.' && isDigit(t.peekN(1))) {
		return t.scanNumber()
	}

	// Multi-character operators (must be checked before single-char)
	switch {
	case b == ':' && t.peekN(1) == ':':
		return t.emitDouble(TkCompTimeAssign, start, 2)
	case b == ':' && t.peekN(1) == '=':
		return t.emitDouble(TkInfer, start, 2)
	case b == '=' && t.peekN(1) == '=':
		return t.emitDouble(TkEq, start, 2)
	case b == '!' && t.peekN(1) == '=':
		return t.emitDouble(TkNeq, start, 2)
	case b == '<' && t.peekN(1) == '=':
		return t.emitDouble(TkLe, start, 2)
	case b == '>' && t.peekN(1) == '=':
		return t.emitDouble(TkGe, start, 2)
	case b == '&' && t.peekN(1) == '&':
		return t.emitDouble(TkAnd, start, 2)
	case b == '|' && t.peekN(1) == '|':
		return t.emitDouble(TkOr, start, 2)
	case b == '-' && t.peekN(1) == '>':
		return t.emitDouble(TkArrow, start, 2)
	case b == '.' && t.peekN(1) == '.' && t.peekN(2) == '.':
		return t.emitDouble(TkEllipsis, start, 3)
	}

	// Single-character tokens
	switch b {
	case '=':
		return t.emitSingle(TkAssign, start)
	case '+':
		return t.emitSingle(TkPlus, start)
	case '-':
		return t.emitSingle(TkMinus, start)
	case '*':
		return t.emitSingle(TkStar, start)
	case '/':
		return t.emitSingle(TkSlash, start)
	case '<':
		return t.emitSingle(TkLt, start)
	case '>':
		return t.emitSingle(TkGt, start)
	case '!':
		return t.emitSingle(TkNot, start)
	case '|':
		return t.emitSingle(TkPipe, start)
	case '(':
		return t.emitSingle(TkLParen, start)
	case ')':
		return t.emitSingle(TkRParen, start)
	case '{':
		return t.emitSingle(TkLBrace, start)
	case '}':
		return t.emitSingle(TkRBrace, start)
	case ';':
		return t.emitSingle(TkSemicolon, start)
	case ':':
		return t.emitSingle(TkColon, start)
	case ',':
		return t.emitSingle(TkComma, start)
	case '.':
		return t.emitSingle(TkDot, start)
	case '#':
		return t.emitSingle(TkHash, start)
	case '?':
		return t.emitSingle(TkQuestion, start)
	case '@':
		return t.emitSingle(TkAt, start)
	}

	// Unknown character
	t.advance()
	span := Span{File: t.file, Start: start, End: t.pos}
	t.diags.Errorf(span, "unexpected character %q (0x%02x)", b, b)
	return Token{Kind: TkError, Span: span, Raw: t.source[start:t.pos]}
}

// ─── Emit helpers ────────────────────────────────────────────────────────────

func (t *Tokenizer) emitSingle(kind TokenKind, start int) Token {
	t.advance()
	return t.makeToken(kind, start)
}

func (t *Tokenizer) emitDouble(kind TokenKind, start int, n int) Token {
	for i := 0; i < n; i++ {
		t.advance()
	}
	return t.makeToken(kind, start)
}

// ─── String scanning ─────────────────────────────────────────────────────────

func (t *Tokenizer) scanString() Token {
	start := t.pos
	// consume opening « (2 bytes: 0xC2 0xAB)
	t.advance()
	t.advance()

	strStart := t.pos // start of inner text (after opening «)

	for t.pos < len(t.source) {
		// Check for closing » (0xC2 0xBB)
		if t.source[t.pos] == 0xC2 && t.peekN(1) == 0xBB {
			t.advance()
			t.advance()
			raw := t.source[start:t.pos]
			inner := t.source[strStart : t.pos-2] // exclude closing »
			return Token{
				Kind:  TkString,
				Span:  Span{File: t.file, Start: start, End: t.pos},
				Raw:   raw,
				Value: string(inner),
			}
		}
		// Newlines are allowed inside strings
		t.advance()
	}

	// Unterminated string
	span := Span{File: t.file, Start: start, End: t.pos}
	t.diags.Error(span, "unterminated string literal")
	return Token{Kind: TkError, Span: span, Raw: t.source[start:t.pos]}
}

// ─── Identifier / keyword scanning ───────────────────────────────────────────

// peekRune decodes the UTF-8 rune at the current position without advancing.
func (t *Tokenizer) peekRune() (rune, int) {
	if t.pos >= len(t.source) {
		return utf8.RuneError, 0
	}
	r, size := utf8.DecodeRune(t.source[t.pos:])
	return r, size
}

// isIdentStart returns true if the byte at the current position starts an
// identifier character (ASCII letter, underscore, or non-ASCII letter).
func (t *Tokenizer) isIdentStart() bool {
	if t.pos >= len(t.source) {
		return false
	}
	b := t.source[t.pos]
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	if b == '_' {
		return true
	}
	if b >= 0x80 {
		r, _ := t.peekRune()
		return unicode.IsLetter(r)
	}
	return false
}

// isIdentCont returns true if the byte at the current position can continue
// an identifier (letters, digits, underscore, or UTF-8 continuation bytes).
func (t *Tokenizer) isIdentCont() bool {
	if t.pos >= len(t.source) {
		return false
	}
	b := t.source[t.pos]
	// UTF-8 continuation bytes are always part of the current rune.
	if b >= 0x80 && b <= 0xBF {
		return true
	}
	if b >= '0' && b <= '9' {
		return true
	}
	return t.isIdentStart()
}

func (t *Tokenizer) scanIdentOrKeyword() Token {
	start := t.pos
	for t.pos < len(t.source) && t.isIdentCont() {
		t.advance()
	}
	raw := t.source[start:t.pos]
	text := string(raw)

	if kind, ok := LookupKeyword(text); ok {
		if kind == TkTrue {
			return t.makeTokenValue(kind, start, true)
		}
		if kind == TkFalse {
			return t.makeTokenValue(kind, start, false)
		}
		return t.makeToken(kind, start)
	}
	return t.makeToken(TkIdent, start)
}

// ─── Number scanning ─────────────────────────────────────────────────────────

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func (t *Tokenizer) scanNumber() Token {
	start := t.pos
	isFloat := false
	lastWasDigit := false
	lastWasUnderscore := false

	for t.pos < len(t.source) {
		b := t.source[t.pos]
		if isDigit(b) {
			lastWasDigit = true
			lastWasUnderscore = false
			t.advance()
			continue
		}
		if b == '_' {
			if !lastWasDigit {
				// Underscore must follow a digit: reject 1__2, _5, etc.
				span := Span{File: t.file, Start: t.pos, End: t.pos + 1}
				t.diags.Error(span, "misplaced underscore in numeric literal")
				// Consume the bad underscore and continue scanning
				// so subsequent digits are still part of the literal.
			}
			lastWasDigit = false
			lastWasUnderscore = true
			t.advance()
			continue
		}
		if b == '.' && !isFloat {
			if t.peekN(1) == '.' {
				break // ellipsis, not a float
			}
			isFloat = true
			lastWasUnderscore = false
			lastWasDigit = false // reset so we require digits after the dot
			t.advance()
			continue
		}
		break
	}

	// If we only consumed a dot with no digits, that's just a dot token.
	if start == t.pos-1 && t.source[start] == '.' {
		return t.emitSingle(TkDot, start)
	}

	// Reject trailing underscore (e.g. "1_")
	if lastWasUnderscore {
		span := Span{File: t.file, Start: t.pos - 1, End: t.pos}
		t.diags.Error(span, "trailing underscore in numeric literal")
	}

	raw := t.source[start:t.pos]

	if isFloat {
		return Token{
			Kind:  TkFloat,
			Span:  Span{File: t.file, Start: start, End: t.pos},
			Raw:   raw,
			Value: string(raw),
		}
	}
	return Token{
		Kind:  TkInt,
		Span:  Span{File: t.file, Start: start, End: t.pos},
		Raw:   raw,
		Value: string(raw),
	}
}

// ─── Convenience ─────────────────────────────────────────────────────────────

// Tokenize is a convenience function that creates a tokenizer, runs it, and
// returns the tokens and diagnostics.
func Tokenize(source []byte, file FileID) (TokenList, DiagnosticList) {
	return NewTokenizer(source, file).Tokenize()
}