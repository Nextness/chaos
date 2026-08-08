package compiler

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// emitSource lowers a source string to MIR and emits fasm assembly, failing
// the test on any front-end error.
func emitSource(t *testing.T, source string) (string, DiagnosticList) {
	t.Helper()
	tokens, _ := Tokenize([]byte(source), 0)
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		t.Fatalf("parse errors in %q: %v", source, result.Diags)
	}
	if diags := CheckProgram(result.Program); diags.HasErrors() {
		t.Fatalf("type errors in %q: %v", source, diags)
	}
	hir, diags := LowerProgram(result.Program)
	if diags.HasErrors() {
		t.Fatalf("lower errors in %q: %v", source, diags)
	}
	mir, diags := LowerToMIR(hir)
	if diags.HasErrors() {
		t.Fatalf("mir errors in %q: %v", source, diags)
	}
	if diags := VerifyMIR(mir); diags.HasErrors() {
		t.Fatalf("verify errors in %q: %v", source, diags)
	}
	backend := NewBackend("fasm")
	asm, diags := backend.Emit(mir)
	return asm, diags
}

func TestFasmEmitSimple(t *testing.T) {
	asm, diags := emitSource(t, "#entry main :: proc -> S64 {\n    return 42;\n}")
	if diags.HasErrors() {
		t.Fatalf("unexpected emit errors: %v", diags)
	}
	for _, want := range []string{
		"format ELF64 executable 3",
		"entry _start",
		"call f_main",
		"mov rdi, rax",
		"mov rax, 60",
		"syscall",
		"f_main:",
		"mov rax, 42",
	} {
		if !strings.Contains(asm, want) {
			t.Errorf("assembly missing %q:\n%s", want, asm)
		}
	}
}

func TestFasmEmitFloatConstant(t *testing.T) {
	asm, diags := emitSource(t, "#entry main :: proc -> S64 {\n    x := 3.0;\n    if x > 2.0 { return 1; }\n    return 0;\n}")
	if diags.HasErrors() {
		t.Fatalf("unexpected emit errors: %v", diags)
	}
	// Integer-looking float constants must carry a decimal point so fasm
	// stores a double, not an integer bit pattern.
	if !strings.Contains(asm, "dq 3.0") || !strings.Contains(asm, "dq 2.0") {
		t.Errorf("float constants not emitted as doubles:\n%s", asm)
	}
}

func TestFasmEntrySelection(t *testing.T) {
	asm, diags := emitSource(t, "other :: proc -> S64 { return 1; }\n#entry main :: proc -> S64 { return 2; }")
	if diags.HasErrors() {
		t.Fatalf("unexpected emit errors: %v", diags)
	}
	if !strings.Contains(asm, "call f_main") {
		t.Errorf("entry wrapper should call f_main:\n%s", asm)
	}
	if strings.Contains(asm, "call f_other") {
		t.Errorf("entry wrapper should not call f_other:\n%s", asm)
	}
}

func TestFasmNoEntry(t *testing.T) {
	// A program without '#entry' still emits a wrapper that exits 0.
	asm, diags := emitSource(t, "helper :: proc -> S64 { return 1; }")
	if diags.HasErrors() {
		t.Fatalf("unexpected emit errors: %v", diags)
	}
	if !strings.Contains(asm, "entry _start") {
		t.Errorf("assembly missing entry wrapper:\n%s", asm)
	}
	if strings.Contains(asm, "call f_") {
		t.Errorf("no-entry wrapper should not call a procedure:\n%s", asm)
	}
}

func TestFasmExitMessage(t *testing.T) {
	// The entry wrapper prints the message to stderr before exiting.
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		t.Skip("fasm not available")
	}
	asm, diags := emitSource(t, "#entry main :: proc {\n    exit 1, «boom»;\n}")
	if diags.HasErrors() {
		t.Fatalf("emit errors: %v", diags)
	}
	dir := t.TempDir()
	asmPath := filepath.Join(dir, "out.asm")
	binPath := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(asmPath, []byte(asm), 0o600); err != nil {
		t.Fatalf("write asm: %v", err)
	}
	if out, err := exec.Command(fasmPath, asmPath, binPath).CombinedOutput(); err != nil {
		t.Fatalf("fasm failed: %v\n%s", err, out)
	}
	if err := os.Chmod(binPath, 0o700); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	var stderr bytes.Buffer
	cmd := exec.Command(binPath)
	cmd.Stderr = &stderr
	err = cmd.Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected non-zero exit, got %v", err)
	}
	if ee.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", ee.ExitCode())
	}
	if stderr.String() != "boom" {
		t.Errorf("stderr = %q, want %q", stderr.String(), "boom")
	}
}

// TestFasmRuntime assembles emitted programs with fasm and checks their exit
// codes. It is skipped when fasm is not installed.
func TestFasmRuntime(t *testing.T) {
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		t.Skip("fasm not available")
	}
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"add", "#entry main :: proc -> S64 {\n    return 1 + 2;\n}", 3},
		{"sub", "#entry main :: proc -> S64 {\n    return 10 - 3 - 2;\n}", 5},
		{"mul", "#entry main :: proc -> S64 {\n    return 6 * 7;\n}", 42},
		{"div", "#entry main :: proc -> S64 {\n    return 10 / 3;\n}", 3},
		{"mod", "#entry main :: proc -> S64 {\n    return 10 % 3;\n}", 1},
		{"if", "#entry main :: proc -> S64 {\n    x := 5;\n    if x > 3 { return 1; }\n    return 0;\n}", 1},
		{"elif", "#entry main :: proc -> S64 {\n    x := 2;\n    r: S64 = 0;\n    if x == 1 { r = 10; } elif x == 2 { r = 20; } else { r = 30; }\n    return r;\n}", 20},
		{"call", "add :: proc (a: S64, b: S64) -> S64 { return a + b; }\n#entry main :: proc -> S64 {\n    return add(2, 3);\n}", 5},
		{"call3", "add3 :: proc (a: S64, b: S64, c: S64) -> S64 { return a + b + c; }\n#entry main :: proc -> S64 {\n    return add3(1, 2, 3);\n}", 6},
		{"recursion", "fact :: proc (n: S64) -> S64 {\n    if n <= 1 { return 1; }\n    return n * fact(n - 1);\n}\n#entry main :: proc -> S64 {\n    return fact(5);\n}", 120},
		{"logical", "#entry main :: proc -> S64 {\n    a := true;\n    b := false;\n    if a && !b { return 7; }\n    return 0;\n}", 7},
		{"neg", "#entry main :: proc -> S64 {\n    x := -5;\n    y := -x;\n    return y;\n}", 5},
		{"unsigned compare", "#entry main :: proc -> S64 {\n    x: U64 = 5;\n    r: S64 = 0;\n    if x < 3 { r = 1; } else { r = 2; }\n    return r;\n}", 2},
		{"s32", "#entry main :: proc -> S32 {\n    x: S32 = 7;\n    y: S32 = 3;\n    z := x - y;\n    return z;\n}", 4},
		{"s32 div", "#entry main :: proc -> S32 {\n    x: S32 = 10;\n    y: S32 = 3;\n    return x / y;\n}", 3},
		{"cmp ge neq", "#entry main :: proc -> S64 {\n    x := 5;\n    r: S64 = 0;\n    if x >= 5 && x != 6 { r = 3; }\n    return r;\n}", 3},
		{"float", "#entry main :: proc -> S64 {\n    x := 2.5;\n    y := 1.5;\n    z := x + y;\n    if z > 3.0 { return 4; }\n    return 0;\n}", 4},
		{"float compare", "#entry main :: proc -> S64 {\n    x := 10.0;\n    y := 4.0;\n    z := x / y;\n    if z > 2.0 && z < 3.0 { return 6; }\n    return 0;\n}", 6},
		{"float call", "scale :: proc (v: F64, f: F64) -> F64 { return v * f; }\n#entry main :: proc -> S64 {\n    r := scale(2.0, 3.0);\n    if r == 6.0 { return 8; }\n    return 0;\n}", 8},
		{"float return", "#entry main :: proc -> F64 {\n    return 3.5;\n}", 3},
		{"global", "G: S64;\n#entry main :: proc -> S64 {\n    G = 42;\n    return G;\n}", 42},
		{"global init", "G :: 5;\n#entry main :: proc -> S64 {\n    return G;\n}", 5},
		{"global init chain", "A :: 3;\nB :: A + 4;\n#entry main :: proc -> S64 {\n    return B;\n}", 7},
		{"global string init", "S :: «world»;\n#entry main :: proc -> S64 {\n    if S == «world» { return 22; }\n    return 0;\n}", 22},
		{"f32", "#entry main :: proc -> S64 {\n    x: F32 = 1.5;\n    y: F32 = 2.5;\n    z := x + y;\n    if z > 3.0 { return 4; }\n    return 0;\n}", 4},
		{"f32 div", "#entry main :: proc -> S64 {\n    x: F32 = 10.0;\n    y: F32 = 4.0;\n    z := x / y;\n    if z > 2.0 && z < 3.0 { return 15; }\n    return 0;\n}", 15},
		{"f32 param", "twice :: proc (v: F32) -> F32 { return v * 2.0; }\n#entry main :: proc -> S64 {\n    r := twice(3.0);\n    if r == 6.0 { return 16; }\n    return 0;\n}", 16},
		{"struct", "Point :: struct { x: S64; y: S64; }\n#entry main :: proc -> S64 {\n    p: Point = .{x=3, y=4};\n    q := p;\n    return 9;\n}", 9},
		{"struct eq", "Point :: struct { x: S64; y: S64; }\n#entry main :: proc -> S64 {\n    p: Point = .{x=1, y=2};\n    q: Point = .{x=1, y=2};\n    if p == q { return 11; }\n    return 0;\n}", 11},
		{"struct neq high half", "Point :: struct { x: S64; y: S64; }\n#entry main :: proc -> S64 {\n    p: Point = .{x=1, y=2};\n    q: Point = .{x=1, y=3};\n    if p != q { return 30; }\n    return 0;\n}", 30},
		{"struct return", "Point :: struct { x: S64; y: S64; }\nmake_p :: proc (a: S64) -> Point {\n    return Point.{x=a, y=a};\n}\n#entry main :: proc -> S64 {\n    p := make_p(7);\n    q := p;\n    return 12;\n}", 12},
		{"struct param", "Point :: struct { x: S64; y: S64; }\nread_p :: proc (p: Point) -> S64 {\n    q := p;\n    return 13;\n}\n#entry main :: proc -> S64 {\n    p: Point = .{x=1, y=2};\n    return read_p(p);\n}", 13},
		{"struct string field", "Rec :: struct { name: String; val: S64; }\n#entry main :: proc -> S64 {\n    a: Rec = .{name=«x», val=1};\n    b: Rec = .{name=«x», val=1};\n    if a == b { return 31; }\n    return 0;\n}", 31},
		{"string eq", "#entry main :: proc -> S64 {\n    s := «hello»;\n    if s == «hello» { return 6; }\n    return 0;\n}", 6},
		{"string neq", "#entry main :: proc -> S64 {\n    s := «abc»;\n    if s != «abd» { return 7; }\n    return 0;\n}", 7},
		{"string param return", "ident :: proc (s: String) -> String {\n    return s;\n}\n#entry main :: proc -> S64 {\n    s := ident(«hello»);\n    if s == «hello» { return 14; }\n    return 0;\n}", 14},
		{"u128 add", "#entry main :: proc -> S64 {\n    x: U128 = 5;\n    y: U128 = 10;\n    z := x + y;\n    if z == 15 { return 8; }\n    return 0;\n}", 8},
		{"u128 mul", "#entry main :: proc -> S64 {\n    x: U128 = 1000;\n    y: U128 = 2000;\n    z := x * y;\n    if z == 2000000 { return 17; }\n    return 0;\n}", 17},
		{"u128 div", "#entry main :: proc -> S64 {\n    x: U128 = 100;\n    y: U128 = 7;\n    z := x / y;\n    if z == 14 { return 18; }\n    return 0;\n}", 18},
		{"u128 mod", "#entry main :: proc -> S64 {\n    x: U128 = 100;\n    y: U128 = 7;\n    z := x % y;\n    if z == 2 { return 19; }\n    return 0;\n}", 19},
		{"u128 param return", "inc :: proc (v: U128) -> U128 { return v + 1; }\n#entry main :: proc -> S64 {\n    x: U128 = 41;\n    y := inc(x);\n    if y == 42 { return 21; }\n    return 0;\n}", 21},
		{"s128 div", "#entry main :: proc -> S64 {\n    x: S128 = 0;\n    x = x - 100;\n    y: S128 = 7;\n    z := x / y;\n    r: S128 = 14;\n    if z + r == 0 { return 20; }\n    return 0;\n}", 20},
		{"exit", "#entry main :: proc {\n    exit 9;\n}", 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm, diags := emitSource(t, tt.src)
			if diags.HasErrors() {
				t.Fatalf("emit errors: %v", diags)
			}
			dir := t.TempDir()
			asmPath := filepath.Join(dir, "out.asm")
			binPath := filepath.Join(dir, "out.bin")
			if err := os.WriteFile(asmPath, []byte(asm), 0o600); err != nil {
				t.Fatalf("write asm: %v", err)
			}
			if out, err := exec.Command(fasmPath, asmPath, binPath).CombinedOutput(); err != nil {
				t.Fatalf("fasm failed: %v\n%s", err, out)
			}
			if err := os.Chmod(binPath, 0o700); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			cmd := exec.Command(binPath)
			if err := cmd.Run(); err != nil {
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					if got := ee.ExitCode(); got != tt.want {
						t.Errorf("exit code = %d, want %d", got, tt.want)
					}
					return
				}
				t.Fatalf("run failed: %v", err)
			}
			if tt.want != 0 {
				t.Errorf("exit code = 0, want %d", tt.want)
			}
		})
	}
}
