package main

import (
	"bytes"
	"fmt"
)

type VariableDetails struct {
	location int
}

type StackData struct {
	size      int
	variables map[string]VariableDetails
}

func (sd *StackData) push(register string) string {
	stringBuffer := bytes.NewBuffer([]byte{})
	stringBuffer.WriteString(fmt.Sprintf("    push %v\n", register))
	(*sd).size++
	return stringBuffer.String()
}

func (sd *StackData) pop(register string) string {
	stringBuffer := bytes.NewBuffer([]byte{})
	stringBuffer.WriteString(fmt.Sprintf("    pop %v\n", register))
	(*sd).size--
	return stringBuffer.String()
}

func GenerateExpression(nodeExpression *NodeExpression, stackData *StackData) string {
	stringBuffer := bytes.NewBuffer([]byte{})
	if nodeExpression.nodeType == intLiteral {
		stringBuffer.WriteString("    ;; Int literal in the stack\n")
		stringBuffer.WriteString(
			fmt.Sprintf("    mov rax, %v\n",
				nodeExpression.nodeExpresionIntegerLiteral.integerLiteral.Value))
		stringBuffer.WriteString(stackData.push("rax"))
		return stringBuffer.String()
	} else if nodeExpression.nodeType == identifier {
		key := nodeExpression.nodeExpresionIdentifier.identifier.Value
		if _, ok := stackData.variables[key]; !ok {
			fmt.Printf("ERROR: Undeclared identifier '%v'", nodeExpression.nodeExpresionIdentifier.identifier.Value)
			return ""
		}
		value := stackData.variables[nodeExpression.nodeExpresionIdentifier.identifier.Value]
		result := stackData.push(fmt.Sprintf("QWORD [rep + %v]\n", (stackData.size-value.location)*4))
		stringBuffer.WriteString(result)
		return stringBuffer.String()
	}
	return ""
}

func GenerateStatement(nodeStatement *NodeStatement, stackData *StackData) string {
	stringBuffer := bytes.NewBuffer([]byte{})
	if nodeStatement.nodeType == exit {
		result := GenerateExpression(&nodeStatement.nodeStatementExit.nodeExpression, stackData)
		stringBuffer.WriteString("    ;; Poping int literal from the stack\n")
		stringBuffer.WriteString("    ;; and using in the exit")
		stringBuffer.WriteString(result)
		stringBuffer.WriteString("    mov rax, 60\n")
		stringBuffer.WriteString(stackData.pop("rdi"))
		stringBuffer.WriteString("    syscall\n")
		return stringBuffer.String()
	} else if nodeStatement.nodeType == let {
		key := nodeStatement.nodeStatementLet.nodeExpression.nodeExpresionIdentifier.identifier.Value
		if _, ok := stackData.variables[key]; !ok {
			fmt.Printf("Variable already defined...\n")
		}
		stackData.variables[key] = VariableDetails{
			location: stackData.size,
		}
		result := GenerateExpression(&nodeStatement.nodeStatementLet.nodeExpression, stackData)
		stringBuffer.WriteString(result)
		return stringBuffer.String()
	}
	return ""
}

func GenerateProgram(nodeProgram NodeProgram, stackData StackData) string {
	stringBuffer := bytes.NewBuffer([]byte{})
	for _, nodeStatement := range nodeProgram.nodeStatement {
		statement := GenerateStatement(&nodeStatement, &stackData)
		stringBuffer.WriteString(statement)
	}
	stringBuffer.WriteString("    ;; Exit Succesfully\n")
	stringBuffer.WriteString("    mov rax, 60\n")
	stringBuffer.WriteString("    mov rdi, 0\n")
	stringBuffer.WriteString("    syscall\n")
	return stringBuffer.String()
}
