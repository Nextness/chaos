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
	if !hasError(diags, "cannot order error values") {
		t.Errorf("expected ordering error, got %v", diags)
	}

	diags = typeCheckSource(t, "A :: error {\n    X;\n    Y;\n}\nmain :: proc {\n    z := A.X! + A.Y!;\n}")
	if !hasError(diags, "cannot apply + to error values") {
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
	if !hasError(diags, "return without a value") {
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
	if !hasError(diags, "the catch block must return or exit") {
		t.Errorf("expected must-diverge error, got %v", diags)
	}

	// An if/else where both branches diverge satisfies the requirement.
	diags = typeCheckSource(t, "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    return x;\n}\nmain :: proc -> S64 {\n    r := f(5) unless catch {\n        if true {\n            return 1;\n        } else {\n            return 2;\n        }\n    }\n    return r;\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected errors for diverging if/else catch: %v", diags)
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
