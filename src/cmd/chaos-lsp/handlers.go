package main

import (
	"encoding/json"
)

// semanticTokenTypes is the legend declared in initialize and used by
// textDocument/semanticTokens/full.
var semanticTokenTypes = []string{"keyword", "number", "string", "comment", "function", "variable", "parameter", "type", "constant", "delimiter"}

// handleRequest dispatches a JSON-RPC request and returns its response.
func (s *Server) handleRequest(msg message) Response {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "shutdown":
		s.mu.Lock()
		if s.state == serverPreInitialize {
			s.mu.Unlock()
			return errorResponse(msg.ID, -32002, "server not initialized")
		}
		if s.state == serverShutdown {
			s.mu.Unlock()
			return errorResponse(msg.ID, -32600, "server is already shutting down")
		}
		s.state = serverShutdown
		s.mu.Unlock()
		return Response{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage("null")}
	default:
		s.mu.Lock()
		state := s.state
		s.mu.Unlock()
		if state == serverPreInitialize {
			return errorResponse(msg.ID, -32002, "server not initialized")
		}
		if state == serverShutdown {
			return errorResponse(msg.ID, -32600, "server is shutting down")
		}
		switch msg.Method {
		case "textDocument/documentSymbol":
			return s.handleDocumentSymbol(msg)
		case "textDocument/semanticTokens/full":
			return s.handleSemanticTokens(msg)
		case "textDocument/definition":
			return s.handleDefinition(msg)
		case "textDocument/references":
			return s.handleReferences(msg)
		case "textDocument/documentHighlight":
			return s.handleDocumentHighlight(msg)
		case "textDocument/hover":
			return s.handleHover(msg)
		case "textDocument/completion":
			return s.handleCompletion(msg)
		default:
			return errorResponse(msg.ID, -32601, "method not found")
		}
	}
}

// handleNotification dispatches a JSON-RPC notification.
func (s *Server) handleNotification(msg message) {
	s.mu.Lock()
	state := s.state
	s.mu.Unlock()
	if state != serverRunning {
		return
	}
	switch msg.Method {
	case "initialized":
		// The client is ready; nothing to do.
	case "$/cancelRequest":
		// Tolerated and ignored.
	case "textDocument/didOpen":
		s.handleDidOpen(msg)
	case "textDocument/didChange":
		s.handleDidChange(msg)
	case "textDocument/didClose":
		s.handleDidClose(msg)
	}
}

// handleInitialize responds with the server capabilities.
func (s *Server) handleInitialize(msg message) Response {
	s.mu.Lock()
	if s.state != serverPreInitialize {
		s.mu.Unlock()
		return errorResponse(msg.ID, -32600, "initialize may only be requested once")
	}
	s.state = serverRunning
	s.mu.Unlock()
	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:       &TextDocumentSyncOptions{OpenClose: true, Change: 2},
			DocumentSymbolProvider: true,
			SemanticTokensProvider: &SemanticTokensOptions{
				Legend: SemanticTokensLegend{TokenTypes: semanticTokenTypes, TokenModifiers: []string{"bold"}},
				Full:   true,
			},
			DefinitionProvider:        true,
			ReferencesProvider:        true,
			DocumentHighlightProvider: true,
			HoverProvider:             true,
			CompletionProvider:        &CompletionOptions{TriggerCharacters: []string{"."}},
		},
		ServerInfo: ServerInfo{Name: "chaos-lsp", Version: "0.1.0"},
	}
	out, err := json.Marshal(result)
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}

// handleDidOpen stores the document and pushes diagnostics.
func (s *Server) handleDidOpen(msg message) {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}
	doc := newDocument(params.TextDocument.URI, params.TextDocument.Version, params.TextDocument.Text)
	s.mu.Lock()
	s.documents[doc.URI] = doc
	s.mu.Unlock()
	s.publishDiagnostics(doc)
}

// handleDidChange applies incremental edits, re-registers the document, and
// pushes diagnostics.
func (s *Server) handleDidChange(msg message) {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}
	s.mu.Lock()
	old := s.documents[params.TextDocument.URI]
	if old == nil || params.TextDocument.Version <= old.Version {
		s.mu.Unlock()
		if old != nil {
			s.logf("chaos-lsp: ignored stale document version %d for %s (current %d)\n", params.TextDocument.Version, params.TextDocument.URI, old.Version)
		}
		return
	}
	text, ok := applyEditsChecked(old.Text, params.ContentChanges)
	if !ok {
		s.mu.Unlock()
		s.logf("chaos-lsp: rejected invalid incremental edit for %s; waiting for a full valid update\n", params.TextDocument.URI)
		return
	}
	doc := newDocument(old.URI, params.TextDocument.Version, text)
	s.documents[doc.URI] = doc
	s.mu.Unlock()
	s.publishDiagnostics(doc)
}

// handleDidClose removes the document and clears its diagnostics.
func (s *Server) handleDidClose(msg message) {
	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return
	}
	s.mu.Lock()
	delete(s.documents, params.TextDocument.URI)
	s.mu.Unlock()
	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []Diagnostic{},
	})
}

// publishDiagnostics tokenizes and parses the document in tolerant mode and
// pushes the resulting diagnostics.
func (s *Server) publishDiagnostics(doc *Document) {
	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         doc.URI,
		Diagnostics: convertDiagnostics(doc.diags, doc.sf),
	})
}

// sendNotification marshals and writes a notification to the client.
func (s *Server) sendNotification(method string, params any) {
	out, err := json.Marshal(params)
	if err != nil {
		return
	}
	body, err := json.Marshal(Notification{JSONRPC: "2.0", Method: method, Params: out})
	if err != nil {
		return
	}
	s.setWriteError(writeMessage(s.writer, body))
}
