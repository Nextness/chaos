package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunServerEndToEnd(t *testing.T) {
	var in bytes.Buffer
	for _, body := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"initialized","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///e2e.chaos","languageId":"chaos","version":1,"text":"main :: proc { value: Bool = 1; }"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	} {
		if err := writeMessage(&in, []byte(body)); err != nil {
			t.Fatalf("frame request: %v", err)
		}
	}

	var out bytes.Buffer
	if code := runServer(&in, &out); code != 0 {
		t.Fatalf("runServer exit code = %d, want 0", code)
	}
	wire := out.String()
	for _, want := range []string{
		`"serverInfo":{"name":"chaos-lsp"`,
		`"method":"textDocument/publishDiagnostics"`,
		`cannot assign S64 to Bool`,
		`"id":2,"result":null`,
	} {
		if !strings.Contains(wire, want) {
			t.Errorf("server output missing %q: %s", want, wire)
		}
	}
}

func TestRunServerExitWithoutShutdown(t *testing.T) {
	var in bytes.Buffer
	if err := writeMessage(&in, []byte(`{"jsonrpc":"2.0","method":"exit"}`)); err != nil {
		t.Fatalf("frame exit: %v", err)
	}
	if code := runServer(&in, &bytes.Buffer{}); code != 1 {
		t.Fatalf("runServer exit code = %d, want 1", code)
	}
}

func TestRunServerContinuesAfterMalformedJSONAndInvalidRequest(t *testing.T) {
	var in bytes.Buffer
	for _, body := range []string{
		`{`,
		`{"jsonrpc":"1.0","method":"initialized"}`,
		`{"jsonrpc":"1.0","id":7,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	} {
		if err := writeMessage(&in, []byte(body)); err != nil {
			t.Fatalf("frame request: %v", err)
		}
	}
	var out bytes.Buffer
	if code := runServer(&in, &out); code != 0 {
		t.Fatalf("runServer exit code = %d, want 0", code)
	}
	wire := out.String()
	if !strings.Contains(wire, `"code":-32700`) || !strings.Contains(wire, `"code":-32600`) {
		t.Fatalf("missing JSON-RPC protocol errors: %s", wire)
	}
	if strings.Count(wire, `"code":-32600`) != 1 {
		t.Fatalf("invalid notification unexpectedly received a response: %s", wire)
	}
}

func TestRunServerReturnsFailureWhenResponseCannotBeWritten(t *testing.T) {
	var in bytes.Buffer
	if err := writeMessage(&in, []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)); err != nil {
		t.Fatal(err)
	}
	if code := runServer(&in, failingWriter{}); code != 1 {
		t.Fatalf("runServer exit code = %d, want 1", code)
	}
}
