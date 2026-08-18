package main

import (
	"fmt"
	"io"
	"os"
	"sync"

	"chaos_compiler/compiler"
)

// Document is the server's in-memory view of one open text document.
type Document struct {
	URI      string
	Version  int
	Text     string
	sf       *compiler.SourceFile
	tokens   compiler.TokenList
	program  *compiler.Program
	analysis *compiler.SemanticAnalysis
	diags    compiler.DiagnosticList
	resolver *resolver
	symbols  []DocumentSymbol
	semantic []int
}

// newDocument builds one immutable analyzed snapshot for a document version.
func newDocument(uri string, version int, text string) *Document {
	d := &Document{URI: uri, Version: version, Text: text}
	sm := &compiler.SourceManager{}
	fileID := sm.Register(d.URI, []byte(d.Text))
	d.sf = sm.Lookup(fileID)
	d.tokens, d.diags = compiler.Tokenize(d.sf.Source, d.sf.ID)
	result := compiler.ParseProgramTolerant(d.tokens)
	d.program = result.Program
	if d.program != nil {
		d.program.Sources = map[compiler.FileID]compiler.SourceFile{d.sf.ID: *d.sf}
	}
	d.diags = append(d.diags, result.Diags...)
	if !d.diags.HasErrors() {
		var semanticDiags compiler.DiagnosticList
		d.analysis, semanticDiags = compiler.AnalyzeProgram(d.program)
		d.diags = append(d.diags, semanticDiags...)
	} else {
		d.analysis = &compiler.SemanticAnalysis{
			ExprTypes:     make(map[compiler.Expr]compiler.Type),
			TypeExprTypes: make(map[compiler.Expr]compiler.Type),
			DeclTypes:     make(map[compiler.Decl]compiler.Type),
			NominalDecls:  make(map[compiler.Type]compiler.Decl),
			ProcDecls:     make(map[compiler.Type]*compiler.ProcDecl),
		}
	}
	d.resolver = newResolver(d)
	d.symbols = documentSymbols(d.program, d.sf)
	allSemantic := append(tokenSemanticTokens(d.tokens, d.sf), astSemanticTokens(d.program, d.sf)...)
	d.semantic = encodeSemanticTokens(allSemantic)
	return d
}

type serverState uint8

const (
	serverPreInitialize serverState = iota
	serverRunning
	serverShutdown
)

// Server holds the LSP server state.
type Server struct {
	mu        sync.Mutex
	documents map[string]*Document
	state     serverState
	writer    io.Writer
	logger    io.Writer
	writeErr  error
}

// NewServer creates a server writing notifications to stdout.
func NewServer() *Server {
	return &Server{
		documents: make(map[string]*Document),
		writer:    os.Stdout,
		logger:    os.Stderr,
	}
}

// isShutdown reports whether a shutdown request has been received.
func (s *Server) isShutdown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == serverShutdown
}

func (s *Server) setWriteError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	if s.writeErr == nil {
		s.writeErr = err
	}
	s.mu.Unlock()
}

func (s *Server) writerError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeErr
}

func (s *Server) logf(format string, args ...any) {
	if s.logger != nil {
		fmt.Fprintf(s.logger, format, args...)
	}
}
