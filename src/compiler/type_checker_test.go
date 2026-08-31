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

func TestTypeCheckUndeclaredIdentifier(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc {\n    value := missing;\n}")
	if !hasError(diags, "use of undeclared variable missing") {
		t.Errorf("expected undeclared-variable error, got %v", diags)
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
	diags := typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n    NOT_FOUND;\n    OUT_OF_BOUNDS;\n}\nmain :: proc {\n    err: Hash_Table_Error = .OUT_OF_MEMORY!;\n    if err == Hash_Table_Error.NOT_FOUND! { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	diags = typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\ndefault_error :: Hash_Table_Error.GENERIC!;")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for compile-time error const: %v", diags)
	}
}

func TestTypeCheckErrorMemberValidation(t *testing.T) {
	// Unknown member names and member access on non-error types are errors.
	diags := typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\nmain :: proc {\n    x := Hash_Table_Error.NOPE!;\n}")
	if !hasError(diags, "unknown error member NOPE in Hash_Table_Error") {
		t.Errorf("expected unknown member error, got %v", diags)
	}

	diags = typeCheckSource(t, "main :: proc {\n    x := S64.NOPE!;\n}")
	if !hasError(diags, "S64 is not an error type") {
		t.Errorf("expected not-an-error-type error, got %v", diags)
	}
}

func TestTypeCheckErrorNominal(t *testing.T) {
	// Error values are nominal: they assign only to their own type and
	// compare only with the same type.
	diags := typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\nmain :: proc {\n    x: S64 = Hash_Table_Error.GENERIC!;\n}")
	if !hasError(diags, "cannot assign Hash_Table_Error to S64") {
		t.Errorf("expected error-to-S64 assignment error, got %v", diags)
	}

	diags = typeCheckSource(t, "Hash_Table_Error :: error {\n    GENERIC;\n}\nmain :: proc {\n    x: Hash_Table_Error = 5;\n}")
	if !hasError(diags, "cannot assign S64 to Hash_Table_Error") {
		t.Errorf("expected S64-to-error assignment error, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nB :: error {\n    Y;\n}\nmain :: proc {\n    if A.X! == B.Y! { }\n}")
	if !hasError(diags, "cannot compare A and B") {
		t.Errorf("expected cross-type comparison error, got %v", diags)
	}
}

func TestTypeCheckErrorNoOrderingOrArithmetic(t *testing.T) {
	// Error values support only == and !=; ordering and arithmetic are
	// rejected.
	diags := typeCheckSource(t, "A :: error {\n    X;\n    Y;\n}\nmain :: proc {\n    if A.X! < A.Y! { }\n}")
	if !hasError(diags, "ordering is not defined for A") {
		t.Errorf("expected ordering error, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n    Y;\n}\nmain :: proc {\n    z := A.X! + A.Y!;\n}")
	if !hasError(diags, "operator + is not defined for A") {
		t.Errorf("expected arithmetic error, got %v", diags)
	}
}

func TestTypeCheckBareErrorMember(t *testing.T) {
	// A bare '.MEMBER' takes the declared error type; without a typed
	// context it cannot be inferred.
	diags := typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    a: A = .X!;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for typed bare member: %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    a := .X!;\n}")
	if !hasError(diags, "cannot infer the error type") {
		t.Errorf("expected inference error for bare member, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    a: S64 = .X!;\n}")
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
	diags := typeCheckSource(t, "A :: error {\n    X;\n}\nfoo :: proc (a: A) -> A {\n    return a;\n}\nmain :: proc {\n    r := foo(.X!);\n    if r == A.X! { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckErrorBangRequired(t *testing.T) {
	// Error literals require the trailing '!'; variables holding error
	// values do not.
	diags := typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    x := A.X;\n}")
	if !hasError(diags, "error values must be instantiated with '!'") {
		t.Errorf("expected bang-required error for explicit literal, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    x: A = .X;\n}")
	if !hasError(diags, "error values must be instantiated with '!'") {
		t.Errorf("expected bang-required error for bare literal, got %v", diags)
	}

	// Variables are fine without the bang.
	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nmain :: proc {\n    x: A = .X!;\n    y := x;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for variable assignment: %v", diags)
	}
}

func TestTypeCheckErrorReturnSpec(t *testing.T) {
	// A '<>' procedure may return either the value type or the error type.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (input1: String, input2: F64) -> (String <> Some_Error) {\n    if true {\n        return .GENERIC!;\n    }\n    return «some string»;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// Plain form without parentheses.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> String <> Some_Error {\n    return .GENERIC!;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// Swapped order: 'Some_Error <> String' is the same as 'String <> Some_Error'.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> Some_Error <> String {\n    return «s»;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for swapped order: %v", diags)
	}

	// Returning an error variable from a '<>' procedure.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (String <> Some_Error) {\n    e: Some_Error = .GENERIC!;\n    return e;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for error variable return: %v", diags)
	}
}

func TestTypeCheckErrorReturnSpecInvalid(t *testing.T) {
	// The error side of '<>' must be a declared error type.
	diags := typeCheckSource(t, "f :: proc -> String <> Nope {\n    return «s»;\n}")
	if !hasError(diags, "Nope is not an error type") {
		t.Errorf("expected undeclared error type error, got %v", diags)
	}

	// A result can carry only one error type.
	diags = typeCheckSource(t, "A :: error {\n    X;\n}\nB :: error {\n    Y;\n}\nf :: proc -> A <> B {\n    return A.X!;\n}")
	if !hasError(diags, "a result can carry only one error type") {
		t.Errorf("expected one-error-type error, got %v", diags)
	}

	// A wrong value type is rejected.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (String <> Some_Error) {\n    return 42;\n}")
	if !hasError(diags, "cannot assign S64 to String") {
		t.Errorf("expected wrong value type error, got %v", diags)
	}

	// A bare return is rejected in a '<>' procedure.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (String <> Some_Error) {\n    return;\n}")
	if !hasError(diags, "return without values") {
		t.Errorf("expected missing value error, got %v", diags)
	}
}

func TestTypeCheckErrorReturnCallType(t *testing.T) {
	// A call to a '<>' procedure has the value-or-error pair type; the value
	// is usable after the error is handled.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (String <> Some_Error) {\n    return «s»;\n}\nmain :: proc {\n    s := f() unless catch {\n        return;\n    }\n    if s == «s» { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckUnlessCatch(t *testing.T) {
	// The value is usable after 'unless catch'; the catch body must diverge.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\nmain :: proc -> S64 {\n    r := f(5) unless catch {\n        return -1;\n    }\n    return r;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}

	// The bare form discards the value.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc -> S64 {\n    f(5) unless catch {\n        return -1;\n    }\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for bare form: %v", diags)
	}
}

func TestTypeCheckUnlessCatchBinding(t *testing.T) {
	// The catch binding has the error type and can be compared against error
	// literals.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\nmain :: proc -> S64 {\n    r := f(-1) unless catch err {\n        if err == .GENERIC! {\n            return 7;\n        }\n        return 8;\n    }\n    return r;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckIfCatch(t *testing.T) {
	// Option 3: bind first, check with 'if result catch', then use the value.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\nmain :: proc -> S64 {\n    r := f(5);\n    if r catch {\n        return -1;\n    }\n    return r;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckErrorMustHandle(t *testing.T) {
	// Using the value before handling the error is rejected.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc -> S64 {\n    r := f(5);\n    return r;\n}")
	if !hasError(diags, "must handle the error before using the value") {
		t.Errorf("expected must-handle error, got %v", diags)
	}

	// Comparisons and exit status are also rejected.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc {\n    r := f(5);\n    if r == 5 { }\n}")
	if !hasError(diags, "must handle the error before using the value") {
		t.Errorf("expected must-handle error for comparison, got %v", diags)
	}

	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc {\n    r := f(5);\n    exit r;\n}")
	if !hasError(diags, "must handle the error before using the value") {
		t.Errorf("expected must-handle error for exit, got %v", diags)
	}
}

func TestTypeCheckCatchMustDiverge(t *testing.T) {
	// A catch block that falls through is rejected.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc -> S64 {\n    r := f(5) unless catch {\n    }\n    return r;\n}")
	if !hasError(diags, "the catch block must diverge") {
		t.Errorf("expected must-diverge error, got %v", diags)
	}

	// An if/else where both branches diverge satisfies the requirement.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc -> S64 {\n    r := f(5) unless catch {\n        if true {\n            return 1;\n        } else {\n            return 2;\n        }\n    }\n    return r;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for diverging if/else catch: %v", diags)
	}

	// A constant-true loop without a reachable break also diverges.
	diags = typeCheckSource(t, "Some_Error :: error { BAD; }\nf :: proc -> (S64 <> Some_Error) { return .BAD!; }\nmain :: proc -> S64 { r := f() unless catch { for true { } } return r; }")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for infinite catch loop: %v", diags)
	}
}

func TestTypeCheckMustReturnRecognizesControlFlowDivergence(t *testing.T) {
	valid := []string{
		"f :: proc -> S64 { for true { } }",
		"f :: proc -> S64 { return 1; value := 2; }",
		"f :: proc (flag: Bool) -> S64 { for true { if flag { return 1; } } }",
		"f :: proc -> S64 { for true { for true { break; } } }",
	}
	for _, source := range valid {
		if diags := typeCheckSource(t, source); diags.HasErrors() {
			t.Errorf("valid diverging procedure failed for %q: %v", source, diags)
		}
	}
	invalid := "f :: proc -> S64 { for true { break; } }"
	if diags := typeCheckSource(t, invalid); !hasError(diags, "not every path returns a value") {
		t.Fatalf("reachable break was treated as divergence: %v", diags)
	}
}

func TestTypeCheckCatchRequiresUnion(t *testing.T) {
	// 'unless catch' and 'if ... catch' require an error-returning value.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    r := 5 unless catch {\n        return -1;\n    }\n    return r;\n}")
	if !hasError(diags, "unless catch requires an error-returning expression") {
		t.Errorf("expected requires-union error, got %v", diags)
	}

	diags = typeCheckSource(t, "main :: proc -> S64 {\n    r := 5;\n    if r catch {\n        return -1;\n    }\n    return r;\n}")
	if !hasError(diags, "catch requires an error-returning value") {
		t.Errorf("expected requires-union error for if catch, got %v", diags)
	}
}

func TestTypeCheckErrorReRaise(t *testing.T) {
	// A matching value-or-error pair can be returned as-is.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\ng :: proc -> (S64 <> Some_Error) {\n    r := f(-1);\n    if r catch {\n        return r;\n    }\n    return r;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckErrorReassignAfterCatch(t *testing.T) {
	// Reassigning an unwrapped variable wraps the new value.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc -> S64 {\n    r := f(5) unless catch {\n        return -1;\n    }\n    r = 9;\n    return r;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

// ──────────────────────────────────────────────
// For loops and arrays
// ──────────────────────────────────────────────

func TestTypeCheckForLoop(t *testing.T) {
	// All three loop forms type-check cleanly.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n"+
		"    a := 0;\n"+
		"    for true { a += 1; if a == 10 then break; }\n"+
		"    for b := 0; b != 10; b += 1 { a += b; }\n"+
		"    arr := []S64.{1, 2, 3};\n"+
		"    for elem: arr { a += elem; }\n"+
		"    for idx, elem: arr { a += elem + idx; }\n"+
		"    for arr { a += #this; }\n"+
		"    for idx, elem: arr { a += #this + #index; }\n"+
		"    return a;\n"+
		"}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckForConditionType(t *testing.T) {
	// A non-Bool, non-array condition is rejected.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    x := 5;\n    for x { return 1; }\n    return 0;\n}")
	if !hasError(diags, "for condition must be Bool, got S64") {
		t.Errorf("expected non-bool for condition error, got %v", diags)
	}
}

func TestTypeCheckBreakContinueOutsideLoop(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    break;\n    return 0;\n}")
	if !hasError(diags, "break outside a loop") {
		t.Errorf("expected break-outside-loop error, got %v", diags)
	}
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    continue;\n    return 0;\n}")
	if !hasError(diags, "continue outside a loop") {
		t.Errorf("expected continue-outside-loop error, got %v", diags)
	}
}

func TestTypeCheckLoopBuiltinsOutsideRange(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    for true { return #this; }\n    return 0;\n}")
	if !hasError(diags, "'#this' is only available inside a range loop") {
		t.Errorf("expected #this-outside-range error, got %v", diags)
	}
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    x := 5;\n    return #index;\n}")
	if !hasError(diags, "'#index' is only available inside a range loop") {
		t.Errorf("expected #index-outside-range error, got %v", diags)
	}
}

func TestTypeCheckForRangeNotArray(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    x := 5;\n    for elem: x { return 1; }\n    return 0;\n}")
	if !hasError(diags, "for range requires an array") {
		t.Errorf("expected requires-array error, got %v", diags)
	}
}

func TestTypeCheckArrayLiteral(t *testing.T) {
	// Elements must match the declared element type; literals adapt.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    arr := []S64.{1, 2, 3};\n    arr2 := []String.{«a», «b»};\n    arr3: []S32 = []S32.{1};\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
	// A String element in an S64 array is an error.
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    arr := []S64.{1, «bad»};\n    return 0;\n}")
	if !hasError(diags, "cannot assign String to S64") {
		t.Errorf("expected element type error, got %v", diags)
	}
}

func TestTypeCheckIndex(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    arr := []S64.{1, 2, 3};\n    x := arr[1];\n    return x;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
	// Indexing a non-array is an error.
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    x := 5;\n    y := x[0];\n    return 0;\n}")
	if !hasError(diags, "cannot index a value of type S64") {
		t.Errorf("expected cannot-index error, got %v", diags)
	}
	// A non-integer index is an error.
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    arr := []S64.{1};\n    s := «x»;\n    y := arr[s];\n    return 0;\n}")
	if !hasError(diags, "array index must be an integer") {
		t.Errorf("expected non-integer index error, got %v", diags)
	}
}

func TestTypeCheckFixedArrayBounds(t *testing.T) {
	// A constant index out of bounds on a fixed-size array is a compile-time
	// error.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    a: [2]S64 = .{1, 2};\n    x := a[2];\n    return 0;\n}")
	if !hasError(diags, "out of bounds") {
		t.Errorf("expected out-of-bounds error, got %v", diags)
	}
	// A negative constant index is out of bounds.
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    a: [2]S64 = .{1, 2};\n    x := a[-1];\n    return 0;\n}")
	if !hasError(diags, "out of bounds") {
		t.Errorf("expected out-of-bounds error for negative index, got %v", diags)
	}
	// An in-bounds constant index is fine.
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    a: [2]S64 = .{1, 2};\n    x := a[1];\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckFixedArrayLiteralCount(t *testing.T) {
	// A fixed-size array literal must provide exactly the declared count.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    a: [2]S64 = .{1};\n    return 0;\n}")
	if !hasError(diags, "array literal has 1 elements but type declares 2") {
		t.Errorf("expected literal-count error, got %v", diags)
	}
}

func TestTypeCheckArrayFields(t *testing.T) {
	// Arrays expose data and count; dynamic arrays also expose capacity.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    a: [dyn]S64 = .{1, 2, 3};\n    n: Size = a.count;\n    c: Size = a.capacity;\n    p: *S64 = a.data;\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
	// A static array has no capacity field.
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    a: []S64 = .{1};\n    c: Size = a.capacity;\n    return 0;\n}")
	if !hasError(diags, "only dynamic arrays have a 'capacity' field") {
		t.Errorf("expected capacity error, got %v", diags)
	}
}

func TestTypeCheckSizeOf(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    if size_of(S64) != 8 { return 1; }\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
	diags = typeCheckSource(t, "main :: proc -> S64 { return size_of(Does_Not_Exist).(S64); }")
	if !hasError(diags, "undeclared variable Does_Not_Exist") {
		t.Errorf("expected unknown size_of operand error, got %v", diags)
	}
}

func TestTypeCheckGenericProc(t *testing.T) {
	// A generic procedure type-checks and instantiates at call sites.
	diags := typeCheckSource(t, "identity <T: S64 | String> :: proc (x: T) -> T { return x; }\nmain :: proc -> S64 {\n    a := identity(42);\n    s := identity(«hi»);\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
	// A type argument outside the constraints is rejected.
	diags = typeCheckSource(t, "identity <T: S64 | String> :: proc (x: T) -> T { return x; }\nmain :: proc -> S64 {\n    b := identity(true);\n    return 0;\n}")
	if !hasError(diags, "does not satisfy the constraints") {
		t.Errorf("expected constraint error, got %v", diags)
	}
}

func TestTypeCheckGenericProcRequiresCompleteInference(t *testing.T) {
	diags := typeCheckSource(t, "first <T: S64, U: String> :: proc (value: T) -> T { return value; }\nmain :: proc -> S64 { return first(1); }")
	if !hasError(diags, "cannot infer type arguments for generic procedure first") {
		t.Errorf("expected incomplete inference error, got %v", diags)
	}
}

func TestTypeCheckRecursiveGenericProcTerminates(t *testing.T) {
	src := "countdown <T: S64> :: proc (n: T) -> T { if n == 0 { return n; } return countdown(n - 1); }\nmain :: proc -> S64 { return countdown(5); }"
	if diags := typeCheckSource(t, src); diags.HasErrors() {
		t.Fatalf("recursive generic produced errors: %v", diags)
	}
}

func TestTypeCheckGenericStruct(t *testing.T) {
	diags := typeCheckSource(t, "Box <T: S64 | String> :: struct { v: T; }\nmain :: proc -> S64 {\n    b: Box = .{v=7};\n    if b.v == 7 { return 0; }\n    return 1;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckGenericStructReusesConcreteType(t *testing.T) {
	src := "Box <T: S64 | String> :: struct { v: T; }\nmain :: proc -> S64 { a: Box = .{v=1}; b: Box = .{v=2}; a = b; return a.v; }"
	if diags := typeCheckSource(t, src); diags.HasErrors() {
		t.Fatalf("same generic struct instance should be assignable: %v", diags)
	}
}

func TestTypeCheckGenericStructRequiresCompleteInference(t *testing.T) {
	src := "Pair <T: S64, U: String> :: struct { first: T; second: U; }\nmain :: proc -> S64 { p: Pair = .{first=1}; return p.first; }"
	if diags := typeCheckSource(t, src); !hasError(diags, "cannot infer type parameter 'U'") {
		t.Fatalf("expected incomplete generic struct inference error, got %v", diags)
	}
}

func TestTypeCheckElifUsesFalsePathState(t *testing.T) {
	src := "f :: proc (take: Bool) -> S64 { x: S64 = ...; if take { x = 1; } elif x == 1 { return x; } return 0; }"
	if diags := typeCheckSource(t, src); !hasError(diags, "is not initialized") {
		t.Fatalf("expected elif false-path initialization error, got %v", diags)
	}
}

func TestTypeCheckRejectsIntegerMinusPointer(t *testing.T) {
	src := "main :: proc -> S64 { x := 1; p: *S64 = *x; q := 2 - p; return 0; }"
	if diags := typeCheckSource(t, src); !hasError(diags, "cannot subtract a pointer from an integer") {
		t.Fatalf("expected integer-minus-pointer error, got %v", diags)
	}
}

func TestTypeCheckRequiresNonNullPointerInitialization(t *testing.T) {
	tests := []struct {
		name, source, message string
	}{
		{"local pointer", "main :: proc -> S64 { p: *S64; return p.*; }", "requires an initializer"},
		{"global pointer", "p: *S64; main :: proc -> S64 { return p.*; }", "requires an initializer"},
		{"plain pointer struct", "Holder :: struct { value: *S64; } main :: proc -> S64 { h: Holder; return 0; }", "contains a non-nullable pointer"},
		{"omitted pointer field", "Holder :: struct { value: *S64; tag: S64; } main :: proc -> S64 { x := 1; h: Holder = .{tag=2}; return 0; }", "field 'value' in struct Holder requires a non-null initializer"},
		{"omitted builtin pointer field", "main :: proc -> S64 { s: String = .{count=0}; return 0; }", "field 'data' in String requires a non-null initializer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diags := typeCheckSource(t, tt.source); !hasError(diags, tt.message) {
				t.Fatalf("expected %q diagnostic, got %v", tt.message, diags)
			}
		})
	}
	for _, source := range []string{
		"main :: proc -> S64 { x := 7; p: *S64 = *x; return p.*; }",
		"main :: proc -> S64 { x := 7; p: *S64 = ...; p = *x; return p.*; }",
		"Holder :: struct { value: *S64; tag: S64; } main :: proc -> S64 { x := 7; h: Holder = .{value=*x, tag=2}; return h.value.*; }",
	} {
		if diags := typeCheckSource(t, source); diags.HasErrors() {
			t.Fatalf("valid initialized pointer source produced errors: %v", diags)
		}
	}
}

func TestTypeCheckNullablePointerIndexRequiresProof(t *testing.T) {
	for _, operation := range []string{
		"value := p[0];",
		"p[0] = 1;",
		"p[0] += 1;",
		"p[0]++;",
		"address := *p[0];",
	} {
		source := "main :: proc -> S64 { p: *S64? = null; " + operation + " return 0; }"
		if diags := typeCheckSource(t, source); !hasError(diags, "cannot index 'p' because it may be null") {
			t.Fatalf("operation %q should require a non-null proof, got %v", operation, diags)
		}
	}
	source := "main :: proc (p: *S64?) -> S64 { if p == null { return 0; } value := p[0]; p[0] = value + 1; return p[0]; }"
	if diags := typeCheckSource(t, source); diags.HasErrors() {
		t.Fatalf("narrowed nullable pointer index produced errors: %v", diags)
	}
}

func TestTypeCheckGenericStructPositional(t *testing.T) {
	// Positional fields must match the declaration-order fields when inferring
	// type arguments for a generic struct.
	diags := typeCheckSource(t, "Pair <T: S64 | String> :: struct { first: T; second: T; }\nmain :: proc -> S64 {\n    p: Pair = .{7, 9};\n    if p.first == 7 && p.second == 9 { return 0; }\n    return 1;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckGenericProcStructType(t *testing.T) {
	// A generic procedure whose constraint references a user-defined struct
	// type must resolve (the instantiation mapping rebuilds nominal type
	// expressions from their declared names).
	src := "Token :: struct { kind: S64; text: String; }\nidentity <T: S64 | String | Token> :: proc (x: T) -> T { return x; }\nmain :: proc -> S64 {\n    t: Token;\n    t.kind = 0;\n    r := identity(t);\n    if r.kind == 0 { return 0; }\n    return 1;\n}"
	if diags := typeCheckSource(t, src); diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
}

func TestTypeCheckIncDecType(t *testing.T) {
	// Increment/decrement requires an integer variable.
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    s := «x»;\n    s++;\n    return 0;\n}")
	if !hasError(diags, "cannot increment or decrement a value of type String") {
		t.Errorf("expected inc-dec type error, got %v", diags)
	}
}

func TestTypeCheckCompoundAssignType(t *testing.T) {
	diags := typeCheckSource(t, "main :: proc -> S64 {\n    a := 5;\n    a += «x»;\n    return 0;\n}")
	if !hasError(diags, "cannot assign String to S64") {
		t.Errorf("expected compound assign type error, got %v", diags)
	}
}

// ──────────────────────────────────────────────
// Void type
// ──────────────────────────────────────────────

func TestTypeCheckVoidReturn(t *testing.T) {
	// A 'Void <> E' procedure may return bare (success) or an error value.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (input1: String) -> (Void <> Some_Error) {\n    if input1 == «» {\n        return;\n    }\n    return .GENERIC!;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors: %v", diags)
	}
	// The reversed 'E <> Void' form is equivalent.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (Some_Error <> Void) {\n    return .GENERIC!;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for reversed form: %v", diags)
	}
	// An explicit '-> Void' result with a bare return is valid.
	diags = typeCheckSource(t, "f :: proc -> Void {\n    return;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for -> Void: %v", diags)
	}
}

func TestTypeCheckVoidInvalidPositions(t *testing.T) {
	// Void is only valid as a function result type.
	cases := []struct {
		name, src, want string
	}{
		{"struct field", "Invalid :: struct { param1: String; param2: Void; }", "Void is only valid as a function result type"},
		{"variable", "main :: proc { x: Void; }", "Void is only valid as a function result type"},
		{"inferred variable", "f :: proc -> Void { }\nmain :: proc { x := f(); }", "Void is only valid as a function result type"},
		{"parameter", "f :: proc (x: Void) { }", "Void is only valid as a function result type"},
		{"array element", "main :: proc { arr := []Void.{ }; }", "Void is only valid as a function result type"},
		{"nested result", "f :: proc -> []Void { }", "Void is only valid as a function result type"},
		{"nested error result", "Some_Error :: error { BAD; }\nf :: proc -> ([]Void <> Some_Error) { return; }", "Void is only valid as a function result type"},
		{"multiple results", "f :: proc -> (Void, S64) { }", "Void must be the only function result"},
	}
	for _, c := range cases {
		diags := typeCheckSource(t, c.src)
		if !hasError(diags, c.want) {
			t.Errorf("%s: expected %q error, got %v", c.name, c.want, diags)
		}
	}
}

func TestTypeCheckVoidBindRejected(t *testing.T) {
	// Binding the value of a 'Void <> E' call is rejected; only the bare
	// 'unless catch' form is valid.
	diags := typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (Void <> Some_Error) {\n    return .GENERIC!;\n}\nmain :: proc -> S64 {\n    x := f() unless catch {\n        return -1;\n    }\n    return 0;\n}")
	if !hasError(diags, "cannot bind a Void value") {
		t.Errorf("expected cannot-bind-Void error, got %v", diags)
	}
	// The bare form is valid.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (Void <> Some_Error) {\n    return .GENERIC!;\n}\nmain :: proc -> S64 {\n    f() unless catch {\n        return -1;\n    }\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for bare form: %v", diags)
	}
}

// ──────────────────────────────────────────────
// Shadowing
// ──────────────────────────────────────────────

func TestTypeCheckShadowingRejected(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"parameter", "f :: proc (input1: String) -> String {\n    input1 := «x»;\n    return input1;\n}", "shadows an existing name"},
		{"global compile-time", "SOME_VAR_1 :: 10;\nf :: proc -> S64 {\n    SOME_VAR_1 :: SOME_VAR_1;\n    return 0;\n}", "shadows an existing name"},
		{"global runtime", "SOME_VAR_3 := 10;\nf :: proc -> S64 {\n    SOME_VAR_3 := SOME_VAR_3;\n    return 0;\n}", "shadows an existing name"},
		{"same scope", "main :: proc -> S64 {\n    x := 1;\n    x := 2;\n    return x;\n}", "shadows an existing name"},
		{"enclosing block", "main :: proc -> S64 {\n    x := 1;\n    {\n        x := 2;\n    }\n    return x;\n}", "shadows an existing name"},
		{"range binding", "main :: proc -> S64 {\n    arr := []S64.{1};\n    x := 5;\n    for x: arr { }\n    return 0;\n}", "shadows an existing name"},
		{"unless catch target", "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (S64 <> Some_Error) {\n    return 1;\n}\nmain :: proc -> S64 {\n    x := 5;\n    x := f() unless catch {\n        return -1;\n    }\n    return x;\n}", "shadows an existing name"},
		{"catch binding", "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (S64 <> Some_Error) {\n    return 1;\n}\nmain :: proc -> S64 {\n    x := 5;\n    r := f() unless catch x {\n        return -1;\n    }\n    return r;\n}", "shadows an existing name"},
		{"struct type", "Point :: struct { x: S64; }\nmain :: proc -> S64 {\n    Point := 5;\n    return 0;\n}", "shadows an existing name"},
		{"error type", "Some_Error :: error {\n    GENERIC;\n}\nmain :: proc -> S64 {\n    Some_Error := 5;\n    return 0;\n}", "shadows an existing name"},
		{"procedure", "helper :: proc -> S64 { return 1; }\nmain :: proc -> S64 { helper := 2; return helper; }", "shadows an existing name"},
		{"struct definition", "Point :: struct { x: S64; }\nPoint :: struct { y: S64; }", "shadows an existing name"},
		{"error definition", "Some_Error :: error { ONE; }\nSome_Error :: error { TWO; }", "shadows an existing name"},
		{"procedure definition", "helper :: proc { }\nhelper :: proc { }", "shadows an existing name"},
	}
	for _, c := range cases {
		diags := typeCheckSource(t, c.src)
		if !hasError(diags, c.want) {
			t.Errorf("%s: expected %q error, got %v", c.name, c.want, diags)
		}
	}
}

func TestTypeCheckShadowDirectiveAllows(t *testing.T) {
	// '#shadow' explicitly allows reusing an outer name.
	diags := typeCheckSource(t, "f :: proc (input1: String) -> String {\n    #shadow input1 := «x»;\n    return input1;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for #shadow param: %v", diags)
	}
	diags = typeCheckSource(t, "SOME_VAR_1 :: 10;\nf :: proc -> S64 {\n    #shadow SOME_VAR_1 :: SOME_VAR_1;\n    return SOME_VAR_1;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for #shadow global: %v", diags)
	}
	diags = typeCheckSource(t, "main :: proc -> S64 {\n    x := 1;\n    #shadow x := 2;\n    return x;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for #shadow same scope: %v", diags)
	}
	diags = typeCheckSource(t, "value :: 1;\n#shadow value :: value + 1;\nmain :: proc -> S64 { return value; }")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for top-level #shadow: %v", diags)
	}
	diags = typeCheckSource(t, "helper :: proc -> S64 { return 1; }\nmain :: proc -> S64 { #shadow helper := 2; return helper; }")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for explicit procedure-name shadow: %v", diags)
	}
	// '#shadow' applies uniformly to type/procedure/value declarations.
	diags = typeCheckSource(t, "Point :: struct { x: S64; }\nmain :: proc -> S64 {\n    #shadow Point := 5;\n    return 0;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected explicit struct-name shadow error: %v", diags)
	}
}

func TestTypeCheckParamPlaceholderNotShadowed(t *testing.T) {
	// A parameter name is a placeholder and does not shadow a global.
	diags := typeCheckSource(t, "var := 10;\nprocedure :: proc (var: S64) -> S64 {\n    return var;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for param placeholder: %v", diags)
	}
}

func TestTypeCheckShadowTypeConflictIsOrderIndependent(t *testing.T) {
	diags := typeCheckSource(t, "Point := 1;\nPoint :: struct { x: S64; }")
	if !hasError(diags, "shadows an existing name") {
		t.Fatalf("expected type shadow error when variable appears first, got %v", diags)
	}
}

func TestTypeCheckTopLevelShadowInitializerUsesPreviousType(t *testing.T) {
	diags := typeCheckSource(t, "value: String = «outer»;\n#shadow value: S64 = value;")
	if !hasError(diags, "cannot assign String to S64") {
		t.Fatalf("expected shadow initializer to use previous String binding, got %v", diags)
	}
}

func TestTypeCheckTopLevelProcedureShadowAllowedExplicitly(t *testing.T) {
	diags := typeCheckSource(t, "helper :: proc -> S64 { return 1; }\n#shadow helper :: 2;")
	if diags.HasErrors() {
		t.Fatalf("unexpected explicit procedure shadow error: %v", diags)
	}
}

func TestTypeCheckEnumValueRanges(t *testing.T) {
	valid := `
Signed8 :: enum { MIN: S8 = -128; MAX = 127; }
Unsigned8 :: enum { MIN: U8 = 0; MAX = 255; }
Signed64 :: enum { MIN: S64 = -9223372036854775808; MAX = 9223372036854775807; }
Signed :: enum { MIN: S128 = -170141183460469231731687303715884105728; }
Unsigned :: enum { MAX: U128 = 340282366920938463463374607431768211455; }
U64_Max :: enum { MAX: U64 = 18446744073709551615; }
main :: proc { }
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("unexpected enum range errors: %v", diags)
	}

	tests := []struct {
		name string
		src  string
		want string
	}{
		{"s8 explicit underflow", "E :: enum { A: S8 = -129; }", "does not fit S8"},
		{"s8 sequential overflow", "E :: enum { A: S8 = 127; B; }", "does not fit S8"},
		{"u8 negative", "E :: enum { A: U8 = -1; }", "does not fit U8"},
		{"u8 sequential overflow", "E :: enum { A: U8 = 255; B; }", "does not fit U8"},
		{"s64 explicit overflow", "E :: enum { A: S64 = 9223372036854775808; }", "does not fit S64"},
		{"s64 sequential overflow", "E :: enum { A: S64 = 9223372036854775807; B; }", "does not fit S64"},
		{"u64 overflow", "E :: enum { A: U64 = 18446744073709551616; }", "does not fit U64"},
		{"u64 sequential overflow", "E :: enum { A: U64 = 18446744073709551615; B; }", "does not fit U64"},
		{"s128 underflow", "E :: enum { A: S128 = -170141183460469231731687303715884105729; }", "does not fit S128"},
		{"s128 sequential overflow", "E :: enum { A: S128 = 170141183460469231731687303715884105727; B; }", "does not fit S128"},
		{"u128 overflow", "E :: enum { A: U128 = 340282366920938463463374607431768211456; }", "does not fit U128"},
		{"u128 sequential overflow", "E :: enum { A: U128 = 340282366920938463463374607431768211455; B; }", "does not fit U128"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := typeCheckSource(t, tt.src)
			if !hasError(diags, tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, diags)
			}
		})
	}
}

func TestTypeCheckOrdinaryIntegerLiteralRanges(t *testing.T) {
	tests := []struct {
		typeName, min, max, below, above string
	}{
		{"S8", "-128", "127", "-129", "128"},
		{"U8", "0", "255", "-1", "256"},
		{"S16", "-32768", "32767", "-32769", "32768"},
		{"U16", "0", "65535", "-1", "65536"},
		{"S32", "-2147483648", "2147483647", "-2147483649", "2147483648"},
		{"U32", "0", "4294967295", "-1", "4294967296"},
		{"S64", "-9223372036854775808", "9223372036854775807", "-9223372036854775809", "9223372036854775808"},
		{"U64", "0", "18446744073709551615", "-1", "18446744073709551616"},
		{"S128", "-170141183460469231731687303715884105728", "170141183460469231731687303715884105727", "-170141183460469231731687303715884105729", "170141183460469231731687303715884105728"},
		{"U128", "0", "340282366920938463463374607431768211455", "-1", "340282366920938463463374607431768211456"},
	}
	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			valid := "main :: proc { low: " + tt.typeName + " = " + tt.min + "; high: " + tt.typeName + " = " + tt.max + "; }"
			if diags := typeCheckSource(t, valid); diags.HasErrors() {
				t.Fatalf("valid boundaries failed: %v", diags)
			}
			for _, value := range []string{tt.below, tt.above} {
				source := "main :: proc { value: " + tt.typeName + " = " + value + "; }"
				if diags := typeCheckSource(t, source); !hasError(diags, "does not fit "+tt.typeName) {
					t.Fatalf("out-of-range %s accepted: %v", value, diags)
				}
			}
		})
	}
}

func TestTypeCheckRejectsEnumArithmetic(t *testing.T) {
	prefix := "E :: enum { A: S64 = 1; B = 2; }\nmain :: proc {\n    x: E = E.A;\n"
	tests := []string{
		"    y: E = E.A + E.B;\n",
		"    y: E = -E.A;\n",
		"    x += E.B;\n",
	}
	for _, body := range tests {
		diags := typeCheckSource(t, prefix+body+"}")
		if !diags.HasErrors() {
			t.Errorf("expected enum arithmetic error for %q, got %v", body, diags)
		}
	}
}

func TestTypeCheckDefiniteInitializationAcrossControlFlow(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "both branches initialize",
			src:  "f :: proc (flag: Bool) -> S64 { x: S64 = ...; if flag { x = 1; } else { x = 2; } return x; }",
		},
		{
			name: "one branch initializes",
			src:  "f :: proc (flag: Bool) -> S64 { x: S64 = ...; if flag { x = 1; } return x; }",
			want: true,
		},
		{
			name: "loop may execute zero times",
			src:  "f :: proc -> S64 { x: S64 = ...; for false { x = 1; } return x; }",
			want: true,
		},
		{
			name: "c loop initializer executes",
			src:  "f :: proc -> S64 { x: S64 = ...; for x = 1; false; x += 1 { } return x; }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := typeCheckSource(t, tt.src)
			if got := hasError(diags, "is not initialized"); got != tt.want {
				t.Fatalf("uninitialized diagnostic = %v, want %v: %v", got, tt.want, diags)
			}
		})
	}
}

func TestTypeCheckNestedDeclarationsAndNoCapture(t *testing.T) {
	valid := `
outer :: proc (input: S64) -> S64 {
    C :: 2;
    Local :: struct { value: S64 = C; }
    inner :: proc (value: S64) -> S64 { return value + C; }
    item: Local = .{};
    return inner(input);
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid nested declarations failed: %v", diags)
	}

	capture := `
outer :: proc (input: S64) -> S64 {
    inner :: proc -> S64 { return input; }
    return inner();
}
`
	if diags := typeCheckSource(t, capture); !hasError(diags, "cannot implicitly capture runtime binding 'input'") {
		t.Fatalf("expected no-capture diagnostic, got %v", diags)
	}

	beforeDeclaration := `
outer :: proc -> S64 {
    value := inner();
    inner :: proc -> S64 { return 1; }
    return value;
}
`
	if diags := typeCheckSource(t, beforeDeclaration); !hasError(diags, "call to unknown procedure inner") {
		t.Fatalf("expected source-order diagnostic, got %v", diags)
	}
}

func TestTypeCheckShadowedNominalAndProcedureIdentity(t *testing.T) {
	source := `
outer :: proc -> S64 {
    Item :: struct { old: S64; }
    first: Item = .{};
    helper :: proc -> S64 { return 1; }
    old_helper :: helper;
    #shadow Item :: struct { newer: S64; }
    #shadow helper :: proc -> S64 { return 2; }
    second: Item = first;
    return old_helper() + helper();
}
`
	diags := typeCheckSource(t, source)
	if !hasError(diags, "cannot assign Item to Item") {
		t.Fatalf("shadowed nominal types compared equal: %v", diags)
	}
}

func TestSemanticAnalysisFormatsStableNominalTypes(t *testing.T) {
	tokens, _ := Tokenize([]byte("Item :: struct { x: S64; }\nvalue: Item = .{};"), 0)
	parsed := ParseProgram(tokens)
	analysis, diags := AnalyzeProgram(parsed.Program)
	if diags.HasErrors() {
		t.Fatalf("analysis failed: %v", diags)
	}
	value := parsed.Program.Decls[1].(*VarDecl)
	if got := analysis.FormatType(analysis.DeclTypes[value]); got != "Item" {
		t.Fatalf("formatted type = %q, want Item", got)
	}
}

func TestTypeCheckForwardCompileTimeStructDefault(t *testing.T) {
	valid := "Item :: struct { value: S64 = LATER; }\nFIRST :: Item.{};\nLATER :: 42;"
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("forward compile-time default failed: %v", diags)
	}
	invalid := "Item :: struct { value: S64 = LATER; }\nLATER :: «wrong»;"
	if diags := typeCheckSource(t, invalid); !hasError(diags, "cannot assign String to S64") {
		t.Fatalf("expected default type mismatch, got %v", diags)
	}
}

func TestSemanticAnalysisProvidesGlobalDependencyOrder(t *testing.T) {
	tokens, _ := Tokenize([]byte("FIRST :: LATER + 1;\nLATER :: 41;"), 0)
	parsed := ParseProgram(tokens)
	analysis, diags := AnalyzeProgram(parsed.Program)
	if diags.HasErrors() {
		t.Fatalf("analysis failed: %v", diags)
	}
	if len(analysis.GlobalOrder) != 2 || analysis.GlobalOrder[0].Name != "LATER" || analysis.GlobalOrder[1].Name != "FIRST" {
		t.Fatalf("global order = %+v, want LATER then FIRST", analysis.GlobalOrder)
	}
}

func TestTypeCheckCompileTimeIntegerEvaluation(t *testing.T) {
	valid := `
base :: 100;
small : S8 : base + 27;
runtime_wrap :: proc -> S8 { value: S8 = 127 + 1; return value; }
runtime_divide :: proc (divisor: S64) -> S64 { return 1 / divisor; }
runtime_float :: proc -> F32 { value: F32 = 1.25 + 2.5; return value; }
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid constant/runtime arithmetic failed: %v", diags)
	}

	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "typed overflow", src: "bad : S8 : 127 + 1;", want: "compile-time integer value 128 does not fit S8"},
		{name: "inferred overflow", src: "bad :: 9223372036854775807 + 1;", want: "compile-time integer value 9223372036854775808 does not fit S64"},
		{name: "division by zero", src: "bad :: 10 / 0;", want: "division by zero in compile-time expression"},
		{name: "modulo by zero", src: "bad :: 10 % 0;", want: "modulo by zero in compile-time expression"},
		{name: "float division by zero", src: "bad :: 10.0 / 0.0;", want: "division by zero in compile-time expression"},
		{name: "typed float overflow", src: "bad : F32 : 340282346638528859811704183484516925440.0 * 2.0;", want: "compile-time floating-point value does not fit F32"},
		{name: "array element overflow", src: "bad : []S8 : []S8.{127 + 1};", want: "compile-time integer value 128 does not fit S8"},
		{name: "struct field overflow", src: "Item :: struct { value: S8; }\nbad :: Item.{127 + 1};", want: "compile-time integer value 128 does not fit S8"},
		{name: "struct default division by zero", src: "Item :: struct { value: S64 = 10 / 0; }", want: "division by zero in compile-time expression"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diags := typeCheckSource(t, tt.src); !hasError(diags, tt.want) {
				t.Fatalf("diagnostics = %v, want %q", diags, tt.want)
			}
		})
	}
}

func TestTypeCheckCompileTimeTypeAliases(t *testing.T) {
	valid := `
Number :: Later;
Later :: S64;
identity :: proc (value: Number) -> Number { return value; }
outer :: proc -> S64 {
    Local :: Number;
    value: Local = identity(42);
    return value;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid type aliases failed: %v", diags)
	}
	invalid := "Not_A_Type :: 42;\nf :: proc (value: Not_A_Type) { }"
	if diags := typeCheckSource(t, invalid); !hasError(diags, "unknown type name 'Not_A_Type'") {
		t.Fatalf("runtime value was accepted as a type alias: %v", diags)
	}
}

// ──────────────────────────────────────────────
// Ifx expressions
// ──────────────────────────────────────────────

func TestTypeCheckIfxInference(t *testing.T) {
	valid := `
main :: proc {
    a := ifx true then 1 else 2;
    b := ifx false 10 else 20;
    s := ifx true then «hi» else «bye»;
    f := ifx false then 1.5 else 2.5;
    bo := ifx true then true else false;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid ifx declarations failed: %v", diags)
	}

	// Different non-literal branch types are rejected.
	if diags := typeCheckSource(t, "main :: proc { x := ifx true then 1 else «two»; }"); !hasError(diags, "ifx branches must have the same type, got S64 and String") {
		t.Fatalf("branch mismatch was not reported: %v", diags)
	}
	// A literal branch adapts to the other branch's type.
	if diags := typeCheckSource(t, "main :: proc { x: U8 = 1; y := ifx true then 1 else x; }"); diags.HasErrors() {
		t.Fatalf("literal branch did not adapt to U8: %v", diags)
	}
}

func TestTypeCheckIfxTypedTarget(t *testing.T) {
	valid := `
main :: proc {
    a: S64 = ifx true then 1 else 2;
    b: U8 = ifx false then 200 else 100;
    c: String = ifx true then «yes» else «no»;
    d := 0;
    d = ifx true then 5 else 6;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid typed ifx failed: %v", diags)
	}

	if diags := typeCheckSource(t, "main :: proc { x: S64 = ifx true then 1 else «two»; }"); !hasError(diags, "cannot assign String to S64") {
		t.Fatalf("typed branch mismatch was not reported: %v", diags)
	}
}

func TestTypeCheckIfxReturns(t *testing.T) {
	valid := `
main :: proc -> S64 {
    return ifx true then 1 else 2;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid ifx return failed: %v", diags)
	}

	invalid := "wrong :: proc -> S64 { return ifx true then 1 else «two»; }"
	if diags := typeCheckSource(t, invalid); !hasError(diags, "cannot assign String to S64") {
		t.Fatalf("invalid ifx return was not reported: %v", diags)
	}
}

func TestTypeCheckIfxConditionMustBeBool(t *testing.T) {
	if diags := typeCheckSource(t, "main :: proc { x := ifx 5 then 1 else 2; }"); !hasError(diags, "ifx condition must be Bool, got S64") {
		t.Fatalf("non-Bool condition was not reported: %v", diags)
	}
	if diags := typeCheckSource(t, "main :: proc { x := ifx true then 1 else 2; }"); diags.HasErrors() {
		t.Fatalf("Bool condition rejected: %v", diags)
	}
}

func TestTypeCheckIfxOnlyAssignmentOrReturn(t *testing.T) {
	want := "ifx expressions are only supported as the value of an assignment or return"
	rejected := []string{
		"main :: proc { x := 1 + ifx true then 2 else 3; }",                                                      // binary operand
		"foo :: proc (v: S64) { }\nmain :: proc { foo(ifx true then 1 else 2); }",                                // call argument
		"main :: proc { a := []S64.{ifx true then 1 else 2}; }",                                                  // array item
		"main :: proc { ifx true then 1 else 2; }",                                                               // bare expression statement
		"main :: proc { x: S64 = 0; x += ifx true then 1 else 2; }",                                              // compound assignment
		"foo :: proc (v: S64) -> S64 { return v; }\nmain :: proc -> S64 { return foo(ifx true then 1 else 2); }", // call argument of a return operand
	}
	for _, src := range rejected {
		if diags := typeCheckSource(t, src); !hasError(diags, want) {
			t.Fatalf("ifx misuse was not rejected in %q: %v", src, diags)
		}
	}
}

func TestTypeCheckIfxNestedRejected(t *testing.T) {
	rejected := []string{
		// Nested in a branch.
		"main :: proc { x := ifx true then ifx false then 1 else 2 else 3; }",
		// Nested in the condition.
		"main :: proc { x := ifx (ifx true then true else false) then 1 else 2; }",
	}
	for _, src := range rejected {
		if diags := typeCheckSource(t, src); !hasError(diags, "ifx expressions are only supported as the value of an assignment or return") {
			t.Fatalf("nested ifx was not rejected in %q: %v", src, diags)
		}
	}
}

func TestTypeCheckIfxParenthesized(t *testing.T) {
	valid := `
main :: proc {
    x := (ifx true then 1 else 2);
    y: S64 = (ifx false then 3 else 4);
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("parenthesized ifx rejected: %v", diags)
	}
}

func TestTypeCheckIfxCompileTime(t *testing.T) {
	valid := `
A :: ifx true then 42 else 7;
B :: ifx false then 1 else 2;
C :: ifx 5 > 3 then 42 else 7;
D :: ifx (true && false) then 1 else 2;
main :: proc -> S64 { return A + B + C + D; }
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid compile-time ifx failed: %v", diags)
	}

	// A runtime value cannot appear in a compile-time ifx.
	invalid := "main :: proc { x: S64 = 1; y :: ifx x > 0 then 1 else 2; }"
	if diags := typeCheckSource(t, invalid); !hasError(diags, "compile-time assignment requires a compile-time-known expression") {
		t.Fatalf("runtime ifx condition in compile-time binding was not reported: %v", diags)
	}

	// A branch that overflows the target is caught at compile time.
	overflow := "bad : S8 : ifx true then 127 + 1 else 0;"
	if diags := typeCheckSource(t, overflow); !hasError(diags, "compile-time integer value 128 does not fit S8") {
		t.Fatalf("branch overflow was not reported: %v", diags)
	}
}

func TestTypeCheckPointersValid(t *testing.T) {
	valid := `
Person :: struct { name: String; age: S64; next: *Person?; }
bump :: proc (p: *S64) -> S64 { return p.*; }
pick :: proc (p: *S64?) -> *S64? {
    if p == null { return null; }
    return p;
}
#entry main :: proc -> S64 {
    a := 10;
    b: *S64 = *a;
    c := b.*;
    b.* = 42;
    person := Person.{name=«x», age=7};
    pp: *Person = *person;
    pp.*.age = 9;
    if pp.*.age == 9 && c == 42 { return 0; }
    n: *S64? = null;
    if n == null { return 1; }
    return 2;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid pointer program failed: %v", diags)
	}
}

func TestTypeCheckPointerArithmetic(t *testing.T) {
	valid := `
#entry main :: proc -> S64 {
	arr := []S64.{10, 20, 30};
	p: *S64 = *arr[0];
	q := p + 2;
	back := q - 1;
	if back == p && q > p && p < q && q != p { return 0; }
	return 1;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid pointer arithmetic failed: %v", diags)
	}

	// A non-nullable pointer cannot hold null.
	nn := "#entry main :: proc -> S64 { a: *S64 = null; return 0; }"
	if diags := typeCheckSource(t, nn); !hasError(diags, "cannot assign null to a non-nullable pointer") {
		t.Fatalf("null to non-nullable pointer was not reported: %v", diags)
	}
}

func TestTypeCheckPointerNullFlow(t *testing.T) {
	// A nullable pointer must be proved non-null before dereference or before
	// being passed to a non-nullable parameter.
	valid := `
use :: proc (p: *S64) -> S64 { return p.*; }
#entry main :: proc -> S64 {
	p: *S64? = null;
	if p == null { return 1; }
	x := p.*;
	y := use(p);
	return x + y;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("null-checked pointer use failed: %v", diags)
	}

	dang := `
use :: proc (p: *S64) -> S64 { return p.*; }
#entry main :: proc -> S64 {
	p: *S64? = null;
	return use(p);
}
`
	if diags := typeCheckSource(t, dang); !hasError(diags, "cannot assign the nullable pointer") {
		t.Fatalf("nullable into non-nullable parameter was not reported: %v", diags)
	}

	unchecked := "#entry main :: proc -> S64 { p: *S64? = null; return p.*; }"
	if diags := typeCheckSource(t, unchecked); !hasError(diags, "cannot dereference") {
		t.Fatalf("deref of unchecked nullable pointer was not reported: %v", diags)
	}
}

func TestTypeCheckPointerMisc(t *testing.T) {
	// Scalar to pointer and pointer to scalar are rejected.
	bad := "#entry main :: proc -> S64 { a: *S64 = 10; return 0; }"
	if diags := typeCheckSource(t, bad); !hasError(diags, "cannot assign") {
		t.Fatalf("scalar literal to pointer was not reported: %v", diags)
	}

	// Non-nullable pointers cannot be compared to null.
	nn := "#entry main :: proc -> S64 { a: *S64; if a == null { return 0; } return 1; }"
	if diags := typeCheckSource(t, nn); !hasError(diags, "cannot compare a non-nullable pointer with null") {
		t.Fatalf("non-nullable == null was not reported: %v", diags)
	}

	// Pointers are runtime-only: compile-time bindings reject them.
	ct := "#entry main :: proc -> S64 { a: S64 = 1; x :: *a; return 0; }"
	if diags := typeCheckSource(t, ct); !hasError(diags, "pointers are only available at runtime") {
		t.Fatalf("compile-time pointer was not reported: %v", diags)
	}

	// Self-referential structs are allowed only through a nullable pointer.
	selfref := "Node :: struct { next: *Node?; } main :: proc { n: Node; }"
	if diags := typeCheckSource(t, selfref); diags.HasErrors() {
		t.Fatalf("valid self-referential struct failed: %v", diags)
	}
	badSelf := "Node :: struct { next: *Node; }"
	if diags := typeCheckSource(t, badSelf); !hasError(diags, "infinite size") {
		t.Fatalf("non-nullable self-reference was not rejected: %v", diags)
	}

	// Qualified enum member references still resolve through field access.
	enumOK := `
Color :: enum { RED: U8 = 1; GREEN; }
main :: proc { c: Color = Color.GREEN; }
`
	if diags := typeCheckSource(t, enumOK); diags.HasErrors() {
		t.Fatalf("enum member through field access failed: %v", diags)
	}
}

func TestTypeCheckPointerAdvanced(t *testing.T) {
	// Double pointers with chained dereference, integer-on-the-left arithmetic,
	// order comparisons, and a null literal on the left of a comparison all
	// type check.
	valid := `
#entry main :: proc -> S64 {
	a := 5;
	p: *S64 = *a;
	pp: **S64 = *p;
	deep := pp.*.*;
	arr := []S64.{10, 20, 30};
	base: *S64 = *arr[0];
	right := base + 2;
	left := 2 + base;
	r: S64 = 0;
	if deep == 5 && right == left { r += 1; }
	if base <= *arr[0] && base >= *arr[0] { r += 1; }
	if base < *arr[2] && *arr[2] > base { r += 1; }
	pn: *S64? = null;
	if null == pn { r += 1; }
	if null != pn { r += 1; }
	return r;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid advanced pointer program failed: %v", diags)
	}
}

func TestTypeCheckPointerNoNullBranch(t *testing.T) {
	// The then-branch of "p != null" sees p as non-null, so both dereference
	// and passing to a non-nullable parameter are allowed inside it.
	valid := `
use :: proc (p: *S64) -> S64 { return p.*; }
#entry main :: proc -> S64 {
	p: *S64? = null;
	if p != null {
		return use(p) + p.*;
	}
	return 0;
}
`
	if diags := typeCheckSource(t, valid); diags.HasErrors() {
		t.Fatalf("valid '!=' narrowing failed: %v", diags)
	}
}

func TestTypeCheckPointerErrorsMore(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"deref non-pointer", "#entry main :: proc -> S64 {\n    a := 5;\n    return a.*;\n}", "cannot dereference a value of type S64"},
		{"field on scalar", "#entry main :: proc -> S64 {\n    a := 5;\n    return a.age;\n}", "cannot access field 'age' on a value of type S64"},
		{"unknown field read", "Person :: struct { name: String; age: S64; }\n#entry main :: proc -> S64 {\n    p := Person.{name=«x», age=1};\n    return p.agee;\n}", "struct Person has no field 'agee'"},
		{"field assign on scalar", "#entry main :: proc -> S64 {\n    a := 5;\n    a.age = 3;\n    return a;\n}", "cannot access a field on a value of type S64"},
		{"unknown field assign", "Person :: struct { name: String; age: S64; }\n#entry main :: proc -> S64 {\n    p := Person.{name=«x», age=1};\n    p.agee = 3;\n    return p.age;\n}", "struct Person has no field 'agee'"},
		{"address of null", "#entry main :: proc -> S64 {\n    x := *null;\n    return 0;\n}", "cannot take the address of null"},
		{"address of non-lvalue", "#entry main :: proc -> S64 {\n    x := *(1 + 2);\n    return 0;\n}", "cannot take the address of this expression"},
		{"null to scalar", "#entry main :: proc -> S64 {\n    x: S64 = null;\n    return 0;\n}", "cannot assign null to S64"},
		{"add two pointers", "#entry main :: proc -> S64 {\n    a := 1;\n    b := 2;\n    p: *S64 = *a;\n    q: *S64 = *b;\n    x := p + q;\n    return 0;\n}", "cannot add two pointers"},
		{"subtract pointer types", "#entry main :: proc -> S64 {\n    a := 1;\n    b: S32 = 2;\n    p: *S64 = *a;\n    q: *S32 = *b;\n    x := p - q;\n    return 0;\n}", "cannot subtract pointers of different types"},
		{"compare pointer types", "#entry main :: proc -> S64 {\n    a := 1;\n    b: S32 = 2;\n    p: *S64 = *a;\n    q: *S32 = *b;\n    if p == q { return 0; }\n    return 1;\n}", "cannot compare *S64 and *S32"},
		{"pointer float offset", "#entry main :: proc -> S64 {\n    a := 1;\n    p: *S64 = *a;\n    x := p + 1.5;\n    return 0;\n}", "pointer arithmetic requires an integer offset"},
		{"compound pointer float offset", "#entry main :: proc -> S64 {\n    a := 1;\n    p: *S64 = *a;\n    p += 1.5;\n    return 0;\n}", "pointer arithmetic requires an integer offset"},
		{"null compared with scalar", "#entry main :: proc -> S64 {\n    if null == 5 { return 0; }\n    return 1;\n}", "null can only be compared with a nullable pointer"},
		{"ordering pointer null", "#entry main :: proc -> S64 {\n    p: *S64? = null;\n    if p < null { return 0; }\n    return 1;\n}", "ordering is not defined for pointers and null"},
		{"incdec string", "#entry main :: proc -> S64 {\n    s := «abc»;\n    s++;\n    return 0;\n}", "cannot increment or decrement a value of type String"},
	}
	for _, tc := range cases {
		if diags := typeCheckSource(t, tc.src); !hasError(diags, tc.want) {
			t.Errorf("%s: did not report %q, got %v", tc.name, tc.want, diags)
		}
	}
}

func TestTypeCheckAddrAndAllocate(t *testing.T) {
	// A pointer assigns to Addr implicitly; Addr casts back to a pointer.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    x := 7;\n    p: *S64 = *x;\n    a: Addr = p;\n    q: *S64 = a.(*S64);\n    return q.*;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for Addr: %v", diags)
	}
	// #allocate returns Addr and accepts an integer size.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a := #allocate 16;\n    #deallocate a;\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for #allocate: %v", diags)
	}
	// #deallocate accepts a pointer value too.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    x := 7;\n    p: *S64 = *x;\n    #deallocate p;\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for #deallocate pointer: %v", diags)
	}
	// A non-pointer cannot assign to Addr.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: Addr = 5;\n    return 0;\n}"); !hasError(diags, "cannot assign S64 to Addr") {
		t.Errorf("expected Addr assignment error, got %v", diags)
	}
	// #allocate size must be an integer.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a := #allocate 1.5;\n    return 0;\n}"); !hasError(diags, "#allocate size must be an integer") {
		t.Errorf("expected #allocate size error, got %v", diags)
	}
	// #deallocate requires an address.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    #deallocate 5;\n    return 0;\n}"); !hasError(diags, "#deallocate requires an address") {
		t.Errorf("expected #deallocate error, got %v", diags)
	}
}

func TestTypeCheckAddrOfUninitialized(t *testing.T) {
	// Taking the address of an uninitialized variable is valid: address-of
	// does not read the value, and the backend zero-initializes every local.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: S64;\n    set_val(*a, 42);\n    return 0;\n}\nset_val :: proc (p: *S64, v: S64) { p.* = v; }"); diags.HasErrors() {
		t.Fatalf("unexpected errors for address-of uninitialized: %v", diags)
	}
	// A struct with a dynamic-array field can be passed by address uninitialized.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    p: Parser;\n    set_pos(*p, 5);\n    return 0;\n}\nParser :: struct { items: [dyn]S64; pos: Size; }\nset_pos :: proc (p: *Parser, n: Size) { p.*.pos = n; }"); diags.HasErrors() {
		t.Fatalf("unexpected errors for address-of uninitialized struct: %v", diags)
	}
	// Reading a plain declaration is valid (zero-initialized).
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: S64;\n    return a;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for reading a zero-initialized declaration: %v", diags)
	}
	// Reading an '= ...' declaration (initialize later) is rejected.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: S64 = ...;\n    return a;\n}"); !hasError(diags, "is not initialized") {
		t.Errorf("expected read-of-init-later error, got %v", diags)
	}
}

func TestTypeCheckInitLaterSemantics(t *testing.T) {
	// A plain declaration is zero-initialized and readable.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: S64;\n    if a == 0 { return 0; }\n    return 1;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for zero-initialized declaration: %v", diags)
	}
	// A plain declaration is comparable with any operator.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: S64;\n    if a != 0 { return 1; }\n    if a < 1 { return 2; }\n    if a >= 0 { return 3; }\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for comparisons on zero-initialized declaration: %v", diags)
	}
	// '= ...' declares initialize-later: reading before assignment is rejected.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: S64 = ...;\n    if a == 0 { return 1; }\n    return 0;\n}"); !hasError(diags, "is not initialized") {
		t.Errorf("expected init-later read error, got %v", diags)
	}
	// Assigning before reading is valid.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: S64 = ...;\n    a = 42;\n    if a != 42 { return 1; }\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for init-later then assign: %v", diags)
	}
	// A dynamic array declared with '= ...' is not initialized.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    a: [dyn]S64 = ...;\n    if a.count == 0 { return 1; }\n    return 0;\n}"); !hasError(diags, "is not initialized") {
		t.Errorf("expected init-later dynamic array read error, got %v", diags)
	}
}

func TestTypeCheckStringFields(t *testing.T) {
	// Reading data and count works.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    s := «hello»;\n    n: Size = s.count;\n    p: *Byte = s.data;\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for String field read: %v", diags)
	}
	// Writing data and count on an uninitialized String works.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    new_var := «hello world»;\n    a: String;\n    a.data = new_var.data;\n    a.count = new_var.count;\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for String field write: %v", diags)
	}
	// Unknown String field is rejected.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    s := «x»;\n    return s.length;\n}"); !hasError(diags, "String has no field 'length'") {
		t.Errorf("expected unknown String field error, got %v", diags)
	}
}

func TestTypeCheckStringIndexing(t *testing.T) {
	// Reading a byte at an index is valid and yields Byte.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    s := «hello»;\n    b: Byte = s[1];\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for String index read: %v", diags)
	}
	// Writing a byte at an index is valid.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    s := «hello»;\n    s[0] = 72;\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for String index write: %v", diags)
	}
	// A non-integer index is rejected.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    s := «hello»;\n    b := s[«x»];\n    return 0;\n}"); !hasError(diags, "string index must be an integer") {
		t.Errorf("expected string index type error, got %v", diags)
	}
}

func TestTypeCheckInterpolation(t *testing.T) {
	// Interpolating a String is valid.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    name := «world»;\n    s := «hello {name}!»;\n    return 0;\n}"); diags.HasErrors() {
		t.Fatalf("unexpected errors for interpolation: %v", diags)
	}
	// Interpolating a non-String is rejected.
	if diags := typeCheckSource(t, "#entry main :: proc -> S64 {\n    n := 5;\n    s := «value {n}»;\n    return 0;\n}"); !hasError(diags, "interpolation requires a String value") {
		t.Errorf("expected interpolation type error, got %v", diags)
	}
}

func TestTypeCheckFlatImport(t *testing.T) {
	// A flat import makes the module's functions directly visible.
	src := "#import «core»;\n#entry main :: proc -> S64 {\n    if !is_digit(48) { return 1; }\n    return 0;\n}"
	if diags := typeCheckSource(t, src); diags.HasErrors() {
		t.Fatalf("unexpected errors for flat import: %v", diags)
	}
}

func TestTypeCheckNamespacedImportMember(t *testing.T) {
	// A namespaced import resolves 'c.member(...)' calls.
	src := "c :: #import «core»;\n#entry main :: proc -> S64 {\n    if !c.is_digit(48) { return 1; }\n    if c.int_to_string(42) != «42» { return 2; }\n    return 0;\n}"
	if diags := typeCheckSource(t, src); diags.HasErrors() {
		t.Fatalf("unexpected errors for namespaced import: %v", diags)
	}
}

func TestTypeCheckNamespacedImportScopeIsolation(t *testing.T) {
	// A namespaced import does not make members visible by bare name.
	src := "c :: #import «core»;\n#entry main :: proc -> S64 {\n    if is_digit(48) { return 1; }\n    return 0;\n}"
	if diags := typeCheckSource(t, src); !hasError(diags, "call to unknown procedure is_digit") {
		t.Errorf("expected bare-name isolation error, got %v", diags)
	}
}

func TestTypeCheckImportUnknownModule(t *testing.T) {
	src := "#import «nonexistent»;\n#entry main :: proc -> S64 { return 0; }"
	if diags := typeCheckSource(t, src); !hasError(diags, "cannot find module") {
		t.Errorf("expected unknown-module error, got %v", diags)
	}
}

func TestTypeCheckImportBadMember(t *testing.T) {
	src := "c :: #import «core»;\n#entry main :: proc -> S64 {\n    c.nonexistent(1);\n    return 0;\n}"
	if diags := typeCheckSource(t, src); !hasError(diags, "has no member") {
		t.Errorf("expected bad-member error, got %v", diags)
	}
}

func TestTypeCheckImportMemberAsValue(t *testing.T) {
	src := "c :: #import «core»;\n#entry main :: proc -> S64 {\n    x := c.is_digit;\n    return 0;\n}"
	if diags := typeCheckSource(t, src); !hasError(diags, "cannot be used as a value") {
		t.Errorf("expected member-as-value error, got %v", diags)
	}
}

func TestTypeCheckNamespaceCollision(t *testing.T) {
	// A namespace binding occupies the same declaration namespace as other
	// top-level names, so a second 'c' is a shadow error.
	src := "c :: #import «core»;\nc :: proc () -> S64 { return 0; }\n#entry main :: proc -> S64 { return 0; }"
	if diags := typeCheckSource(t, src); !hasError(diags, "shadows an existing name") {
		t.Errorf("expected namespace collision error, got %v", diags)
	}
}

func TestTypeCheckModuleLocalDoesNotShadowMainGlobal(t *testing.T) {
	// The module's functions are checked in isolation: a local variable named
	// 'c' inside core.chaos must not collide with a main-program global 'c'.
	src := "core_ns :: #import «core»;\nc: S64 = 5;\n#entry main :: proc -> S64 {\n    if c != 5 { return 1; }\n    if !core_ns.is_digit(48) { return 2; }\n    return 0;\n}"
	if diags := typeCheckSource(t, src); diags.HasErrors() {
		t.Fatalf("unexpected errors for module-local isolation: %v", diags)
	}
}
