package main

// ─── File identity ───────────────────────────────────────────────────────────

// FileID uniquely identifies a source file within a compilation session.
type FileID int32

// SourceFile represents a single source file known to the compiler.
type SourceFile struct {
	ID          FileID
	Path        string
	Source      []byte
	LineOffsets []int // cached line-start byte offsets (see BuildLineOffsets)
}

// SourceManager maps FileID values to SourceFile records.
type SourceManager struct {
	files []SourceFile
}

// Register adds a source file and returns its FileID.
func (sm *SourceManager) Register(path string, source []byte) FileID {
	id := FileID(len(sm.files))
	sm.files = append(sm.files, SourceFile{
		ID:          id,
		Path:        path,
		Source:      source,
		LineOffsets: BuildLineOffsets(source),
	})
	return id
}

// Lookup returns the SourceFile for a given FileID, or nil if unknown.
func (sm *SourceManager) Lookup(id FileID) *SourceFile {
	if int(id) < len(sm.files) {
		return &sm.files[id]
	}
	return nil
}

// ─── Span ────────────────────────────────────────────────────────────────────

// Span represents a half-open byte range [Start, End) in a source file.
// Start is the byte offset of the first byte (inclusive); End is the byte
// offset of the first byte *after* the span (exclusive). An empty span has
// Start == End. The EOF sentinel span is {File, len, len}.
type Span struct {
	File  FileID
	Start int // inclusive
	End   int // exclusive
}

// ─── TokenKind ───────────────────────────────────────────────────────────────

type TokenKind uint8

const (
	// Special
	TkEOF   TokenKind = iota // end of file / end of input
	TkError                   // invalid token (produced during error recovery)

	// Literals
	TkIdent  // identifier
	TkInt    // integer literal (raw text stored in Value)
	TkFloat  // float literal (raw text stored in Value)
	TkString // string literal «...» (inner text stored in Value)
	TkTrue   // keyword true
	TkFalse  // keyword false

	// Assignment / declaration
	TkAssign         // =
	TkInfer          // :=
	TkCompTimeAssign // ::

	// Delimiters
	TkLParen    // (
	TkRParen    // )
	TkLBrace    // {
	TkRBrace    // }
	TkSemicolon // ;
	TkColon     // :
	TkComma     // ,
	TkDot       // .

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
	TkEOF:            "EOF",
	TkError:          "Error",
	TkIdent:          "identifier",
	TkInt:            "integer literal",
	TkFloat:          "float literal",
	TkString:         "string literal",
	TkTrue:           "true",
	TkFalse:          "false",
	TkAssign:         "=",
	TkInfer:          ":=",
	TkCompTimeAssign: "::",
	TkLParen:         "(",
	TkRParen:         ")",
	TkLBrace:         "{",
	TkRBrace:         "}",
	TkSemicolon:      ";",
	TkColon:          ":",
	TkComma:          ",",
	TkDot:            ".",
	TkPlus:           "+",
	TkMinus:          "-",
	TkStar:           "*",
	TkSlash:          "/",
	TkEq:             "==",
	TkNeq:            "!=",
	TkLt:             "<",
	TkGt:             ">",
	TkLe:             "<=",
	TkGe:             ">=",
	TkAnd:            "&&",
	TkOr:             "||",
	TkNot:            "!",
	TkArrow:          "->",
	TkEllipsis:       "...",
	TkHash:           "#",
	TkQuestion:       "?",
	TkAt:             "@",
	TkPipe:           "|",
	TkExit:           "exit",
	TkIf:             "if",
	TkElif:           "elif",
	TkElse:           "else",
	TkProc:           "proc",
	TkThen:           "then",
	TkReturn:         "return",
	TkAs:             "as",
}

func (k TokenKind) String() string {
	if int(k) < len(tokenKindNames) {
		return tokenKindNames[k]
	}
	return "unknown"
}

// ─── Token ───────────────────────────────────────────────────────────────────

// Token represents one lexical token with its source span and raw text.
//
// Value carries the decoded literal:
//   - TkInt, TkFloat: raw source text as string (parsing deferred to semantic phase)
//   - TkString: inner text (without guillemets) as string
//   - TkTrue, TkFalse: bool
//   - all others: nil
type Token struct {
	Kind  TokenKind
	Span  Span
	Raw   []byte // raw source bytes (slice into the original source buffer)
	Value any    // parsed literal: string, bool, or nil
}

// Text returns the raw source text of the token.
func (t Token) Text() string { return string(t.Raw) }

// ─── Token list ──────────────────────────────────────────────────────────────

// TokenList is a simple slice of tokens produced by the tokenizer.
type TokenList []Token

// ─── Keyword lookup ──────────────────────────────────────────────────────────

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