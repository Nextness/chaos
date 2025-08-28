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
	strBuf := bytes.Buffer{}

	GenerateHeader(&buffer)

	strCount := 0
	for _, node := range prog.Nodes {
		if node.NodeType == nodeIdentifier {
			continue
		}
		if node.NodeType == nodeExit {
			if node.ExMsg != "" {
				strLen := len(node.ExMsg) + 1
				strName := fmt.Sprintf("str_%d", strCount)
				strCount++

				strBuf.WriteString(fmt.Sprintf("%s db \"%s\", 10\n", strName, node.ExMsg))
				strBuf.WriteString(fmt.Sprintf("%s_size = $-%s\n", strName, strName))

				buffer.WriteString("    mov rax, 1\n")
				buffer.WriteString("    mov rdi, 1\n")
				buffer.WriteString(fmt.Sprintf("    mov rsi, %s\n", strName))
				buffer.WriteString(fmt.Sprintf("    mov rdx, %d\n", strLen))
				buffer.WriteString("    syscall\n")
			}
			buffer.WriteString("    mov rax, 60\n")
			buffer.WriteString(fmt.Sprintf("    mov rdi, %d\n", node.ExVal))
			buffer.WriteString("    syscall\n")
			buffer.WriteString("    ret\n")
		}
	}

	buffer.WriteString("\n")
	buffer.WriteString("segment readable\n\n")
	buffer.Write(strBuf.Bytes())

	return &buffer
}
