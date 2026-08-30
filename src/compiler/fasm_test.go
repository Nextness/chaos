package compiler

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// emitSource lowers a source string to MIR and emits fasm assembly, failing
// the test on any front-end error.
func emitSource(t *testing.T, source string) (string, DiagnosticList) {
	t.Helper()
	sm := &SourceManager{}
	fileID := sm.Register("test.chaos", []byte(source))
	tokens, _ := Tokenize([]byte(source), fileID)
	result := ParseProgram(tokens)
	if result.Diags.HasErrors() {
		t.Fatalf("parse errors in %q: %v", source, result.Diags)
	}
	result.Program.Sources = map[FileID]SourceFile{fileID: *sm.Lookup(fileID)}
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

func TestFasmRuntimeDivisionByZeroLocation(t *testing.T) {
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		t.Skip("fasm not available")
	}
	for _, tt := range []struct {
		name, expression string
	}{
		{"integer", "1 / 0"},
		{"float", "1.0 / 0.0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := "#entry main :: proc -> S64 {\n    value := " + tt.expression + ";\n    return 0;\n}"
			asm, diags := emitSource(t, source)
			if diags.HasErrors() {
				t.Fatalf("emit errors: %v", diags)
			}
			dir := t.TempDir()
			asmPath := filepath.Join(dir, "out.asm")
			binPath := filepath.Join(dir, "out.bin")
			if err := os.WriteFile(asmPath, []byte(asm), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(fasmPath, asmPath, binPath).CombinedOutput(); err != nil {
				t.Fatalf("fasm failed: %v\n%s", err, out)
			}
			if err := os.Chmod(binPath, 0o700); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd := exec.Command(binPath)
			cmd.Stderr = &stderr
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("run error = %v, want exit status 1", err)
			}
			line := "    value := " + tt.expression + ";"
			want := "division by zero at test.chaos:2:" + strconv.Itoa(strings.Index(line, "/")+1) + "\n"
			if got := stderr.String(); got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
		})
	}
}

func TestFasmEmitSimple(t *testing.T) {
	asm, diags := emitSource(t, "#entry main :: proc -> S64 {\n    return 42;\n}")
	if diags.HasErrors() {
		t.Fatalf("unexpected emit errors: %v", diags)
	}
	for _, want := range []string{
		"format ELF64 executable 3",
		"entry _start",
		"call chaos_fn_0",
		"mov rdi, rax",
		"mov rax, 60",
		"syscall",
		"chaos_fn_0:",
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
	if !strings.Contains(asm, "call chaos_fn_1") {
		t.Errorf("entry wrapper should call the selected symbol:\n%s", asm)
	}
	if strings.Contains(asm, "_start:\n    call chaos_fn_0") {
		t.Errorf("entry wrapper should not call the other procedure:\n%s", asm)
	}
}

func TestFasmNoEntry(t *testing.T) {
	// Backend emission is executable production and therefore requires entry.
	asm, diags := emitSource(t, "helper :: proc -> S64 { return 1; }")
	if !diags.HasErrors() {
		t.Fatalf("expected missing-entry error, got assembly:\n%s", asm)
	}
	if asm != "" {
		t.Errorf("backend returned partial assembly after an error:\n%s", asm)
	}
}

func TestFasmExitMessage(t *testing.T) {
	// The entry wrapper prints the message to stderr before exiting.
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		t.Skip("fasm not available")
	}
	asm, diags := emitSource(t, "#entry main :: proc -> S64 {\n    exit 1, «boom»;\n}")
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

func TestFasmRejectsUnsupportedFloatTypes(t *testing.T) {
	// F16 and F128 are accepted by the front end but must be rejected by the
	// backend instead of being silently emitted as F64.
	for _, src := range []string{
		"#entry main :: proc -> S64 {\n    x: F16 = 1.5;\n    return 0;\n}",
		"#entry main :: proc -> S64 {\n    x: F128 = 1.5;\n    return 0;\n}",
		"G: F128 = 1.5;\n#entry main :: proc -> S64 {\n    return 0;\n}",
		"Rec :: struct { f: F16; }\n#entry main :: proc -> S64 {\n    x: Rec = .{f=1.5};\n    return 0;\n}",
	} {
		asm, diags := emitSource(t, src)
		if !diags.HasErrors() {
			t.Errorf("expected error for %q, got none:\n%s", src, asm)
		}
	}
}

func TestFasmRejectsInvalidRecursiveAndPrimitiveLayoutsWithoutPanicking(t *testing.T) {
	for _, makeType := range []func(*TypeTable, *SymbolTable) TypeID{
		func(types *TypeTable, symbols *SymbolTable) TypeID {
			record := types.InternStruct("Recursive")
			types.SetStructFields(record, []TypeField{{Symbol: symbols.Declare("next"), Name: "next", Type: record}})
			return record
		},
		func(types *TypeTable, _ *SymbolTable) TypeID {
			return types.intern("BogusInt", TypeKindInt)
		},
	} {
		types := NewTypeTable()
		symbols := NewSymbolTable()
		entry := symbols.Declare("main")
		invalid := makeType(types, symbols)
		fn := &MIRFunction{
			Symbol: entry, Name: "main", Locals: []LocalID{0}, LocalTypes: []TypeID{invalid}, LocalMutable: []bool{true},
			Results: []TypeID{types.S64()}, ResultType: types.S64(),
			Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: types.S64(), Imm: MIRImmediate{Kind: MIRImmInt}}}, Term: MIRTerminator{Kind: MIRReturn, Value: 0}}},
		}
		program := &MIRProgram{Symbols: symbols, Types: types, Functions: []*MIRFunction{fn}, Entry: entry}
		assembly, diags := NewBackend("fasm").Emit(program)
		if !diags.HasErrors() {
			t.Fatalf("invalid layout type %s reached emission", types.Lookup(invalid).Name)
		}
		if assembly != "" {
			t.Fatalf("backend returned partial assembly for %s", types.Lookup(invalid).Name)
		}
	}
}

func TestFasmEmitNegativeLiteral(t *testing.T) {
	// Runtime 128-bit negation must still propagate the borrow across halves;
	// signed literals themselves lower directly as exact constants.
	asm, diags := emitSource(t, "#entry main :: proc -> S64 {\n    magnitude: S128 = 100;\n    x := -magnitude;\n    if x == -100 { return 1; }\n    return 0;\n}")
	if diags.HasErrors() {
		t.Fatalf("unexpected emit errors: %v", diags)
	}
	// The 128-bit negation must negate the high half too.
	if !strings.Contains(asm, "adc rdx, 0") {
		t.Errorf("missing 128-bit negation borrow propagation:\n%s", asm)
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
		{"stack integer args", "sum7 :: proc (a: S64, b: S64, c: S64, d: S64, e: S64, f: S64, g: S64) -> S64 { return a+b+c+d+e+f+g; }\n#entry main :: proc -> S64 { return sum7(1,2,3,4,5,6,21); }", 42},
		{"stack float args", "sum9 :: proc (a: F64, b: F64, c: F64, d: F64, e: F64, f: F64, g: F64, h: F64, i: F64) -> F64 { return a+b+c+d+e+f+g+h+i; }\n#entry main :: proc -> S64 { if sum9(1.0,2.0,3.0,4.0,5.0,6.0,7.0,8.0,6.0) == 42.0 { return 42; } return 0; }", 42},
		{"multiple returns", "pair :: proc (x: S64) -> (S64, Bool) { return x + 1, true; }\n#entry main :: proc -> S64 { value, ok := pair(40); if ok { return value + 1; } return 0; }", 42},
		{"multiple returns with error", "E :: error { BAD; }\npair :: proc (x: S64) -> (S64, Bool <> E) { if x < 0 { return .BAD!; } return x + 1, true; }\n#entry main :: proc -> S64 { value, ok := pair(40) unless catch { return 0; } if ok { return value + 1; } return 0; }", 42},
		{"nested procedure", "#entry main :: proc -> S64 { C :: 1; add :: proc (x: S64) -> S64 { return x + C; } return add(41); }", 42},
		{"normalized unicode symbol", "cafe\u0301 :: proc -> S64 { return 42; }\n#entry main :: proc -> S64 { return café(); }", 42},
		{"compile time type aliases", "Number :: Later;\nLater :: S64;\nidentity :: proc (x: Number) -> Number { return x; }\n#entry main :: proc -> S64 { Local :: Number; x: Local = identity(42); return x; }", 42},
		{"nested nominal type", "#entry main :: proc -> S64 { Item :: struct { a: S64 = 42; b: String; } x: Item = .{}; y: Item = .{a=42, b=«»}; if x == y { return 42; } return 0; }", 42},
		{"global default dependency", "Item :: struct { a: S64 = LATER; }\nFIRST :: Item.{};\nLATER :: 42;\n#entry main :: proc -> S64 { if FIRST == Item.{a=42} { return 42; } return 0; }", 42},
		{"recursion", "fact :: proc (n: S64) -> S64 {\n    if n <= 1 { return 1; }\n    return n * fact(n - 1);\n}\n#entry main :: proc -> S64 {\n    return fact(5);\n}", 120},
		{"logical", "#entry main :: proc -> S64 {\n    a := true;\n    b := false;\n    if a && !b { return 7; }\n    return 0;\n}", 7},
		{"neg", "#entry main :: proc -> S64 {\n    x := -5;\n    y := -x;\n    return y;\n}", 5},
		{"unsigned compare", "#entry main :: proc -> S64 {\n    x: U64 = 5;\n    r: S64 = 0;\n    if x < 3 { r = 1; } else { r = 2; }\n    return r;\n}", 2},
		{"s32", "#entry main :: proc -> S64 {\n    x: S32 = 7;\n    y: S32 = 3;\n    z := x - y;\n    if z == 4 { return 4; }\n    return 0;\n}", 4},
		{"s32 div", "#entry main :: proc -> S64 {\n    x: S32 = 10;\n    y: S32 = 3;\n    if x / y == 3 { return 3; }\n    return 0;\n}", 3},
		{"cmp ge neq", "#entry main :: proc -> S64 {\n    x := 5;\n    r: S64 = 0;\n    if x >= 5 && x != 6 { r = 3; }\n    return r;\n}", 3},
		{"float", "#entry main :: proc -> S64 {\n    x := 2.5;\n    y := 1.5;\n    z := x + y;\n    if z > 3.0 { return 4; }\n    return 0;\n}", 4},
		{"float compare", "#entry main :: proc -> S64 {\n    x := 10.0;\n    y := 4.0;\n    z := x / y;\n    if z > 2.0 && z < 3.0 { return 6; }\n    return 0;\n}", 6},
		{"float call", "scale :: proc (v: F64, f: F64) -> F64 { return v * f; }\n#entry main :: proc -> S64 {\n    r := scale(2.0, 3.0);\n    if r == 6.0 { return 8; }\n    return 0;\n}", 8},
		{"float return", "value :: proc -> F64 { return 3.5; }\n#entry main :: proc -> S64 {\n    if value() == 3.5 { return 3; }\n    return 0;\n}", 3},
		{"global", "G: S64;\n#entry main :: proc -> S64 {\n    G = 42;\n    return G;\n}", 42},
		{"global init", "G :: 5;\n#entry main :: proc -> S64 {\n    return G;\n}", 5},
		{"global init chain", "A :: 3;\nB :: A + 4;\n#entry main :: proc -> S64 {\n    return B;\n}", 7},
		{"global string init", "S :: «world»;\n#entry main :: proc -> S64 {\n    if S == «world» { return 22; }\n    return 0;\n}", 22},
		{"enum bare compare", "E :: enum { A: S64 = 7; B = 9; }\n#entry main :: proc -> S64 {\n    x: E = E.B;\n    if x == .B { return 42; }\n    return 0;\n}", 42},
		{"enum bare return", "E :: enum { A: S64 = 7; }\nvalue :: proc -> E { return .A; }\n#entry main :: proc -> S64 {\n    if value() == E.A { return 44; }\n    return 0;\n}", 44},
		{"enum narrow global", "E :: enum { A: U8 = 1; B = 2; }\nG: E = E.A;\nH: S64 = 42;\n#entry main :: proc -> S64 {\n    G = E.B;\n    return H;\n}", 42},
		{"enum u64 max", "E :: enum { ZERO: U64 = 0; MAX = 18446744073709551615; }\n#entry main :: proc -> S64 {\n    if E.MAX != E.ZERO { return 45; }\n    return 0;\n}", 45},
		{"enum u128 high half", "E :: enum { ZERO: U128 = 0; HIGH = 18446744073709551616; }\nidentity :: proc (v: E) -> E { return v; }\n#entry main :: proc -> S64 {\n    x: E = identity(E.HIGH);\n    if x != E.ZERO { return 43; }\n    return 0;\n}", 43},
		{"f32", "#entry main :: proc -> S64 {\n    x: F32 = 1.5;\n    y: F32 = 2.5;\n    z := x + y;\n    if z > 3.0 { return 4; }\n    return 0;\n}", 4},
		{"f32 div", "#entry main :: proc -> S64 {\n    x: F32 = 10.0;\n    y: F32 = 4.0;\n    z := x / y;\n    if z > 2.0 && z < 3.0 { return 15; }\n    return 0;\n}", 15},
		{"f32 param", "twice :: proc (v: F32) -> F32 { return v * 2.0; }\n#entry main :: proc -> S64 {\n    r := twice(3.0);\n    if r == 6.0 { return 16; }\n    return 0;\n}", 16},
		{"struct", "Point :: struct { x: S64; y: S64; }\n#entry main :: proc -> S64 {\n    p: Point = .{x=3, y=4};\n    q := p;\n    return 9;\n}", 9},
		{"struct eq", "Point :: struct { x: S64; y: S64; }\n#entry main :: proc -> S64 {\n    p: Point = .{x=1, y=2};\n    q: Point = .{x=1, y=2};\n    if p == q { return 11; }\n    return 0;\n}", 11},
		{"struct neq high half", "Point :: struct { x: S64; y: S64; }\n#entry main :: proc -> S64 {\n    p: Point = .{x=1, y=2};\n    q: Point = .{x=1, y=3};\n    if p != q { return 30; }\n    return 0;\n}", 30},
		{"struct return", "Point :: struct { x: S64; y: S64; }\nmake_p :: proc (a: S64) -> Point {\n    return Point.{x=a, y=a};\n}\n#entry main :: proc -> S64 {\n    p := make_p(7);\n    q := p;\n    return 12;\n}", 12},
		{"struct param", "Point :: struct { x: S64; y: S64; }\nread_p :: proc (p: Point) -> S64 {\n    q := p;\n    return 13;\n}\n#entry main :: proc -> S64 {\n    p: Point = .{x=1, y=2};\n    return read_p(p);\n}", 13},
		{"struct string field", "Rec :: struct { name: String; val: S64; }\n#entry main :: proc -> S64 {\n    a: Rec = .{name=«x», val=1};\n    b: Rec = .{name=«x», val=1};\n    if a == b { return 31; }\n    return 0;\n}", 31},
		{"struct padding equality", "Rec :: struct { small: U8; wide: U64; tail: U8; }\n#entry main :: proc -> S64 { a: Rec = .{small=1, wide=2, tail=3}; b: Rec = .{small=1, wide=2, tail=3}; if a == b { return 42; } return 0; }", 42},
		{"string eq", "#entry main :: proc -> S64 {\n    s := «hello»;\n    if s == «hello» { return 6; }\n    return 0;\n}", 6},
		{"string neq", "#entry main :: proc -> S64 {\n    s := «abc»;\n    if s != «abd» { return 7; }\n    return 0;\n}", 7},
		{"string param return", "ident :: proc (s: String) -> String {\n    return s;\n}\n#entry main :: proc -> S64 {\n    s := ident(«hello»);\n    if s == «hello» { return 14; }\n    return 0;\n}", 14},
		{"u128 add", "#entry main :: proc -> S64 {\n    x: U128 = 5;\n    y: U128 = 10;\n    z := x + y;\n    if z == 15 { return 8; }\n    return 0;\n}", 8},
		{"u128 mul", "#entry main :: proc -> S64 {\n    x: U128 = 1000;\n    y: U128 = 2000;\n    z := x * y;\n    if z == 2000000 { return 17; }\n    return 0;\n}", 17},
		{"u128 div", "#entry main :: proc -> S64 {\n    x: U128 = 100;\n    y: U128 = 7;\n    z := x / y;\n    if z == 14 { return 18; }\n    return 0;\n}", 18},
		{"u128 mod", "#entry main :: proc -> S64 {\n    x: U128 = 100;\n    y: U128 = 7;\n    z := x % y;\n    if z == 2 { return 19; }\n    return 0;\n}", 19},
		{"u128 param return", "inc :: proc (v: U128) -> U128 { return v + 1; }\n#entry main :: proc -> S64 {\n    x: U128 = 41;\n    y := inc(x);\n    if y == 42 { return 21; }\n    return 0;\n}", 21},
		{"s128 div", "#entry main :: proc -> S64 {\n    x: S128 = 0;\n    x = x - 100;\n    y: S128 = 7;\n    z := x / y;\n    r: S128 = 14;\n    if z + r == 0 { return 20; }\n    return 0;\n}", 20},
		{"neg literal s32", "#entry main :: proc -> S64 {\n    x: S32 = -7;\n    if x == -7 { return 32; }\n    return 0;\n}", 32},
		{"neg literal s128", "#entry main :: proc -> S64 {\n    x: S128 = -100;\n    if x == -100 { return 33; }\n    return 0;\n}", 33},
		{"neg literal f32", "#entry main :: proc -> S64 {\n    x: F32 = -1.5;\n    if x == -1.5 { return 34; }\n    return 0;\n}", 34},
		{"neg literal return", "f :: proc -> S128 {\n    return -100;\n}\n#entry main :: proc -> S64 {\n    x: S128 = f();\n    if x == -100 { return 35; }\n    return 0;\n}", 35},
		{"s128 neg var", "#entry main :: proc -> S64 {\n    y: S128 = 7;\n    x: S128 = 0;\n    x = x - 7;\n    if x == -y { return 36; }\n    return 0;\n}", 36},
		{"u128 neg var", "#entry main :: proc -> S64 {\n    y: U128 = 7;\n    x: U128 = 0;\n    x = x - 7;\n    if x == -y { return 37; }\n    return 0;\n}", 37},
		{"string lt", "#entry main :: proc -> S64 {\n    s := «aaa»;\n    if s < «bbb» { return 38; }\n    return 0;\n}", 38},
		{"string gt", "#entry main :: proc -> S64 {\n    s := «bbb»;\n    if s > «aaa» { return 39; }\n    return 0;\n}", 39},
		{"string le", "#entry main :: proc -> S64 {\n    s := «abc»;\n    if s <= «abcd» { return 40; }\n    return 0;\n}", 40},
		{"string ge prefix", "#entry main :: proc -> S64 {\n    s := «abcd»;\n    if s >= «abc» { return 41; }\n    return 0;\n}", 41},
		{"string empty", "#entry main :: proc -> S64 {\n    s := «»;\n    if s == «» && s < «a» { return 42; }\n    return 0;\n}", 42},
		{"error eq", "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n    NOT_FOUND;\n    OUT_OF_BOUNDS;\n}\n#entry main :: proc -> S64 {\n    err: Hash_Table_Error = .OUT_OF_MEMORY!;\n    if err == Hash_Table_Error.OUT_OF_MEMORY! { return 43; }\n    return 0;\n}", 43},
		{"error neq", "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n    NOT_FOUND;\n    OUT_OF_BOUNDS;\n}\n#entry main :: proc -> S64 {\n    err: Hash_Table_Error = .GENERIC!;\n    if err != Hash_Table_Error.NOT_FOUND! { return 44; }\n    return 0;\n}", 44},
		{"error const global", "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n    NOT_FOUND;\n    OUT_OF_BOUNDS;\n}\nDEFAULT :: Hash_Table_Error.NOT_FOUND!;\n#entry main :: proc -> S64 {\n    if DEFAULT == Hash_Table_Error.NOT_FOUND! { return 45; }\n    return 0;\n}", 45},
		{"error exit status", "Hash_Table_Error :: error {\n    GENERIC;\n    OUT_OF_MEMORY;\n    NOT_FOUND;\n    OUT_OF_BOUNDS;\n}\n#entry main :: proc -> S64 {\n    exit Hash_Table_Error.NOT_FOUND!;\n}", 2},
		{"unless catch success", "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\n#entry main :: proc -> S64 {\n    r := f(5) unless catch {\n        return -1;\n    }\n    return r;\n}", 10},
		{"unless catch error", "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\n#entry main :: proc -> S64 {\n    r := f(-1) unless catch {\n        return 99;\n    }\n    return r;\n}", 99},
		{"unless catch binding", "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\n#entry main :: proc -> S64 {\n    r := f(-1) unless catch err {\n        if err == .GENERIC! {\n            return 7;\n        }\n        return 8;\n    }\n    return r;\n}", 7},
		{"if catch", "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\n#entry main :: proc -> S64 {\n    r := f(5);\n    if r catch {\n        return -1;\n    }\n    return r;\n}", 10},
		{"if catch error", "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\n#entry main :: proc -> S64 {\n    r := f(-1);\n    if r catch {\n        return 99;\n    }\n    return r;\n}", 99},
		{"bare unless catch", "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\n#entry main :: proc -> S64 {\n    f(-1) unless catch {\n        return 42;\n    }\n    return 0;\n}", 42},
		{"error re-raise", "Some_Error :: error {\n    GENERIC;\n    SOMETHING_ELSE;\n}\nf :: proc (x: S64) -> (S64 <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return x * 2;\n}\ng :: proc -> (S64 <> Some_Error) {\n    r := f(-1);\n    if r catch {\n        return r;\n    }\n    return r;\n}\n#entry main :: proc -> S64 {\n    r := g() unless catch {\n        return 5;\n    }\n    return r;\n}", 5},
		{"error string value", "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (x: S64) -> (String <> Some_Error) {\n    if x < 0 {\n        return .GENERIC!;\n    }\n    return «hello»;\n}\n#entry main :: proc -> S64 {\n    s := f(1) unless catch {\n        return -1;\n    }\n    if s == «hello» {\n        return 11;\n    }\n    return 0;\n}", 11},
		{"exit", "#entry main :: proc -> S64 {\n    exit 9;\n}", 9},
		{"while break", "#entry main :: proc -> S64 {\n    a := 0;\n    for true {\n        a += 1;\n        if a == 10 then break;\n    }\n    return a;\n}", 10},
		{"c-for += after", "#entry main :: proc -> S64 {\n    sum: S64 = 0;\n    for i := 1; i <= 10; i += 1 {\n        sum += i;\n    }\n    return sum;\n}", 55},
		{"c-for ++ after", "#entry main :: proc -> S64 {\n    for a := 0; a != 10; a++ {\n    }\n    return a;\n}", 10},
		{"c-for -- after", "#entry main :: proc -> S64 {\n    for a := 10; a != 0; a-- {\n    }\n    return a;\n}", 0},
		{"continue", "#entry main :: proc -> S64 {\n    arr := []S64.{1, 2, 3, 4, 5};\n    sum: S64 = 0;\n    for i := 0; i < 5; i++ {\n        if arr[i] == 3 then continue;\n        sum += arr[i];\n    }\n    return sum;\n}", 12},
		{"range elem", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    sum: S64 = 0;\n    for elem: arr {\n        sum += elem;\n    }\n    return sum;\n}", 60},
		{"range idx elem", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    sum: S64 = 0;\n    for idx, elem: arr {\n        sum += elem + idx;\n    }\n    return sum;\n}", 63},
		{"range implicit this", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    sum: S64 = 0;\n    for arr {\n        sum += #this;\n    }\n    return sum;\n}", 60},
		{"range this index", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    sum: S64 = 0;\n    for idx, elem: arr {\n        sum += #this + #index;\n    }\n    return sum;\n}", 63},
		{"range string array", "count_a :: proc (s: String) -> S64 {\n    if s == «apple» then return 1;\n    return 0;\n}\n#entry main :: proc -> S64 {\n    arr := []String.{«apple», «banana», «apple»};\n    n: S64 = 0;\n    for elem: arr {\n        n += count_a(elem);\n    }\n    return n;\n}", 2},
		{"array param", "total :: proc (items: []S64) -> S64 {\n    sum: S64 = 0;\n    for elem: items {\n        sum += elem;\n    }\n    return sum;\n}\n#entry main :: proc -> S64 {\n    arr := []S64.{5, 6, 7};\n    return total(arr);\n}", 18},
		{"array index", "#entry main :: proc -> S64 {\n    arr := []S64.{7, 8, 9};\n    return arr[1];\n}", 8},
		{"array element equality", "#entry main :: proc -> S64 { a := []S64.{1,2,3}; b := []S64.{1,2,3}; if a == b { return 42; } return 0; }", 42},
		{"array nested equality", "#entry main :: proc -> S64 { a := [][]S64.{[]S64.{1,2}, []S64.{3}}; b := [][]S64.{[]S64.{1,2}, []S64.{3}}; if a == b { return 42; } return 0; }", 42},
		{"array string inequality", "#entry main :: proc -> S64 { a := []String.{«a»,«b»}; b := []String.{«a»,«c»}; if a != b { return 42; } return 0; }", 42},
		{"runtime narrow wrap", "#entry main :: proc -> S64 { x: S8 = 127; x += 1; if x == -128 { return 42; } return 0; }", 42},
		{"signed min division wraps", "#entry main :: proc -> S64 { x: S64 = -9223372036854775808; y: S64 = -1; if x / y == x { return 42; } return 0; }", 42},
		{"prefix inc dec", "#entry main :: proc -> S64 {\n    a := 5;\n    ++a;\n    ++a;\n    --a;\n    return a;\n}", 6},
		{"empty array", "#entry main :: proc -> S64 {\n    arr := []S64.{};\n    n: S64 = 0;\n    for elem: arr {\n        n += 1;\n    }\n    return n;\n}", 0},
		{"void union success", "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (input1: String) -> (Void <> Some_Error) {\n    if input1 == «» {\n        return;\n    }\n    return .GENERIC!;\n}\n#entry main :: proc -> S64 {\n    f(«») unless catch {\n        return -1;\n    }\n    return 7;\n}", 7},
		{"void union error", "Some_Error :: error {\n    GENERIC;\n}\nf :: proc (input1: String) -> (Void <> Some_Error) {\n    if input1 == «» {\n        return;\n    }\n    return .GENERIC!;\n}\n#entry main :: proc -> S64 {\n    f(«x») unless catch {\n        return -1;\n    }\n    return 7;\n}", 255},
		{"void union reversed", "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (Some_Error <> Void) {\n    return .GENERIC!;\n}\n#entry main :: proc -> S64 {\n    f() unless catch {\n        return -1;\n    }\n    return 7;\n}", 255},
		{"void union fall off end", "Some_Error :: error {\n    GENERIC;\n}\nf :: proc -> (Void <> Some_Error) {\n    x := 1;\n}\n#entry main :: proc -> S64 {\n    f() unless catch {\n        return -1;\n    }\n    return 9;\n}", 9},
		{"explicit void result", "f :: proc -> Void {\n    return;\n}\n#entry main :: proc -> S64 {\n    f();\n    return 9;\n}", 9},
		{"void result fall off end", "f :: proc -> Void {\n    x := 1;\n}\n#entry main :: proc -> S64 {\n    f();\n    return 9;\n}", 9},
		{"shadow directive", "f :: proc (input1: S64) -> S64 {\n    #shadow input1 := input1 + 1;\n    return input1;\n}\n#entry main :: proc -> S64 {\n    return f(41);\n}", 42},
		{"shadow global", "SOME_VAR :: 10;\nf :: proc -> S64 {\n    #shadow SOME_VAR :: SOME_VAR + 5;\n    return SOME_VAR;\n}\n#entry main :: proc -> S64 {\n    return f();\n}", 15},
		{"shadow top-level global", "SOME_VAR :: 10;\n#shadow SOME_VAR :: SOME_VAR + 5;\n#entry main :: proc -> S64 {\n    return SOME_VAR;\n}", 15},
		{"shadow global label collision", "x :: 1;\n#shadow x :: x + 1;\nx_1 :: 40;\n#entry main :: proc -> S64 {\n    return x + x_1;\n}", 42},
		{"shadow unless target", "Some_Error :: error { BAD; }\nf :: proc (value: S64) -> (S64 <> Some_Error) {\n    return value + 1;\n}\n#entry main :: proc -> S64 {\n    value := 40;\n    #shadow value := f(value) unless catch {\n        return -1;\n    }\n    return value + 1;\n}", 42},
		{"pointer addr of deref", "#entry main :: proc -> S64 {\n    a := 10;\n    b: *S64 = *a;\n    c := b.*;\n    if c == 10 { return 0; }\n    return 1;\n}", 0},
		{"pointer store through", "#entry main :: proc -> S64 {\n    a := 10;\n    b: *S64 = *a;\n    b.* = 42;\n    if a == 42 { return 42; }\n    return 0;\n}", 42},
		{"pointer param", "read_v :: proc (p: *S64) -> S64 {\n    return p.*;\n}\n#entry main :: proc -> S64 {\n    a := 7;\n    return read_v(*a);\n}", 7},
		{"pointer array element", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    p: *S64 = *arr[1];\n    p.* = 21;\n    if arr[1] == 21 && p.* == 21 { return 21; }\n    return 0;\n}", 21},
		{"pointer arithmetic", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    p: *S64 = *arr[0];\n    q := p + 2;\n    if q.* == 30 { return 30; }\n    return 0;\n}", 30},
		{"pointer struct field read", "Person :: struct { name: String; age: S64; }\n#entry main :: proc -> S64 {\n    person := Person.{name=«x», age=7};\n    pp: *Person = *person;\n    if pp.*.age == 7 { return 7; }\n    return 0;\n}", 7},
		{"pointer struct field write", "Person :: struct { name: String; age: S64; }\n#entry main :: proc -> S64 {\n    person := Person.{name=«x», age=7};\n    pp: *Person = *person;\n    pp.*.age = 9;\n    pp.*.name = «new»;\n    if person.age == 9 && person.name == «new» { return 9; }\n    return 0;\n}", 9},
		{"nullable null check", "#entry main :: proc -> S64 {\n    p: *S64? = null;\n    if p == null { return 11; }\n    return 0;\n}", 11},
		{"nullable non-null", "#entry main :: proc -> S64 {\n    a := 10;\n    p: *S64? = *a;\n    if p == null { return 1; }\n    return p.* + 1;\n}", 11},
		{"nullable return", "make_ptr :: proc (x: S64) -> *S64? {\n    if x < 0 { return null; }\n    y := x;\n    return *y;\n}\n#entry main :: proc -> S64 {\n    p := make_ptr(5);\n    if p == null { return 2; }\n    return p.*;\n}", 5},
		{"self-referential struct", "Node :: struct { value: S64; next: *Node?; }\n#entry main :: proc -> S64 {\n    n1 := Node.{value=1, next=null};\n    n2 := Node.{value=2, next=*n1};\n    next_p: *Node? = n2.next;\n    if next_p == null { return 2; }\n    return next_p.*.value;\n}", 1},
		{"pointer err union", "Some_Error :: error { GENERIC; }\nmaybe :: proc (take: Bool) -> (*S64 <> Some_Error) {\n    if take { x: S64 = 42; return *x; }\n    return .GENERIC!;\n}\n#entry main :: proc -> S64 {\n    p := maybe(true) unless catch { return 3; };\n    return p.*;\n}", 42},
		{"linked list loop", "Node :: struct { value: S64; next: *Node?; }\n#entry main :: proc -> S64 {\n    n1 := Node.{value=1, next=null};\n    n2 := Node.{value=2, next=*n1};\n    n3 := Node.{value=3, next=*n2};\n    cur: *Node? = *n3;\n    sum := 0;\n    for cur != null {\n        node := cur.*;\n        sum += node.value;\n        cur = node.next;\n    }\n    if sum == 6 { return 6; }\n    return 0;\n}", 6},
		{"pointer diff", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    a: *S64 = *arr[0];\n    b: *S64 = *arr[2];\n    return b - a;\n}", 2},
		{"pointer inc", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    p: *S64 = *arr[0];\n    p++;\n    if p.* == 20 { return 20; }\n    return 0;\n}", 20},
		{"pointer dec", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    p: *S64 = *arr[1];\n    p--;\n    if p.* == 10 { return 10; }\n    return 0;\n}", 10},
		{"pointer compound add", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    p: *S64 = *arr[0];\n    p += 2;\n    if p.* == 30 { return 30; }\n    return 0;\n}", 30},
		{"pointer integer left add", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    p: *S64 = *arr[0];\n    q := 2 + p;\n    if q.* == 30 { return 30; }\n    return 0;\n}", 30},
		{"pointer order comparisons", "#entry main :: proc -> S64 {\n    arr := []S64.{10, 20, 30};\n    p: *S64 = *arr[0];\n    q: *S64 = *arr[1];\n    if p <= q && q >= p && q > p && p < q { return 20; }\n    return 0;\n}", 20},
		{"double pointer", "#entry main :: proc -> S64 {\n    a := 5;\n    p: *S64 = *a;\n    pp: **S64 = *p;\n    if pp.*.* == 5 && pp.* == p && pp == *p { return 5; }\n    return 0;\n}", 5},
		{"null left comparison", "#entry main :: proc -> S64 {\n    a := 10;\n    p: *S64? = *a;\n    if null == p { return 1; }\n    if null != p { return 10; }\n    return 0;\n}", 10},
		{"allocate returns addr", "#entry main :: proc -> S64 {\n    a := #allocate 16;\n    b: Addr = a;\n    #deallocate a;\n    return 0;\n}", 0},
		{"allocate cast deref", "#entry main :: proc -> S64 {\n    a := #allocate 16;\n    p: *S64 = a.(*S64);\n    p.* = 42;\n    #deallocate a;\n    return p.*;\n}", 42},
		{"addr from pointer", "#entry main :: proc -> S64 {\n    x := 7;\n    p: *S64 = *x;\n    a: Addr = p;\n    q: *S64 = a.(*S64);\n    return q.*;\n}", 7},
		{"string data count read", "#entry main :: proc -> S64 {\n    s := «hello»;\n    if s.count == 5 { return 5; }\n    return 0;\n}", 5},
		{"string data count write", "#entry main :: proc -> S64 {\n    new_var := «hello world»;\n    a: String;\n    a.data = new_var.data;\n    a.count = new_var.count;\n    if a.count == 11 { return 11; }\n    return 0;\n}", 11},
		{"string data pointer", "#entry main :: proc -> S64 {\n    s := «abc»;\n    p: *Byte = s.data;\n    if p.* == 97 { return 97; }\n    return 0;\n}", 97},
		{"string index read", "#entry main :: proc -> S64 {\n    s := «abc»;\n    if s[0] == 97 && s[1] == 98 && s[2] == 99 { return 99; }\n    return 0;\n}", 99},
		{"string index write", "#entry main :: proc -> S64 {\n    s := «abc»;\n    s[0] = 122;\n    if s[0] == 122 && s == «zbc» { return 122; }\n    return 0;\n}", 122},
		{"numeric convert", "#entry main :: proc -> S64 {\n    b: Byte = 65;\n    n: S64 = b.(S64);\n    f: F64 = n.(F64);\n    back: S64 = f.(S64);\n    big: S64 = 300;\n    small: Byte = big.(Byte);\n    if n == 65 && f == 65.0 && back == 65 && small == 44 { return 0; }\n    return 1;\n}", 0},
		{"string interpolation", "#entry main :: proc -> S64 {\n    name := «world»;\n    s := «hello {name}!»;\n    if s == «hello world!» { return 12; }\n    return 0;\n}", 12},
		{"string interpolation multiple", "#entry main :: proc -> S64 {\n    a := «foo»;\n    b := «bar»;\n    s := «{a}-{b}-{a}»;\n    if s == «foo-bar-foo» { return 9; }\n    return 0;\n}", 9},
		{"string interpolation leading", "#entry main :: proc -> S64 {\n    name := «x»;\n    s := «{name}»;\n    if s == «x» { return 3; }\n    return 0;\n}", 3},
		{"string interpolation empty", "#entry main :: proc -> S64 {\n    name := «»;\n    s := «a{name}b»;\n    if s == «ab» { return 2; }\n    return 0;\n}", 2},
		{"fixed array literal", "#entry main :: proc -> S64 {\n    a: [2]S64 = .{10, 20};\n    if a[0] == 10 && a[1] == 20 && a.count == 2 { return 2; }\n    return 0;\n}", 2},
		{"dynamic array literal", "#entry main :: proc -> S64 {\n    a: [dyn]S64 = .{1, 2, 3};\n    if a.count == 3 && a.capacity == 3 && a[2] == 3 { return 3; }\n    return 0;\n}", 3},
		{"dynamic array uninit capacity", "#entry main :: proc -> S64 {\n    a: [dyn]S64;\n    if a.count == 0 && a.capacity == 1 { return 1; }\n    return 0;\n}", 1},
		{"dynamic array field write", "#entry main :: proc -> S64 {\n    a: [dyn]S64 = .{1, 2, 3};\n    a.count = 5;\n    a.capacity = 10;\n    if a.count == 5 && a.capacity == 10 { return 5; }\n    return 0;\n}", 5},
		{"dynamic array string", "#entry main :: proc -> S64 {\n    a: [dyn]String = .{«a», «b», «c»};\n    if a[0] == «a» && a[2] == «c» { return 3; }\n    return 0;\n}", 3},
		{"size_of", "#entry main :: proc -> S64 {\n    if size_of(S64) != 8 { return 1; }\n    if size_of(String) != 16 { return 2; }\n    if size_of([dyn]S64) != 24 { return 3; }\n    x: S64 = 5;\n    if size_of(x) != 8 { return 4; }\n    return 0;\n}", 0},
		{"pointer index read write", "#entry main :: proc -> S64 {\n    a: [dyn]S64 = .{1, 2, 3};\n    p: *S64 = a.data;\n    v := p[1];\n    p[2] = 30;\n    if v == 2 && a[2] == 30 { return 30; }\n    return 0;\n}", 30},
		{"generic proc", "identity <T: S64 | String> :: proc (x: T) -> T { return x; }\n#entry main :: proc -> S64 {\n    a := identity(42);\n    s := identity(«hi»);\n    if a == 42 && s == «hi» { return 42; }\n    return 0;\n}", 42},
		{"generic struct", "Box <T: S64 | String> :: struct { v: T; }\n#entry main :: proc -> S64 {\n    b: Box = .{v=7};\n    if b.v == 7 { return 7; }\n    return 0;\n}", 7},
		{"append grow", "#import «core»;\n#entry main :: proc -> S64 {\n    a: [dyn]S64;\n    append(*a, 10); append(*a, 20); append(*a, 30); append(*a, 40);\n    if a.count == 4 && a[0] == 10 && a[3] == 40 && a.capacity >= 4 { return 4; }\n    return 0;\n}", 4},
		{"append pop", "#import «core»;\n#entry main :: proc -> S64 {\n    a: [dyn]S64 = .{1, 2, 3};\n    last := pop_last(*a);\n    first := pop_first(*a);\n    if last == 3 && first == 1 && a.count == 1 && a[0] == 2 { return 2; }\n    return 0;\n}", 2},
		{"append string", "#import «core»;\n#entry main :: proc -> S64 {\n    a: [dyn]String = .{«a», «b»};\n    append(*a, «c»); append(*a, «d»);\n    if a.count == 4 && a[0] == «a» && a[3] == «d» { return 4; }\n    return 0;\n}", 4},
		{"char classification", "#import «core»;\n#entry main :: proc -> S64 {\n    if !is_digit(48) { return 1; }\n    if is_digit(65) { return 2; }\n    if !is_alpha(65) { return 3; }\n    if !is_space(32) { return 4; }\n    if !is_alnum(57) { return 5; }\n    if !is_upper(90) { return 6; }\n    if !is_lower(97) { return 7; }\n    return 0;\n}", 0},
		{"int to string", "#import «core»;\n#entry main :: proc -> S64 {\n    if int_to_string(-12345) != «-12345» { return 1; }\n    if int_to_string(0) != «0» { return 2; }\n    if int_to_string(987654321) != «987654321» { return 3; }\n    return 0;\n}", 0},
		{"string to int", "#import «core»;\n#entry main :: proc -> S64 {\n    if string_to_int(«-9876») != -9876 { return 1; }\n    if string_to_int(«42») != 42 { return 2; }\n    if string_to_int(«0») != 0 { return 3; }\n    return 0;\n}", 0},
		{"string to float", "#import «core»;\n#entry main :: proc -> S64 {\n    if string_to_float(«1.5E2») != 150.0 { return 1; }\n    if string_to_float(«0.25») != 0.25 { return 2; }\n    if string_to_float(«-2.5») != -2.5 { return 3; }\n    return 0;\n}", 0},
		{"append struct", "#import «core»;\nToken :: struct { kind: S64; text: String; }\n#entry main :: proc -> S64 {\n    lx: [dyn]Token;\n    t: Token;\n    t.kind = 1;\n    t.text = «hello»;\n    append(*lx, t);\n    if lx.count == 1 && lx[0].kind == 1 && lx[0].text == «hello» { return 1; }\n    return 0;\n}", 1},
		{"append struct field zero cap", "#import «core»;\nToken :: struct { kind: S64; text: String; }\nHolder :: struct { tag: S64; items: [dyn]Token; }\n#entry main :: proc -> S64 {\n    h: Holder;\n    h.tag = 1;\n    t0: Token;\n    t0.kind = 7;\n    t0.text = «alpha»;\n    append(*h.items, t0);\n    buf: Addr = #allocate 8;\n    p: *Byte = buf.(*Byte);\n    p[0] = 58;\n    t1: Token;\n    t1.kind = 9;\n    t1.text = «beta»;\n    append(*h.items, t1);\n    if h.items.count == 2 && h.items[0].kind == 7 && h.items[0].text == «alpha» && h.items[1].kind == 9 { return 9; }\n    return 0;\n}", 9},
		{"append struct with string field", "#import «core»;\nRec :: struct { kind: S64; text: String; }\nHolder :: struct { tag: S64; items: [dyn]Rec; }\n#entry main :: proc -> S64 {\n    h: Holder;\n    h.tag = 1;\n    r: Rec;\n    r.kind = 3;\n    r.text = «hello world»;\n    append(*h.items, r);\n    buf: Addr = #allocate 8;\n    p: *Byte = buf.(*Byte);\n    p[0] = 65;\n    got := h.items[0];\n    if got.kind == 3 && got.text == «hello world» { return 3; }\n    return 0;\n}", 3},
		{"generic proc struct type", "Token :: struct { kind: S64; }\nidentity <T: S64 | String | Token> :: proc (x: T) -> T { return x; }\n#entry main :: proc -> S64 {\n    t: Token;\n    t.kind = 5;\n    r := identity(t);\n    if r.kind == 5 { return 5; }\n    return 0;\n}", 5},
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

// TestFasmArgv assembles a program whose entry procedure takes the process
// argument vector and checks that argc and argv are passed correctly.
func TestFasmArgv(t *testing.T) {
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		t.Skip("fasm not found; skipping runtime tests")
	}
	src := "#entry main :: proc (argc: S64, argv: *String) -> S64 {\n    if argc != 3 { return 1; }\n    if argv[1] != «alpha» { return 3; }\n    if argv[2] != «beta» { return 4; }\n    return 0;\n}"
	asm, diags := emitSource(t, src)
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
	cmd := exec.Command(binPath, "alpha", "beta")
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Errorf("exit code = %d, want 0", ee.ExitCode())
			return
		}
		t.Fatalf("run failed: %v", err)
	}
}

// TestFasmPrintAndFileIO assembles programs that use the print, read_file, and
// file_exists builtins and checks their stdout output and exit codes.
func TestFasmPrintAndFileIO(t *testing.T) {
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		t.Skip("fasm not found; skipping runtime tests")
	}
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(filePath, []byte("file contents\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	tests := []struct {
		name string
		src  string
		want string
		code int
	}{
		{"print", "#entry main :: proc -> S64 {\n    print(«hello»);\n    println(«world»);\n    return 0;\n}", "helloworld\n", 0},
		{"file exists", "#entry main :: proc -> S64 {\n    if file_exists(«" + filePath + "») { println(«yes»); } else { println(«no»); }\n    if file_exists(«/nonexistent/path.xyz») { println(«bad»); } else { println(«missing»); }\n    return 0;\n}", "yes\nmissing\n", 0},
		{"read file", "#entry main :: proc -> S64 {\n    content := read_file(«" + filePath + "»);\n    print(content);\n    return 0;\n}", "file contents\n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm, diags := emitSource(t, tt.src)
			if diags.HasErrors() {
				t.Fatalf("emit errors: %v", diags)
			}
			asmPath := filepath.Join(dir, tt.name+".asm")
			binPath := filepath.Join(dir, tt.name+".bin")
			if err := os.WriteFile(asmPath, []byte(asm), 0o600); err != nil {
				t.Fatalf("write asm: %v", err)
			}
			if out, err := exec.Command(fasmPath, asmPath, binPath).CombinedOutput(); err != nil {
				t.Fatalf("fasm failed: %v\n%s", err, out)
			}
			if err := os.Chmod(binPath, 0o700); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			out, err := exec.Command(binPath).CombinedOutput()
			if err != nil {
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					if got := ee.ExitCode(); got != tt.code {
						t.Errorf("exit code = %d, want %d", got, tt.code)
					}
				} else {
					t.Fatalf("run failed: %v", err)
				}
			}
			if string(out) != tt.want {
				t.Errorf("stdout = %q, want %q", string(out), tt.want)
			}
		})
	}
}

// TestFasmGenericAppendPreservesStructFields verifies that appending a struct
// with a String field through a generic procedure preserves every field after
// the dynamic array grows. This is a regression test for generic
// instantiation sharing IdentExpr nodes, which left stale type facts that
// corrupted the aggregate copy in append's growth path.
func TestFasmGenericAppendPreservesStructFields(t *testing.T) {
	fasmPath, err := exec.LookPath("fasm")
	if err != nil {
		t.Skip("fasm not found; skipping runtime tests")
	}
	src := `#import «core»;
Item :: struct { kind: S64; text: String; line: S64; column: S64; }
#entry main :: proc -> S64 {
    arr: [dyn]Item;
    it: Item;
    it.kind = 1;
    it.text = «hello»;
    it.line = 7;
    it.column = 9;
    append(*arr, it);
    it.kind = 2;
    it.text = «world»;
    it.line = 8;
    it.column = 10;
    append(*arr, it);
    it.kind = 3;
    it.text = «foo»;
    it.line = 9;
    it.column = 11;
    append(*arr, it);
    it.kind = 4;
    it.text = «bar»;
    it.line = 10;
    it.column = 12;
    append(*arr, it);
    if arr.count != 4 { return 1; }
    if arr[0].kind != 1 { return 2; }
    if arr[0].text != «hello» { return 3; }
    if arr[0].line != 7 { return 4; }
    if arr[1].kind != 2 { return 5; }
    if arr[1].text != «world» { return 6; }
    if arr[1].line != 8 { return 7; }
    if arr[2].text != «foo» { return 8; }
    if arr[3].text != «bar» { return 9; }
    if arr[3].line != 10 { return 10; }
    if arr[3].column != 12 { return 11; }
    return 0;
}`
	asm, diags := emitSource(t, src)
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
	if err := exec.Command(binPath).Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Errorf("exit code = %d, want 0", ee.ExitCode())
			return
		}
		t.Fatalf("run failed: %v", err)
	}
}
