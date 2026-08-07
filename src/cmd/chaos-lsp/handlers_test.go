package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func newTestServer() (*Server, *bytes.Buffer) {
	var buf bytes.Buffer
	s := NewServer()
	s.writer = &buf
	return s, &buf
}

func TestInitializeReturnsCapabilities(t *testing.T) {
	s, _ := newTestServer()
	resp := s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize"})
	if resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}
	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Capabilities.TextDocumentSync == nil || !result.Capabilities.TextDocumentSync.OpenClose || result.Capabilities.TextDocumentSync.Change != 2 {
		t.Errorf("textDocumentSync = %+v, want openClose + change 2", result.Capabilities.TextDocumentSync)
	}
	if !result.Capabilities.DocumentSymbolProvider {
		t.Error("documentSymbolProvider not advertised")
	}
	if result.Capabilities.SemanticTokensProvider == nil {
		t.Error("semanticTokensProvider not advertised")
	}
	if result.ServerInfo.Name != "chaos-lsp" {
		t.Errorf("serverInfo name = %q, want chaos-lsp", result.ServerInfo.Name)
	}
}

func TestDidChangePublishesDiagnostics(t *testing.T) {
	s, buf := newTestServer()
	// Open a valid document.
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.chaos","languageId":"chaos","version":1,"text":"x :: 42;"}}`),
	})
	buf.Reset()
	// Introduce a bad source via incremental edit: replace "42" with "$".
	s.handleNotification(message{
		Method: "textDocument/didChange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.chaos","version":2},"contentChanges":[{"range":{"start":{"line":0,"character":5},"end":{"line":0,"character":7}},"text":"$"}]}`),
	})
	out := buf.String()
	if !strings.Contains(out, "textDocument/publishDiagnostics") {
		t.Fatalf("no publishDiagnostics in output: %q", out)
	}
	if !strings.Contains(out, "unexpected character") {
		t.Errorf("output missing error diagnostic: %q", out)
	}
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	s, buf := newTestServer()
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.chaos","languageId":"chaos","version":1,"text":"x :: 42;"}}`),
	})
	buf.Reset()
	s.handleNotification(message{
		Method: "textDocument/didClose",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.chaos","version":1}}`),
	})
	out := buf.String()
	if !strings.Contains(out, `"diagnostics":[]`) {
		t.Errorf("didClose should publish empty diagnostics: %q", out)
	}
}

func TestRequestAfterShutdownRejected(t *testing.T) {
	s, _ := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "initialize"})
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "shutdown"})
	resp := s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "textDocument/documentSymbol"})
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Errorf("expected -32600 after shutdown, got %+v", resp.Error)
	}
}

func TestSemanticTokensEmptyDocument(t *testing.T) {
	s, _ := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///empty.chaos","languageId":"chaos","version":1,"text":""}}`),
	})
	resp := s.handleRequest(message{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "textDocument/semanticTokens/full",
		Params:  json.RawMessage(`{"textDocument":{"uri":"file:///empty.chaos"}}`),
	})
	if resp.Error != nil {
		t.Fatalf("semanticTokens error: %+v", resp.Error)
	}
	if !strings.Contains(string(resp.Result), `"data":[]`) {
		t.Errorf("semanticTokens result = %s, want data:[]", resp.Result)
	}
}
