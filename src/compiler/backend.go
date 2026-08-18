// Backend interface for the Chaos compiler.
//
// A backend consumes a verified MIR program and emits target assembly text.
// The MIR is target-independent, so multiple backends (fasm, gas, nasm, ...)
// can share the same lowering pipeline and differ only in how they render
// instructions, labels, and data.
package compiler

import "slices"

// Backend emits target assembly from a verified MIR program.
type Backend interface {
	// Name returns the backend's target name (for example "fasm").
	Name() string
	// Emit renders the MIR program as assembly text. Diagnostics report
	// constructs the backend does not yet support. When any error is reported,
	// the returned text is empty and must not be assembled.
	Emit(prog *MIRProgram) (string, DiagnosticList)
}

// NewBackend returns a backend by name, or nil when the name is unknown.
func NewBackend(name string) Backend {
	switch name {
	case "fasm":
		return &FasmBackend{}
	}
	return nil
}

// SupportedBackends returns the canonical, sorted backend registry.
func SupportedBackends() []string {
	backends := []string{"fasm"}
	slices.Sort(backends)
	return backends
}

// ValidateTarget checks target-specific source capabilities after semantic
// analysis and before lowering. Backend emission repeats these checks as a
// defense for callers that construct MIR directly.
func ValidateTarget(program *Program, analysis *SemanticAnalysis, target string) DiagnosticList {
	var diags DiagnosticList
	if NewBackend(target) == nil {
		diags.Error(Span{}, "unknown backend '"+target+"'", "use one of: "+joinBackendNames())
		return diags
	}
	if program == nil || analysis == nil {
		diags.Error(Span{}, "target validation requires an analyzed program", "run semantic analysis before target validation")
		return diags
	}
	checkType := func(expr Expr) {
		if expr == nil {
			return
		}
		t := analysis.TypeExprTypes[expr]
		if containsUnsupportedFasmFloat(t) {
			diags.Error(expr.nodeSpan(), "float type "+analysis.FormatType(t)+" is not yet supported by the fasm backend", "use F32 or F64")
		}
	}
	WalkAST(program, func(node Node) bool {
		switch n := node.(type) {
		case *VarDecl:
			checkType(n.DeclType)
		case *ProcDecl:
			for i := range n.Params {
				checkType(n.Params[i].Type)
			}
			for _, result := range n.Results {
				checkType(result)
			}
			checkType(n.ErrorResult)
		case *StructDecl:
			for i := range n.Fields {
				checkType(n.Fields[i].Type)
			}
		case *EnumDecl:
			for i := range n.Members {
				checkType(n.Members[i].Type)
			}
		case *ArrayInitExpr:
			checkType(n.Elem)
		}
		return true
	})
	return diags
}

func containsUnsupportedFasmFloat(t Type) bool {
	s := string(t)
	if info, ok := LookupBuiltinType(s); ok && info.Kind == BuiltinFloat && !info.FasmSupported {
		return true
	}
	if len(s) > 2 && s[:2] == "[]" {
		return containsUnsupportedFasmFloat(Type(s[2:]))
	}
	if len(s) > 1 && s[0] == '(' && s[len(s)-1] == ')' {
		for _, item := range splitSemanticTuple(s[1 : len(s)-1]) {
			if containsUnsupportedFasmFloat(Type(item)) {
				return true
			}
		}
	}
	if i := indexErrorUnion(s); i >= 0 {
		return containsUnsupportedFasmFloat(Type(s[:i])) || containsUnsupportedFasmFloat(Type(s[i+2:]))
	}
	return false
}

func splitSemanticTuple(s string) []string {
	// Current semantic types do not nest comma-bearing source type syntax
	// except tuples, whose components are already flattened by tupleType.
	var parts []string
	start := 0
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

func indexErrorUnion(s string) int {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '<' && s[i+1] == '>' {
			return i
		}
	}
	return -1
}

func joinBackendNames() string {
	names := SupportedBackends()
	if len(names) == 0 {
		return "<none>"
	}
	result := names[0]
	for _, name := range names[1:] {
		result += ", " + name
	}
	return result
}
