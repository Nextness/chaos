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

	for _, node := range prog.AllocatedVars {
		if node.NodeType == nodeIdentifier {
			if result, ok := cast[int](node.VarDecl.VarValue.LiteralInt.Value); ok {
				varName := node.VarDecl.VarName.Symbol
				datBuf.WriteString(fmt.Sprintf("    %s dq %d\n", varName, result))
				continue
			}
			if node.VarDecl.VarValue != nil {
				varName := node.VarDecl.VarName.Symbol
				datBuf.WriteString(fmt.Sprintf("    %s dq %d\n", varName, 0))
				continue
			}
		}
	}

	strCount := 0
	for opid, node := range prog.Nodes {
		if node.NodeType == nodeIdentifier {
			if _, ok := cast[int](node.VarDecl.VarValue.LiteralInt.Value); ok {
				continue
			}
			if node.VarDecl.VarValue != nil {
				varName := node.VarDecl.VarName.Symbol
				value := castAssert[string](node.VarDecl.VarValue.VarDecl.VarName.Symbol)
				buffer.WriteString(fmt.Sprintf("    ; %06d. assign\n", opid))
				buffer.WriteString(fmt.Sprintf("    mov rax, [%s]\n", value))
				buffer.WriteString(fmt.Sprintf("    mov [%s], rax\n", varName))
			}
		}

		if node.NodeType == nodeExit {
			buffer.WriteString(fmt.Sprintf("    ; %06d. exit\n", opid))
			if node.Exit.ExMsg.LiteralString.Value != "" {
				msg := castAssert[string](node.Exit.ExMsg.LiteralString.Value)

				strName := fmt.Sprintf("str_%d", strCount)
				strSize := fmt.Sprintf("%s_size", strName)
				strCount++

				strBuf.WriteString(fmt.Sprintf("%s db \"%s\", 10\n", strName, msg))
				strBuf.WriteString(fmt.Sprintf("%s = $-%s\n", strSize, strName))

				buffer.WriteString("    mov rax, 1\n")
				buffer.WriteString("    mov rdi, 1\n")
				buffer.WriteString(fmt.Sprintf("    mov rsi, %s\n", strName))
				buffer.WriteString(fmt.Sprintf("    mov rdx, %s\n", strSize))
				buffer.WriteString("    syscall\n")
			}

			if node.Exit.ExVal.VarDecl.VarName.Symbol != "" {
				buffer.WriteString("    mov rax, 60\n")
				buffer.WriteString(fmt.Sprintf("    mov rdi, [%s]\n", castAssert[string](node.Exit.ExVal.VarDecl.VarName.Symbol)))
				buffer.WriteString("    syscall\n")
				buffer.WriteString("    ret\n")
			} else if status, ok := cast[int](node.Exit.ExVal.LiteralInt.Value); ok {
				buffer.WriteString("    mov rax, 60\n")
				buffer.WriteString(fmt.Sprintf("    mov rdi, %d\n", status))
				buffer.WriteString("    syscall\n")
				buffer.WriteString("    ret\n")
			}
			continue
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
