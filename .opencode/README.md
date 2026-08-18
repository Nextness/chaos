# OpenCode tooling

This directory contains repository-local OpenCode agent profiles, commands, and
plugins used while developing Chaos. It is tooling configuration, not compiler
source and not part of the emitted language runtime.

`package.json` and `package-lock.json` pin the tooling dependencies.
`.opencode/node_modules/` is generated locally and must remain untracked.
