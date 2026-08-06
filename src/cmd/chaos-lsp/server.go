package main

import (
	"io"
	"os"
	"sync"

	"chaos_new/compiler"
)

// Document is the server's in-memory view of one open text document.
type Document struct {
	URI  string
	Text string
	sf   *compiler.SourceFile
}

// refresh (re)registers the document's source with a fresh SourceManager so
// spans and line offsets stay accurate after edits.
func (d *Document) refresh() {
	sm := &compiler.SourceManager{}
	fileID := sm.Register(d.URI, []byte(d.Text))
	d.sf = sm.Lookup(fileID)
}

// Server holds the LSP server state.
type Server struct {
	mu          sync.Mutex
	documents   map[string]*Document
	initialized bool
	shutdown    bool
	writer      io.Writer
}

// NewServer creates a server writing notifications to stdout.
func NewServer() *Server {
	return &Server{
		documents: make(map[string]*Document),
		writer:    os.Stdout,
	}
}

// isShutdown reports whether a shutdown request has been received.
func (s *Server) isShutdown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdown
}
