# Change Log

This file records changes to `src_new_impl`. Every group of changes must be
listed under an explicit semantic version.

## Versioning

- The current version is `0.2.0`.
- Every new addition increments the minor version (`0.1.0` → `0.2.0`).
- Compatible fixes that do not add functionality may increment the patch
  version (`0.2.0` → `0.2.1`).
- Each version should describe its additions, changes, fixes, and removals.

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
