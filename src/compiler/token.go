package compiler

import "strconv"

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
	if id >= 0 && int(id) < len(sm.files) {
		return &sm.files[id]
	}
	return nil
}

// Span represents a half-open byte range [Start, End) in a source file.
// Start is the byte offset of the first byte (inclusive); End is the byte
// offset of the first byte *after* the span (exclusive). An empty span has
// Start == End. The EOF sentinel span is {File, len, len}.
type Span struct {
	File  FileID
	Start int // inclusive
	End   int // exclusive
}

type TokenKind uint8

const (
	// Special
	TkEOF   TokenKind = iota // end of file / end of input
	TkError                  // invalid token (produced during error recovery)

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
	TkLBracket  // [
	TkRBracket  // ]
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

	// Compile-time directives and comments
	TkDirec   // compile-time directive name after '#'
	TkComment // comment token (// line or /** ... **/ block)
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
	TkLBracket:       "[",
	TkRBracket:       "]",
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
	TkDirec:          "directive",
	TkComment:        "comment",
}

func (k TokenKind) String() string {
	if int(k) < len(tokenKindNames) {
		return tokenKindNames[k]
	}
	panic("TokenKind.String: " + strconv.Itoa(int(k)) + " has no entry in tokenKindNames")
}

// Token represents one lexical token with its source span and raw text.
// Value carries the decoded literal text:
//   - TkInt, TkFloat: raw source text as string
//   - TkString: inner text (without guillemets) as string
//   - TkTrue, TkFalse: "true" or "false" (same as Raw)
//   - all others: empty string (callers check Kind to determine if Value is set)
type Token struct {
	Kind  TokenKind
	Span  Span
	Raw   []byte // raw source bytes (slice into the original source buffer)
	Value string // decoded literal text, empty for non-literal tokens
}

// Text returns the raw source text of the token.
func (t Token) Text() string {
	return string(t.Raw)
}

// TokenList is a simple slice of tokens produced by the tokenizer.
type TokenList []Token

// This is a global variable that is initialized by the compiler
var keywordTokens map[string]TokenKind

func init() {
	type keywordEntry struct {
		text string
		kind TokenKind
	}
	keywordEntries := []keywordEntry{
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
	}
	keywordTokens = make(map[string]TokenKind, 16)
	for _, entry := range keywordEntries {
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
