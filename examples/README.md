# Chaos language examples and end-to-end tests

This directory holds example Chaos programs and an end-to-end test suite that compiles every example through the full pipeline and validates each step.

## Layout

- `valid/` — programs that must compile cleanly, assemble, and run.
- `invalid/` — programs that must fail, with the expected failure stage and diagnostic recorded in the file header.
- `golden/` — expected dumps (tokens, AST, HIR, MIR, fasm assembly, rendered diagnostics) captured from the compiler.
- `e2e_test.go` — the test harness.

## Running

```sh
make e2e            # from the repository root
cd examples && go test -count=1 ./...
```

The runtime check (assembly, execution, exit status, stderr) is skipped when `fasm` is not installed; compilation and dump validation always run.

To regenerate the golden files after an intentional change to the compiler output:

```sh
cd examples && go test -count=1 -update ./...
```

## How a valid example looks

```chaos
// expect-exit: 42
#entry main :: proc -> S64 {
    return 42;
}
```

The `expect-exit` header declares the expected exit status. `expect-stderr` (optional) declares the exact expected stderr content. The harness then:

1. Tokenizes, parses, type-checks, target-validates, lowers to HIR and MIR, verifies, and emits fasm assembly, failing if any stage reports an error.
2. Compares the dump of each step (tokens, AST, HIR, MIR, asm) against the golden files.
3. Assembles the emitted assembly with fasm, runs the binary, and checks the exit status and stderr.

## How an invalid example looks

```chaos
// expect-stage: type
// expect-error: cannot assign S64 to String
#entry main :: proc -> S64 {
    x: String = 5;
    return 0;
}
```

The `expect-stage` header declares the first pipeline stage that must report errors (`tokenize`, `parse`, `type`, `target`, `lower`, `mir`, `verify`, `emit`). `expect-error` is a substring that must appear in the rendered diagnostics. The harness verifies both, and compares the rendered diagnostics against the golden file.

`TestExampleMetadata` validates the harness itself: every example must carry the metadata its directory requires, so a typo in a header comment fails loudly instead of silently skipping a check.
