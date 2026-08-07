package compiler

import (
	"strings"
	"testing"
)

// typeCheckSource tokenizes, parses, and type-checks a source string, failing
// the test on parse errors.
func typeCheckSource(t *testing.T, source string) DiagnosticList {
	t.Helper()
	tokens, _ := Tokenize([]byte(source), 0)
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		for _, d := range result.Diags {
			t.Logf("parse diag: %s: %s", d.Severity, d.Message)
		}
		t.Fatalf("unexpected parse errors in %q", source)
	}
	return CheckProgram(result.Program)
}

func hasError(diags DiagnosticList, substr string) bool {
	for _, d := range diags {
		if d.Severity == SeverityError && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestTypeCheckLiteralInference(t *testing.T) {
	// Inferred declarations take the literal's default type: 10 -> S64,
	// 10.1 -> F64, «...» -> String, true/false -> Bool.
	diags := typeCheckSource(t, "main :: proc {\n    a := 10;\n    b := 10.1;\n    c := «hi»;\n    d := true;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// a is inferred as S64, so assigning a String to it is an error.
	diags = typeCheckSource(t, "main :: proc {\n    a := 10;\n    a = «hi»;\n}")
	if !hasError(diags, "cannot assign String to S64") {
		t.Errorf("expected String-to-S64 assignment error, got %v", diags)
	}
}

func TestTypeCheckLiteralAdaptation(t *testing.T) {
	// Literals adapt to a compatible typed target: an integer literal to any
	// integer type, a float literal to any float type.
	diags := typeCheckSource(t, "main :: proc {\n    a: U64 = 10;\n    b: S32 = 10;\n    c: F32 = 10.1;\n    d: F64 = 10.1;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// An integer literal cannot adapt to String.
	diags = typeCheckSource(t, "main :: proc {\n    a: String = 10;\n}")
	if !hasError(diags, "cannot assign S64 to String") {
		t.Errorf("expected S64-to-String error, got %v", diags)
	}
}

func TestTypeCheckAssignmentMismatch(t *testing.T) {
	// Assigning a non-literal of a different type is an error.
	diags := typeCheckSource(t, "main :: proc {\n    a: S64 = 10;\n    b: String = «hi»;\n    a = b;\n}")
	if !hasError(diags, "cannot assign String to S64") {
		t.Errorf("expected String-to-S64 assignment error, got %v", diags)
	}
}

func TestTypeCheckComparison(t *testing.T) {
	// Comparing values of the same type is fine.
	diags := typeCheckSource(t, "main :: proc {\n    a: S64 = 1;\n    b: S64 = 2;\n    if a > b { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// Comparing S32 and S64 is an error.
	diags = typeCheckSource(t, "main :: proc {\n    a: S32 = 1;\n    b: S64 = 2;\n    if a > b { }\n}")
	if !hasError(diags, "cannot compare S32 and S64") {
		t.Errorf("expected S32/S64 comparison error, got %v", diags)
	}

	// A literal adapts to the other operand's type in a comparison.
	diags = typeCheckSource(t, "main :: proc {\n    a: U64 = 1;\n    if a > 10 { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors comparing U64 with literal: %v", diags)
	}
}

func TestTypeCheckIfConditionMustBeBool(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc {\n    if 10 { }\n}")
	if !hasError(diags, "if condition must be Bool, got S64") {
		t.Errorf("expected non-bool if condition error, got %v", diags)
	}
}

func TestTypeCheckReturnType(t *testing.T) {
	// A String literal matches a String return type.
	diags := typeCheckSource(t, "f :: proc () -> String {\n    return «hi»;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// An integer literal adapts to an S64 return type.
	diags = typeCheckSource(t, "f :: proc () -> S64 {\n    return 10;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// Returning a String from an S64 procedure is an error.
	diags = typeCheckSource(t, "f :: proc () -> S64 {\n    return «hi»;\n}")
	if !hasError(diags, "cannot assign String to S64") {
		t.Errorf("expected String-to-S64 return error, got %v", diags)
	}
}

func TestTypeCheckStructLiteralFields(t *testing.T) {
	// A literal adapts to the field type.
	diags := typeCheckSource(t, "S :: struct {\n    a: String;\n    b: U64;\n}\nmain :: proc {\n    x := S.{a=«hi», b=10};\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// An integer literal cannot adapt to a String field.
	diags = typeCheckSource(t, "S :: struct {\n    a: String;\n    b: U64;\n}\nmain :: proc {\n    x := S.{a=10, b=10};\n}")
	if !hasError(diags, "cannot assign S64 to String") {
		t.Errorf("expected S64-to-String field error, got %v", diags)
	}
}

func TestTypeCheckCallArguments(t *testing.T) {
	// A literal adapts to the parameter type.
	diags := typeCheckSource(t, "f :: proc (x: S64) -> S64 {\n    return x;\n}\nmain :: proc {\n    y := f(10);\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// A String literal cannot adapt to an S64 parameter.
	diags = typeCheckSource(t, "f :: proc (x: S64) -> S64 {\n    return x;\n}\nmain :: proc {\n    y := f(«hi»);\n}")
	if !hasError(diags, "cannot assign String to S64") {
		t.Errorf("expected String-to-S64 argument error, got %v", diags)
	}
}
