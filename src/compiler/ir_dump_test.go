package compiler

import (
	"strings"
	"testing"
)

func TestDumpHIR(t *testing.T) {
	hir, _ := lowerSource(t, "Point :: struct { x: S64; }\nmain :: proc (n: S64) -> S64 {\n    return n;\n}")
	out := DumpHIR(hir)
	for _, want := range []string{
		"HIR",
		"Struct Point",
		"Field x: S64",
		"Proc main(n: S64) -> S64",
		"Return Ident(n)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpHIR output missing %q:\n%s", want, out)
		}
	}
}

func TestDumpMIR(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc -> S64 {\n    x := 1 + 2;\n    return x;\n}")
	out := DumpMIR(mir)
	for _, want := range []string{
		"MIR",
		"Function main() -> S64",
		"bb0:",
		"add",
		"return",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpMIR output missing %q:\n%s", want, out)
		}
	}
}

func TestDumpMIRStructInit(t *testing.T) {
	_, mir := lowerSource(t, "Point :: struct { x: S64; y: S64; }\nmain :: proc {\n    p := Point.{y=2, x=1};\n}")
	out := DumpMIR(mir)
	if !strings.Contains(out, "struct.init") {
		t.Errorf("DumpMIR output missing struct.init:\n%s", out)
	}
}

func TestDumpHIRBroad(t *testing.T) {
	hir, _ := lowerSource(t, "G :: 1;\nmain :: proc {\n    x := 1;\n    x = 2;\n    y := -x;\n    exit 1, «boom»;\n}")
	out := DumpHIR(hir)
	for _, want := range []string{
		"Global G: S64",
		"VarDecl x: S64",
		"Assign x = Int(2)",
		"Unary(- Ident(x))",
		"Exit status=Int(1) message=String(\"boom\")",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpHIR output missing %q:\n%s", want, out)
		}
	}
}

func TestDumpMIRBroad(t *testing.T) {
	_, mir := lowerSource(t, "G :: 1;\nmain :: proc {\n    x := 1;\n    x = 2;\n    y := -x;\n    exit 1, «boom»;\n}")
	out := DumpMIR(mir)
	for _, want := range []string{
		"store.local",
		"neg",
		"exit status=v",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpMIR output missing %q:\n%s", want, out)
		}
	}
}

func TestDumpHIRCoversImplementedRuntimeNodes(t *testing.T) {
	source := "Box :: struct { value: S64; }\n#entry main :: proc -> S64 {\n    box := Box.{};\n    zero: S64;\n    addr := #allocate 8;\n    pointer: *S64 = addr.(*S64);\n    pointer.* = zero;\n    name := «x»;\n    text := «hello {name}»;\n    print(text);\n    println(text);\n    exists := file_exists(«/tmp»);\n    content := read_file(«/tmp/value»);\n    converted := zero.(F64);\n    #deallocate addr;\n    return 0;\n}"
	hir, _ := lowerSource(t, source)
	hir.Globals = append(hir.Globals, &HIRGlobal{Name: "void_zero", Type: hir.Types.Void(), Init: &HIRZero{Type: hir.Types.Void()}})
	out := DumpHIR(hir)
	for _, want := range []string{
		"Zero(Void)",
		"Allocate(Int(8))",
		"Cast(Ident(addr) as *S64)",
		"AddrStore",
		"Interpolate(\"hello \", Ident(name), \"\")",
		"Print(Ident(text))",
		"Println(Ident(text))",
		"FileExists(String(\"/tmp\"))",
		"ReadFile(String(\"/tmp/value\"))",
		"Convert(Ident(zero) to F64)",
		"Deallocate Ident(addr)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpHIR output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Stmt *compiler.") || strings.Contains(out, "Expr(*compiler.") {
		t.Errorf("DumpHIR used a generic fallback:\n%s", out)
	}
}
