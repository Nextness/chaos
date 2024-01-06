package main

import (
	"fmt"
	"strings"
)

func GenerateAssembly(nodeExit NodeExit) string {
	result := []string{
		"global _start\n",
		"\n",
		"_start:\n",
		"    mov rax, 60\n",
		fmt.Sprintf("    mov rdi, %v\n", nodeExit.nodeExpression.token),
		"    syscall\n",
	}
	return strings.Join(result, "")
}
