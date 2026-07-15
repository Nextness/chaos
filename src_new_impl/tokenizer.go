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
//   - EOF is always safe: every scan loop checks bounds before reading.
//   - Block comments handle nesting correctly.
//   - Line/column tracking is maintained during scanning for diagnostics.
type Tokenizer struct {
	source []byte   // the full source buffer
	pos    int      // current byte offset in source
	file   FileID   // file identity for Span
	line   int      // 1-based line at pos
	col    int      // 1-based byte column at pos
	diags  DiagnosticList
	tokens TokenList
}

// NewTokenizer creates a tokenizer for the given source buffer.
func NewTokenizer(source []byte, file FileID) *Tokenizer {
	return &Tokenizer{
		source: source,
		file:   file,
		line:   1,
		col:    1,
	}
}

// Tokenize runs the scanner and returns the token list and any diagnostics.
func (t *Tokenizer) Tokenize() (TokenList, DiagnosticList) {
	t.tokens = nil
	t.diags = nil
	for {
		tok := t.next()
		t.tokens = append(t.tokens, tok)
		if tok.Kind == TkEOF || tok.Kind == TkError {
			break
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

// advance consumes one byte and updates line/column tracking.
func (t *Tokenizer) advance() {
	if t.pos >= len(t.source) {
		return
	}
	if t.source[t.pos] == '\n' {
		t.line++
		t.col = 1
	} else {
		t.col++
	}
	t.pos++
}

// advanceRune consumes one full UTF-8 rune from the current position and
// updates line/column tracking. For ASCII bytes this is equivalent to
// advance(); for multi-byte runes it advances by the full encoded width.
func (t *Tokenizer) advanceRune() {
	if t.pos >= len(t.source) {
		return
	}
	if t.source[t.pos] == '\n' {
		t.line++
		t.col = 1
		t.pos++
		return
	}
	_, size := utf8.DecodeRune(t.source[t.pos:])
	if size <= 0 {
		size = 1 // safety: advance at least 1 byte
	}
	t.pos += size
	t.col++
}

// skipWhitespace advances past spaces, tabs, carriage returns (but not newlines,
// which are handled by advance). Newlines are consumed by advance but are not
// significant tokens in Chaos — they are whitespace.
func (t *Tokenizer) skipWhitespace() {
	for t.pos < len(t.source) {
		b := t.source[t.pos]
		if b == ' ' || b == '\t' || b == '\r' {
			t.advance()
			continue
		}
		if b == '\n' {
			t.advance()
			continue
		}
		break
	}
}

// makeToken creates a token with the given kind spanning from start to current pos.
func (t *Tokenizer) makeToken(kind TokenKind, start int) Token {
	return Token{
		Kind: kind,
		Span: Span{File: t.file, Start: start, End: t.pos - 1},
		Raw:  t.source[start:t.pos],
	}
}

// makeTokenValue creates a token with a parsed value.
func (t *Tokenizer) makeTokenValue(kind TokenKind, start int, value any) Token {
	tok := t.makeToken(kind, start)
	tok.Value = value
	return tok
}

// ─── Token dispatch ──────────────────────────────────────────────────────────

func (t *Tokenizer) next() Token {
	t.skipWhitespace()

	start := t.pos
	if start >= len(t.source) {
		return Token{
			Kind: TkEOF,
			Span: Span{File: t.file, Start: start, End: start - 1},
		}
	}

	b := t.source[start]

	// Line comment: //
	if b == '/' && t.peekN(1) == '/' {
		return t.scanLineComment()
	}

	// Block comment: /** ... **/
	if b == '/' && t.peekN(1) == '*' && t.peekN(2) == '*' {
		return t.scanBlockComment()
	}

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
	span := Span{File: t.file, Start: start, End: t.pos - 1}
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

// ─── Comment scanning ────────────────────────────────────────────────────────

func (t *Tokenizer) scanLineComment() Token {
	// consume //
	t.advance()
	t.advance()
	for t.pos < len(t.source) {
		if t.source[t.pos] == '\n' {
			break
		}
		t.advance()
	}
	// Line comments are not tokens; recurse to get the next real token.
	return t.next()
}

func (t *Tokenizer) scanBlockComment() Token {
	start := t.pos
	// consume /**
	t.advance()
	t.advance()
	t.advance()
	depth := 1
	for t.pos < len(t.source) {
		// Check for /** opener before advancing (fixes P0-16: nested opener
		// immediately after outer opener was missed by the old lexer).
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
			if depth == 0 {
				// Block comments are not tokens; recurse.
				return t.next()
			}
			continue
		}
		t.advance()
	}
	// Unterminated block comment
	span := Span{File: t.file, Start: start, End: t.pos - 1}
	t.diags.Error(span, "unterminated block comment")
	return Token{Kind: TkError, Span: span, Raw: t.source[start:t.pos]}
}

// ─── String scanning ─────────────────────────────────────────────────────────

func (t *Tokenizer) scanString() Token {
	start := t.pos
	// consume opening « (2 bytes: 0xC2 0xAB)
	t.advance()
	t.advance()

	// Track string start for diagnostics
	strStart := t.pos

	for t.pos < len(t.source) {
		// Check for closing » (0xC2 0xBB)
		if t.source[t.pos] == 0xC2 && t.peekN(1) == 0xBB {
			t.advance()
			t.advance()
			// Raw includes the guillemets; value is the inner text.
			raw := t.source[start:t.pos]
			inner := t.source[strStart : t.pos-2]
			return Token{
				Kind:  TkString,
				Span:  Span{File: t.file, Start: start, End: t.pos - 1},
				Raw:   raw,
				Value: string(inner),
			}
		}
		// Handle newlines inside strings (allowed in Chaos)
		if t.source[t.pos] == '\n' {
			t.advance()
			continue
		}
		t.advance()
	}

	// Unterminated string
	span := Span{File: t.file, Start: start, End: t.pos - 1}
	t.diags.Error(span, "unterminated string literal")
	return Token{Kind: TkError, Span: span, Raw: t.source[start:t.pos]}
}

// ─── Identifier / keyword scanning ───────────────────────────────────────────

// peekRune decodes the UTF-8 rune at the current position without advancing.
// It returns the rune and its byte width. If the encoding is invalid or at EOF,
// it returns utf8.RuneError and 1 (so the caller can still advance past it).
func (t *Tokenizer) peekRune() (rune, int) {
	if t.pos >= len(t.source) {
		return utf8.RuneError, 0
	}
	r, size := utf8.DecodeRune(t.source[t.pos:])
	return r, size
}

// isIdentByte returns true if the byte at the current position starts an
// identifier character (letter, digit, underscore, or non-ASCII letter).
func (t *Tokenizer) isIdentStart() bool {
	if t.pos >= len(t.source) {
		return false
	}
	b := t.source[t.pos]
	// ASCII fast path
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	if b == '_' {
		return true
	}
	// Non-ASCII: decode the full rune and check
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
	// UTF-8 continuation bytes (0x80-0xBF) are always part of the current rune.
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
		// true and false carry a bool value
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

	// Consume digits and optional underscores.
	for t.pos < len(t.source) {
		b := t.source[t.pos]
		if isDigit(b) {
			t.advance()
			continue
		}
		if b == '_' {
			// Underscore separators: skip but don't stop.
			t.advance()
			continue
		}
		if b == '.' && !isFloat {
			// Check that the dot is not followed by another dot (ellipsis)
			if t.peekN(1) == '.' {
				break
			}
			isFloat = true
			t.advance()
			continue
		}
		break
	}

	// If we only consumed a dot with no digits after it, that's just a dot token.
	if start == t.pos-1 && t.source[start] == '.' {
		return t.emitSingle(TkDot, start)
	}

	raw := t.source[start:t.pos]

	if isFloat {
		return Token{
			Kind:  TkFloat,
			Span:  Span{File: t.file, Start: start, End: t.pos - 1},
			Raw:   raw,
			Value: string(raw),
		}
	}
	return Token{
		Kind:  TkInt,
		Span:  Span{File: t.file, Start: start, End: t.pos - 1},
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