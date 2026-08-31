# Language design sketches

Files in this directory record speculative Chaos syntax, library ideas, and
self-hosting experiments. They are not part of the compiler source tree and are
not expected to parse, type-check, or compile with the current implementation.

Implemented behavior is documented in the repository `README.md` and exercised
under `examples/`. Imports, generic procedures and structs, and pointers have
implemented subsets; sketches here may propose different or additional forms.
A construct appearing only here, such as maps, async/coroutines, atomics, C
interop, or a self-hosted compiler, remains design work until it has compiler
and end-to-end tests.

Do not bulk-rewrite these sketches to match the active grammar: they preserve
design history and may intentionally compare alternative syntaxes.
