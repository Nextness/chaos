# Change Log

This file records changes to `src`. Every group of changes must be
listed under an explicit semantic version.

## Versioning

- The authoritative current development version is `compiler.Version`. This
  file records the released feature history and currently ends at `0.11.0`.
- Every new addition increments the minor version (`0.2.0` → `0.3.0`).
- Compatible fixes that do not add functionality may increment the patch
  version (`0.3.0` → `0.3.1`).
- Each version should describe its additions, changes, fixes, and removals.

## 0.11.0 - 2026-08-13

### Added

- Error handling with `unless catch` and `if ... catch`.
  - `result := expr unless catch [err] { body }` binds the value-or-error
    pair from an error-returning expression and runs the catch body when it
    is an error; the bare `expr unless catch [err] { body }` form discards
    the value. `if result catch [err] { body }` checks a previously bound
    pair. The optional `err` binding holds the error and can be compared
    against error literals (`.GENERIC!`).
  - The catch body must return or exit, so the value is always defined
    afterward; the variable then holds the unwrapped value (flow-sensitive
    typing). Using the value before handling the error is rejected.
  - Error-returning procedures now lower to a value-or-error pair
    (a struct of value, error, and hasError fields) through HIR, MIR, and
    the fasm backend, replacing the previous "not yet supported" rejection.
    A matching pair can be re-raised with `return result;`, and reassigning
    an unwrapped variable wraps the new value.
  - The language server highlights `unless` and `catch` as keywords and the
    catch binding as a variable; definition, references, and hover work on
    the catch binding.

## 0.10.0 - 2026-08-12

### Added

- Error-returning procedure results with the `<>` token:
  `-> Type <> ErrorType` (or `-> (Type <> ErrorType)`) declares that a
  procedure returns a value but can also return an error that the caller
  must handle. `Some_Error <> String` and `String <> Some_Error` are
  equivalent; the front end validates that the error side is a declared
  error type and accepts both value and error returns. Error handling is
  not implemented yet: lowering rejects error-returning procedures with a
  "not yet supported" diagnostic.
- Error literals now require a trailing `!`: `Some_Error.GENERIC!` and
  `.GENERIC!`. Variables holding error values do not use the bang.

### Changed

- Error member references without `!` are rejected by the type checker
  ("error values must be instantiated with '!'").

## 0.9.0 - 2026-08-12

### Added

- Error types and error values. An error type is declared with
  `Name :: error { MEMBER; ... }` (or equivalently `Name : Error : error {...}`);
  each member is a distinct error value numbered sequentially from 0.
  - Error values are nominal: they assign only to their own error type and
    compare only with `==` and `!=`; ordering and arithmetic are rejected.
  - Members are referenced as `Type.MEMBER` or, in a typed context, as the
    bare `.MEMBER` form (declaration type, call argument, return value,
    comparison against an error-typed operand).
  - Error types are compile-time only: `Name := error {...}` and
    `Name : Error = error {...}` are rejected. Members cannot be assigned
    explicit values (`A = 1;` is an error), and duplicate member names are
    rejected.
  - Error values are backed by a 16-bit ordinal in the fasm backend, so they
    work in globals, locals, comparisons, call arguments, returns, and as
    `exit` status codes.
  - The language server highlights error type names as types, members as
    constants, and the `error` keyword as a keyword; document symbols,
    definition, references, hover, and `Type.` member completion are
    supported.

## 0.8.1 - 2026-08-08

### Fixed

- Negative literals (for example `-100` or `-1.5`) now adapt to a typed
  target like their positive counterparts, in declarations, comparisons,
  returns, parameters, and struct fields.
- Return literals now adapt to the procedure's result type during lowering;
  `return 7;` from an `S128` procedure no longer fails the MIR verifier.
- Unary negation of 128-bit integers now negates both halves (previously
  only the low 64 bits were negated).
- String ordering comparisons (`<`, `>`, `<=`, `>=`) now compare strings
  byte-wise and then by length; previously they behaved like equality.
- Empty string constants are emitted with a reserved byte so fasm accepts
  the data section; previously `db` with no operands failed to assemble.
- F16 and F128 are rejected by the fasm backend with a diagnostic instead
  of being silently emitted as F64 with wrong precision.

## 0.8.0 - 2026-08-08

### Added

- The fasm backend now supports every type and construct the front end
  accepts; the previous "not yet supported" diagnostics are gone.
  - F32: 4-byte float constants, arithmetic, comparisons, negation, and
    argument passing alongside F64.
  - String: values are a pointer and length pair, with byte-wise comparison
    (==, !=), string constants in the data section, and two-register
    argument passing (pointer in one register, length in the next).
  - Structs: values are aggregates of their fields with zeroed padding,
    struct literals, byte-wise comparison, copy semantics, parameters passed
    by address, and results returned through a hidden pointer in RDI (sret).
  - 128-bit integers (S128, U128): low/high half representation, add, sub,
    mul, signed and unsigned div/mod (binary long division), comparisons,
    and two-register argument passing.
  - Global initializers: a synthetic `__global_init` function stores every
    global initializer and is called before the entry procedure. Global data
    is sized per type instead of always eight bytes.
- `exit` now prints its message to stderr before exiting with the status.

### Changed

- `MIRProgram` gained a `GlobalInit` field holding the synthetic global
  initializer function.

## 0.7.0 - 2026-08-08

### Added

- A backend interface (`backend.go`) that consumes a verified MIR program and
  emits target assembly, so multiple backends (fasm, gas, nasm, ...) can
  share the same lowering pipeline.
- A fasm backend (`fasm.go`) that emits flat ELF64 executables using the
  System V AMD64 calling convention. It supports integer types S8-S64 and
  U8-U64 (plus Size and Byte), Bool, and F64, with const, local/global
  load-store, arithmetic, signed and unsigned comparisons, logical
  operators, calls, return, branch/jump, exit, and a process-entry wrapper
  that calls the `#entry` procedure and exits with its result.
- `#entry` tracking: the parser records the entry procedure name on the AST,
  and it is propagated through the HIR and MIR so the backend can select the
  entry point.
- The `chaosc` CLI gained an `-asm` flag that emits fasm assembly for a
  cleanly type-checked program. Program output (dump, IR, assembly) now goes
  to stdout while diagnostics go to stderr.
- Unit tests for the fasm emitter, including runtime tests that assemble
  emitted programs with fasm and check their exit codes (skipped when fasm
  is not installed).

### Changed

- `chaosc`'s `run` function now takes separate writers for diagnostics and
  program output.

## 0.6.0 - 2026-08-08

### Added

- A two-level intermediate representation for the compiler:
  - Typed HIR (`hir.go`): a source-close form with resolved symbols and
    types, structured control flow, and source spans.
  - Three-address CFG MIR (`mir.go`): functions of basic blocks with typed
    values and explicit terminators (jump, branch, return, exit, unreachable).
- Shared IR tables (`ir.go`): `SymbolTable` interns declaration names into
  stable `SymbolID`s (distinct across shadowing); `TypeTable` interns
  built-in and struct types into `TypeID`s.
- AST-to-HIR lowering (`lower.go`): resolves names and types, infers
  expression types using the type checker's rules (including literal
  adaptation), and covers every construct the front end supports: procs,
  globals, structs, var decls, assignments, return, exit, if/elif/else,
  calls, and struct literals.
- HIR-to-MIR lowering (`lower_mir.go`): lowers structured control flow into
  basic blocks, assigns locals in declaration order, and reorders struct
  literal fields to declaration order.
- `VerifyMIR` (`ir_verify.go`): checks that every block has a terminator,
  every value use is defined, local/global accesses reference declared
  slots, instruction argument types match, and returns match the function
  signature.
- `DumpHIR` and `DumpMIR` (`ir_dump.go`) for human-readable inspection.
- The `chaosc` CLI gained an `-ir` flag that lowers a cleanly type-checked
  file and prints the HIR and MIR (after running the verifier).
- Unit tests for the IR tables, both lowering passes, the verifier, and the
  dumps.

## 0.5.0 - 2026-08-06

### Added

- Every diagnostic now carries a `Suggestion` describing how to fix the
  problem. The tokenizer and parser attach a suggestion to each error.

### Changed

- Replaced `log/slog` output with compact, rust-like printf diagnostics.
  Each diagnostic renders as `[ERROR] line:col:file - reason` followed by
  the source line, a caret underline aligned to the error span, and a
  `-> suggestion` line. The suggestion is still emitted when the span has
  no source context (for example at EOF).
- Removed the `-log-level`, `-log-format`, and `-log-source` flags and the
  `logging.go` module. The driver writes diagnostics and driver errors
  directly to stderr with `fmt.Fprintf`.
- Removed the token debug logging from the driver; a successful compile now
  produces no output.
- `Diagnostic.Render` and `RenderAll` now write to an `io.Writer` given a
  `*SourceFile` instead of emitting slog records.
- `DiagnosticList.Error` and `DiagnosticList.Warn` now require a suggestion
  argument.

## 0.4.1 - 2026-07-16

### Fixed

- Parser no longer hangs on bare `proc` inside a block body (e.g., `main ::
  proc { proc }`). The `syncStmt` set no longer includes `TkProc`, and
  `parseBlock` has a real progress guard that consumes a token when
  `parseStmt` returns nil without advancing.
- Binary operators are now left-associative: `10 - 3 - 2` parses as `(10 -
  3) - 2` instead of `10 - (3 - 2)`. The right-operand binding power is
  `bp + 1`.
- Chained calls as expression statements (e.g., `f()();`) now parse
  correctly.
- `else if` (as opposed to `elif`) now produces an error diagnostic instead
  of a warning. The parser still constructs a synthetic `BlockStmt` for error
  recovery.
- Trailing commas in parameter lists (e.g., `(x: S64,)`) and call arguments
  (e.g., `f(1,)`) now produce error diagnostics instead of being silently
  accepted.
- Malformed procedure result lists (e.g., `-> S64,`) now emit a diagnostic
  for the missing type after the comma instead of silently accepting the
  declaration.
- Tokenizer now rejects underscores immediately before the decimal point
  (e.g., `1_.0`) and floats with no digit after the decimal point (e.g.,
  `1.`).
- `SourceManager.Lookup` now guards against negative `FileID` values,
  returning `nil` instead of panicking.
- `Param` no longer implements the `Expr` interface (it is only a `Node`).
- `Program` now implements `Node` with a `nodeSpan()` method.
- `ExprStmt` moved from `parser.go` to `ast.go` alongside the other AST
  types.
- `parseTestCase` in parser tests now merges tokenizer diagnostics into the
  result, so lexical errors do not silently pass parser tests.

### Changed

- Updated `AGENTS.md` and `CHANGELOG.md` to reflect current coverage (87.6%),
  parser availability, source counts, and the legacy-compiler scope of
  `report.md`.

## 0.4.0 - 2026-07-16

### Added

- AST node types (`ast.go`): `Expr`, `Stmt`, `Decl` interfaces with concrete
  structs for all language constructs: `IdentExpr`, `IntExpr`, `FloatExpr`,
  `StringExpr`, `BoolExpr`, `BinaryExpr`, `UnaryExpr`, `CallExpr`,
  `ParenExpr`, `ErrorExpr`, `VarDecl`, `AssignStmt`, `ReturnStmt`,
  `ExitStmt`, `IfStmt`, `BlockStmt`, `ProcDecl`, `Param`, `ExprStmt`, and
  `Program`. Every node carries a `Span` for source-location diagnostics.
  Binary and unary operators are typed enums (`BinaryOp`, `UnaryOp`).
- Recursive-descent parser with a correct Pratt expression parser
  (`parser.go`). Supports the full grammar subset:
  - Variable declarations (`::`, `:=`, `: T`, `: T = expr`)
  - Procedure declarations with parameters, return types, and body
  - Assignments, return (with and without value), exit (with optional message)
  - If/elif/else conditionals
  - Full expression support: integers, floats, strings, booleans,
    identifiers, unary (`-`, `!`), binary (arithmetic, comparison, logical),
    parenthesized groups, and function calls with full-expression arguments
  - Error recovery with diagnostic collection (no panics)
- `ExprStmt` node type for expression-position statements (e.g., bare calls
  used as statements).
- `spanUnion` helper for computing the union of two `Span` values.
- `tokenPrecedence` and `tokToBinaryOp` mapping functions.
- Unit tests for all AST node types, type assertions, span accessors,
  operator enumeration, `spanUnion`, `tokToBinaryOp`, `tokenPrecedence`.
- Comprehensive parser tests covering: empty input, all declaration forms,
  all statement forms, expression literals, binary operators, unary operators,
  function calls with full-expression arguments, operator precedence
  (including the old-compiler regression test), paren grouping, chained
  calls, if/elif/else, nested blocks, error recovery, stray semicolons, and
  span correctness.

### Changed

- The compiler pipeline now runs the parser after tokenization. Parse
  diagnostics are rendered and cause exit code 1 on errors.
- The `Parser` struct replaces the old compiler's `ChaosSlice` cursor
  pattern with a simple `tokens`/`pos`/`diags` model.

## 0.3.0 - 2026-07-16

### Added

- Comprehensive unit tests for diagnostic severity mapping, diagnostic lists,
  line-offset calculation, source coordinates, structured rendering, caret
  alignment, span clamping, and multi-diagnostic output.
- Unit tests for source registration and lookup, token-kind names and invalid
  values, raw token text, and keyword lookup.
- Regression tests for numeric dot sequences, including `1...0`, `1..0`,
  consecutive floats, and misplaced underscores after a decimal point.
- Boundary tests for tokenizer lookahead, EOF handling, rune decoding,
  identifier checks, and zero-padded unknown-byte diagnostics.
- Tests for invalid log levels, text-handler attributes, CLI help, invalid
  flags, invalid logging configuration, argument validation, missing files,
  and tokenizer failures in the compiler driver.
- Coverage-profile verification. The package now has 99.1% statement coverage,
  exceeding the 85% target.

## 0.2.0 - 2026-07-16

### Added

- Diagnostic error for `..` (two dots) in numeric literal context, pointing
  at the second dot and its following digits (e.g., `1..0` flags `.0`).
- `TokenKind.String()` now panics with the numeric value instead of silently
  returning `"unknown"` for out-of-range values, catching mismatch between
  the enum and the name table at the first call site.

### Changed

- `Token.Value` unified from `any` (string or bool) to plain `string`.
  `TkTrue` and `TkFalse` store `"true"`/`"false"` strings instead of `bool`;
  type conversion is deferred to the semantic phase.
- `init()` keyword table builder uses a named local struct type
  `keywordEntry` instead of an anonymous struct literal for readability.
- `emitDouble` renamed to `emitN` to reflect that it handles any length N.
- All single-character token emission now uses `emitN(kind, start, 1)`
  instead of a separate `emitSingle` helper.
- `scanNumber` dot check now verifies three dots (`peekN(2)`) before
  breaking for ellipsis, so `..` is not mistaken for `...`.

### Removed

- `emitSingle` helper function (all call sites replaced with `emitN`).

## 0.1.0 - 2026-07-16

### Added

- A centralized `log/slog` configuration shared by the compiler driver.
- The `-log-level` option for selecting the minimum emitted log level.
- The `-log-format` option for selecting text or JSON output.
- The `-log-source` option for including Go source locations in log records.
- Structured token attributes for file, line, column, kind, span, raw text,
  literal value, and EOF state.
- Structured diagnostic attributes for severity, source location, span, source
  line, and caret underline.
- Tests covering logging configuration, invalid formats, JSON attributes, and
  structured CLI token output.
- This change log and its semantic-version tracking policy.

### Changed

- Token output now uses `slog` records instead of formatted standard output.
- File-read and command-line errors now use structured `slog` error records.
- Compiler diagnostics now map their severity to the corresponding `slog`
  level and emit through the configured logger.
- The compiler driver now returns explicit exit codes from a testable `run`
  function.
- Diagnostic and tokenizer string construction no longer relies on
  `fmt.Sprintf`.

### Removed

- Direct `fmt.Printf`, `fmt.Fprintf`, and `fmt.Sprintf` usage from
  `src_new_impl`.
- Hard-coded ANSI color output from diagnostic rendering.
