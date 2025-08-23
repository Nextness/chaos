package main

// import (
// 	"bytes"
// 	"os"
// )
//
// type Syscalls int
//
// const (
// 	sysExit = Syscalls(60)
// )
//
// func generateHeader(stream *bytes.Buffer) {
// 	writeStringf(stream, "format ELF64 executable\n")
// 	writeStringf(stream, "entry _chaos_start\n\n")
// 	writeStringf(stream, "_chaos_start:\n")
// }
//
// func generateExit(stream *bytes.Buffer, instructionExit *InstructionExit) {
// 	writeStringf(stream, "    mov rax, %d\n", sysExit)
// 	writeStringf(stream, "    mov rdi, %d\n", instructionExit.value)
// 	writeStringf(stream, "    syscall\n")
// }
//
// func generateMain(stream *bytes.Buffer, instructions []Instruction) {
// 	for idx, instruction := range instructions {
// 		writeStringf(stream, "; Instruction %d\n", idx)
// 		if i, ok := cast[*InstructionExit](instruction); ok {
// 			generateExit(stream, i)
// 			continue
// 		}
// 	}
// }
//
// func generateFooter(stream *bytes.Buffer) {
// 	writeStringf(stream, "; Instruction Last\n")
// 	writeStringf(stream, "    mov rax, %d\n", sysExit)
// 	writeStringf(stream, "    mov rdi, %d\n", 0)
// 	writeStringf(stream, "    syscall\n")
// }
//
// func generateProgram(instructions []Instruction) {
// 	output := &bytes.Buffer{}
// 	generateHeader(output)
// 	generateMain(output, instructions)
// 	generateFooter(output)
// 	writeStringf(output, "\n")
// 	if err := os.WriteFile("testing.asm", output.Bytes(), 0644); err != nil {
// 		panic("you suck")
// 	}
// }
