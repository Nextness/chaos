package main

// import (
// 	"bytes"
// 	"fmt"
// )
//
// type Instruction interface {
// }
//
// type InstructionExit struct {
// 	value   int
// 	message string
// }
//
// var _ Instruction = &InstructionExit{}
//
// func writeStringf(stream *bytes.Buffer, format string, args ...any) {
// 	stream.WriteString(fmt.Sprintf(format, args...))
// }
//
// func irChaos(es *ExecutionState) []Instruction {
// 	instructions := []Instruction{}
//
// 	// for _, node := range es.entryPoint.statements {
// 	// 	if node.NodeType() == nodeExit {
// 	// 		instructionExit := &InstructionExit{}
// 	// 		n := castAssert[*NodeExit](node)
// 	// 		if token := castAssert[*TokenLiteral](n.message); token != nil {
// 	// 			instructionExit.message = castAssert[string](token.value)
// 	// 		}
// 	// 		token := castAssert[*TokenLiteral](n.status)
// 	// 		instructionExit.value = castAssert[int](token.value)
// 	// 		instructions = append(instructions, instructionExit)
// 	// 		continue
// 	// 	}
// 	// }
//
// 	return instructions
// }
