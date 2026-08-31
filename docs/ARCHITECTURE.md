# Chaos Compiler Architecture

This document describes how the current compiler is implemented. It follows data from a source file to an executable and records the contracts between stages. It is not a language reference; source examples appear only when they help identify a compiler path.

The implementation is written in Go. The compiler library is in `src/compiler`, the command-line driver is in `src/cmd/chaosc`, and the language server is in `src/cmd/chaos-lsp`.

## Current pipeline

```text
Chaos source bytes
    |
    v
SourceManager + SourceFile
    |
    v
Tokenizer ------------------------------> TokenList
    |                                         |
    | diagnostics                             v
    +------------------------------------ recursive-descent/Pratt parser
                                              |
                                              v
                                             AST
                                              |
                                              v
                                      semantic analysis
                                              |
                                  +-----------+-----------+
                                  |                       |
                                  v                       v
                          SemanticAnalysis          diagnostics
                                  |
                                  v
                         target validation
                                  |
                                  v
                            AST -> typed HIR
                                  |
                                  v
                         HIR -> control-flow MIR
                                  |
                                  v
                             MIR verifier
                                  |
                                  v
                           fasm backend emitter
                                  |
                                  v
                          fasm assembly text
                                  |
                                  v
                        external fasm process
                                  |
                                  v
                         Linux x86-64 ELF file
```

Each stage returns diagnostics instead of terminating the process. The CLI renders those diagnostics and stops after the first stage that reports an error. Source spans are preserved through the AST, HIR, MIR, verifier, and backend so failures can continue to point at the original file.

The production stage entry points are:

| Stage | Entry point | Main output |
|---|---|---|
| Source registration | `SourceManager.Register` | `SourceFile` and `FileID` |
| Tokenization | `Tokenize` | `TokenList` |
| Parsing | `ParseProgram` | `Program` AST |
| Semantic analysis | `AnalyzeProgram` | `SemanticAnalysis` |
| Target check | `ValidateTarget` | target diagnostics |
| HIR lowering | `LowerAnalyzedProgram` | `HIR` |
| MIR lowering | `LowerToMIR` | `MIRProgram` |
| MIR verification | `VerifyMIR` | invariant diagnostics |
| Code generation | `Backend.Emit` | assembly text |
| Assembly | CLI `assembleFasm` | ELF64 executable |

## Driver and compilation modes

`src/cmd/chaosc/main.go` owns the production pipeline. It accepts one root
source file per invocation and registers it in a fresh source manager. Import
resolution adds flat or namespaced source modules from paths relative to the
importer and from the configured standard-library directory. There is no
separate compilation, object-file writer, or linker stage.

The driver always tokenizes, parses, and analyzes before selecting its final output:

| Invocation | Last stage run | Output |
|---|---|---|
| `chaosc -check file.chaos` | semantic analysis | no artifact |
| `chaosc -dump file.chaos` | semantic analysis | tokens and AST |
| `chaosc -ir file.chaos` | MIR verification | HIR and MIR |
| `chaosc -asm file.chaos` | backend emission | fasm source |
| `chaosc file.chaos` | external assembly | `a.out` |

`-o` directs a textual artifact or executable to a named path. Text and binary artifacts are first written beside the destination and then installed with an atomic rename. The executable path is marked executable only after `fasm` succeeds. Temporary assembly and output files are removed on failure.

Target validation is run for assembly and executable modes. Front-end checks and IR dumps are target-independent and therefore do not require `fasm`.

## Stage 1: source management and positions

`src/compiler/token.go` defines the source-level identity model:

- `FileID` is a small compilation-session identifier.
- `SourceFile` stores the path, immutable source bytes, and cached line starts.
- `SourceManager` owns registered files and assigns their IDs.
- `Span` is a half-open byte range `[Start, End)` associated with a `FileID`.

The CLI currently registers one file, but the source and IR structures use file IDs and source maps rather than a global filename. `Program.Sources`, `HIR.Sources`, and `MIRProgram.Sources` carry the source records needed by later diagnostics and runtime trap messages.

Internally, offsets and spans are byte-based. CLI diagnostics convert them to Unicode scalar columns, while `SpanToRange` converts them to the UTF-16 code units required by LSP. `ClampSpan` protects reporting boundaries from invalid or reversed internal spans.

## Stage 2: tokenization

`src/compiler/tokenizer.go` contains a cursor-based scanner over the source byte slice. `Tokenize` produces a token list and an independent diagnostic list.

Important tokenizer behavior:

- Every token retains its raw source bytes and half-open span.
- Literal and identifier values are stored in `Token.Value`.
- Identifiers and directive names are normalized to NFC with `golang.org/x/text/unicode/norm` before later name lookup.
- Comments are emitted as tokens. The parser skips them transparently, while the LSP reuses them for semantic highlighting.
- Invalid tokens are represented by `TkError`; scanning continues after a diagnostic and includes a progress guard.
- The token kind table is exhaustive. An invalid `TokenKind.String` call panics intentionally so enum/table drift is detected during development.

The scanner does not construct semantic values. For example, it preserves the text of numeric literals so the semantic and lowering stages can perform exact range checks without first truncating to a machine integer.

## Stage 3: parsing and AST construction

`src/compiler/parser.go` is a hand-written recursive-descent parser. Expression precedence and postfix operations are handled by a Pratt parser. AST node definitions live in `src/compiler/ast.go`.

The parser constructs declarations, statements, and expressions without resolving names or types. Nodes retain source spans, and declaration names, members, parameters, and fields also retain their narrower name spans for diagnostics and editor operations.

`Program` contains:

- top-level declarations in source order;
- the selected entry declaration, if one was parsed;
- the source map attached by the driver.

Two parser modes share the same implemented grammar:

- `ParseProgram` is the strict compiler path. Unknown syntax is diagnosed.
- `ParseProgramTolerant` is the editor path. It creates normal nodes for every
  implemented construct and performs broader recovery around incomplete or
  unknown input. Malformed generic declarations are skipped as recovery forms
  rather than being represented as ordinary declarations.

Both parsers recover after errors and guarantee forward progress. Tolerant parsing is not used to compile executables.

`src/compiler/ast_walk.go` provides the compiler-owned exhaustive AST walker. Target validation and drift tests use it for tasks that do not need a custom scope-aware traversal.

## Stage 4: semantic analysis

`src/compiler/type_checker.go` performs name resolution, type checking, compile-time validation, and control-flow checks. `AnalyzeProgram` returns both diagnostics and a reusable `SemanticAnalysis`; `CheckProgram` is a convenience wrapper when only diagnostics are needed.

Semantic analysis has two related namespaces:

- value scopes hold variables, constants, parameters, initialization state, mutability, and compile-time values;
- declaration scopes hold procedures and nominal type declarations.

Top-level declarations are registered before their bodies are checked, so the global namespace is forward-visible. Local declarations are registered when their statement is reached, making local lookup source-ordered. Nested procedures and nominal declarations receive lexical identities. A nested procedure cannot implicitly capture an outer runtime value; compile-time-known outer bindings remain available.

The program analysis proceeds approximately as follows:

1. Create the global scope and register top-level procedures and nominal types.
2. Register global values and their declared types.
3. Validate nominal declarations, enum values, and by-value record cycles.
4. Build the global initializer dependency graph, reject cycles, and check globals in dependency order.
5. Validate struct fields and defaults after compile-time globals are known.
6. Validate the selected entry declaration.
7. Check procedure signatures, bodies, nested scopes, and must-diverge paths.

Compile-time integer work uses `math/big.Int`. Exact literal and enum values are checked against their declared fixed-width type before lowering; arbitrary precision is not a runtime representation. Compile-time aggregate expressions are validated recursively. Ordinary procedure execution at compile time is currently rejected.

`SemanticAnalysis` is the contract consumed by later stages and editor tools. It contains:

- resolved expression types;
- resolved type-expression types;
- declaration types;
- stable nominal declaration identities;
- resolved procedure identities;
- the dependency order for global initialization.

Nominal semantic identities are distinct even when lexical shadowing reuses a source spelling. User-facing diagnostics format those identities back to their source names.

## Stage 5: target validation

`src/compiler/backend.go` defines the backend interface and the backend registry. The only registered backend is `fasm`.

`ValidateTarget` runs after semantic analysis and before HIR lowering in modes that produce target code. It rejects source types that are valid in the front end but unsupported by the selected target. The current concrete example is `F16` and `F128`: they remain known types, but the fasm backend accepts only `F32` and `F64` floating-point storage and operations.

The backend repeats its supported-type checks before emission. This protects library callers that construct or pass MIR without using the CLI driver.

## Stage 6: AST to HIR lowering

`src/compiler/lower.go` converts a semantically valid AST into typed HIR. `LowerAnalyzedProgram` requires the exact `SemanticAnalysis` produced for that AST; passing no analysis is an error. `LowerProgram` is a convenience path that runs analysis before lowering.

HIR is defined in `src/compiler/hir.go`. It remains close to the source:

- control flow is still represented as blocks, conditionals, loops, returns, and catches;
- every expression has a `TypeID`;
- declaration references use stable `SymbolID` values instead of strings;
- every node retains its source span.

`src/compiler/ir.go` contains the shared IR infrastructure:

- `SymbolTable` assigns a fresh ID for every declaration, including shadowed declarations with the same spelling;
- `TypeTable` interns built-ins, arrays, structs, errors, enums, and synthetic tuple types;
- `IRType` records kind-specific metadata such as fields, array element type, and enum underlying type.

Lowering first registers top-level symbols and nominal types, resolves type aliases and field layouts, then lowers globals in `SemanticAnalysis.GlobalOrder`. Procedures are lowered after global values are available. Nested procedures are emitted as independent HIR procedures with stable symbols; their source visibility has already been enforced by semantic analysis.

This stage also removes several source-level conveniences:

- compile-time bindings are folded and substituted at references;
- constant scalar operations are folded with exact integer intermediates;
- missing aggregate values become recursive typed zero/default expressions;
- range loops become the structured HIR loop form with hidden index state;
- multiple source results become a synthetic tuple record;
- an error-return result becomes a synthetic record containing `value`, `error`, and `hasError` fields.

There is no separate optimization pipeline. Constant folding in `src/compiler/const_fold.go` is part of HIR construction and compile-time value materialization.

## Stage 7: HIR to MIR lowering

`src/compiler/lower_mir.go` converts structured HIR into the control-flow MIR defined by `src/compiler/mir.go`.

A `MIRProgram` contains:

- the shared symbol and type tables;
- runtime globals;
- a list of functions;
- the entry function symbol;
- an optional synthetic global initializer function;
- the source map.

Each `MIRFunction` owns local slots and an ordered list of basic blocks. A basic block contains typed three-address instructions followed by exactly one terminator. Values use function-local `ValueID` values; local slots use `LocalID`; globals and call targets use `SymbolID`.

Structured control flow is made explicit here:

- conditions become branch terminators and join blocks;
- loops become header/body/after/exit blocks;
- `break` and `continue` target the appropriate loop blocks;
- returns and exits become terminators;
- catch handling branches on the synthetic union's `hasError` field.

Compile-time globals are omitted from executable MIR because their uses were already substituted. Runtime global initializers are lowered into the reserved `__chaos_global_init` function, which stores globals in semantic dependency order and is called before the entry function.

Source-level multiple results already occupy one physical tuple value by this point. MIR functions therefore have zero or one physical result.

## Stage 8: MIR verification

`src/compiler/ir_verify.go` is the safety boundary before code generation. `VerifyMIR` treats malformed IR as a compiler error and reports diagnostics instead of allowing the backend to guess.

The verifier checks, among other invariants:

- symbol and type table consistency;
- built-in type metadata and valid composite-type references;
- absence of recursive by-value record layouts;
- unique function, global, block, local, and value identities;
- the reserved global initializer signature and placement;
- valid instruction opcodes, immediates, arity, and operand types;
- local/global mutability and initialization rules;
- value definition, use order, reachability, and dominance;
- valid jump and branch targets;
- exactly one terminator per reachable block;
- return and exit types, plus a valid entry function reference;
- exact integer constant ranges and supported finite constant forms;
- absence of compile-time bindings in executable MIR.

The CLI invokes the verifier explicitly. `FasmBackend.Emit` invokes it again, so direct backend users receive the same protection.

## Stage 9: fasm code generation

`src/compiler/fasm.go` emits flat-assembler source for a Linux x86-64 ELF64 executable. It consumes verified MIR and does not invoke a system linker.

The emitter performs these main passes:

1. Check target type support and validate the entry signature.
2. Assign stable labels to globals and functions.
3. Collect floating-point constants, string bytes, and division-trap messages.
4. Emit writable data for globals and constants.
5. Emit the `_start` process wrapper.
6. Assign stack slots and emit every MIR function and basic block.
7. Emit reusable equality helpers for aggregate types.

`_start` calls `__chaos_global_init` when present, calls the selected entry procedure, and performs the Linux `exit` syscall with its result. Source-level `exit` and division-by-zero traps also lower directly to Linux write/exit syscalls.

The backend uses a System V AMD64-shaped internal calling convention:

- integer-like values use the six integer argument registers;
- `F32`/`F64` values use up to eight XMM argument registers;
- excess arguments use aligned outgoing stack space;
- records are passed by address and returned through a hidden result pointer;
- strings and 128-bit integers occupy two machine words;
- dynamic arrays occupy three machine words and aggregate arguments are passed
  by address;
- scalar results use `RAX`, floating results use `XMM0`, and two-word results
  use `RAX:RDX`;
- callee-saved registers and 16-byte call alignment are preserved.

One `Layout` calculation is shared by global storage, stack slots, fields, array strides, calls, copies, and comparisons. Current runtime representations include:

| IR kind | Runtime representation |
|---|---|
| `Bool` | one byte |
| fixed integer | declared bit width |
| enum | its underlying integer layout |
| error | 16-bit ordinal |
| `F32` / `F64` | 4 / 8 bytes |
| string | pointer and byte length |
| runtime/fixed array | element pointer and element count |
| dynamic array | element pointer, element count, and capacity |
| struct / tuple | aligned fields in declaration order |

Fixed and runtime array literal buffers live in function stack frames; dynamic
array buffers use the checked bump arena. Semantic analysis rejects unsupported
array escapes until an ownership model exists. Every element-address operation,
including reads, writes, compound writes, increments, and address-taking,
checks negative and upper bounds. Struct and array equality use generated
recursive, element-wise helpers.

Integer arithmetic emits fixed-width wrapping operations. Exact integer text survives to emission so 128-bit and boundary values can be encoded correctly. Division and modulo emit a zero check; the failure path writes a message with the preserved source path, line, and column to stderr and exits with status 1.

## Stage 10: external assembly and artifact installation

Executable mode looks up `fasm` on `PATH`. The CLI writes assembly to a temporary file, asks `fasm` to produce a temporary ELF file beside the final destination, marks it executable, and atomically renames it into place.

An assembler failure leaves the requested destination untouched and includes the assembler output in the CLI diagnostic. `-asm` stops before this step and therefore works without an installed assembler.

## Diagnostics and failure propagation

`src/compiler/diagnostic.go` defines `Diagnostic` and `DiagnosticList`. Tokenizer, parser, analysis, lowering, verification, and backend calls all use the same severity/span/message/suggestion representation.

The CLI renders diagnostics in a compact source form and exits nonzero when a stage reports an error. Warnings do not stop the pipeline. Internal recovery nodes and unknown types allow a stage to collect useful diagnostics, but the driver does not intentionally pass an errored stage into the next one.

The LSP converts the same compiler diagnostics to LSP ranges, keeping compiler and editor errors on one source of truth.

## Language server architecture

The language server is an editor front end over the tokenizer, tolerant parser, semantic analyzer, and source-position utilities. It does not lower to HIR/MIR or run the backend.

```text
stdio JSON-RPC frame
        |
        v
request/notification dispatcher
        |
        v
validated document version + incremental edits
        |
        v
immutable Document snapshot
        |
        +-- SourceFile
        +-- TokenList
        +-- tolerant AST
        +-- SemanticAnalysis
        +-- diagnostics
        +-- lexical resolver index
        +-- document symbols
        +-- encoded semantic tokens
```

`src/cmd/chaos-lsp/jsonrpc.go` implements `Content-Length` framed JSON-RPC 2.0 with bounded header and body sizes. `main.go` owns the initialize/shutdown/exit lifecycle. Protocol output goes to stdout; logging goes to stderr.

`newDocument` in `server.go` builds one immutable analyzed snapshot for every
accepted document version. It tokenizes and tolerantly parses the complete
text, then runs semantic analysis even when recoverable editor diagnostics are
present so independent valid regions retain useful facts. The snapshot also
precomputes its resolver, document outline, and semantic-token encoding.

Incremental changes are checked against the current version and translated from UTF-16 positions to byte offsets. Invalid edits and stale versions are rejected without replacing the last good snapshot. A mutex protects the document map, lifecycle state, and writer error state.

The resolver in `resolve.go` builds a lexical scope tree and occurrence index from the tolerant AST and semantic facts. It powers definition, references, document highlights, hover, and completion. `semantic.go` combines lexical tokens with AST classifications, sorts them, and emits the LSP delta encoding. `symbols.go` builds the document outline.

The server keeps a document-oriented index rather than a persistent workspace
database. Open documents form an in-memory import overlay; changes rebuild
dependent snapshots, and import-aware navigation can resolve symbols across
those modules. Supported requests are:

- document symbols;
- full semantic tokens;
- definition;
- references;
- document highlights;
- hover;
- completion.

Diagnostics are published on open and every accepted change, and cleared on close.

## Dumps, tests, and architecture checks

`src/compiler/dump.go` renders tokens and the AST. `src/compiler/ir_dump.go` renders HIR and MIR. These dumps are debugging formats, not stable serialized IR or an interchange format.

The `examples` Go module drives real source files through every compiler stage and compares token, AST, HIR, MIR, assembly, and diagnostic output with golden files. When `fasm` is present, valid examples are also assembled and executed.

The main quality commands are:

```bash
make check      # formatting, vet, source tests, and end-to-end tests
make race       # race-enabled source and end-to-end tests
make coverage   # coverage profiles under build/coverage
```

Unit tests exercise individual stages. Fuzz targets cover tokenization, parsing, diagnostics, MIR verification, and JSON-RPC/editor input. Visitor drift tests ensure new AST node kinds are added to generic traversal and tooling paths.

## Extension points and present boundaries

The main intended extension points are:

- `Backend` and the backend registry for another code generator;
- `ValidateTarget` for source-level target capability checks;
- the canonical built-in registry in `builtin_types.go`;
- `TypeTable` and MIR opcodes for new runtime representations;
- the compiler AST walker for generic tooling;
- `SourceManager` for a future multi-file compilation session.

The current architectural boundaries are:

- one root source file per CLI compilation, with source-level imports;
- no object files or linker integration;
- one backend: fasm for Linux x86-64 ELF64;
- no standalone optimization pass;
- compile-time folding, but no execution of ordinary procedures at compile time;
- checked fixed/runtime array indexing and bump-allocated dynamic arrays, but
  no settled escaping ownership or copy/aliasing model;
- `F16` and `F128` known to the front end but rejected for fasm emission;
- open-document import overlays without a persistent workspace symbol graph.

## Source map

| Path | Responsibility |
|---|---|
| `src/cmd/chaosc/main.go` | CLI orchestration, modes, fasm invocation, atomic outputs |
| `src/compiler/token.go` | source records, spans, tokens, token kinds |
| `src/compiler/tokenizer.go` | source bytes to tokens |
| `src/compiler/parser.go` | strict/tolerant parsing and recovery |
| `src/compiler/ast.go` | AST node model |
| `src/compiler/ast_walk.go` | exhaustive generic AST traversal |
| `src/compiler/type_checker.go` | semantic analysis and reusable facts |
| `src/compiler/builtin_types.go` | canonical built-in type registry |
| `src/compiler/backend.go` | backend interface, registry, target validation |
| `src/compiler/lower.go` | AST and semantic facts to HIR |
| `src/compiler/const_fold.go` | HIR compile-time folding |
| `src/compiler/hir.go` | typed, structured HIR model |
| `src/compiler/ir.go` | symbol/type tables and shared IR IDs |
| `src/compiler/lower_mir.go` | HIR to control-flow MIR |
| `src/compiler/mir.go` | MIR model and opcodes |
| `src/compiler/ir_verify.go` | MIR invariant validation |
| `src/compiler/fasm.go` | Linux x86-64 layout, ABI, and assembly emission |
| `src/compiler/diagnostic.go` | compiler diagnostics and CLI rendering |
| `src/compiler/position.go` | byte, scalar, and UTF-16 position conversion |
| `src/compiler/dump.go` | token and AST debug dumps |
| `src/compiler/ir_dump.go` | HIR and MIR debug dumps |
| `src/cmd/chaos-lsp/` | JSON-RPC server and editor-facing indexes |
| `examples/e2e_test.go` | full-pipeline golden and executable tests |
