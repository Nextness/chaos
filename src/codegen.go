package main

import (
	"bytes"
	"fmt"
)

func GenerateHeader(buffer *bytes.Buffer) {
	buffer.WriteString("format ELF64 executable 3\n\n")
	buffer.WriteString("segment readable executable\n\n")
	buffer.WriteString("start:\n")
}

func GenerateCode(prog *Program) *bytes.Buffer {
	buffer := bytes.Buffer{}

	GenerateHeader(&buffer)
	for _, node := range prog.Nodes {
		if node.NodeType == nodeIdentifier {
			continue
		}
		if node.NodeType == nodeExit {
			buffer.WriteString("    mov rax, 60\n")
			buffer.WriteString(fmt.Sprintf("    mov rdi, %d\n", node.ExVal))
			buffer.WriteString("    syscall\n")
			buffer.WriteString("    ret")
		}
	}

	return &buffer
}
