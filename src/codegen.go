package main

import (
	"bytes"
	"fmt"
)

const entryPoint string = "main"

func GenerateCode(prog *Program) *bytes.Buffer {
	buffer := bytes.Buffer{}
	datBuf := bytes.Buffer{}
	strBuf := bytes.Buffer{}

	buffer.WriteString("format ELF64 executable 3\n\n")
	buffer.WriteString(fmt.Sprintf("entry %s\n\n", entryPoint))
	buffer.WriteString("segment readable executable\n\n")
	buffer.WriteString(fmt.Sprintf("%s:\n", entryPoint))

	strCount := 0
	for opid, node := range prog.Nodes {

		if node.NodeType == nodeIdentifier {
			if result, ok := cast[int](node.VarValue.LiteralInt.Value); ok {
				varName := node.VarName.Symbol
				datBuf.WriteString(fmt.Sprintf("    ; %d op->assign\n", opid))
				datBuf.WriteString(fmt.Sprintf("    %s dq %d\n", varName, result))
				continue
			}
			if node.VarValue != nil {
				varName := node.VarName.Symbol
				datBuf.WriteString(fmt.Sprintf("    ; %d op->reassign\n", opid))
				datBuf.WriteString(fmt.Sprintf("    %s dq %s\n", varName, node.VarValue.VarName.Symbol))
			}
		}

		if node.NodeType == nodeExit {
			buffer.WriteString(fmt.Sprintf("    ; %d op->exit\n", opid))
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

	buffer.WriteString("\n")
	buffer.WriteString("segment readable writable\n")
	buffer.Write(datBuf.Bytes())

	return &buffer
}
