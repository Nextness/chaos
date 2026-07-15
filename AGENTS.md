# Chaos Language — Agent Guide

## Build & Test

```bash
go build chaosBuild.go       # bootstrap build binary
./chaosBuild                 # compile compiler (output: ./build/chaosc)
./chaosBuild -c file.chaos   # compile a .chaos source
./chaosBuild -c file.chaos -r # compile + assemble via fasm → ELF binary
./chaosBuild -test           # run all .chaos test files in ./tests/
./chaosBuild -default        # compile main.chaos (only when DEBUG=true, which is hardcoded)
```

- **Linux-only** (`runtime.GOOS == "linux"` hardcoded in `chaosBuild.go`).
- **No `go.sum`, no external deps.** Only stdlib.
- **`-r` flag requires `fasm`** (Flat Assembler) on `$PATH`. It looks for `<file>.asm` output from the compiler, assembles it, and runs the resulting binary.
- **Test runner** iterates all files in `./tests/`, runs `./build/chaosc tests/<file>` on each, and prints errors per file. It does NOT distinguish expected-pass vs expected-fail — every file is run the same way. Failure files (e.g. `assignments_failure.chaos`) are expected to cause compiler errors.

## Architecture

Pipeline: `src/chaosLexer.go → src/chaosAst.go → src/chaosInferenceCheckType.go → src/chaosCodegen.go`

- **Codegen is stubbed.** `src/chaosCodegen.go` writes an ELF64 header + debug output only; the real codegen (lines 30–278) is commented out.
- **IR is defined but unused.** `src/chaosIr.go` has `ProgramToIR()` but the pipeline never calls it.
- **No for/while loops** in the language yet — only `if`/`elif`/`else`.
- String literals use Unicode guillemets: `«like this»`.
- `::` = compile-time assignment, `:=` = runtime assignment.

## Source layout

- `src/` is the real compiler (Go, package `main`, ~3000 lines across 8 files).
- `chaosBuild.go` is a separate bootstrap binary (not part of `src/`).
- `language_design/` is speculative — those features (structs, generics, async, C interop, modules, imports, self-hosted compiler) are **not implemented**. The `*.chaos` files there are design sketches in aspirational syntax, not runnable code.
- `main.chaos` and `main2.chaos` are scratch files for development, not canonical examples.

## Compiler quirks

- Uses a custom `ChaosSlice[T]` data structure (`src/chaosSlice.go`) instead of standard Go slices for token/node streams. It has cursor-based navigation (`CSGet`, `CSConsume`, `CSMatch`, etc.).
- Custom `assert[T]` and `castAssert[T]` utilities in `src/chaosUtils.go` — panics with stack trace on failure. `assert` returns the zero value of `T` on success (used for type-safe assertions in expressions).
- `chaosDebug(...)` prints JSON-indented debug output. `todo[V](...)` marks unimplemented code paths and exits.
- `panicHandler()` in `main()` recovers panics and prints a filtered stack trace. Only active when `debug` is `true` (hardcoded).
- The compiler entrypoint is `src/chaos.go` `main()`. It reads `.chaos` files, calls `compileChaos()`, and writes `main.asm`.

## .gitignore quirk

The `.gitignore` uses `*` (ignore everything) then `!/**/` and `!*.*` to un-ignore files with extensions. This means **binary files without extensions are invisible to git** (e.g. `chaosBuild`, `build/chaosc`). Only files with a dot in the name are tracked.

## Project state

- Solo/experimental project, no CI, no linting, no formatting, no pre-commit.
- Goal: eventually self-host the compiler in Chaos itself.
- LICENSE: "Not to be used in any form" — closed source.
- OpenCode agent config at `.opencode/agent/developer.md` (primary agent, write/edit/bash require permission).
