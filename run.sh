#! /bin/bash

set -xe;

go build ./source/main.go ./source/tokenizer.go ./source/readFile.go

./main

# nasm -f elf64 -o "chaos_compiler.o" "chaos_compiler.asm"
# ld "chaos_compiler.o" -o "chaos_compiler"

# ./chaos_compiler
