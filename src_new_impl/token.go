package main

// ─── File identity ───────────────────────────────────────────────────────────

// FileID uniquely identifies a source file within a compilation session.
type FileID int32

// ─── Span ────────────────────────────────────────────────────────────────────

// Span represents a contiguous byte range in a source file. This is the
// canonical position type for every token, AST node, and diagnostic. The
// tokenizer converts line/column tracking into byte offsets so that later
// phases never need to re-derive positions.
type Span struct {
	File  FileID
	Start int // byte offset of the first byte (inclusive)
	End   int // byte offset of the last byte (inclusive); End < Start means empty
}

// ─── TokenKind ───────────────────────────────────────────────────────────────

type TokenKind uint8

const (
	// Special
	TkEOF   TokenKind = iota // end of file / end of input
	TkError                   // invalid token (produced during error recovery)

	// Literals
	TkIdent  // identifier
	TkInt    // integer literal (parsed as string, not host int)
	TkFloat  // float literal (parsed as string)
	TkString // string literal «...»
	TkTrue   // keyword true
	TkFalse  // keyword false

	// Assignment / declaration
	TkAssign         // =
	TkInfer          // :=
	TkCompTimeAssign // ::

	// Delimiters
	TkLParen   // (
	TkRParen   // )
	TkLBrace   // {
	TkRBrace   // }
	TkSemicolon // ;
	TkColon    // :
	TkComma    // ,
	TkDot      // .

	// Arithmetic
	TkPlus  // +
	TkMinus // -
	TkStar  // *
	TkSlash // /

	// Comparison
	TkEq  // ==
	TkNeq // !=
	TkLt  // <
	TkGt  // >
	TkLe  // <=
	TkGe  // >=

	// Logical
	TkAnd // &&
	TkOr  // ||
	TkNot // !

	// Other operators
	TkArrow    // ->
	TkEllipsis // ...
	TkHash     // #
	TkQuestion // ?
	TkAt       // @
	TkPipe     // |

	// Keywords
	TkExit
	TkIf
	TkElif
	TkElse
	TkProc
	TkThen
	TkReturn
	TkAs
)

var tokenKindNames = [...]string{
	TkEOF:           "EOF",
	TkError:         "Error",
	TkIdent:         "identifier",
	TkInt:           "integer literal",
	TkFloat:         "float literal",
	TkString:        "string literal",
	TkTrue:          "true",
	TkFalse:         "false",
	TkAssign:        "=",
	TkInfer:         ":=",
	TkCompTimeAssign: "::",
	TkLParen:        "(",
	TkRParen:        ")",
	TkLBrace:        "{",
	TkRBrace:        "}",
	TkSemicolon:     ";",
	TkColon:         ":",
	TkComma:         ",",
	TkDot:           ".",
	TkPlus:          "+",
	TkMinus:         "-",
	TkStar:          "*",
	TkSlash:         "/",
	TkEq:            "==",
	TkNeq:           "!=",
	TkLt:            "<",
	TkGt:            ">",
	TkLe:            "<=",
	TkGe:            ">=",
	TkAnd:           "&&",
	TkOr:            "||",
	TkNot:           "!",
	TkArrow:         "->",
	TkEllipsis:      "...",
	TkHash:          "#",
	TkQuestion:      "?",
	TkAt:            "@",
	TkPipe:          "|",
	TkExit:          "exit",
	TkIf:            "if",
	TkElif:          "elif",
	TkElse:          "else",
	TkProc:          "proc",
	TkThen:          "then",
	TkReturn:        "return",
	TkAs:            "as",
}

func (k TokenKind) String() string {
	if int(k) < len(tokenKindNames) {
		return tokenKindNames[k]
	}
	return "unknown"
}

// ─── Token ───────────────────────────────────────────────────────────────────

// Text returns the raw source text of the token. It is only valid when the
// tokenizer retains the source buffer (see Tokenizer.Source).
func (t Token) Text() string { return string(t.Raw) }

// Token represents one lexical token with its source span and raw text.
// The Value field carries the parsed literal for int/float/string/bool tokens.
type Token struct {
	Kind  TokenKind
	Span  Span
	Raw   []byte // raw source bytes (slice into the original source buffer)
	Value any    // parsed value: int64, float64, string, bool, or nil
}

// ─── Position ────────────────────────────────────────────────────────────────

// Position is a human-readable 1-based line:column location. It is computed
// lazily from a Span via a line-index table. Tokenizer methods that need to
// format diagnostics use this during scanning; after tokenization, positions
// are derived from Spans.
type Position struct {
	Line   int // 1-based
	Column int // 1-based byte column
}

// ─── Token list ──────────────────────────────────────────────────────────────

// TokenList is a simple slice of tokens produced by the tokenizer.
type TokenList []Token

// ─── Keyword lookup ──────────────────────────────────────────────────────────

// keywordTokens maps identifier text to the corresponding keyword TokenKind.
// It is nil for identifiers that are not keywords.
var keywordTokens map[string]TokenKind

func init() {
	keywordTokens = make(map[string]TokenKind, 16)
	for _, entry := range []struct {
		text string
		kind TokenKind
	}{
		{"true", TkTrue},
		{"false", TkFalse},
		{"exit", TkExit},
		{"if", TkIf},
		{"elif", TkElif},
		{"else", TkElse},
		{"proc", TkProc},
		{"then", TkThen},
		{"return", TkReturn},
		{"as", TkAs},
	} {
		keywordTokens[entry.text] = entry.kind
	}
}

// LookupKeyword returns the keyword kind and true if text is a keyword,
// or TkIdent and false otherwise.
func LookupKeyword(text string) (TokenKind, bool) {
	k, ok := keywordTokens[text]
	if ok {
		return k, true
	}
	return TkIdent, false
}