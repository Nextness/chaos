package compiler

import "testing"

func TestValidateTargetUsesResolvedTypeAliases(t *testing.T) {
	for _, tt := range []struct {
		name string
		src  string
		want bool
	}{
		{"supported alias", "Number :: S64;\nf :: proc (x: Number) -> Number { return x; }", false},
		{"unsupported direct", "f :: proc (x: F16) { }", true},
		{"unsupported alias", "Half :: F16;\nf :: proc (x: Half) { }", true},
		{"unsupported nested array", "f :: proc (x: []F128) { }", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tokens, _ := Tokenize([]byte(tt.src), 0)
			parsed := ParseProgram(tokens)
			analysis, semanticDiags := AnalyzeProgram(parsed.Program)
			if semanticDiags.HasErrors() {
				t.Fatalf("semantic errors: %v", semanticDiags)
			}
			diags := ValidateTarget(parsed.Program, analysis, "fasm")
			if got := diags.HasErrors(); got != tt.want {
				t.Fatalf("target error = %v, want %v: %v", got, tt.want, diags)
			}
		})
	}
}

func TestSupportedBackendsAndUnknownTarget(t *testing.T) {
	if got := SupportedBackends(); len(got) != 1 || got[0] != "fasm" {
		t.Fatalf("supported backends = %v", got)
	}
	if diags := ValidateTarget(&Program{}, &SemanticAnalysis{}, "missing"); !hasError(diags, "unknown backend 'missing'") {
		t.Fatalf("unknown backend diagnostics = %v", diags)
	}
}
