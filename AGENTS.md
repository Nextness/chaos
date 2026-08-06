# Chaos Language — Agent Guide

## Build & Test — Old compiler (`src/`)

```bash
go build chaosBuild.go       # bootstrap build binary
./chaosBuild                 # compile compiler (output: ./build/chaosc)
./chaosBuild -c file.chaos   # compile a .chaos source
./chaosBuild -c file.chaos -r # compile + assemble via fasm → ELF binary
./chaosBuild -test           # run all .chaos test files in ./tests/
./chaosBuild -default        # compile main.chaos (only when DEBUG=true, hardcoded)
```

- **Linux-only** (`runtime.GOOS == "linux"` hardcoded in `chaosBuild.go`).
- **No `go.sum`, no external deps.** Only stdlib.
- **`-r` flag requires `fasm`** on `$PATH`. It looks for `<file>.asm` output from the compiler, assembles it, and runs the resulting binary.
- **Test runner** iterates all files in `./tests/`, runs `./build/chaosc tests/<file>` on each. It does NOT distinguish expected-pass vs expected-fail — every file is run the same way. Failure files (e.g. `assignments_failure.chaos`) are expected to cause compiler errors. Runner exits 0 unconditionally regardless of test failures.

## Build & Test — New compiler (`src_new_impl/`)

```bash
go build ./...               # build
go test ./...                # run all tests (87.6% coverage)
go vet ./...                 # static analysis
go run . file.chaos          # tokenize and parse a file (diagnostics to stderr)
```

- Separate `go.mod` (module `chaos_new`), no external deps, only stdlib.
- Diagnostics and driver errors are written to stderr with `fmt.Fprintf` in a compact rust-like format: `[ERROR] line:col:file - reason`, the source line, a caret underline, and a `-> suggestion` line. No `log/slog` is used.
- A successful compile produces no output.

## Architecture

### Old compiler pipeline (`src/`)

`src/chaos.go` → `src/chaosLexer.go` → `src/chaosAst.go` → `src/chaosInferenceCheckType.go` → `src/chaosCodegen.go`

- **Codegen is stubbed.** Writes only an fasm ELF64 preamble + AST JSON debug output. The real codegen (commented out at lines 30–278) expects a removed `Program` model.
- **IR is defined but unused.** `src/chaosIr.go` has `ProgramToIR()` but the pipeline never calls it.
- Name resolution is done during parsing (mixed responsibility). No forward references or recursion.
- Type checking only handles top-level identifiers and `exit` nodes. Procedure bodies, binary expressions, and branches are skipped.
- Entrypoint is `src/chaos.go` `main()`. Reads `.chaos` files, calls `compileChaos()`, writes `main.asm`. Multiple input files all overwrite the same output.

### New compiler pipeline (`src_new_impl/`)

`tokenizer.go` → `parser.go` → (semantic analysis not yet implemented)

- Tokenizer produces `[]Token` + `DiagnosticList` (error recovery continues past `TkError` tokens).
- Custom data structures: `Span{File, Start, End}` (half-open), `SourceManager` (FileID → file path, source bytes, line offsets).
- `Token.Value` is `string` for all literal tokens (int, float, string, true/false). BoolExpr in the AST stores a Go `bool`; the token-to-bool conversion happens during parsing, not during tokenization.
- String literals use Unicode guillemets `«like this»` (2 bytes each: 0xC2 0xAB / 0xC2 0xBB).
- `::` = compile-time assignment, `:=` = runtime assignment (in old compiler too, but only mutability is recorded).

## Source layout

- `src/` — old compiler (Go, package `main`, ~2600 lines across 8 files). Active but incomplete.
- `src_new_impl/` — new compiler implementation (Go, package `main`, ~4,700 lines across 13 files). Tokenizer and parser complete. Roughly 2,200 production lines across 7 non-test files, with the remainder (~2,400) in test files.
- `chaosBuild.go` — separate bootstrap binary (not part of `src/`).
- `language_design/` — speculative design sketches. **Not implemented.** Features here (structs, generics, async, C interop, modules, self-hosted compiler) do not exist in either compiler.
- `main.chaos` and `main2.chaos` — scratch files for development, not canonical examples.
- `tests/` — 6 compile-smoke fixtures. Some "ok" files fail because the type checker is incomplete.

## Compiler quirks — Old compiler (`src/`)

- Uses custom `ChaosSlice[T]` data structure (`src/chaosSlice.go`) with cursor-based navigation (`CSGet`, `CSConsume`, `CSMatch`, etc.) instead of standard Go slices for token/node streams.
- Custom `assert[T]` and `castAssert[T]` utilities in `src/chaosUtils.go` — panics with stack trace on failure.
- `chaosDebug(...)` prints JSON-indented debug output. `todo[V](...)` marks unimplemented code paths and exits.
- `panicHandler()` in `main()` recovers panics and prints a filtered stack trace. Only active when `debug` is `true` (hardcoded).
- Pratt parser at `src/chaosAst.go:288-294` has a precedence bug: consumes operator before checking `lbp < minBp`.
- `ChaosContentParseScope` requires `isFunction` — anonymous braces and conditionals are only parsed inside a procedure body.
- `if` rejected at depth ≤ 1 (restricted to procedure/nested bodies).
- `return` always requires an expression — `return;` cannot parse.
- Float literals become `nodeIntLiteral` due to an unreachable `nodeFloatLiteral` path — can panic via `castAssert[int]`.
- `Conditions` store `[]BinOp` (not `[]Expr`), so `if true` or `if flag` dereferences nil.
- `runCommand` wraps every command in `bash -c` — filenames are not quoted.
- `compileChaos` ignores the `filepath` parameter — every input overwrites `main.asm`.

## Compiler quirks — New compiler (`src_new_impl/`)

- Tokenizer uses a cursor-based approach (`pos`, `lastPos`, `source` slice) with `peekN(n)`, `advance()`, `skipTrivia()`.
- Error recovery: `TkError` tokens are emitted and scanning continues (progress guard ensures forward motion).
- Comments (`//` line, `/**/` block) are skipped iteratively via `skipTrivia()` — no recursion.
- Numeric separator validation: `_` must follow a digit (rejects `1__2`, `_5`, trailing `_`).
- `TokenKind.String()` panics for out-of-range values (catches enum/name-table mismatch).
- Diagnostic for `..` (two dots) in numeric literals: `1..0` flags `.0` as error. `...` ellipsis is handled correctly.
- Emit helpers: `emitN(kind, start, n)` handles all token lengths (1, 2, 3 characters).
- Numeric validation rejects underscores before decimal point (`1_.0`) and floats with no digit after the dot (`1.`).

## .gitignore quirk

The `.gitignore` uses `*` (ignore everything) then `!/**/` and `!*.*` to un-ignore files with extensions. This means **binary files without extensions are invisible to git** (e.g. `chaosBuild`, `build/chaosc`). Only files with a dot in the name are tracked.

## Project state

- Solo/experimental project, no CI, no linting, no formatting, no pre-commit.
- Two compilers exist: `src/` (old, incomplete pipeline) and `src_new_impl/` (new, tokenizer and parser complete).
- Goal: eventually self-host the compiler in Chaos itself.
- LICENSE: "Not to be used in any form" — closed source.
- Full defect audit at `report.md` (P0-P2 defects, architectural recommendations for the legacy compiler only).
- OpenCode agent config at `.opencode/agent/developer.md` (write/edit/bash require permission).