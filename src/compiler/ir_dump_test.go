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
		"Global G: S64",
		"store.local",
		"neg",
		"exit status=v",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DumpMIR output missing %q:\n%s", want, out)
		}
	}
}
