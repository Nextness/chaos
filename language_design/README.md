# Language design sketches

Files in this directory record speculative Chaos syntax, library ideas, and
self-hosting experiments. They are not part of the compiler source tree and are
not expected to parse, type-check, or compile with the current implementation.

Implemented behavior is documented in the repository `README.md` and exercised
under `examples/`. A construct appearing only here—such as modules/imports,
generics, pointers, maps, async/coroutines, atomics, C interop, or a self-hosted
compiler—must be treated as design work until it has compiler and end-to-end
tests.

Do not bulk-rewrite these sketches to match the active grammar: they preserve
design history and may intentionally compare alternative syntaxes.
