package main

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chaos_compiler/compiler"
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
	if result.ServerInfo.Version != compiler.Version {
		t.Errorf("serverInfo version = %q, want %q", result.ServerInfo.Version, compiler.Version)
	}
}

func TestDidChangePublishesDiagnostics(t *testing.T) {
	s, buf := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
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

func TestOpenImportOverlayReanalyzesDependents(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "values.chaos")
	if err := os.WriteFile(modulePath, []byte("answer :: proc -> S64 { return 42; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	moduleURI := (&url.URL{Scheme: "file", Path: modulePath}).String()
	mainURI := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "main.chaos")}).String()
	mainSource := "#import «values»;\n#entry main :: proc -> S64 { return answer(); }"
	inputs := map[string]documentInput{
		mainURI:   {version: 1, text: mainSource},
		moduleURI: {version: 2, text: "answer :: proc -> String { return «changed»; }"},
	}
	documents := rebuildDocuments(inputs)
	if !documents[mainURI].diags.HasErrors() {
		t.Fatal("dependent document ignored the unsaved imported signature")
	}
	inputs[moduleURI] = documentInput{version: 3, text: "answer :: proc -> S64 { return 42; }"}
	documents = rebuildDocuments(inputs)
	if documents[mainURI].diags.HasErrors() {
		t.Fatalf("dependent document did not recover after import update: %v", documents[mainURI].diags)
	}
}

func TestDidChangeRejectsStaleAndInvalidVersionsWithoutMutatingSnapshot(t *testing.T) {
	s, buf := newTestServer()
	var logs bytes.Buffer
	s.logger = &logs
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///versions.chaos","languageId":"chaos","version":2,"text":"x :: «😀»;"}}`),
	})
	buf.Reset()

	// Equal and lower versions are stale, even if their edits would otherwise
	// be valid.
	s.handleNotification(message{
		Method: "textDocument/didChange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///versions.chaos","version":2},"contentChanges":[{"text":"corrupt"}]}`),
	})
	if got := s.documents["file:///versions.chaos"]; got.Version != 2 || got.Text != "x :: «😀»;" {
		t.Fatalf("stale update mutated document: version=%d text=%q", got.Version, got.Text)
	}
	if buf.Len() != 0 {
		t.Fatalf("stale update published diagnostics: %q", buf.String())
	}

	// UTF-16 character 7 lies in the middle of the emoji surrogate pair.
	s.handleNotification(message{
		Method: "textDocument/didChange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///versions.chaos","version":3},"contentChanges":[{"range":{"start":{"line":0,"character":7},"end":{"line":0,"character":7}},"text":"!"}]}`),
	})
	if got := s.documents["file:///versions.chaos"]; got.Version != 2 || got.Text != "x :: «😀»;" {
		t.Fatalf("invalid edit mutated document: version=%d text=%q", got.Version, got.Text)
	}
	if buf.Len() != 0 {
		t.Fatalf("invalid edit published diagnostics: %q", buf.String())
	}

	// A later full-document update recovers from an invalid incremental edit.
	s.handleNotification(message{
		Method: "textDocument/didChange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///versions.chaos","version":4},"contentChanges":[{"text":"answer :: 42;"}]}`),
	})
	if got := s.documents["file:///versions.chaos"]; got.Version != 4 || got.Text != "answer :: 42;" {
		t.Fatalf("full update did not recover document: version=%d text=%q", got.Version, got.Text)
	}
	if !strings.Contains(buf.String(), "textDocument/publishDiagnostics") {
		t.Fatalf("valid recovery did not publish diagnostics: %q", buf.String())
	}
	if log := logs.String(); !strings.Contains(log, "ignored stale") || !strings.Contains(log, "rejected invalid incremental edit") {
		t.Fatalf("missing version/edit logs: %q", log)
	}
}

func TestDidOpenPublishesTypeDiagnostics(t *testing.T) {
	s, buf := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///type.chaos","languageId":"chaos","version":1,"text":"main :: proc { value: Bool = 1; }"}}`),
	})
	if out := buf.String(); !strings.Contains(out, "cannot assign S64 to Bool") {
		t.Errorf("publishDiagnostics missing type error: %q", out)
	}
}

func TestDidOpenPublishesShadowingDiagnostic(t *testing.T) {
	s, buf := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///shadow.chaos","languageId":"chaos","version":1,"text":"main :: proc { x := 1; x := 2; }"}}`),
	})
	if out := buf.String(); !strings.Contains(out, "use '#shadow' or rename") {
		t.Errorf("publishDiagnostics missing shadowing error: %q", out)
	}
}

func TestReferencesHonorsIncludeDeclaration(t *testing.T) {
	s, _ := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///refs.chaos","languageId":"chaos","version":1,"text":"main :: proc { x := 1; y := x; }"}}`),
	})

	request := func(include bool) []Location {
		params, err := json.Marshal(ReferenceParams{
			TextDocument: TextDocumentIdentifier{URI: "file:///refs.chaos"},
			Position:     Position{Line: 0, Character: 28},
			Context:      ReferenceContext{IncludeDeclaration: include},
		})
		if err != nil {
			t.Fatal(err)
		}
		resp := s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "textDocument/references", Params: params})
		if resp.Error != nil {
			t.Fatalf("references error: %+v", resp.Error)
		}
		var locs []Location
		if err := json.Unmarshal(resp.Result, &locs); err != nil {
			t.Fatalf("unmarshal references: %v", err)
		}
		return locs
	}
	if got := request(false); len(got) != 1 {
		t.Errorf("references without declaration = %d, want 1", len(got))
	}
	if got := request(true); len(got) != 2 {
		t.Errorf("references with declaration = %d, want 2", len(got))
	}
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	s, buf := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
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

func TestLifecycleRejectsDuplicateInitializeAndShutdownBeforeInitialize(t *testing.T) {
	s, _ := newTestServer()
	resp := s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "shutdown"})
	if resp.Error == nil || resp.Error.Code != -32002 {
		t.Fatalf("shutdown before initialize = %+v, want -32002", resp.Error)
	}
	resp = s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "initialize"})
	if resp.Error != nil {
		t.Fatalf("first initialize failed: %+v", resp.Error)
	}
	resp = s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`3`), Method: "initialize"})
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("duplicate initialize = %+v, want -32600", resp.Error)
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

func TestDidOpenNoDiagnosticsForNewSyntax(t *testing.T) {
	// Init-later declarations ('a: S64 = ...;') and trailing commas are
	// valid; the LSP must not report diagnostics for them.
	s, buf := newTestServer()
	s.handleRequest(message{JSONRPC: "2.0", ID: json.RawMessage(`0`), Method: "initialize"})
	s.handleNotification(message{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///new.chaos","languageId":"chaos","version":1,"text":"#entry main :: proc -> S64 {\n    a: S64 = ...;\n    a = 42;\n    return a;\n}\nPoint :: struct { x: S64; y: S64; }\nmake :: proc (p: Point) -> S64 { return p.x; }\nmain2 :: proc { p := Point.{ x = 1, y = 2, }; make(p,); }"}}`),
	})
	out := buf.String()
	if !strings.Contains(out, "textDocument/publishDiagnostics") {
		t.Fatalf("no publishDiagnostics in output: %q", out)
	}
	if strings.Contains(out, `"severity":1`) {
		t.Errorf("unexpected error diagnostics for new syntax: %q", out)
	}
}
