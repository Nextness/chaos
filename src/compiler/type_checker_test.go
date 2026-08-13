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

func TestTypeCheckNegativeLiteralAdaptation(t *testing.T) {
	// A negated literal adapts like its positive counterpart.
	diags := typeCheckSource(t, "main :: proc {\n    a: S32 = -7;\n    b: S128 = -100;\n    c: F32 = -1.5;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// A negated literal adapts to the other operand's type in a comparison.
	diags = typeCheckSource(t, "main :: proc {\n    a: S128 = -1;\n    if a == -7 { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors comparing S128 with negated literal: %v", diags)
	}

	// A negated integer literal still cannot adapt to String.
	diags = typeCheckSource(t, "main :: proc {\n    a: String = -10;\n}")
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

func TestTypeCheckErrorDeclValid(t *testing.T) {
	// Valid error declarations, member references, equality, and
	// compile-time error constants produce no diagnostics.
	diags := typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n    NOT_FOUND;\n    OUT_OF_BOUNDS;\n}\nmain :: proc {\n    err: Hash_Table_Error = .OUT_OF_MEMORY;\n    if err == Hash_Table_Error.NOT_FOUND { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	diags = typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\ndefault_error :: Hash_Table_Error.GENERIC;")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for compile-time error const: %v", diags)
	}
}

func TestTypeCheckErrorMemberValidation(t *testing.T) {
	// Unknown member names and member access on non-error types are errors.
	diags := typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\nmain :: proc {\n    x := Hash_Table_Error.NOPE;\n}")
	if !hasError(diags, "unknown error member NOPE in Hash_Table_Error") {
		t.Errorf("expected unknown member error, got %v", diags)
	}

	diags = typeCheckSource(t, "main :: proc {\n    x := S64.NOPE;\n}")
	if !hasError(diags, "S64 is not an error type") {
		t.Errorf("expected not-an-error-type error, got %v", diags)
	}
}

func TestTypeCheckErrorNominal(t *testing.T) {
	// Error values are nominal: they assign only to their own type and
	// compare only with the same type.
	diags := typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\nmain :: proc {\n    x: S64 = Hash_Table_Error.GENERIC;\n}")
	if !hasError(diags, "cannot assign Hash_Table_Error to S64") {
		t.Errorf("expected error-to-S64 assignment error, got %v", diags)
	}

	diags = typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\nmain :: proc {\n    x: Hash_Table_Error = 5;\n}")
	if !hasError(diags, "cannot assign S64 to Hash_Table_Error") {
		t.Errorf("expected S64-to-error assignment error, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nB :: error {\n    Y;\n}\nmain :: proc {\n    if A.X == B.Y { }\n}")
	if !hasError(diags, "cannot compare A and B") {
		t.Errorf("expected cross-type comparison error, got %v", diags)
	}
}

func TestTypeCheckErrorNoOrderingOrArithmetic(t *testing.T) {
	// Error values support only == and !=; ordering and arithmetic are
	// rejected.
	diags := typeCheckSource(t, "A :: error {\n    X;\n    Y;\n}\nmain :: proc {\n    if A.X < A.Y { }\n}")
	if !hasError(diags, "cannot order error values") {
		t.Errorf("expected ordering error, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n    Y;\n}\nmain :: proc {\n    z := A.X + A.Y;\n}")
	if !hasError(diags, "cannot apply + to error values") {
		t.Errorf("expected arithmetic error, got %v", diags)
	}
}

func TestTypeCheckBareErrorMember(t *testing.T) {
	// A bare '.MEMBER' takes the declared error type; without a typed
	// context it cannot be inferred.
	diags := typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    a: A = .X;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for typed bare member: %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    a := .X;\n}")
	if !hasError(diags, "cannot infer the error type") {
		t.Errorf("expected inference error for bare member, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    a: S64 = .X;\n}")
	if !hasError(diags, "cannot assign an error member to S64") {
		t.Errorf("expected bare member to S64 error, got %v", diags)
	}
}

func TestTypeCheckErrorDuplicateMembers(t *testing.T) {
	diags := typeCheckSource(t, "A :: error {\n    X;\n    X;\n}")
	if !hasError(diags, "duplicate error member X in A") {
		t.Errorf("expected duplicate member error, got %v", diags)
	}
}

func TestTypeCheckErrorCallAndReturn(t *testing.T) {
	// Error values flow through call arguments and returns.
	diags := typeCheckSource(t, "A :: error {\n    X;\n}\nfoo :: proc (a: A) -> A {\n    return a;\n}\nmain :: proc {\n    r := foo(.X);\n    if r == A.X { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}
