package main

// LSP wire types (JSON-RPC 2.0 over stdio). Only the subset used by the
// server is defined.

// Position is a 0-based line/character position in UTF-16 code units.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a half-open range of Positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Diagnostic is a compiler message attached to a range.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// PublishDiagnosticsParams is the payload of textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// TextDocumentItem describes an open document.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// VersionedTextDocumentIdentifier identifies a document version.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// DidOpenTextDocumentParams is the payload of textDocument/didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentContentChangeEvent is one incremental edit. Range is nil for a
// full-document replacement.
type TextDocumentContentChangeEvent struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}

// DidChangeTextDocumentParams is the payload of textDocument/didChange.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidCloseTextDocumentParams is the payload of textDocument/didClose.
type DidCloseTextDocumentParams struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
}

// TextDocumentSyncOptions advertises open/close and incremental sync.
type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"`
}

// SemanticTokensLegend declares the semantic token types and modifiers.
type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

// SemanticTokensOptions advertises the semantic tokens provider.
type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

// CompletionOptions advertises the completion provider.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// ServerCapabilities is the capabilities object returned by initialize.
type ServerCapabilities struct {
	TextDocumentSync          *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	DocumentSymbolProvider    bool                     `json:"documentSymbolProvider,omitempty"`
	SemanticTokensProvider    *SemanticTokensOptions   `json:"semanticTokensProvider,omitempty"`
	DefinitionProvider        bool                     `json:"definitionProvider,omitempty"`
	ReferencesProvider        bool                     `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider bool                     `json:"documentHighlightProvider,omitempty"`
	HoverProvider             bool                     `json:"hoverProvider,omitempty"`
	CompletionProvider        *CompletionOptions       `json:"completionProvider,omitempty"`
}

// ServerInfo identifies the server in the initialize result.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the response to initialize.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

// DocumentSymbolParams is the payload of textDocument/documentSymbol.
type DocumentSymbolParams struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
}

// DocumentSymbol is a symbol in the document outline.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SemanticTokensParams is the payload of textDocument/semanticTokens/full.
type SemanticTokensParams struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
}

// SemanticTokens is the response to textDocument/semanticTokens/full.
type SemanticTokens struct {
	Data []int `json:"data"`
}

// TextDocumentPositionParams is the payload of definition, references,
// documentHighlight, hover, and completion.
type TextDocumentPositionParams struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
	Position     Position                        `json:"position"`
}

// ReferenceContext is the context of a references request.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams is the payload of textDocument/references.
type ReferenceParams struct {
	TextDocument VersionedTextDocumentIdentifier `json:"textDocument"`
	Position     Position                        `json:"position"`
	Context      ReferenceContext                `json:"context"`
}

// DocumentHighlight is a range highlighted for the identifier under the cursor.
type DocumentHighlight struct {
	Range Range `json:"range"`
	Kind  int   `json:"kind,omitempty"`
}

// MarkupContent is a markdown or plaintext value.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover is the response to textDocument/hover.
type Hover struct {
	Contents MarkupContent `json:"contents"`
}

// CompletionItem is one completion suggestion.
type CompletionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// CompletionList is the response to textDocument/completion.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}
