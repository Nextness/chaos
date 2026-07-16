# Change Log

This file records changes to `src_new_impl`. Every group of changes must be
listed under an explicit semantic version.

## Versioning

- The current version is `0.4.1`.
- Every new addition increments the minor version (`0.2.0` → `0.3.0`).
- Compatible fixes that do not add functionality may increment the patch
  version (`0.3.0` → `0.3.1`).
- Each version should describe its additions, changes, fixes, and removals.

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
