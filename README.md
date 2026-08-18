# Chaos Language

Chaos is an experimental compiled language being built toward eventual
self-hosting. The active compiler is written in Go and currently implements a
complete single-file path from UTF-8 source text to a Linux ELF64 executable.

The compiler pipeline is:

```text
source -> tokens -> AST -> semantic analysis -> HIR -> MIR -> verification -> fasm -> ELF64
```

The repository is closed source under the terms in `LICENSE`.

## Requirements

- Go 1.24.1 or newer.
- `fasm` on `PATH` to produce or execute binaries. Front-end checks, IR dumps,
  assembly emission, and most tests do not require it.

The only Go dependency is `golang.org/x/text`, used for NFC normalization of
Unicode identifiers.

## Build and check

From the repository root:

```bash
make                         # build build/chaosc and build/chaos-lsp
make check                   # format check, vet, compiler/LSP tests, e2e tests
make race                    # slower race-enabled source and e2e suites
make coverage                # package coverage under build/coverage/
```

`make e2e` always validates the full compilation pipeline and golden dumps. If
`fasm` is unavailable, it reports that executable runtime checks are skipped.

## Compile a program

This is a complete compilable Chaos program:

```chaos
#entry main :: proc -> S64 {
    return 42;
}
```

Save it as `hello.chaos`, then use one of the explicit modes:

```bash
./build/chaosc -check hello.chaos              # target-independent front-end check
./build/chaosc -dump hello.chaos               # tokens and AST
./build/chaosc -ir hello.chaos                 # HIR and MIR
./build/chaosc -asm -o hello.asm hello.chaos   # verified fasm assembly
./build/chaosc -o hello hello.chaos             # assemble a Linux ELF64 executable
./hello                                         # exits with status 42
```

With no mode flag, `chaosc` performs a full compilation and writes `a.out`
unless `-o` selects another path. Exactly one of `-check`, `-dump`, `-ir`, and
`-asm` may be selected. The only current backend is `-backend fasm`.
Diagnostics are written to stderr. Textual mode output goes to stdout unless
`-o` is supplied.

The designated entry point currently has one exact form: it takes no
parameters and returns `S64`.

```chaos
#entry any_name :: proc -> S64 { ... }
```

## Implemented language surface

The active single-file language includes:

- fixed-width signed/unsigned integers, `F32`, `F64`, `Bool`, and `String`;
- compile-time `::` and runtime `:=` declarations with explicit `#shadow`;
- procedures, recursion, lexical nested declarations without implicit runtime
  capture, up to 100 parameters/arguments/results;
- structs with named/positional construction, recursive defaults, and typed
  zero initialization for omitted fields;
- nominal integer-backed enums and nominal error types;
- multiple value returns, error-return unions, `unless catch`, and `if catch`;
- arrays, range/C-style/condition loops, branching, arithmetic, comparisons,
  `return`, and `exit`;
- element-wise array equality and field-wise struct equality;
- Unicode identifiers normalized to NFC.

Current deliberate restrictions include non-escaping array storage, no array
bounds checks yet, no compile-time execution of ordinary procedure calls, and
no fasm support for `F16` or `F128`. Runtime integer arithmetic wraps. Integer
or floating-point division by zero exits with status 1 and a source location.

## Repository layout

- `src/compiler/` — tokenizer through verified MIR and the fasm backend.
- `src/cmd/chaosc/` — compiler command-line driver.
- `src/cmd/chaos-lsp/` — stdio JSON-RPC language server.
- `examples/` — valid/invalid end-to-end fixtures and reviewed golden output.
- `language_design/` — speculative syntax/library sketches, not compiler input.
- `docs/` — historical audits and implementation notes.
- `.opencode/` — repository-local OpenCode agent configuration; its installed
  dependencies remain ignored.

Modules/imports, a standard library, generics, pointers, maps, async, C interop,
and the self-hosted compiler are design work only. Files under
`language_design/` may intentionally use syntax the strict compiler rejects.

## Language server

`build/chaos-lsp` speaks LSP over `Content-Length` framed JSON-RPC 2.0 on
stdin/stdout. It provides diagnostics, symbols, semantic tokens, definition,
references, highlights, hover, and completion from one immutable analyzed
snapshot per accepted document version.
