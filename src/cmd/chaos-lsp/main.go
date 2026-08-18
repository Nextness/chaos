package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

func main() {
	os.Exit(runServer(os.Stdin, os.Stdout))
}

// runServer processes framed JSON-RPC messages until EOF or an exit
// notification. It returns the LSP-required exit status: zero after shutdown,
// one when the client exits without first shutting down.
func runServer(in io.Reader, out io.Writer) int {
	server := NewServer()
	server.writer = out
	reader := bufio.NewReader(in)

	for {
		body, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				if server.isShutdown() {
					return 0
				}
				return 1
			}
			server.logf("chaos-lsp: invalid JSON-RPC frame: %v\n", err)
			return 1
		}
		var msg message
		if err := json.Unmarshal(body, &msg); err != nil {
			if !writeResponse(server.writer, errorResponse(json.RawMessage("null"), -32700, "parse error")) {
				return 1
			}
			continue
		}
		if err := validateMessage(msg); err != nil {
			if len(msg.ID) == 0 {
				server.logf("chaos-lsp: ignored invalid JSON-RPC notification\n")
				continue
			}
			id := msg.ID
			if !validRequestID(id) {
				id = json.RawMessage("null")
			}
			if !writeResponse(server.writer, errorResponse(id, -32600, "invalid request")) {
				return 1
			}
			continue
		}
		isRequest := len(msg.ID) > 0
		if isRequest {
			resp := server.handleRequest(msg)
			if !writeResponse(server.writer, resp) {
				return 1
			}
		} else {
			if msg.Method == "exit" {
				if server.isShutdown() {
					return 0
				}
				return 1
			}
			server.handleNotification(msg)
			if server.writerError() != nil {
				return 1
			}
		}
	}
}

func validateMessage(msg message) error {
	if msg.JSONRPC != "2.0" {
		return io.ErrUnexpectedEOF
	}
	if msg.Method == "" {
		return io.ErrUnexpectedEOF
	}
	if len(msg.ID) > 0 && !validRequestID(msg.ID) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func validRequestID(id json.RawMessage) bool {
	var value any
	if err := json.Unmarshal(id, &value); err != nil {
		return false
	}
	switch value.(type) {
	case nil, string, float64:
		return true
	}
	return false
}

func writeResponse(writer io.Writer, response Response) bool {
	body, err := json.Marshal(response)
	if err != nil {
		return false
	}
	return writeMessage(writer, body) == nil
}
