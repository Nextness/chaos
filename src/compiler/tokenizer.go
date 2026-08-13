package compiler

import (
	"strconv"
	"unicode"
	"unicode/utf8"
)

// Tokenizer converts a source buffer into a TokenList. It is a stateful
// cursor-based scanner that reports errors via a DiagnosticList instead of
// panicking.
type Tokenizer struct {
	source  []byte // the full source buffer
	pos     int    // current byte offset in source
	file    FileID // file identity for Span
	diags   DiagnosticList
	tokens  TokenList
	lastPos int // position before the current token, used for progress guard

	// pendingDirec is set when a '#' is immediately followed by an identifier
	// start; the next token is then scanned as a TkDirec directive name.
	pendingDirec bool
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

// skipTrivia advances past whitespace. Comments are not trivia: they are
// emitted as TkComment tokens by scanComment so the LSP can highlight them.
func (t *Tokenizer) skipTrivia() {
	for t.pos < len(t.source) {
		b := t.source[t.pos]

		// Whitespace
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			t.advance()
			continue
		}

		// Not trivia
		break
	}
}

// scanComment scans a comment starting at the current position and returns a
// token. Line comments (//) and block comments (/** ... **/) both produce
// TkComment tokens. An unterminated block comment produces a TkError token
// plus a diagnostic; scanComment always advances pos so the TkError-only
// progress guard in Tokenize cannot stall.
func (t *Tokenizer) scanComment() Token {
	start := t.pos

	// Line comment '//' consume until newline or EOF
	if t.source[t.pos] == '/' && t.peekN(1) == '/' {
		t.advance()
		t.advance()
		for t.pos < len(t.source) && t.source[t.pos] != '\n' {
			t.advance()
		}
		return t.makeToken(TkComment, start)
	}

	// Block comment '/** ... **/' handle nesting
	t.advance()
	t.advance()
	t.advance()
	depth := 1
	for t.pos < len(t.source) && depth > 0 {
		// Check open before advance
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
		t.diags.Error(Span{File: t.file, Start: t.pos, End: t.pos}, "unterminated block comment", "add a closing '**/' to end the comment")
		return Token{Kind: TkError, Span: Span{File: t.file, Start: start, End: t.pos}, Raw: t.source[start:t.pos]}
	}
	return t.makeToken(TkComment, start)
}

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
func (t *Tokenizer) makeTokenValue(kind TokenKind, start int, value string) Token {
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

	// A directive name follows a '#' with no intervening whitespace. Scan it
	// directly as TkDirec, bypassing keyword lookup so #proc and #true stay
	// directives.
	if t.pendingDirec {
		t.pendingDirec = false
		if t.isIdentStart() {
			return t.scanDirec()
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

	// Comments: // line and /** ... **/ block
	if b == '/' && (t.peekN(1) == '/' || (t.peekN(1) == '*' && t.peekN(2) == '*')) {
		return t.scanComment()
	}

	// Multi-character operators (must be checked before single-char)
	switch {
	case b == '=' && t.peekN(1) == '=':
		return t.emitN(TkEq, start, 2)
	case b == '!' && t.peekN(1) == '=':
		return t.emitN(TkNeq, start, 2)
	case b == '<' && t.peekN(1) == '=':
		return t.emitN(TkLe, start, 2)
	case b == '<' && t.peekN(1) == '>':
		return t.emitN(TkErrorReturn, start, 2)
	case b == '>' && t.peekN(1) == '=':
		return t.emitN(TkGe, start, 2)
	case b == '&' && t.peekN(1) == '&':
		return t.emitN(TkAnd, start, 2)
	case b == '|' && t.peekN(1) == '|':
		return t.emitN(TkOr, start, 2)
	case b == '-' && t.peekN(1) == '>':
		return t.emitN(TkArrow, start, 2)
	case b == '.' && t.peekN(1) == '.' && t.peekN(2) == '.':
		return t.emitN(TkEllipsis, start, 3)
	}

	// Single-character tokens
	switch b {
	case '=':
		return t.emitN(TkAssign, start, 1)
	case '+':
		return t.emitN(TkPlus, start, 1)
	case '-':
		return t.emitN(TkMinus, start, 1)
	case '*':
		return t.emitN(TkStar, start, 1)
	case '/':
		return t.emitN(TkSlash, start, 1)
	case '%':
		return t.emitN(TkPercent, start, 1)
	case '<':
		return t.emitN(TkLt, start, 1)
	case '>':
		return t.emitN(TkGt, start, 1)
	case '!':
		return t.emitN(TkNot, start, 1)
	case '|':
		return t.emitN(TkPipe, start, 1)
	case '(':
		return t.emitN(TkLParen, start, 1)
	case ')':
		return t.emitN(TkRParen, start, 1)
	case '{':
		return t.emitN(TkLBrace, start, 1)
	case '}':
		return t.emitN(TkRBrace, start, 1)
	case '[':
		return t.emitN(TkLBracket, start, 1)
	case ']':
		return t.emitN(TkRBracket, start, 1)
	case ';':
		return t.emitN(TkSemicolon, start, 1)
	case ':':
		return t.emitN(TkColon, start, 1)
	case ',':
		return t.emitN(TkComma, start, 1)
	case '.':
		return t.emitN(TkDot, start, 1)
	case '#':
		// Compile-time directive: #<name> becomes TkHash + TkDirec(name).
		// If the byte after '#' starts an identifier, the next token is
		// scanned as a directive name.
		if t.isIdentStartAt(t.pos + 1) {
			t.pendingDirec = true
		}
		return t.emitN(TkHash, start, 1)
	case '?':
		return t.emitN(TkQuestion, start, 1)
	case '@':
		return t.emitN(TkAt, start, 1)
	}

	// Unknown character
	t.advance()
	span := Span{File: t.file, Start: start, End: t.pos}
	hexValue := strconv.FormatUint(uint64(b), 16)
	if len(hexValue) == 1 {
		hexValue = "0" + hexValue
	}
	t.diags.Error(span, "unexpected character "+strconv.QuoteRuneToASCII(rune(b))+" (0x"+hexValue+")", "remove the character or replace it with a valid token")
	return Token{Kind: TkError, Span: span, Raw: t.source[start:t.pos]}
}

func (t *Tokenizer) emitN(kind TokenKind, start int, n int) Token {
	for i := 0; i < n; i++ {
		t.advance()
	}
	return t.makeToken(kind, start)
}

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
	t.diags.Error(span, "unterminated string literal", "close the string with '»'")
	return Token{Kind: TkError, Span: span, Raw: t.source[start:t.pos]}
}

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
	return t.isIdentStartAt(t.pos)
}

// isIdentStartAt returns true if the byte at the given position starts an
// identifier character.
func (t *Tokenizer) isIdentStartAt(pos int) bool {
	if pos >= len(t.source) {
		return false
	}
	b := t.source[pos]
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
		r, _ := utf8.DecodeRune(t.source[pos:])
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
			return t.makeTokenValue(kind, start, "true")
		}
		if kind == TkFalse {
			return t.makeTokenValue(kind, start, "false")
		}
		return t.makeToken(kind, start)
	}
	return t.makeToken(TkIdent, start)
}

// scanDirec scans a compile-time directive name after '#'. The name is scanned
// directly, bypassing LookupKeyword/scanIdentOrKeyword, so #proc and #true
// stay directives.
func (t *Tokenizer) scanDirec() Token {
	start := t.pos
	for t.pos < len(t.source) && t.isIdentCont() {
		t.advance()
	}
	raw := t.source[start:t.pos]
	return t.makeTokenValue(TkDirec, start, string(raw))
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func (t *Tokenizer) scanNumber() Token {
	start := t.pos
	isFloat := false
	sawDigitAfterDot := false
	lastWasDigit := false
	lastWasUnderscore := false

	for t.pos < len(t.source) {
		b := t.source[t.pos]
		if isDigit(b) {
			lastWasDigit = true
			lastWasUnderscore = false
			if isFloat {
				sawDigitAfterDot = true
			}
			t.advance()
			continue
		}
		if b == '_' {
			if !lastWasDigit {
				// Underscore must follow a digit: reject 1__2, _5, etc.
				span := Span{File: t.file, Start: t.pos, End: t.pos + 1}
				t.diags.Error(span, "misplaced underscore in numeric literal", "remove the underscore or place it between digits")
				// Consume the bad underscore and continue scanning
				// so subsequent digits are still part of the literal.
			}
			lastWasDigit = false
			lastWasUnderscore = true
			t.advance()
			continue
		}
		if b == '.' && !isFloat {
			if t.peekN(1) == '.' && t.peekN(2) == '.' {
				break // ellipsis, not a float
			}
			if t.peekN(1) == '.' {
				// Two dots (not three): "1..0" is invalid syntax.
				// Emit a diagnostic pointing at the second dot and any
				// following digits (e.g. ".0" in "1..0").
				end := t.pos + 2
				for end < len(t.source) && isDigit(t.source[end]) {
					end++
				}
				span := Span{File: t.file, Start: t.pos + 1, End: end}
				t.diags.Error(span, "invalid syntax: cannot start a float literal with '.' after '.'", "remove the extra '.'")
				break
			}
			isFloat = true
			if lastWasUnderscore {
				span := Span{File: t.file, Start: t.pos - 1, End: t.pos}
				t.diags.Error(span, "misplaced underscore before decimal point", "remove the underscore before the decimal point")
			}
			lastWasUnderscore = false
			lastWasDigit = false // reset so we require digits after the dot
			t.advance()
			continue
		}
		break
	}

	// Reject trailing underscore (e.g. "1_")
	if lastWasUnderscore {
		span := Span{File: t.file, Start: t.pos - 1, End: t.pos}
		t.diags.Error(span, "trailing underscore in numeric literal", "remove the trailing underscore")
	}

	// Reject float with no digit after decimal point (e.g. "1.")
	if isFloat && !sawDigitAfterDot {
		span := Span{File: t.file, Start: t.pos - 1, End: t.pos}
		t.diags.Error(span, "expected digit after decimal point", "add a digit after the decimal point")
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

// Tokenize is a convenience function that creates a tokenizer, runs it, and
// returns the tokens and diagnostics.
func Tokenize(source []byte, file FileID) (TokenList, DiagnosticList) {
	return NewTokenizer(source, file).Tokenize()
}
