package compiler

// Debug enables verbose compiler tracing used to diagnose lowering and
// code-generation issues. It is off by default so normal builds, tests, and
// the language server produce no tracing output. The chaosc CLI sets it from
// the -debug flag.
var Debug bool
