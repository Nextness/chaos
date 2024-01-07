#! /bin/bash

set -xe;

rm -f ./main ./chaos_compiler.asm ./chaos_compiler.o ./chaos_compiler

go build ./source/main.go ./source/tokenizer.go ./source/readFile.go ./source/parser.go ./source/codeGeneration.go \
         ./source/cmd.go ./source/utils.go

./main --file_path "./example/exit_with.chaos" -D

nasm -f elf64 -o "chaos_compiler.o" "chaos_compiler.asm"
ld "chaos_compiler.o" -o "chaos_compiler"

./chaos_compiler
