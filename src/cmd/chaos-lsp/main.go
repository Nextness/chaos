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
				return 0
			}
			// Malformed frame: skip and continue.
			continue
		}
		var msg message
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}
		isRequest := len(msg.ID) > 0 && string(msg.ID) != "null"
		if isRequest {
			resp := server.handleRequest(msg)
			out, err := json.Marshal(resp)
			if err != nil {
				continue
			}
			writeMessage(server.writer, out)
		} else {
			if msg.Method == "exit" {
				if server.isShutdown() {
					return 0
				}
				return 1
			}
			server.handleNotification(msg)
		}
	}
}
