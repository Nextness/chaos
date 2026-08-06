package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

func main() {
	server := NewServer()
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		body, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
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
			writeMessage(writer, out)
		} else {
			if msg.Method == "exit" {
				if server.isShutdown() {
					os.Exit(0)
				}
				os.Exit(1)
			}
			server.handleNotification(msg)
		}
	}
}
