> **Legacy audit:** this document describes the removed pre-rewrite compiler.
> It is retained only as project history. See [`PLAN.md`](../PLAN.md) and the
> repository [`README.md`](../README.md) for the active compiler.

# Chaos compiler: implementation map, defect audit, and roadmap

Audit date: 2026-07-15  
Scope: the current working tree under `src/` and `language_design/`, plus the entrypoints, build driver, and tests needed to explain how those directories are used. The request named `langauge_design`; that directory does not exist, so this report uses the repository's `language_design/` directory.

This is an implementation audit, not a description of the intended language. A construct is called “implemented” only when the active compiler pipeline can lex, parse, validate, and emit meaningful code for it. The files under `language_design/` are syntax and library sketches; they are not accepted programs.

## 1. Executive result

Chaos is currently a partial front end followed by a stub backend. It can tokenize and build ASTs for a small language subset, but it cannot yet compile a Chaos program into a meaningful runnable executable.

| Stage | Current state | Consequence |
|---|---|---|
| Build/bootstrap | Go compiler and bootstrap driver build on Linux | The compiler executable can be produced. |
| Lexer | Broad but fragile token coverage | Basic ASCII programs with trailing delimiters tokenize; malformed/EOF cases can panic. |
| Parser | Variables, calls, procedures, expressions, `if` branches, `exit`, and value-returning `return` are partially parsed | Precedence, scope, calls, floats, unsupported tokens, and malformed blocks contain correctness bugs. |
| Name resolution | Performed ad hoc during parsing with `[]Scope` | No forward references or recursion; no symbol identities; shadowing and const rules are not enforced. |
| Type checking | Only selected top-level identifiers and `exit` nodes | Procedure bodies and binary expressions are not checked; several valid-looking declarations fail. |
| Compile-time execution | Not implemented | `::` only changes `Node.Reassignable`; it does not evaluate code. |
| IR | Dormant six-op tree structure | `ProgramToIR` is never called and cannot represent most AST nodes. |
| Code generation | Stub | `ChaosCodeGen` emits only fasm source directives and a label, then prints AST JSON. |
| Assembly/run | Broken wiring | Output names disagree, the emitted label has no instructions, and `-r` assembles one path/name then runs another. |
| Tests | Compile-smoke files with an unreliable runner | Two “ok” fixtures fail; two others pass because important semantic work is skipped. |

The source separation—lexer, AST, semantic pass, IR, codegen—is a sensible high-level decomposition. The data flow between those stages does not yet have stable contracts. Parsing mutates semantic scopes, type checking mutates duplicate AST/scope views, the IR is bypassed, and codegen consumes the AST while doing no lowering.

### Corrections to the previous report

The earlier version of this file contained several material inaccuracies:

- `src/chaosCodegen.go:14-17` writes fasm source text, not an ELF64 binary header.
- `tokInfer` is an internal marker placed in `VarDecl.Type` at `src/chaosAst.go:537-539`. The lexer never emits it; source `:=` is tokenized as `tokColon` followed by `tokAssignment`.
- Float text is parsed into a Go `float64` at `src/chaosLexer.go:621-630`, but `ChaosContentParseBaseNode` turns every `tokNumberLiteral` into `nodeIntLiteral` at `src/chaosAst.go:191-197`. `nodeFloatLiteral` is unreachable.
- `warning` prints its colored banner once but prints every warning reason (`src/chaosUtils.go:68-76`); it does not suppress all later warnings.
- `NodeTypeToString` does not panic for `nodeNoOp`: its final `assert` condition is true and therefore returns the zero string (`src/chaosAst.go:114-147`).
- The Pratt-parser issue is not merely suspected. It consumes the operator before deciding whether to stop and tests equality rather than `lbp < minBp` (`src/chaosAst.go:288-294`), producing incorrect associativity/precedence.
- The current tree contains 2,592 lines under `src/` and 2,094 lines under `language_design/`, including the `.docs` file—not approximately 3,000 and 1,300.

## 2. Repository and evidence

### 2.1 Relevant files

| Path | Lines | Active role |
|---|---:|---|
| `src/chaos.go` | 63 | Compiler CLI and pipeline coordinator |
| `src/chaosLexer.go` | 813 | Bytes to tokens |
| `src/chaosAst.go` | 698 | Tokens to AST plus parse-time name lookup |
| `src/chaosInferenceCheckType.go` | 315 | Partial inference/type checks |
| `src/chaosIr.go` | 155 | Unused IR experiment |
| `src/chaosCodegen.go` | 278 | 19 active lines of fasm preamble/debug plus 249 lines of obsolete commented codegen |
| `src/chaosSlice.go` | 134 | Generic cursor wrapper |
| `src/chaosUtils.go` | 136 | Assertions, diagnostics, casts, debug helpers |
| `chaosBuild.go` | 182 | Bootstrap, compiler build, smoke-test loop, fasm invocation |
| `tests/*.chaos` | 62 total | Six compile-smoke fixtures |
| `language_design/*` | 2,094 total | Twenty-eight `.chaos` design sketches plus one `.docs` sketch |

`go.mod:1-3` declares module `chaos`, Go 1.24.1, with no external dependencies. `README.md:14` states the self-hosting objective. `README.md:27-31` shows `#entry`, `print`, and `#import` syntax that the active parser does not implement.

### 2.2 Validation performed

The following checks were run against the current working tree:

- `go build -o /tmp/chaosc-audit ./src/`: passes with Go 1.24.3.
- `go vet ./src/...`: passes. Go vet does not detect the compiler's semantic logic defects.
- Each `tests/*.chaos` file was executed directly with the built compiler from `/tmp` so its hardcoded `main.asm` did not alter the repository.

Observed fixture results:

| Fixture | Intended by name | Actual exit | What the result proves |
|---|---|---:|---|
| `tests/assignments_failure.chaos:1` | failure | 1 | Undefined-name rejection occurs during parsing. |
| `tests/assignments_ok.chaos:1-18` | success | 1 | Uninitialized declarations/reassignments are not supported by the type pass; it reports “Unexpected node”. |
| `tests/bin_ops_failure.chaos:1` | failure | 1 | Undefined operands are rejected during parsing. |
| `tests/bin_ops_ok.chaos:1-10` | success | 0 | It reaches codegen only because binop checking is explicitly skipped at `src/chaosInferenceCheckType.go:220-223`. |
| `tests/procs_ok.chaos:1-25` | success | 1 | The final standalone call becomes `nodeCall`, which the top-level type loop rejects at `src/chaosInferenceCheckType.go:312-313`. |
| `tests/scopes_ok.chaos:1-7` | success | 0 | The entire procedure assignment is skipped at `src/chaosInferenceCheckType.go:225-230`; its locals, condition, anonymous scope, and return are never validated. |

In particular, `tests/procs_ok.chaos:2-8` declares `-> S64` and returns `variable_4`, whose inferred type should be `String`. The file only fails later on the standalone call, not on that return mismatch.

## 3. Exact active call order

### 3.1 Bootstrap/build path

Running `./chaosBuild ...` enters `chaosBuild.go:106`:

1. `main` calls `goRebuildYourSelfTek` (`chaosBuild.go:107`).
2. `goRebuildYourSelfTek` calls `touchFile` for `./chaosBuild.go` and `./chaosBuild` (`chaosBuild.go:56-71`). If the source ctime is newer, it renames/removes the binary, calls `runCommand("go build ./chaosBuild.go")`, and recursively checks again (`chaosBuild.go:71-85`).
3. `main` parses `-default`, `-c`, `-r`, and `-test` (`chaosBuild.go:109-114`).
4. `MakeDirIfNotExist("./build")` creates the build directory (`chaosBuild.go:116`).
5. `runCommand("go build -o ./build/chaosc ./src/")` builds the active compiler (`chaosBuild.go:118-125`).
6. Exactly one later branch is normally used:
   - `-test`: `os.ReadDir` → loop → `runCommand("./build/chaosc tests/<name>")` → unconditional exit 0 (`chaosBuild.go:127-142`).
   - `-default`: compile `main.chaos` (`chaosBuild.go:145-152`).
   - `-c file`: compile the supplied path (`chaosBuild.go:154-159`).
7. `-r` derives an asm name, invokes `fasm`, chmods `testing`, then attempts to execute a different name (`chaosBuild.go:161-180`).

`runCommand` itself wraps every command string in `bash -c` (`chaosBuild.go:17-38`), so filenames are neither quoted nor structurally separated into argv elements.

### 3.2 Compiler path

Running `./build/chaosc file.chaos` enters `src/chaos.go:32`:

1. `main` defers `panicHandler` (`src/chaos.go:33`).
2. For every CLI argument, it validates the `.chaos` suffix, stats and reads the file, then calls `compileChaos(arg, src)` (`src/chaos.go:42-55`).
3. `compileChaos` executes this active pipeline (`src/chaos.go:10-29`):

```text
[]byte
  -> bytes.NewBuffer
  -> ChaosContentTokenize
  -> ChaosContentAST
  -> ChaosInferAndCheckType       (mutates AST/scope objects)
  -> ChaosCodeGen                 (does not call ProgramToIR)
  -> os.Create("main.asm")
  -> bytes.Buffer.WriteTo(file)
```

The `filepath` parameter to `compileChaos` is unused (`src/chaos.go:10`), so source identity is lost and every input overwrites `main.asm`.

### 3.3 Parser call graph

`ChaosContentAST` (`src/chaosAst.go:660-688`) repeatedly calls `ChaosContentASTParseStatement`:

```text
ChaosContentAST
  -> ChaosContentASTParseStatement
       -> ChaosContentParseScope                  for "{"
            -> ChaosContentASTParseStatement      recursively
       -> ChaosContentParsePrimaryExpression      for identifier-led statements
            -> checkIdentifierInScope
            -> ChaosContentParseExpression
                 -> ChaosContentParseBaseNode
                      -> checkIdentifierInScope
                      -> ChaosContentParseBaseNode recursively for call args
                 -> infixBindingPower
                 -> ChaosContentParseExpression   recursively for RHS
            -> ChaosContentParseProcDefinition
                 -> MakeType("Void")
                 -> ChaosContentParseScope
       -> ChaosContentParseExit
            -> ChaosContentParseExpression
       -> ChaosContentParseConditionalBranches
            -> ChaosContentParseConditionalIf
                 -> ChaosContentParseExpression
                 -> ChaosContentParseScope
            -> ChaosContentParseConditionalElif   recursively
            -> ChaosContentParseConditionalElse
       -> ChaosContentParseExpression             after "return"
```

Name lookup therefore occurs while the tree is still being built. This prevents clean separation between parsing, declaration collection, name resolution, and type checking.

### 3.4 Type-pass call graph

`ChaosInferAndCheckType` (`src/chaosInferenceCheckType.go:296-315`) iterates only `program.data`:

```text
ChaosInferAndCheckType
  -> InferAndTypeCheckIdentifier   when NodeType == nodeIdentifier
       -> MakeType / MakeTypeList
       -> TypeCheckLiterals
       -> warning                   for binop/proc RHS
  -> TypeCheckExit                 when NodeType == nodeExit
       -> MakeTypeList
       -> TypeCheckLiterals
  -> assert(false)                 for every other top-level node type
```

There is no recursive call into `Proc.Scope`, `Conditions.Scopes`, or `AnonymousScope`. There is no expression checker. The program cursor is left at `program.count`, although codegen bypasses the cursor and ranges over `program.data`.

### 3.5 Intended but disconnected backend path

`ProgramToIR` (`src/chaosIr.go:30-155`) recursively lowers selected node shapes, but nothing calls it. `ChaosCodeGen` ranges over AST nodes directly, calls `chaosDebug`, and returns a buffer containing only:

```asm
format ELF64 executable 3

entry main

segment readable executable

main:
```

The old `GenerateCode` at `src/chaosCodegen.go:30-278` is fully commented and expects a removed `Program` model with fields such as `AllocatedProcs`, `AllocatedVars`, and `Nodes` (`src/chaosCodegen.go:43,88,117`). Uncommenting it is not a restoration strategy.

## 4. Complete active-source symbol map

### 4.1 `src/chaos.go`

| Symbol | Lines | Calls / state |
|---|---:|---|
| `compileChaos(filepath, src) bool` | 10-30 | Calls all four active stages; ignores `filepath`; always returns true; writes `main.asm`. |
| `main()` | 32-63 | Defers `panicHandler`, reads every CLI argument, calls `compileChaos` once per file. Multiple inputs overwrite the same output. |

### 4.2 `src/chaosLexer.go`

| Symbol | Lines | Purpose |
|---|---:|---|
| `TokenType` and constants | 11-54 | 38 real token values plus `tokCount` sentinel. |
| enum-count assertion | 56-59 | Executes at package initialization through `assert`. |
| `TokenTypeToString` | 61-143 | Manual enum string conversion; fallback incorrectly asserts an already-true invariant. |
| `Position` | 145-147 | Zero-based line/column; fields are unexported and disappear from JSON debug output. |
| `Token` | 149-155 | `Symbol`, `TokenType`, `Position`, `Length`, and dynamically typed `Value`. |
| `ChaosContentTokenize` | 157-813 | Single cursor loop; calls `CSMatch`/`CSGet`/`CSConsume`, character helpers, strconv, and `assert`. |

Token coverage and parser use:

| Category | Tokens | Active parser status |
|---|---|---|
| Assign/declaration | `=`, `:` | Used. `::` and `:=` are two-token sequences. |
| Arithmetic | `+`, `-`, `*` | Binary forms parsed; no unary forms, division, remainder, shifts, or bitwise operators. |
| Comparison/logical | `< > <= >= == !=`, logical AND, logical OR | Parsed into `BinOpOperation`; never type checked or generated. |
| Delimiters | `( ) { } , ;` | Used. Braces are named “Braket” in constants. |
| Literals | number, `«string»`, `true/false` | Used, but every number becomes an integer AST node. |
| Keywords | `exit if elif else proc then return` | Partially used. |
| Lexed but not meaningfully parsed | `... as # !` | A top-level occurrence is often skipped token-by-token; inside a scope it can hang parsing. |
| Never emitted | `tokNewline`, `tokInfer` | Newlines are discarded; `tokInfer` is synthesized in AST declarations. |

### 4.3 `src/chaosAst.go`

Data model:

| Symbol | Lines | Fields / meaning |
|---|---:|---|
| `NodeType` | 7-25 | 14 values: null, no-op, four literals, identifier, exit, binop, proc, call, conditions, anonymous scope, return. |
| `BinOpOperation` | 27,30-44 | Null/no-op plus arithmetic, comparisons, `or`, and `and`. |
| `Scope []Node` | 28 | A list of syntax nodes, not a symbol table or lexical-scope object. |
| `VarDecl` | 46-51 | Name token, type token, assignment node, initialized flag. |
| `Exit` | 53-56 | Status expression and optional message expression. |
| `Literal` | 58-62 | Separate Int/String/Boolean token fields; no Float field. |
| `BinOp` | 64-68 | Operation plus value-copied lhs/rhs nodes. |
| `Proc` | 70-75 | Body scope, input nodes, output nodes, arity. Parser always creates one output, including `Void`. |
| `Call` | 77-81 | Name, input nodes, arity. |
| `Conditions` | 83-87 | Redundant count, `[]BinOp` evaluations, and branch node slices. |
| `Return` | 89-91 | Slice of outputs, although parser creates exactly one. |
| `Node` | 93-107 | Tagged union with many nullable pointers plus flags `Reassignable`, `Reassigned`, and misspelled `Infered`. |

Functions:

| Function | Lines | Direct role / callees |
|---|---:|---|
| `NodeTypeToString` | 114-148 | Manual kind names; duplicates `nodeNull` test at line 117 and omits `nodeNoOp`. |
| `checkIdentifierInScope` | 150-166 | Searches scopes from `depth` outward. Treats prior `nodeCall` names as definitions. |
| `infixBindingPower` | 168-186 | Returns float binding powers; called by expression parser. |
| `ChaosContentParseBaseNode` | 188-249 | Parses literals, identifiers, and expression-position calls. Call arguments recursively use only base nodes. |
| `ChaosContentParseExpression` | 251-306 | Pratt-like recursive parser; handles grouping and binops. |
| `ChaosContentParseScope` | 308-347 | Requires `isFunction`; parses braced or single-statement/`then` scopes. |
| `ChaosContentParseConditionalIf` | 349-358 | Forces expression result into `*cond.BinOp`. |
| `ChaosContentParseConditionalElif` | 360-373 | Same condition restriction; recursively collects chained `elif`. |
| `ChaosContentParseConditionalElse` | 375-385 | Stores `opNoOp` as an else-condition sentinel. |
| `ChaosContentParseConditionalBranches` | 387-402 | Allocates `Conditions` then calls if → elif → else parsers. |
| `ChaosContentParseProcDefinition` | 404-477 | Hardcoded parameter/return parsing; makes implicit `Void`; adds inputs to a temporary scope; parses body. |
| `ChaosContentParseExit` | 479-501 | Parses status and optional message as expressions; consumes semicolon. |
| `ChaosContentParsePrimaryExpression` | 503-615 | Parses reassignment, five declaration shapes, proc declarations, and statement-position calls. |
| `ChaosContentASTParseStatement` | 617-658 | Dispatches block, identifier, exit, if, return. Unsupported token returns `nodeNoOp` without consuming it. |
| `ChaosContentAST` | 660-688 | Creates global scope and program; it alone consumes top-level no-op/unsupported tokens. |

### 4.4 `src/chaosInferenceCheckType.go`

| Symbol | Lines | Role / limitations |
|---|---:|---|
| `warnOnce` | 5 | Unused global. |
| `MakeType` | 7-12 | Encodes a type as a `tokIdentifier` token. |
| `MakeTypeList` | 14-23 | Recreates lists of string-named type tokens. |
| `TypeCheckLiterals` | 25-34 | String equality membership test. |
| `InferAndTypeCheckIdentifier` | 36-233 | Handles literal, identifier, and call RHS; skips binop and proc RHS; assumes nullable union fields are valid. |
| `TypeCheckExit` | 235-294 | Allows any integer spelling for status and String for message; only literal/identifier expressions. |
| `ChaosInferAndCheckType` | 296-315 | Flat top-level dispatcher for only identifier/exit nodes. |

Type spellings exist only as string comparisons at `src/chaosInferenceCheckType.go:62-64,95,126,241-243`: `S128/S64/S32/S16/S8`, `U128/U64/U32/U16/U8`, `String`, `Bool`, and parser-created `Void`. There is no type declaration registry, no invalid-type diagnostic, no size/alignment information, and no `F*` support.

### 4.5 `src/chaosIr.go`

| Symbol | Lines | Role |
|---|---:|---|
| `OpType` | 3-14 | `irNull`, `irNoOp`, `irExit`, `irOpCall`, `irAutoVar`, `irProcDef`. |
| `IROp` | 16-28 | One wide tagged record for symbols, primitive values, proc children, and exit children. |
| `ProgramToIR` | 30-155 | Recursive selected-node conversion; unused by pipeline and asserts on most node kinds. |

`IROp` loses call arguments, procedure outputs, return values, control flow, binary operators, scopes, source spans, mutability, compile-time/runtime distinction, and typed temporaries.

### 4.6 `src/chaosCodegen.go`

| Symbol | Lines | Role |
|---|---:|---|
| `entryPoint = "main"` | 8 | Hardcoded assembly entry label; unrelated to any source `#entry` declaration. |
| `ChaosCodeGen` | 10-28 | Writes fasm preamble/label, prints every AST node as JSON, ignores `scope`. |
| commented `GenerateCode` | 30-278 | Obsolete code for removed `Program` structure; partial globals, calls, exit, addition, and signed less-than. |

### 4.7 `src/chaosSlice.go`

| Symbol | Lines | Calls / purpose |
|---|---:|---|
| `ChaosSlice[T]` | 6-10 | Stores `data`, redundant `count`, and mutable `cursor`. |
| `CSUint8Uint8Comparison` | 13-17 | Compares a byte to only the first byte of a `[]byte`. |
| `CSStringUint8Comparison` | 19-23 | Compares a byte to the first byte of a string. |
| `CSTokenTypeComparison` | 25-29 | Token-kind matcher. |
| `CSNodeNodeTypeComparison` | 31-35 | Node-kind matcher. |
| `CSTokenTypeAssert` | 38-48 | Token kind assertion with stringified diagnostic. |
| `CSNodeTypeAssert` | 50-60 | Node kind assertion. |
| `CSCheckBounds` | 63-68 | Checks only upper bound, not negative indexes. |
| `CSGet` | 70-83 | Peek at cursor plus optional offset. |
| `CSGetPointer` | 85-98 | Pointer to current slice element, used for mutation in type pass. |
| `CSMatch` | 100-110 | Repeated peeks; has no safe “remaining length” precheck. |
| `CSConsume` | 112-125 | Returns current element and advances by optional amount. |
| `CSConsumeAssert` | 127-134 | Consumes and asserts a sequence, returns its last element. |

### 4.8 `src/chaosUtils.go`

| Symbol | Lines | Role / issue |
|---|---:|---|
| `debug` | 13 | Hardcoded true. |
| `debugCall`, `triggerCustomPanic`, `printWarningOnce` | 15-17 | Process-global diagnostic state. |
| `isNum/isAlpha/isAlphanum` | 19-32 | Applies Unicode predicates to individual bytes, so it is not actually Unicode-aware. |
| `getFrames` | 34-40 | Captures at most 20 stack frames. |
| `customPanic` | 42-66 | Prints selected stack frames. |
| `warning` | 68-78 | Prints one banner and every reason. |
| `assert[T]` | 80-91 | On success returns zero `T`; on failure prints and panics. |
| `panicHandler` | 93-101 | Exits 1 only when `debug` is true; with debug false it would recover silently. |
| `chaosDebug` | 103-113 | JSON-indents values and always prints to stdout. |
| `todo[T]` | 115-119 | Debug-prints and calls `os.Exit(1)`, bypassing defers. |
| `cast/castAssert` | 121-131 | Runtime assertions around `any` values. |
| `almostEqual` | 133-136 | Epsilon comparison used for parser binding powers. |

### 4.9 `chaosBuild.go`

| Symbol | Lines | Role / issue |
|---|---:|---|
| `runCommand` | 17-39 | Linux `bash -c` wrapper; non-Linux path silently returns nil. |
| `touchFile` | 41-54 | Returns Linux inode ctime, not modification time; hard-casts `Stat_t`. |
| `goRebuildYourSelfTek` | 56-87 | Self-rebuild loop; requires existing binary and uses a newline in backup filename. |
| `MakeDirIfNotExist` | 89-102 | Single-level mkdir; exits on failure. |
| `DEBUG` | 104 | Hardcoded true. |
| `main` | 106-182 | Rebuild/build/test/compile/assemble orchestration. |

## 5. What source syntax is implemented today

### 5.1 Declarations and reassignment

`ChaosContentParsePrimaryExpression` implements these token patterns (`src/chaosAst.go:512-581`):

| Source | AST intent | Current semantic result |
|---|---|---|
| `x :: expr;` | inferred, initialized, non-reassignable | Parses. Literal/identifier/call may infer at top level; const value is not evaluated. |
| `x := expr;` | inferred, initialized, reassignable | Parses as `:` then `=`. |
| `x : T : expr;` | typed, initialized, non-reassignable | Parses; `T` is not validated as a declared type. |
| `x : T = expr;` | typed, initialized, reassignable | Parses; literal type spelling is compared later. |
| `x : T;` | typed, uninitialized, reassignable | Parses but fails the top-level type pass because RHS is `nodeNull`. |
| `x = expr;` | reassignment | Parses, but does not consult the original declaration's `Reassignable` and omits the destination type; top-level checking normally fails. |

The comments in the agent guide call `::` compile-time and `:=` runtime assignment. The implementation only records mutability on the declaration node (`src/chaosAst.go:534,559`); neither storage duration nor evaluation stage is represented.

### 5.2 Expressions

Active atoms are number, string, boolean, existing identifier, call, and parenthesized expression (`src/chaosAst.go:188-258`). Active binary operators are `+ - * < > <= >= == != || &&` (`src/chaosAst.go:260-303`).

Not implemented: unary `-`/`!`, division/remainder, bitwise operators, shifts, indexing, member access, address/dereference, casts despite lexed `as`, arrays, struct literals, ranges, assignment expressions, ternary/`ifx`, and short-circuit semantics.

Both expression-position calls (`src/chaosAst.go:226-245`) and statement-position calls (`src/chaosAst.go:584-607`) parse each argument with `ChaosContentParseBaseNode`. Thus `f(1 + 2)` cannot be parsed as one argument. Commas are optional in the loop, so malformed `f(1 2)` is accepted as two base-node arguments.

### 5.3 Procedures, calls, scopes, and control flow

Implemented parser shape:

```chaos
name :: proc (a: S64, b: Bool) -> S64 {
    local := a;
    if local < 2 then exit 1, «message»;
    return local;
}
```

Limitations:

- Procedures are only recognized on the compile-time/second-colon declaration path (`src/chaosAst.go:556-565`), normally written `name :: proc`; they are not general expressions.
- Parameters must be simple `name: IdentifierType` pairs (`src/chaosAst.go:420-443`).
- The parser records exactly one output. Missing `->` creates one `Void` output (`src/chaosAst.go:449-468`).
- Calls do not validate callee kind, arity, argument types, or result count.
- `return` always requires an expression (`src/chaosAst.go:641-652`); `return;` cannot parse, even for `Void`.
- `if` is rejected unless parser depth is greater than 1 (`src/chaosAst.go:634-638`), effectively restricting it to procedure/nested bodies.
- Conditions must produce `nodeBinOp` because the parser dereferences `cond.BinOp` (`src/chaosAst.go:352-353,365-366`). `if true` or `if flag` panics.
- Anonymous braces and conditional branches are parsed only in a function context because `ChaosContentParseScope` asserts `isFunction` (`src/chaosAst.go:308-310`).
- No loop, break, continue, switch, defer, labels, or entrypoint directive is active.

### 5.4 Name and scope behavior

`checkIdentifierInScope` (`src/chaosAst.go:150-166`) searches already-built nodes from inner to outer scope. Consequences:

- Use before declaration and forward procedure calls are rejected.
- A procedure is not inserted until its body finishes parsing (`src/chaosAst.go:561-565`), so direct recursion cannot resolve its own name.
- Declarations are inserted before their initializer is parsed (`src/chaosAst.go:575-579`), so self-reference such as `x := x;` resolves even though it is not initialized.
- Duplicate definitions and shadowing are not diagnosed.
- A previous standalone call's name is treated as a declaration (`src/chaosAst.go:160-162`).
- Reassignment only checks that some name exists; it does not resolve to a declaration object or enforce constness.
- `Scope` contains copied syntax nodes rather than `name -> Symbol` entries. Type inference manually tries to update both the current program node and a matching scope copy.

## 6. Defect register

Severity meanings: P0 blocks trustworthy compilation or can hang/crash on ordinary input; P1 makes implemented-looking language behavior incorrect; P2 is maintainability, diagnostics, or future-backend risk.

### 6.1 P0 defects

| ID | Reference | Defect, example, and fix direction |
|---|---|---|
| P0-01 | `src/chaosCodegen.go:10-28` | No instructions are generated. Add an IR-backed backend and always emit a terminating path. |
| P0-02 | `src/chaosIr.go:30-155`; `src/chaos.go:12-15` | IR is disconnected and cannot represent expressions/control flow. Replace or extend it, then make codegen consume it—not raw AST. |
| P0-03 | `src/chaosAst.go:288-294` | Pratt loop consumes an operator before checking precedence and uses equality. For `1 * 2 + 3` the RHS can absorb lower-precedence `+`. Peek operator, stop on `lbp < minBp`, then consume. |
| P0-04 | `src/chaosAst.go:191-197`; `src/chaosLexer.go:621-630` | A `float64` token becomes `nodeIntLiteral`; later `castAssert[int]` can panic. Split int/float tokens or inspect `Token.Value` and add `Literal.Float`. |
| P0-05 | `src/chaosAst.go:321-325,657` | Unsupported/EOF tokens inside a scope return no-op without consumption, so malformed or unclosed scopes can loop forever. Every parser branch must consume or return an error; explicitly stop on EOF with “unclosed block”. |
| P0-06 | `src/chaosSlice.go:63-68,100-109`; `src/chaosLexer.go:169-213,238,609,655` | Lookahead and scanning can read beyond EOF. Examples: final identifier/number without whitespace, line comment without newline, unterminated string/comment. Make `match` return false when insufficient bytes remain and scanners detect EOF. |
| P0-07 | `src/chaosInferenceCheckType.go:296-315` | Semantic traversal ignores procedure/branch/anonymous scopes and rejects standalone calls. Implement a recursive statement and expression checker. |
| P0-08 | `src/chaosInferenceCheckType.go:220-230` | Binops and entire procedure definitions are accepted with warnings only. This is why incorrect “ok” fixtures pass. |
| P0-09 | `src/chaosInferenceCheckType.go:36-233` | Uninitialized declarations and reassignments have unsupported AST shapes. `tests/assignments_ok.chaos` fails. Add declaration, definite-initialization, lvalue, and assignment rules. |
| P0-10 | `src/chaosInferenceCheckType.go:312-313` | Any top-level `nodeCall` aborts; `tests/procs_ok.chaos:25` demonstrates it. |
| P0-11 | `src/chaosAst.go:352-353,365-366` | Bare boolean conditions dereference nil `BinOp`. Store/check an expression node, not a copied `BinOp`. |
| P0-12 | `chaosBuild.go:127-142` | Test failures are printed but the runner exits 0 unconditionally. Track failures, distinguish expected failure, and exit nonzero on mismatch. |
| P0-13 | `src/chaos.go:17`; `chaosBuild.go:162-176` | Run pipeline has three name mismatches: compiler writes `main.asm`; driver expects `<source>.asm`; fasm writes `testing`; driver executes `<source-without-extension>`. Use an explicit artifact object/path throughout. |
| P0-14 | `chaosBuild.go:161-176` | `-default -r` derives `file` from empty `*filename` rather than `defaultFile`, so it looks for `.asm`. |
| P0-15 | `src/chaosIr.go:141-145` | Exit status identifier/int is written into `exitMsg`; int branch checks `Message` instead of `Status`. Correct fields and add IR tests. |
| P0-16 | `src/chaosLexer.go:184-213` | The nested-opener check happens only after an unconditional cursor increment (`206-207`). A nested `/**` beginning at the current cursor—especially immediately after the outer opener—is missed. Check open/close delimiters before advancing. |

### 6.2 P1 defects

| ID | Reference | Defect / impact |
|---|---|---|
| P1-01 | `src/chaosAst.go:114-147` | `nodeNoOp` check repeats `nodeNull`; fallback asserts `nodeCount == 14`, which is true, and returns `""`. Use a switch and `assert(false)`/`panic` for unknown values. |
| P1-02 | `src/chaosLexer.go:139-142` | `TokenTypeToString` has the same always-true fallback pattern. |
| P1-03 | `src/chaosLexer.go:608-618` | Numeric loop condition excludes underscore; `1_000` stops at `_` and then errors, despite dead code intended to omit underscores. |
| P1-04 | `src/chaosLexer.go:609-623` | Multiple dots are consumed into one number and rejected late; numeric token `Length` remains zero. |
| P1-05 | `src/chaosLexer.go:19-32` | Unicode classification is performed per UTF-8 byte. Non-ASCII identifiers are not reliably accepted. Decode runes or intentionally specify ASCII. |
| P1-06 | `src/chaosLexer.go:223-251` | String scanning does not update line/column and compares only the first byte of the multibyte guillemet. Following diagnostic positions are wrong. |
| P1-07 | `src/chaosLexer.go:184-213` | Block-comment ordinary characters do not increment column; unterminated comments panic through bounds checks. |
| P1-08 | `src/chaosAst.go:226-245,584-607` | Call args are base nodes, expressions are unsupported, and commas are not required. |
| P1-09 | `src/chaosAst.go:512-527` | Reassignment sets `Reassignable=true` on the new assignment node rather than checking the declaration; constants can be reassigned syntactically. |
| P1-10 | `src/chaosAst.go:150-166,575-579` | Parse-time lookup permits use-before-initialization and blocks recursion/forward references. |
| P1-11 | `src/chaosAst.go:404-477` | Proc parameter/output parsing is hardcoded, accepts arbitrary type identifiers, cannot express multiple/optional/error returns, and cannot parse variadics although `...` is lexed. |
| P1-12 | `src/chaosAst.go:641-652` | Empty `return;` is unsupported, conflicting with `Void` and many design examples. |
| P1-13 | `src/chaosInferenceCheckType.go:176-216` | Callee lookup assumes any same-named identifier has `Assignment.Proc`. Calling a variable can nil-dereference. No arity/arg checks exist. |
| P1-14 | `src/chaosInferenceCheckType.go:200-207` | Explicitly typed call result indexes `Outputs[0]` without modeling zero/multiple results. Parser's synthetic Void masks the zero-result case. |
| P1-15 | `src/chaosInferenceCheckType.go:138-198` | The source/destination update loops depend on scope order and `continue`; duplicate names can update the wrong node. |
| P1-16 | `src/chaosInferenceCheckType.go:43-135` | Integer inference always selects `S64` with no literal range/sign check; `strconv.Atoi` is host-`int`-sized and cannot support declared S128/U128. |
| P1-17 | `src/chaosInferenceCheckType.go:235-291` | `exit` rejects binop/call status/message even if their eventual types are valid; scope loops assume `VarDecl != nil`. |
| P1-18 | `src/chaosCodegen.go:8,14-17` | Hardcoded assembly `main` has no relation to source `#entry`, and no duplicate/missing-entry validation exists. |
| P1-19 | `src/chaos.go:18-23` | `defer file.Close()` is registered before checking `err`; a failed create can lead to a nil receiver during panic unwinding. |
| P1-20 | `chaosBuild.go:17-38,122,134,148,155,169,176` | Shell command strings allow whitespace/globbing/metacharacter breakage and command injection through filenames. Use `exec.Command(name, args...)`. |
| P1-21 | `chaosBuild.go:77-79` | Backup filename includes a literal newline and rename/remove errors are ignored. |
| P1-22 | `chaosBuild.go:41-50` | Self-rebuild compares ctime rather than mtime and requires `./chaosBuild` to exist before it can check itself. |
| P1-23 | `chaosBuild.go:128-142` | ReadDir errors are ignored, non-`.chaos` entries are executed, each test overwrites `main.asm`, and no output/assembly/runtime assertion exists. |
| P1-24 | `src/chaosUtils.go:93-100` | If `debug` becomes false, `panicHandler` still recovers but does not exit/re-panic, turning compiler crashes into success. |

### 6.3 P2 design/maintainability issues

- `Token.Value any` (`src/chaosLexer.go:149-155`), `CSMatch(... any)` (`src/chaosSlice.go:100`), and the wide nullable `Node` union (`src/chaosAst.go:93-107`) move invariants from the Go type system into runtime casts/panics.
- `ChaosSlice.count` duplicates `len(data)` and can drift (`src/chaosSlice.go:6-10`). Optional variadic offsets/amounts obscure simple parser operations.
- `Position` contains no file identity or end offset (`src/chaosLexer.go:145-147`), and later AST/IR nodes carry no span. Good diagnostics and multi-file compilation require a `Span` on every syntax/semantic node.
- Parser control uses float binding powers and `almostEqual` (`src/chaosAst.go:168-186,289-291`); integer precedence levels are sufficient and safer.
- `Conditions.Count` duplicates slice lengths, `Conditions.Evaluations []BinOp` cannot hold arbitrary Bool expressions, and else uses `opNoOp` rather than an optional else branch (`src/chaosAst.go:83-87,375-385`).
- `Node.Reassignable`/`Reassigned` live on every node although mutability belongs to a resolved symbol; `Infered` is misspelled and never set (`src/chaosAst.go:93-107`).
- Type names as tokens/strings (`src/chaosInferenceCheckType.go:7-33`) cannot express identity, aliases, pointers, aggregates, generics, layout, or target-dependent types.
- Diagnostics are stack traces rather than source diagnostics; normal user errors are implemented as internal assertions.
- `chaosDebug` always prints full AST JSON, and unexported token positions are omitted (`src/chaosUtils.go:103-113`).
- The compiler CLI accepts multiple files but has no module/link model and overwrites one artifact (`src/chaos.go:42-55`).

## 7. Does the current structure make sense?

### 7.1 Parts worth retaining

- The file-level phase split in `src/` is understandable and aligns with a conventional compiler pipeline.
- A hand-written lexer and Pratt parser are reasonable for a small experimental language. Neither requires an external dependency.
- A distinct semantic pass and IR are the right concepts for the self-hosting goal.
- Emitting readable assembly is useful while semantics and calling conventions are still changing.
- The zero-dependency Go bootstrap keeps the initial toolchain small.

### 7.2 Boundaries that should change

The main structural problem is not the number of files; it is mixed responsibilities:

1. `chaosAst.go` resolves names while parsing. Parsing should create syntax even when names are unresolved.
2. `Scope` is an AST-node list. A semantic scope should map names to stable `SymbolID` values and have a parent link.
3. The type checker handles declarations rather than expressions. A compiler needs one recursive `checkExpr` that every declaration, condition, call, return, and exit reuses.
4. Type checking mutates both `program` and `scope` views. A typed tree/HIR should be one authoritative representation.
5. The experimental IR is a nested serialization of selected AST nodes rather than a control-flow/temporary representation.
6. Codegen skips IR. This forces backend logic to rediscover name, type, storage, and control-flow facts from a loosely validated AST.
7. User diagnostics use panic/assert infrastructure intended for compiler invariants.

Recommended stage ownership:

```text
SourceManager / FileID / Span
        |
        v
Lexer  ---------------------> []Token
        |
        v
Parser ---------------------> syntax AST (unresolved names)
        |
        v
Declaration collection ----> scopes, SymbolID, TypeDeclID
        |
        v
Name resolution -----------> resolved AST/HIR
        |
        v
Type + effect checking ----> typed HIR
        |
        +--> compile-time evaluator / macro expansion
        |         |
        |         +--> newly generated typed HIR (rechecked)
        v
MIR lowering -------------> typed temporaries + basic blocks
        |
        v
MIR validation/optimization
        |
        v
x86-64 lowering ----------> machine-like instructions, stack slots
        |
        v
fasm emission ------------> .asm -> executable
```

This can still live in one Go `main` package initially. Package splitting becomes useful after the data contracts stabilize.

### 7.3 Data-structure changes

| Current | Recommended | Why |
|---|---|---|
| `Position{line,column}` | `Span{FileID, Start, End}` plus line map | Precise diagnostics and multi-file support without storing line/column everywhere. |
| `Token.Value any` | Typed literal payload or `LiteralKind + raw text` | Avoid host-sized early conversion and runtime casts. |
| Wide `Node` tagged union | Dedicated structs implementing `Stmt`/`Expr`, or a validated union with constructors | Make invalid field combinations harder to create. |
| `Scope []Node` | `Scope{Parent ScopeID; Symbols map[string]SymbolID}` | Correct lookup, duplicates, shadowing, recursion, and forward declarations. |
| Type encoded as `Token.Symbol` | Interned `TypeID` over `Type{Kind, Bits, Signed, Elem, Fields, ...}` | Structural checks, target layout, aliases, generics. |
| Name text in every use | `ResolvedName{SymbolID, Span}` | Identity is stable across shadowing and lowering. |
| AST value copies | Arena/index IDs or pointers with one owner | Avoid duplicated/mutated semantic views. |
| `IROp` tree | Typed HIR plus basic-block MIR | Represent evaluation order and branches explicitly. |
| `assert` for source errors | `Diagnostic{Severity, Span, Message, Notes}` collector | Report multiple useful errors and keep panics for compiler bugs. |

Replacing `ChaosSlice` is optional. A compact parser can use:

```go
type Parser struct {
    tokens []Token
    pos    int
    diags  []Diagnostic
}

func (p *Parser) peek(n int) Token
func (p *Parser) at(kind TokenKind) bool
func (p *Parser) bump() Token
func (p *Parser) expect(kind TokenKind) Token
```

This retains cursor convenience without `any` comparison callbacks or a duplicated count.

## 8. Type checking and semantic analysis

### 8.1 Required ordering

Type checking should not begin until declarations have stable identities:

1. Parse all top-level declarations.
2. Create symbols for procedures/types/globals, detecting duplicates. This enables recursion and forward calls.
3. Resolve names in initializers and procedure bodies to `SymbolID`.
4. Resolve type syntax to `TypeID`.
5. Check expressions and statements recursively.
6. Run definite-initialization, return-path, and control-flow checks.
7. Evaluate permitted compile-time declarations and replace them with typed constants.

### 8.2 Minimal type representation

For the implemented subset:

```go
type TypeKind uint8

const (
    TypeInvalid TypeKind = iota
    TypeVoid
    TypeBool
    TypeInt
    TypeFloat
    TypeString
    TypeProc
)

type Type struct {
    Kind       TypeKind
    Bits       uint16        // 8, 16, 32, 64, 128
    Signed     bool
    Params     []TypeID
    Results    []TypeID
}
```

`String` also needs a concrete ABI representation. A recommended initial value representation is `{data pointer, byte length}` rather than a magic Go-like or C-like string.

Integer literals should initially have an “untyped integer” semantic type backed by `math/big.Int`, not host `int`. Context then checks range:

- `x: U8 = 255;`: valid.
- `x: U8 = 256;`: compile error at the literal.
- `x := 1;`: choose the language default, currently intended as `S64`.
- `x: U8 = -1;`: compile error after unary-minus analysis.

The same approach can use `math/big.Float`/exact decimal or a clearly specified IEEE conversion for untyped floats.

### 8.3 One expression checker

A central API should synthesize a type and typed expression:

```go
func (c *Checker) checkExpr(expr Expr, expected TypeID) TypedExpr
```

Rules:

- Literal: synthesize untyped value; coerce/range-check if `expected` is present.
- Identifier: fetch resolved symbol type and check initialization/use permissions.
- Call: require `TypeProc`, check arity, check each argument against the corresponding parameter, return a tuple/result type.
- Arithmetic: require compatible numeric operands; decide promotion rules; return numeric type.
- Comparison: require comparable operands; return `Bool`.
- `&&`/`||`: require `Bool`; preserve short-circuit evaluation in HIR/MIR.
- Assignment: require an assignable lvalue, mutable runtime symbol, compatible RHS, and initialization state.

Every context reuses it:

- declaration initializer: `checkExpr(init, declaredType)`;
- inferred declaration: `checkExpr(init, none)` then bind result;
- condition: `checkExpr(cond, Bool)`;
- return: check result tuple against procedure signature;
- `exit`: check status against the chosen exit-status type and message against `String`;
- compile-time declaration: additionally require a compile-time-evaluable expression.

### 8.4 Procedure and control-flow rules

For `tests/procs_ok.chaos:2-8` a correct checker should report:

```chaos
vars :: proc (...) -> S64 {
    variable_4 := input1; // String
    return variable_4;    // error: expected S64, found String
}
```

Add:

- parameter symbols initialized on entry;
- duplicate parameter diagnostics;
- result-count and result-type checks;
- no value on `return;` only for zero/`Void` result;
- all reachable paths return for non-Void procedures;
- unreachable-code warnings after unconditional return/exit;
- branch-local scopes and joins for definite initialization;
- eventually loop fixed-point analysis for initialization.

### 8.5 Mutability and compile-time meaning

Do not use `Node.Reassignable` as the source of truth. A symbol should record:

```go
type Stage uint8 // Runtime or CompileTime

type Symbol struct {
    Name       string
    Type       TypeID
    Mutable    bool
    Stage      Stage
    DeclSpan   Span
    Storage    StorageClass
}
```

Then:

- `x :: 1;` → compile-time, immutable constant;
- `x := 1;` → runtime, mutable variable;
- `f :: proc ...` → compile-time binding whose value is a procedure definition, with runtime callable code;
- `x : T;` → runtime mutable storage, uninitialized until assignment;
- `x = ...` → resolve `x`, require runtime + mutable, type-check RHS.

If the intended language allows immutable runtime values or mutable compile-time variables, syntax and symbol flags should model those dimensions independently rather than overloading one colon choice.

### 8.6 Advanced type-system choices

Recommended order:

1. Nominal primitive/procedure/struct types with local inference.
2. Pointers, arrays/slices, tuples, enums, and explicit aliases/distinct types.
3. Error/result types and optionals.
4. Interfaces/scaffolds and constraints.
5. Parametric generics with monomorphization.

Bidirectional checking is a better fit than full Hindley–Milner inference: annotations flow into literals/structs/lambdas, while ordinary expressions synthesize types. Generics, mutation, overloading, subtyping-like scaffolds, and staged execution make unrestricted HM inference a poor match.

## 9. Intermediate representation

### 9.1 Why the current IR cannot be extended incrementally without redesign

`IROp` (`src/chaosIr.go:16-28`) is a recursive bag of optional values. It has no result temporary, operand list, block, terminator, or type identity. Adding one `OpType` per AST construct would retain AST nesting and make control-flow codegen difficult.

Use two levels:

- Typed HIR: close to source, contains resolved symbols, types, structured `if`/loops, and source spans. Good for diagnostics and compile-time evaluation.
- MIR: procedures containing basic blocks, typed values, and explicit terminators. Good for assembly lowering and verification.

### 9.2 Minimal MIR

```go
type ValueID uint32
type BlockID uint32
type LocalID uint32

type Function struct {
    Symbol  SymbolID
    Params  []Local
    Results []TypeID
    Locals  []Local
    Blocks  []BasicBlock
}

type BasicBlock struct {
    Instrs []Instr
    Term   Terminator
}

type Instr struct {
    Result ValueID
    Op     Opcode
    Type   TypeID
    Args   []ValueID
    Imm    Immediate
    Span   Span
}

// Terminators: Jump, Branch, Return, Unreachable.
```

Minimal opcodes: integer/float/string/bool constants, load/store local/global, integer/float arithmetic, comparisons with signedness, call, string address/length, and optional cast. `exit` can initially lower to a target intrinsic, later to a standard-library procedure.

### 9.3 Lowering example

Source:

```chaos
main :: proc -> S64 {
    x := 1 + 2;
    if x < 4 then return x;
    return 0;
}
```

Typed MIR sketch:

```text
fn main() -> s64:
bb0:
    v0:s64  = const 1
    v1:s64  = const 2
    v2:s64  = add v0, v1
    store.local x, v2
    v3:s64  = load.local x
    v4:s64  = const 4
    v5:bool = cmp.lt.s v3, v4
    branch v5, bb1, bb2
bb1:
    v6:s64 = load.local x
    return v6
bb2:
    v7:s64 = const 0
    return v7
```

This makes evaluation order, result types, and control flow explicit. A verifier can ensure each block has one terminator, every use is defined, types match, and returns match the function signature before backend work.

### 9.4 IR alternatives

| Option | Benefits | Costs / fit |
|---|---|---|
| Direct AST-to-assembly | Fastest prototype | Repeats semantic logic in backend; control flow and optimization become brittle. Not recommended beyond a throwaway integer-only milestone. |
| Stack bytecode IR | Easy interpreter/compile-time VM; simple expression lowering | Less natural for register allocation; still needs CFG metadata. Good if VM/CTFE reuse is the main priority. |
| Three-address CFG MIR | Straightforward assembly, validation, and simple optimization | Requires block/value infrastructure. Recommended baseline. |
| SSA MIR | Excellent data-flow/optimization properties | Phi construction and destruction add complexity. Add after a non-SSA MIR works, or build SSA only if optimization is an immediate goal. |
| External IR (LLVM/QBE) | Outsources much backend work | Adds dependencies and toolchain/API constraints; less aligned with the current zero-dependency/fasm experiment. |

## 10. Assembly and executable generation

### 10.1 Decide the artifact model first

Current `format ELF64 executable 3` asks fasm to create a complete flat ELF executable. This is viable for a Linux-only, syscall-only compiler, but it complicates C interop and standard libraries.

Options:

| Target | Good for | Limitations |
|---|---|---|
| fasm flat ELF executable | Tiny self-contained Linux experiments | Manual ELF/section model, no normal linker/relocations, difficult libc/FFI. |
| fasm ELF object + system linker | C ABI, libraries, separate compilation | Requires linker orchestration and relocation-aware emission. |
| C transpilation | Fast route to portable runnable semantics | C becomes the effective backend/ABI; diagnostics and exact low-level control are weaker. |
| LLVM IR | Optimization and multiple targets | Large dependency and integration surface. |
| Custom bytecode VM | Same runtime for programs and compile time | Not native assembly; native backend remains later work. |

Recommended near-term: keep fasm, but emit ELF objects and link once C interop/modules matter. A flat syscall-only executable is acceptable for the first integer/bool/function milestone.

### 10.2 Required x86-64 decisions

- Entry: distinguish process entry (`_start`-like, receives kernel stack) from a language `main` procedure. A source `#entry` should select one procedure; a wrapper should call it and exit with its result.
- Calling convention: either document a private convention or use System V AMD64. For SysV, integer/pointer args use RDI, RSI, RDX, RCX, R8, R9; result uses RAX; stack must be 16-byte aligned at calls; preserve callee-saved registers.
- Storage: globals in readable/writable data; immutable constants/strings in readable data; locals in stack slots initially.
- Width: emit byte/word/dword/qword operations according to `TypeID`. Define extension/truncation.
- Signedness: `setl`/`setle` for signed comparisons and `setb`/`setbe` for unsigned. Normalize booleans to 0 or 1.
- Arithmetic: define overflow behavior, division traps, and shift masking.
- Strings: emit bytes safely, not interpolated unescaped inside `db "..."`. Store pointer and length; decide UTF-8 semantics.
- Branches: every MIR block gets a unique label and one terminator.
- Calls/returns: prologue, stack-frame layout, parameter moves, return value, epilogue. Recursion then works naturally.
- Syscalls: Linux x86-64 `write` uses syscall number 1; `exit` uses 60. These are target details and should live behind a target interface.
- Relocations/symbols: sanitize or mangle Chaos identifiers and generate collision-free internal labels.

### 10.3 Problems in the commented backend

Even as reference, `src/chaosCodegen.go:30-278` should not be revived unchanged:

- It uses removed `Program` allocation lists.
- Calls have no arguments, stack frame, or ABI (`117-123`).
- All variables are global qwords (`88-115`), ignoring type width and lexical scope.
- Only addition and signed less-than have partial cases (`130-221`).
- `setl r8b` followed by storing all of `r8` leaves upper bits undefined (`182-218`).
- User strings are inserted directly into quoted fasm source and always gain newline 10 (`58-65,243-250`).
- Procedure `exit` emits a process syscall and then unreachable `ret` (`68-80`).
- It has no return, condition, boolean, float, reassignment, short-circuit, or nested-expression lowering.

### 10.4 Minimal backend milestones

1. MIR-to-fasm for `const`, local load/store, add/sub/multiply, return, and one process-entry wrapper.
2. Comparisons and conditional branches.
3. Calls with stack frames and recursion.
4. Globals and immutable constants.
5. String data plus `write`/`exit` intrinsics.
6. Remaining integer widths/signedness, then floats using SSE2.
7. Object/linker mode for modules and C ABI.

Each milestone needs golden MIR and assembly tests plus assembled runtime tests.

## 11. Compile-time execution

### 11.1 Current versus intended behavior

Current `::` behavior:

- Parser marks declaration `Reassignable=false` (`src/chaosAst.go:556-560`).
- Type checker may infer a primitive type.
- No expression is evaluated and no constant value is stored.
- IR loses the compile-time distinction.
- Codegen does nothing with it.

The design sketches imply a much larger staged system:

- `#run`: `language_design/randomSyntaxShit.chaos:29-31` and `cleanState.chaos:31-40`.
- `#let` mutable compile-time state: `randomSyntaxShit.chaos:178-196`.
- `#if/#while/#insert` generation: `types.chaos:18-21`.
- `#assert`: `flag.chaos:7`.
- macros/hooks/AST matching: `randomSyntaxShit.chaos:178-209`.
- reflection such as `type_of`/`size_of`: `randomSyntaxShit.chaos:101-106` and many container sketches.
- imports and target/OS queries: `chaos.chaos:168-171` and `coroutine.chaos:11-25`.

These are separate features and should not all be called “compile-time execution”.

### 11.2 Recommended staged implementation

Phase A — constant folding:

- Evaluate pure typed HIR literals, unary/binary operations, comparisons, and references to earlier constants.
- Store a typed `ConstValue` on the symbol.
- Detect dependency cycles.
- Reject runtime variables/calls in constant expressions.

Phase B — compile-time function interpreter:

- Interpret typed HIR or a bytecode lowered from it.
- Support local values, branches, loops, calls, structs/arrays once those runtime semantics exist.
- Add recursion depth/instruction fuel and a useful compile-time stack trace.

Phase C — explicit `#run` and reflection:

- Define which functions/effects are legal.
- Represent types/AST/source locations as compiler-known values.
- Re-type-check any generated declarations.

Phase D — hygienic macros:

- Keep AST syntax objects distinct from runtime values.
- Give generated identifiers hygiene/context IDs.
- Track expansion stacks and source spans.
- Define phase ordering between imports, macros, declaration collection, resolution, and type checking.

### 11.3 Compile-time safety and reproducibility

Decisions required before allowing arbitrary effects:

- Host versus target: cross-compilation cannot execute target machine code. An interpreter/VM is target-independent.
- File/environment/network access: unrestricted access makes builds non-reproducible and unsafe. Use explicit capabilities and record dependencies.
- C FFI: target C functions generally cannot run during cross-compilation; host tools must be explicitly separated.
- Determinism: define time/random behavior or require explicit nondeterministic capabilities.
- Caching: cache on function bytecode/HIR, arguments, compiler version, target, and declared external dependencies.
- Termination: fuel, memory limits, recursion limits, and cycle diagnostics.
- Stage leakage: a compile-time pointer/address cannot become a runtime pointer. Only serializable typed constants may cross stages.

### 11.4 Implementation alternatives

| Approach | Effort | Characteristics |
|---|---:|---|
| AST/HIR recursive evaluator | 2-5 weeks for primitive constant/functions | Best first step; easy source diagnostics; can become slow/duplicated if runtime grows. |
| Stack/register bytecode VM | 6-12 weeks for a useful core | Reusable for tests and compile time; target-independent; requires a VM/runtime definition. |
| Compile and execute host-native code | 10-20+ weeks | Fast execution but difficult relocation, sandboxing, target/host split, and crash isolation. |
| Generate Go and run it | 4-8 weeks prototype | Leverages host toolchain but introduces semantic mismatches and weakens self-hosting portability. |

Start with a typed HIR evaluator, then move to a bytecode VM if `#run` becomes central. Do not build macros before the symbol/type/span model is stable.

## 12. `language_design/` map

### 12.1 How to read this directory

None of these files is in the active build or test pipeline. `chaosBuild.go:118-125` builds only `src/`; `chaosBuild.go:127-140` runs only `tests/`. The files combine several incompatible language generations:

- current-ish brace/proc syntax, e.g. `assignment.chaos`;
- older prose syntax (`proc ... expects ... executes ... end proc`), e.g. the latter half of `main.chaos` and `coroutine.chaos`;
- underscore identifiers and later hyphenated identifiers in duplicate sketches (`chaos.chaos:35-129`), even though the active lexer treats `-` as an operator;
- C-like `for` syntax, range loops, iterator loops, macros, annotations, and DSL forms;
- several alternatives for error results: `T | Error`, `T <> Error`, postfix `!`, postfix `?`, `unless catch`, `unless error`, and `catch`.

Most files fail in the lexer before grammar matters because the active lexer has no tokens for brackets, dot, question mark, percent, slash, pipe, at-sign, backtick, quote, or single ampersand. `#`, `!`, `...`, and `as` are lexed but not parsed.

The design directory is useful as a requirements corpus. It should eventually become versioned RFCs plus small conformance examples, because the sketches currently mix competing syntax and contain algorithm/identifier typos that should not silently define the language.

### 12.2 Per-file inventory

#### `language_design/assignment.chaos` (36 lines)

- Definitions: global `something_new` (`3`), `something_else` (`5-23`), and `#entry main` (`25-35`).
- Call flow: `main` constructs strings/integers then calls `something_else(10, true)` (`26-34`); `something_else` declares locals, branches, and calls `exit` (`6-22`).
- Requirements represented: five declaration forms, proc parameters/results, variadics, entry annotation, comments, conditions, string interpolation.
- Active overlap: most declarations/proc/if/exit shapes parse if `#entry` and variadic input are removed. Interpolation remains plain bytes; call/type/body checking and codegen are absent.

#### `language_design/async.chaos` (105 lines)

- Definitions: `Async_Error` (`4-10`), `Future` scaffold (`12-26`), `Future_Header` (`28-36`), first `Future_Header impl` with `new_future`/`await`/`get` (`38-74`), generic `Future_Of<T>` (`76-79`), second impl with `init`/`destroy` (`83-101`), import/module declarations (`103-104`).
- Call flow: `new_future` allocates then calls `init` (`39-45`); `get` validates, calls `await` and `ctx.mem_cpy` (`66-73`); `init` calls condition/mutex setup (`84-93`); `destroy` tears them down (`96-100`).
- Requirements: scaffolds/interfaces, structs, methods, `Self`, generics, pointers, atomics, time types, named/default args, field access/update, errors, defer, modules.
- Sketch defects: `timeout`/`time_out` differ (`48,53,57,60,66,70`), `state` is declared but `start` is used (`55-57`), `tn_sec` is likely a typo (`58`), and `&&` is used where arithmetic/modulo appears intended (`58`).

#### `language_design/asyncTasks.chaos` (31 lines)

- Definitions: `Async_Error` (`3-6`), `get_user_by_username` (`8-13`), `get_user` (`15-27`), module/import (`29-30`).
- Call flow: `get_user` creates an anonymous async procedure, obtains a global session, calls `get_user_by_username`, awaits it, logs the error path, and returns the user (`16-26`).
- Requirements: first-class/anonymous procedures with capture, `Async<T>`, scheduler/task runtime, await, SQL/string interpolation, result propagation, modules.

#### `language_design/atomic.chaos` (1 line)

- One use-site declaration, `some_atomic_value: Atomic<U32>`.
- It does not specify atomic operations, memory order, layout, target support, or whether `Atomic<T>` is a generic library type or compiler intrinsic.

#### `language_design/bpe.chaos` (60 lines)

- Definitions: `Bpe_Error` (`4-6`), `dump_tokens` (`8-10`), `load_tokens` (`12-19`), `load_pairs` (`21-59`).
- Call flow: dump delegates to `write_entire_file`; load reads bytes and appends tokens; pair loading reads/string-views data then dispatches to version parsers (`26-58`).
- Requirements: pointers, member/index access, `size_of`, loops/ranges, mutation operators, casts, switch/case, errors.
- Sketch defects: only `GENERIC` is declared but `INVALID_DATA`/`GENERIC_ERROR` are returned (`35,49,58`); `version` is passed by value but assigned (`21,33,54`); `tmp_sb` is redeclared as zero (`22`); ASCII string quotes appear at `30`; names vary `parse_pairs`/`parse_pair` (`43-55`).

#### `language_design/chaos.chaos` (172 lines)

- Self-hosted compiler sketch `compile_chaos` (`4-17`) mirrors the Go pipeline: buffer → tokenize → AST → type check → codegen → write `main.asm`.
- A compiler CLI `main(argc, argv)` starts at `19` and is incomplete at `28-32`.
- An underscore-style build entry is at `35-81`.
- A mostly duplicate hyphen-style build `main` is at `83-129`.
- `go-rebuild-your-self-tek` sketches self-rebuild at `133-166`.
- Module/scope/import forms are at `168-171`.
- Requirements: self-hosting standard library (filesystem, process, buffers, arrays), argv/variadics, errors/defer, loops/slices, string interpolation, modules/import aliases.
- Sketch defects: `tokens_slice` versus `token_slice` (`6-8`), multiple conflicting `main` declarations, hyphen identifiers incompatible with current lexer, `default_file_chaos` unused (`67`), and two different error syntaxes.

#### `language_design/cleanState.chaos` (77 lines)

- Definitions: generic macro `some_operation` (`1-5`), `Vector2<T>` (`7-9`), `Version` and pointer-using `string` (`11-20`), `at_another_point` (`22-29`), closure-producing `new_counter` (`31-41`), `New_Error` (`47-49`), `switch_test_new` (`51-65`), `optional_return_assignment` (`67-71`), and result destructuring examples (`73-76`).
- Requirements: generics/constraints, structs and literals, implicit `using` fields, allocation, closures/captures, multiple/optional return values, switch/labels, error unions, destructuring.
- Sketch gaps: several referenced types/values are undefined (`Custom`, `Some_Struct`, `result`, `Some_Enum`, `Error`); it intentionally compares alternative syntax forms.

#### `language_design/coroutine.chaos` (28 lines)

- Old-dialect `init` (`3-5`) and macro `yield` (`9-27`).
- `yield` computes the OS at compile time then embeds an assembly context-switch sequence (`11-25`).
- Requirements: macros, target queries, inline assembly, coroutine ABI/context object, scheduler/runtime.
- Backend conflict: assembly uses AT&T/GAS syntax (`pushq %rdi`), while active output targets fasm Intel syntax (`src/chaosCodegen.go:14`).

#### `language_design/doubleLinkedList.chaos` (98 lines)

- Helpers `length`, `head`, `tail`, `print` (`4-23`).
- Generic `Double_Linked_List` and `Node` structs (`25-35`), `Dll_Error` (`37-41`).
- Operations `init_node` (`43-50`), `push_head` (`52-68`), `insert_node` (`70-91`), `delete_head` stub (`93-95`), module declaration (`97`).
- Requirements: generic recursive structs, pointers/optionals, methods/field access, loops/ranges, division, error results, allocator/logging, increment, modules.
- Sketch defects: `dll.hear` typo (`9`), `Nonde` typo and missing input to `init_node` (`43-44,53`), `push_head` parameter is `node` but body uses `dll` (`52-68`), `newNode`/`new_node` differ (`59-64`), unsigned `index < 0` is dead (`71`), and `push_tail/report_oom` are absent.

#### `language_design/error.chaos` (14 lines)

- `Example_Error` enum-like error (`1-5`) and entry `main` that creates/prints one (`7-10`), plus import (`13`).
- Requirements: nominal errors, member-style cases, postfix error marker, printable reflection, entry/import.

#### `language_design/fin-plan.chaos` (48 lines)

- `Inss_Range`/`Income_Tax_Range` structs and map aliases/tables (`1-24`).
- `inss_deduction_calculation` (`26-36`) and `income_tax_deduction_calculation` (`38-47`).
- Requirements: floats, named struct construction, maps, infinity constant, iterator loops, member access, compound assignment.
- Sketch defects: underscore appears where subtraction is intended (`30,32`), `inome_tax_table` and `monthy_deduction` are typos (`40,42`), and `break` lacks semicolon (`43`).

#### `language_design/flag.chaos` (48 lines)

- `Flag_Type` enum array and compile-time exhaustive assert (`1-7`), `Flag_Value` union (`9-14`), `Flag` (`16-22`), capacity constant (`24`), `Flag_Content` (`26-33`), global context (`35`), `new_flag` (`37-47`).
- Call flow: `new_flag` reads global context, asserts capacity, selects a slot, clears it with `mem_set`, fills metadata, returns it.
- Requirements: enums with values/reflection, unions, fixed/dynamic arrays, pointers/references, globals, compile-time assertions, memory intrinsics.
- Sketch defects: `Flag_Context` does not match `Flag_Content` (`35`), `Name` is undefined (`37`), `flags_count` lacks `ctx.` at `42`, and return `Flag!` conflicts with returning `*Flag`.

#### `language_design/hashTable.chaos` (168 lines)

- `Hash_Table_Error` (`4-9`), iterator macro (`11-13`), generic `Entry`/`Hash_Table` (`15-24`).
- Public operations: `init_hash_table` (`26-39`), `destroy` (`41-50`), indexed get/set operator overloads (`52-79`), `expand` (`81-95`), `set` (`97-105`), `next` (`107-120`).
- Private/file-scope constants and helpers: FNV constants (`122-125`), `hash_key` (`127-138`), `set_entry` (`140-166`), allocator import alias (`168`).
- Call flow: init allocates table/entries; set may expand then calls set_entry; expand reallocates and re-inserts; get probes; iterator next scans entries.
- Requirements: generics, structs, pointer arithmetic, allocation/defer, operator overloading, file scope, imports, macros, nullables, loops.
- Sketch defects include `currenty_entry` (`46`), inconsistent `entry/entries` and `hash_key/has_key` names, logical `&&` used as bit masking (`54,67,141`), inverted `size == null` dereference logic (`155-157`), no wraparound in probing, and missing iterator type definition.

#### `language_design/interopC.chaos` (11 lines)

- Entry `main` creates/imports a C struct and invokes C `printf` (`1-10`).
- Requirements: C declarations/header import model, type/layout mapping, varargs promotions, C strings, symbol linking, ownership, target ABI, and build-driver linker flags. None exists in the active compiler.

#### `language_design/jimp.chaos` (101 lines)

- `Jimp_Token` enum (`1-21`), parser state `Jimp` (`23-37`), file-scope marker (`39`).
- Helpers `jimp__append_to_string` (`41-51`), `jimp__skip_whitespaces` (`53-57`), punctuation/symbol tables (`59-77`), partial `jimp__get_token` (`79-101`).
- Call flow: get_token skips whitespace, detects EOF/punctuation, then begins symbol matching.
- Requirements: enums, structs, pointer/string arithmetic, realloc, arrays with indexed initializers, compile-time `len`, C-style for loop, casts.
- Sketch defects: `jimp.poinmt` (`55`), `jimp__punts` versus `jimp__puncts` (`59,88`), singular/plural whitespace function (`53,80`), `symbold_count`/`symbols` typos (`94-95`), empty symbol loop, and incomplete string/number tokenization.

#### `language_design/lexer-chaos.chaos` (287 lines)

- Self-hosting token model: `Symbol`/`Token_Atom` (`1-2`), `Token_Type` (`4-38`), string table (`40-74`), `Position` (`76-78`), `Token` scaffold and token implementations (`80-116`), `Content_State` (`118-121`).
- Overloaded `Content_State.match_at` functions (`123-149`).
- A current-ish partial `tokenize_chaos` is at `151-213`.
- An older prose-dialect lexer fragment continues outside that procedure at `217-286`.
- Requirements: enums/reflection arrays, interfaces/scaffolds, overloads, methods, dynamic arrays, errors/context, self-hosted buffer/unicode support.
- Sketch defects: token names in string table do not match enum (`TOKEN_INFER_TYPE` vs `TOK_INFER_TYPE`, `TOK_STRING/TOKEN_NUMBER` absent), `Token_Kind` is undefined, `offsef/peak` typos (`124-137`), `cs` versus `content_state` (`152-155`), line comments seek NUL rather than newline (`157-161`), multiline cursor can fail to advance (`165-193`), and old fragment advances one-character `=` by three (`271-280`).

#### `language_design/linkedList.chaos` (77 lines)

- Generic `Node`/`List` (`1-8`), `new` (`10-15`), `make` (`17-20`), `insert` (`22-32`), `remove` (`34-47`), `reverse` (`49-61`), `destroy` (`63-72`), module/imports (`74-76`).
- Call flow: constructors allocate; insert traverses then allocates; remove/reverse/destroy traverse linked nodes.
- Requirements: generics, pointers/optionals, allocator, loops, member access, error propagation, modules.
- Sketch defects: calls `new_node` although definition is `new` (`24,31`); insert exits loop with `current == null` then writes `current.next` (`27-31`); remove compares data to null instead of target and assigns `previous.data = current.next` (`38-40`); final import string is unterminated (`76`).

#### `language_design/logger.chaos` (38 lines)

- `Logger_Level` (`1-10`), `Log` scaffold (`12-21`), `Log_State` (`23-29`), `Log_State impl log` (`31-35`), import (`37`).
- Requirements: enums, scaffolds/base implementation, defaults, dynamic stream arrays, methods, formatting/time/OS facilities.
- Current `warning`/`chaosDebug` utilities are unrelated and do not implement this design.

#### `language_design/main.chaos` (75 lines)

- A current-ish incomplete `chaos_tokenizer` appears at `1-10`.
- An older prose-dialect `chaos-tokenizer` is at `12-48`.
- Old distinct token/node aliases and enum appear at `50-66`; module/imports at `68-74`.
- Requirements: self-hosted tokenizer, buffers, arrays, character types, switch/case, modules.
- Sketch defects include `len(file_content;)` (`4`), missing semicolons/empty loop, `cotent` typo (`25`), incomplete comparison (`28`), and incompatible hyphenated identifiers.

#### `language_design/randomSyntaxShit.chaos` (267 lines)

This is the broadest feature incubator:

- hashed enum concept and `Another_Enum` (`1-10`);
- `Random_Error` (`12-17`), generic `Json_Response<T>` and `something_crazy` plus `#run` (`19-33`);
- redacted struct fields/reflection (`35-52`);
- entry/error handling and labeled loops (`54-72`);
- complex/quaternion literals (`74-76`);
- `handling_errors` and propagation (`79-99`);
- generic truthiness/reflection `is_falsy/is_truthy` (`101-113`);
- ownership/allocation/error-defer sketch (`115-132`);
- captured local procedure (`134-142`);
- generic constraint `String_Limit` and constrained overload alternatives (`144-169`);
- scope/import aliases (`171-176`);
- compile-time state, macro hooks, window/drawing macros (`178-201`);
- AST pattern macro (`203-209`);
- raylib entry example (`211-227`);
- generic scaffolds, multiple bases, and implementation block (`231-266`).

Nearly every advanced subsystem appears here: generics, reflection, macros/hygiene, compile-time state, ownership, FFI, closures, errors, complex numbers, annotations, constraints, overloads, modules, and a graphics ABI. It should be split into independent RFCs before implementation. There are deliberate alternatives and typos (`handling_error` vs `handling_errors` at `99`, `other_checkes` at `112`, screen-name inconsistencies at `211-222`), so it cannot be treated as a grammar test.

#### `language_design/randomSyntaxShit.chaos.docs` (54 lines)

- Documentation DSL metadata/imports at `12-20`.
- `main` builds sections/paragraphs, checks whether `Context.definition` changed, and links examples (`23-49`).
- Imports at `51-53`.
- Requirements: a separate docs build mode, reflection over definitions, stable definition hashes, incremental state/diffs, Markdown/HTML emitters, filesystem links, and error reporting. This is tooling beyond the language compiler core.

#### `language_design/rule110.chaos` (80 lines)

- Entry simulation `main` (`7-16`), constants/tables (`18-35`), `Cell` enum array (`37-40`), inline `pattern` (`42-44`), generic `Row` (`46-48`), `next_row` (`50-60`), `print_row` (`62-68`), `random_row` (`70-76`), imports (`79-80`).
- Call flow: main creates a random row, loops print → next_row; next_row calls pattern and indexes the rule table; print uses putc; random uses rand.
- Requirements: fixed arrays, enum-indexed tables, generics/compile-time dimensions, for loops, shifts/bitwise OR, modulo, indexing, boundary policies, imports.
- Sketch defects: `||` looks like bitwise OR in `pattern` (`43`), `prev` differs from parameter `previous` (`53-55`), `Array_Boundry_Check` is misspelled (`47`), and two macro-call spellings `@pattern`/`pattern` coexist.

#### `language_design/scaffold.chaos` (33 lines)

- Generic `Some_Interface` scaffold (`1-5`), implementing `Example<T>` (`7-12`), `do_some_operation` (`14-17`), entry usage (`19-29`), module/import (`31-32`).
- Requirements: interface contracts, base implementation, methods/implicit receiver, generics, struct literals, member/deref access, modules.
- Semantic decision needed: static monomorphized constraint versus runtime interface/vtable. The examples use both field requirements and method dispatch.

#### `language_design/string.chaos` (37 lines)

- `String_Error` (`1-3`), overloaded `length` (`5-19`), overloaded `copy` (`21-35`), import (`37`).
- Requirements: overload resolution, pointers/addressing, mutation/increment expressions, errors, NUL/string representation.
- Sketch issues: pointer/address initialization differs between overloads (`6,12`); copy never allocates destination (`21-34`); comparing a pointer/string to `«\0»` exposes an unresolved string ABI choice.

#### `language_design/structs.chaos` (13 lines)

- `Hello_World` with default field (`1-4`), entry creates partial literal and prints fields (`6-9`), module/import (`11-12`).
- Requirements: nominal structs, layout/default initialization, field access, aggregate literals, interpolation, modules.

#### `language_design/tests.chaos` (15 lines)

- Annotation/macro suite `@new_test_suite` wraps `first_test_suit :: test` (`1-15`).
- Nested test procedures are at `3-6` and `8-13`.
- Requirements: annotations/macros, test declarations, setup/teardown types, assertions, expected-error handling, compiler test discovery and reporting.
- This is an aspirational test DSL, separate from the current filename-driven loop in `chaosBuild.go:127-142`.

#### `language_design/trying_shit.chaos` (47 lines)

- Entry constructs option/demand/probability arrays and nested loops (`1-19`).
- `calculate_profit` (`21-39`), constants (`41-43`), `Options` alias (`45`), math import (`47`).
- Requirements: array/tuple literals, inferred lengths, dynamic nested arrays, for/range/zipped loops, floats/casts, append/printing, aliases/imports.
- Algorithm concern: `LOSS_DEMAND_NOT_MET_PER_UNIT` is declared but not applied to `demand_loss` (`27-28`).

#### `language_design/types.chaos` (29 lines)

- Numeric union aliases `Any_Integer` through `Any_Number` (`1-16`).
- Compile-time-generated generic `Matrix` (`18-22`), generic `Vector` (`24-26`), module (`28`).
- Requirements: union/constraint types, signed/unsigned/float/complex/quaternion primitives, generics with value parameters, compile-time loops and AST/text insertion.
- Sketch defect: `Any_Number` references `Quaternion` instead of `Any_Quaternion` (`16`). String-encoded constraints (`18`) are an alternative syntax that should be resolved.

#### `language_design/utils.chaos` (44 lines)

- Three empty `is_uppercase` overloads (`1-11`).
- `Lexer_Error_Config` (`13-17`), `new_lexer_error` (`19-25`), `new_example` (`27-43`).
- Requirements: overloads, dynamic optional arrays, struct literals, variadics, methods/append, loops/interpolation.
- Sketch defects: `assert example > 0` should refer to the length variable (`28-29`); `self` is used although the parameter is `error_config` (`32-41`); empty overload bodies leave behavior undefined.

## 13. Missing implementation and estimated effort

### 13.1 Estimation assumptions

Ranges below assume one experienced compiler engineer working full time, including focused tests and documentation but excluding a polished IDE, debugger, package ecosystem, and production hardening. Features interact; estimates are not safely additive. The current parser/semantic representation will increase costs unless the foundation is corrected first.

### 13.2 Foundation and correctness

| Work | Effort | Current structures affected | Main risks |
|---|---:|---|---|
| Safe lexer, spans, diagnostics, literal handling | 2-4 weeks | `Token`, `Position`, `ChaosSlice`, lexer, assert utilities | Unicode policy, malformed-input recovery, exact numeric semantics |
| Parser repair and explicit grammar | 3-6 weeks | All parser functions, AST node model | Preserving desired syntax while design sketches conflict |
| Symbol tables/name resolution | 3-5 weeks | Replace `Scope []Node` and `checkIdentifierInScope` | Forward declarations, shadowing, recursion, generated names |
| Primitive type model and recursive checker | 4-8 weeks | Replace string types and flat inference pass | Numeric coercions, result tuples, definite initialization |
| Diagnostic/error architecture | 2-4 weeks, overlaps above | `assert`/`panicHandler` and every phase signature | Recovery without cascaded nonsense errors |
| Real test harness | 1-3 weeks | `chaosBuild.go`, fixtures, Go unit tests | Stable expected diagnostics and runtime environments |

### 13.3 Minimal runnable compiled language

| Feature | Effort after foundation | Compiler impact | Potential issues |
|---|---:|---|---|
| Typed HIR + MIR and verifier | 4-7 weeks | New representations and lowering pass | Evaluation order, source-span preservation |
| x86-64/fasm integer backend | 5-9 weeks | Replace codegen, artifact plumbing | ABI, stack alignment, signedness, labels |
| Procedures/calls/returns | 3-5 weeks | Resolver, checker, MIR, stack frames | Recursion, more than six args, multiple results |
| `if/elif/else` and short circuit | 2-4 weeks | AST/HIR redesign, CFG branches | Reachability and branch-local initialization |
| Globals/local storage and reassignment | 2-4 weeks | Symbol storage classes, MIR load/store | Initialization order and mutable globals |
| Bool/String and exit/print runtime | 3-6 weeks | Data layout, constants, runtime intrinsics | UTF-8, escaping, ownership/lifetime |
| Loops + break/continue | 3-5 weeks | Tokens/AST, checker context, CFG | Nested labels and data-flow joins |
| Robust build/assemble/run CLI | 1-3 weeks | `chaos.go` and `chaosBuild.go` | Artifact naming, tool discovery, link errors |

A realistic small compiled MVP—integers/bools/strings, variables, procedures, calls, conditionals, loops, useful diagnostics, and runnable Linux x86-64 output—is approximately 4-7 person-months from the current state if scope is held tight.

### 13.4 Core data and language facilities

| Feature | Effort | Structural impact | Key issues |
|---|---:|---|---|
| Pointers, addresses, null/optional | 4-7 weeks | Type/layout, lvalues, MIR memory ops, backend | Safety model, null checks, pointer arithmetic |
| Arrays, slices, dynamic arrays, indexing | 5-9 weeks | Type/value model, bounds policy, runtime allocator | Ownership, length/capacity ABI, generics |
| Structs and field/default initialization | 5-9 weeks | Nominal types, layout/alignment, member AST/MIR | Recursive types, padding, zero/default semantics |
| Enums, aliases, distinct types, tuples/unions | 4-8 weeks | Type declarations and representation | Exhaustiveness, tag layout, coercions |
| Floats | 2-5 weeks | Literal parser, types, MIR ops, SSE backend | NaN/comparison/cast semantics |
| Integer widths up to 128 | 2-5 weeks | Big literals, legalizer/backend helpers | Native versus library lowering |
| Defer/error-defer | 3-6 weeks | Control-flow cleanup lowering | Multiple exits, panic/error interaction |
| Operator/method overloading | 5-10 weeks | Resolver/type checker and desugaring | Ambiguity, coherence, diagnostics |

### 13.5 Advanced design-directory features

| Feature | Effort after core | Required architecture changes | Risks |
|---|---:|---|---|
| Modules/imports/separate compilation | 6-12 weeks | Source manager, module graph, namespaces, artifacts/linker | Cycles, initialization order, cache invalidation |
| Error/result system | 4-8 weeks | Sum/result types, `?`/`unless` lowering, ABI | Competing syntax in sketches, cleanup semantics |
| Compile-time constants/functions | 5-10 weeks | Typed evaluator, `ConstValue`, dependency graph | Termination, target/host split |
| Reflection + `size_of/type_of` | 3-7 weeks | Reified type metadata available at compile time | Stable API before layouts settle |
| Generics by monomorphization | 10-20 weeks | Generic symbols/types, substitution, instantiation cache | Recursive instantiation, code size, constraints |
| Scaffolds/interfaces/constraints | 8-16 weeks | Trait solver and/or runtime vtables | Static versus dynamic model is unresolved |
| Hygienic macros/AST generation | 10-20 weeks | Syntax objects, phase ordering, hygiene, expansion spans | Debuggability and build determinism |
| C interop | 5-10 weeks after object/link mode | C ABI types, externs, linker args, varargs | Platform ABI and ownership |
| Standard allocator/containers/strings | 8-20+ weeks | Runtime/stdlib and language core types | Bootstrapping, memory safety, performance |
| Atomics/threads | 5-10 weeks | Memory model, backend atomics, runtime | Ordering semantics and platform support |
| Async/tasks | 12-24 weeks | Scheduler, futures, stackless lowering or state machines | Cancellation, lifetime, synchronization |
| Stackful coroutines/inline assembly | 10-20 weeks | Target-specific context ABI and unsafe escape hatch | Register/stack unwind correctness |
| Complex/quaternion/matrix types | 4-12 weeks mostly library, after generics/operators | Numeric types and operator support | ABI and compile-time dimensions |
| Documentation/test DSL tooling | 6-12 weeks each | Macro/reflection/tool subcommands | Stable metadata and incremental state |

Self-hosting is not one feature. It needs a stable compiler subset, files/processes/collections/strings, modules, allocator/runtime, deterministic build graph, and a bootstrap comparison strategy. For one engineer, a credible self-hosted compiler matching even the small MVP is more likely 18-36+ months than 6-12 months; the full design-sketch scope is a multi-year project.

### 13.6 How missing features affect the current structure

- Loops and short-circuit conditions require CFG IR; adding only AST nodes will push complexity into codegen.
- Structs/pointers/arrays require real type layout and lvalue/address semantics; `Token.Symbol` types and global-qword codegen cannot support them.
- Generics require declaration/type identity and an instantiation cache; parse-time node scopes cannot model them.
- Modules require file-aware spans, namespaces, a module graph, output objects, and linker integration; hardcoded `main.asm` must disappear.
- Errors/defer require cleanup-aware control-flow lowering; they should be designed before async/coroutines.
- Compile-time execution needs typed HIR and stage-aware symbols; it should not execute raw, unresolved AST.
- Macros require hygiene and expansion spans; simple string insertion such as `types.chaos:18-21` will make name capture and diagnostics unmanageable.
- C interop requires a linker/object model and concrete layouts/calling convention; flat syscall-only fasm output is insufficient.
- Async/coroutines depend on functions, errors, allocation, atomics, ownership/lifetimes, and a runtime. Implementing them earlier would force repeated redesign.

## 14. Alternative implementation options

### 14.1 Front end

| Choice | Advantages | Disadvantages | Recommendation |
|---|---|---|---|
| Repair hand-written lexer + Pratt parser | Small dependency-free codebase; flexible diagnostics; matches current direction | Grammar remains implicit unless documented; easy cursor mistakes | Recommended, with parser/lexer unit tests and a written grammar. |
| Parser generator | Explicit grammar and generated state machine | Expression/diagnostic integration can be awkward; adds generation workflow | Reconsider only if grammar ambiguity grows substantially. |
| PEG parser | Compact grammar and easy experimentation | Backtracking/performance and error recovery require care | Useful for syntax prototyping, less compelling for the self-hosted final compiler. |

The current parser does not need replacement merely because it is handwritten. It needs bounded lookahead, progress invariants, a correct Pratt loop, dedicated syntax node types, and name resolution moved out.

### 14.2 Semantic representation

| Choice | Tradeoff |
|---|---|
| Mutate AST in place | Minimal objects, but unresolved and typed states become easy to confuse. |
| Build a separate typed HIR | More conversion code, but one canonical resolved/type-checked form and better compile-time/backend contracts. Recommended. |
| Use one arena with phase-specific side tables | Efficient and preserves syntax IDs; requires disciplined APIs. Good later optimization. |

### 14.3 Backend

- Direct fasm text from MIR is the shortest native path and keeps assembly visible.
- A C backend can deliver runnable structs/loops/functions sooner and serve as a semantic oracle while native codegen matures.
- A bytecode interpreter can make the language executable and unlock compile-time evaluation before native assembly is complete.
- LLVM/QBE can reduce machine-backend work but changes the dependency/bootstrap story.

A pragmatic dual route is possible:

1. typed HIR and MIR as shared truth;
2. a simple interpreter for semantic/runtime tests and compile-time execution;
3. fasm backend for native Linux output;
4. compare interpreter and native results in differential tests.

### 14.4 Error handling in the Go compiler

Returning `error` from every tiny parser function is not the only alternative to panic. A compiler context can collect diagnostics:

```go
type Context struct {
    Sources SourceManager
    Diags   []Diagnostic
}
```

Parser `expect` reports an error and synchronizes at `;`, `}`, or a declaration starter. Functions return invalid placeholder nodes so later phases can avoid cascades. Internal impossible states still panic. This gives multiple source errors without threading arbitrary strings through every call.

### 14.5 Bootstrapping strategy

Three viable self-hosting paths:

1. Grow the Go compiler until it compiles a Chaos rewrite; keep Go as stage 0.
2. Implement a small bytecode compiler/VM in Go, write later phases in Chaos, then add native lowering.
3. Transpile Chaos to C for the first self-hosted compiler, then use that compiler to build a native backend.

Path 1 is most direct from this tree. Whichever path is chosen, define a “bootstrap subset” much smaller than all of `language_design/` and freeze its syntax/semantics before starting the rewrite.

## 15. Prioritized implementation plan

### Phase 0 — make current results trustworthy (1-2 weeks)

1. Replace `-r` artifact guessing with explicit source/asm/binary paths.
2. Make tests declare expected success/failure and return a failing process status on mismatch.
3. Add Go unit tests for `CSMatch`/lexer EOF behavior, token positions, numeric forms, parser progress, and precedence.
4. Stop unconditional AST debug output; add `--dump-tokens`/`--dump-ast` flags.
5. Fix the small direct defects: enum string functions, IR exit fields (or remove dormant IR until redesign), file-close ordering, self-rebuild newline.

Acceptance criteria:

- `go test ./...` has meaningful tests.
- Every malformed fixture terminates quickly with a source location.
- `chaosBuild -test` returns nonzero for any unexpected result.
- One artifact naming rule is used by compiler, fasm, and runner.

### Phase 1 — stabilize the front end (2-5 weeks)

1. Add `FileID/Span` and safe rune/byte policy.
2. Correct literals, underscore handling, EOF and comments.
3. Define the supported grammar in one document.
4. Repair Pratt parsing and support full expressions in calls.
5. Ensure every parser loop either consumes a token or returns a diagnostic.
6. Redesign conditions to hold expressions and support `return;`.

Acceptance criteria:

- Golden AST tests prove precedence and associativity.
- Unterminated string/comment/block cases never panic/hang.
- Unsupported syntax is diagnosed rather than silently skipped.

### Phase 2 — resolver and type system (4-8 weeks)

1. Collect top-level symbols before resolving bodies.
2. Add lexical scope objects and stable symbol IDs.
3. Add primitive/procedure types and exact numeric literals.
4. Implement recursive expression/statement checks.
5. Implement call, return, mutability, initialization, condition, and exit rules.

Acceptance criteria:

- Recursive/forward procedure calls work according to a documented rule.
- `tests/procs_ok.chaos:2-8` reports its return mismatch.
- Binary expression result types are tested.
- Reassigning `::` and using an uninitialized variable are rejected at the correct span.

### Phase 3 — typed IR and executable baseline (6-12 weeks)

1. Create typed HIR and basic-block MIR.
2. Add an MIR verifier and textual dump.
3. Lower globals, locals, arithmetic, comparisons, branch, call, return, and exit.
4. Emit fasm with a defined x86-64 calling convention.
5. Add runtime tests that assemble and execute programs.

Acceptance criteria:

- A source procedure returns a computed exit status.
- Nested calls/recursion and conditionals produce correct runtime results.
- Interpreter (if added), MIR, assembly, and native output can be compared.

### Phase 4 — usable core language (2-4 additional months)

Add loops, pointers, arrays/slices, strings, structs, enums/results, allocator/runtime, and modules in dependency order. Keep each feature end-to-end: syntax → resolver → type rules → HIR/MIR → backend → runtime tests.

### Phase 5 — staged language and self-hosting

1. Constant evaluation and `::` semantics.
2. Compile-time function interpreter and reflection.
3. Freeze bootstrap subset and build core standard library.
4. Rewrite compiler incrementally in Chaos, preserving stage-0 Go compiler.
5. Only then choose generics/scaffolds/macros/FFI/async work based on the RFCs extracted from `language_design/`.

## 16. Recommended immediate bug-fix order

If work must continue on the current representation before the larger redesign:

1. Fix parser hangs/out-of-bounds (`P0-05`, `P0-06`) because they make fuzzing and iteration unsafe.
2. Fix test truthfulness/artifacts (`P0-12` through `P0-14`).
3. Fix Pratt parsing and floats (`P0-03`, `P0-04`).
4. Move call args to full expression parsing and make comma handling strict (`P1-08`).
5. Add recursive semantic traversal and explicit handlers for declarations, calls, returns, conditions, and scopes (`P0-07` through `P0-11`).
6. Replace dormant IR rather than expanding `IROp`.
7. Build the smallest MIR-to-fasm slice before adding any `language_design/` feature.

Avoid implementing structs, generics, errors, or macros directly in the current `Scope []Node` plus string-type model. Those features would have to be rebuilt after symbol/type/IR work.

## 17. Final assessment

The repository is an exploratory compiler prototype with a useful amount of syntax experimentation, not yet a functioning compiled language implementation. The immediate goal should be a trustworthy, small end-to-end language rather than broadening syntax:

```text
safe source handling
  -> correct AST
  -> real symbols/types
  -> typed CFG IR
  -> verified x86-64/fasm output
  -> executable tests
```

Once that path works for variables, expressions, functions, branches, loops, Bool/String, and exit/print, `language_design/` can be converted from an inconsistent idea notebook into prioritized RFCs and conformance programs. Compile-time execution should then be built on the same typed HIR/VM, not as special cases in the parser.
