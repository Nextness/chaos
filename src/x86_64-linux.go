package main

import (
	"bytes"
	"os"
)

func generateHeader(stream *bytes.Buffer) {
	writeStringf(stream, "format ELF64 executable\n")
	writeStringf(stream, "entry _chaos_start\n\n")
	writeStringf(stream, "_chaos_start:\n")
}

func generateExit(stream *bytes.Buffer, instructionExit *InstructionExit) {
	SYS_EXIT := 60
	writeStringf(stream, "    mov rax, %d\n", SYS_EXIT)
	writeStringf(stream, "    mov rdi, %d\n", instructionExit.value)
	writeStringf(stream, "    syscall\n")
}

func generateMain(stream *bytes.Buffer, instructions []Instruction) {
	for _, instruction := range instructions {
		if i, ok := cast[*InstructionExit](instruction); ok {
			generateExit(stream, i)
		}
	}
}

func generateProgram(instructions []Instruction) {
	output := &bytes.Buffer{}
	generateHeader(output)
	generateMain(output, instructions)
	writeStringf(output, "\n")
	if err := os.WriteFile("testing.asm", output.Bytes(), 0644); err != nil {
		panic("you suck")
	}
}
