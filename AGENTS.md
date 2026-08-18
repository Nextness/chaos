# Chaos Language — Agent Guide

## Build & Test

The Go module root is `src/` (module `chaos_compiler`). Two binaries are built into
`./build` by `make`:

```bash
make                  # build both binaries (build/chaosc, build/chaos-lsp)
make build/chaosc     # compiler CLI
make build/chaos-lsp  # language server
make test             # go test -C src ./...
make vet              # go vet -C src ./...
make e2e              # go test -C examples ./...
make check            # format check, vet, source tests, and e2e tests
```

- `go build ./...` from `src/` is a compile-check; `make` produces the binaries.
- `-C` must be the first flag: `go test -C src ./...` (not `go test -count=1 -C src ./...`).
- The compiler depends on `golang.org/x/text` for NFC identifier
  normalization; both Go modules have a `go.sum`.
- `chaosc <file.chaos>` compiles through fasm to `a.out` by default. `-check`,
  `-dump`, `-ir`, and `-asm` select non-executable modes, and `-o` selects an
  output path. Diagnostics go to stderr in
  a compact rust-like format: `[ERROR] line:col:file - reason`, the source line,
  a caret underline, and a `-> suggestion` line. No `log/slog` is used.
- A successful `-check` produces no output; the default mode produces an ELF64
  executable.
- `chaos-lsp` speaks stdio JSON-RPC 2.0 and is wired into Neovim via the `chaos`
  module at `~/.config/nvim/lua/chaos/init.lua`, loaded from
  `~/.config/nvim/init.lua` with `require('chaos').setup()`.

## Architecture

### Compiler library (`src/compiler/`)

`token.go` → `tokenizer.go` → `parser.go`/`ast.go` → `type_checker.go` →
`lower.go`/`hir.go` → `lower_mir.go`/`mir.go` → `ir_verify.go` → `fasm.go`,
plus `diagnostic.go` and `position.go`.

- Tokenizer produces `[]Token` + `DiagnosticList` (error recovery continues past
  `TkError` tokens).
- Custom data structures: `Span{File, Start, End}` (half-open), `SourceManager`
  (FileID → file path, source bytes, line offsets).
- `Token.Value` is `string` for all literal tokens (int, float, string,
  true/false) and for directive names (`TkDirec`). BoolExpr in the AST stores a
  Go `bool`; the token-to-bool conversion happens during parsing, not during
  tokenization.
- String literals use Unicode guillemets `«like this»` (2 bytes each: 0xC2 0xAB
  / 0xC2 0xBB).
- `::` = compile-time assignment, `:=` = runtime assignment.
- Comments (`//` line, `/** ... **/` block) are emitted as `TkComment` tokens;
  the parser skips them transparently in `peek()`.
- `#<name>` tokenizes as `TkHash` + `TkDirec(name)` (directive names bypass
  keyword lookup, so `#proc` and `#true` stay directives).
- `ParseProgram` is strict. `ParseProgramTolerant`, used by the LSP, builds real
  nodes for every implemented construct, including enums and every loop form;
  it recovers only genuinely unknown/incomplete editor input. Entry syntax is
  `#entry name :: proc -> S64 { ... }`.

### Language server (`src/cmd/chaos-lsp/`)

`main.go` → `jsonrpc.go` → `server.go` → `handlers.go` → `convert.go`, plus
`symbols.go` (documentSymbol), `semantic.go` (semantic tokens), and `resolve.go`
(definition, references, documentHighlight, hover, completion).

- Hand-rolled `Content-Length` framed JSON-RPC 2.0 over stdio.
- Incremental sync (`textDocumentSync` change = 2); diagnostics are pushed on
  open and on change.
- Semantic tokens are delta-encoded as 5 ints per token
  `[deltaLine, deltaStart, length, typeIndex, modifiers]` (modifiers always 0);
  Neovim's parser iterates the array in steps of 5.
- Highlighting is LSP-based: the `chaos` module links the `@lsp.type.*` groups
  to the base highlight groups. There is no active `after/syntax/chaos.vim`
  (it is disabled). The gruber-darker colorscheme hides all `@lsp.*` groups on
  `ColorScheme`, so the module re-applies the links on `LspAttach`.

## Source layout

- `src/compiler/` — compiler library (package `compiler`).
- `src/cmd/chaosc/` — compiler CLI (package `main`).
- `src/cmd/chaos-lsp/` — language server (package `main`).
- `main.chaos` — the owner's personal/manual scratch file; it is not an
  automated compiler fixture and agents must not rewrite it as part of tests.
- `language_design/` — speculative design sketches. **Not implemented.**
  Features here (generics, async, C interop, modules, self-hosted compiler) do
  not exist in the compiler.

## Compiler quirks

- Tokenizer uses a cursor-based approach (`pos`, `lastPos`, `source` slice) with
  `peekN(n)`, `advance()`, `skipTrivia()`.
- Error recovery: `TkError` tokens are emitted and scanning continues (progress
  guard ensures forward motion).
- Unterminated block comments emit `TkError` (not `TkComment`) plus a
  diagnostic.
- Numeric separator validation: `_` must follow a digit (rejects `1__2`, `_5`,
  trailing `_`).
- `TokenKind.String()` panics for out-of-range values (catches enum/name-table
  mismatch).
- Diagnostic for `..` (two dots) in numeric literals: `1..0` flags `.0` as
  error. `...` ellipsis is handled correctly.
- Emit helpers: `emitN(kind, start, n)` handles all token lengths (1, 2, 3
  characters).
- Numeric validation rejects underscores before decimal point (`1_.0`) and
  floats with no digit after the dot (`1.`).
- Tolerant mode is heuristic: skipping to matched braces can hide real errors in
  unknown forms; recognized constructs still report errors.

## .gitignore

The `.gitignore` ignores `build/`, `.vscode`, `chaos_compiler*`, and `*.asm`.
Binaries under `build/` are not tracked.

## Project state

- Solo/experimental project, no CI or pre-commit. `make check` enforces Go
  formatting, vet, compiler/LSP tests, and end-to-end tests.
- Goal: eventually self-host the compiler in Chaos itself.
- LICENSE: "Not to be used in any form" — closed source.
- The current remediation record is `PLAN.md`; the audit for the removed
  compiler is archived under `docs/legacy-compiler-audit-2026-07-15.md`.
- OpenCode agent config at `.opencode/agent/developer.md` (write/edit/bash
  require permission).
