package compiler

import "testing"

func findMIRFunction(mir *MIRProgram, name string) *MIRFunction {
	for _, fn := range mir.Functions {
		if fn.Name == name {
			return fn
		}
	}
	return nil
}

// findInstr returns the first instruction in a function with the given
// opcode, or nil.
func findInstr(fn *MIRFunction, op MIROpcode) *MIRInstr {
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			if ins.Op == op {
				return ins
			}
		}
	}
	return nil
}

// constInt returns the integer immediate of the const instruction that
// produced a value, if any.
func constInt(fn *MIRFunction, v ValueID) (int64, bool) {
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			if ins.Result == v && ins.Op == MIRConst && ins.Imm.Kind == MIRImmInt {
				return ins.Imm.Int, true
			}
		}
	}
	return 0, false
}

func TestMIRSimpleProc(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc -> S64 {\n    x := 1 + 2;\n    return x;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	if len(fn.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(fn.Blocks))
	}
	bb := fn.Blocks[0]
	if len(bb.Instrs) != 5 {
		t.Fatalf("instructions = %d, want 5 (const, const, add, store, load)", len(bb.Instrs))
	}
	if bb.Term.Kind != MIRReturn || bb.Term.Value == NoValue {
		t.Errorf("terminator = %+v, want return with a value", bb.Term)
	}
	if len(fn.Locals) != 1 || len(fn.LocalTypes) != 1 {
		t.Errorf("locals = %+v, want one local", fn.Locals)
	}
}

func TestMIRIf(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    a := true;\n    if a { } else { }\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	if len(fn.Blocks) != 4 {
		t.Fatalf("blocks = %d, want 4 (entry, then, else, join)", len(fn.Blocks))
	}
	entry := fn.Blocks[0]
	if entry.Term.Kind != MIRBranch {
		t.Errorf("entry terminator = %v, want branch", entry.Term.Kind)
	}
	if entry.Term.Then != 1 || entry.Term.Else != 2 {
		t.Errorf("branch targets = then %d else %d, want 1 and 2", entry.Term.Then, entry.Term.Else)
	}
	join := fn.Blocks[3]
	if join.Term.Kind != MIRReturn {
		t.Errorf("join terminator = %v, want return", join.Term.Kind)
	}
}

func TestMIRIfElif(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    a := true;\n    b := false;\n    if a { } elif b { } else { }\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	// entry, then, else(cond2), then2, else2, join2, join
	if len(fn.Blocks) != 7 {
		t.Fatalf("blocks = %d, want 7 for if/elif/else", len(fn.Blocks))
	}
}

func TestMIRExit(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    exit 1, «boom»;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	bb := fn.Blocks[0]
	if bb.Term.Kind != MIRTermExit {
		t.Fatalf("terminator = %v, want exit", bb.Term.Kind)
	}
	if bb.Term.Status == NoValue || bb.Term.Message == NoValue {
		t.Errorf("exit missing status or message: %+v", bb.Term)
	}
}

func TestMIRStructInitOrder(t *testing.T) {
	_, mir := lowerSource(t, "Point :: struct { x: S64; y: S64; }\nmain :: proc {\n    p := Point.{y=2, x=1};\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	si := findInstr(fn, MIRStructInit)
	if si == nil {
		t.Fatal("struct.init instruction not found")
	}
	if len(si.Args) != 2 {
		t.Fatalf("struct.init args = %d, want 2", len(si.Args))
	}
	// Fields must be reordered to declaration order: x=1 first, then y=2.
	x, _ := constInt(fn, si.Args[0])
	y, _ := constInt(fn, si.Args[1])
	if x != 1 || y != 2 {
		t.Errorf("struct.init args = [%d, %d], want [1, 2] (declaration order)", x, y)
	}
}

func TestMIRCall(t *testing.T) {
	_, mir := lowerSource(t, "add :: proc (a: S64, b: S64) -> S64 { return a + b; }\nmain :: proc {\n    x := add(1, 2);\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	call := findInstr(fn, MIRCall)
	if call == nil {
		t.Fatal("call instruction not found")
	}
	if len(call.Args) != 2 {
		t.Errorf("call args = %d, want 2", len(call.Args))
	}
	if mir.Types.Lookup(call.Type).Name != "S64" {
		t.Errorf("call type = %s, want S64", mir.Types.Lookup(call.Type).Name)
	}
	if mir.Symbols.Lookup(call.Imm.Symbol) != "add" {
		t.Errorf("call target = %s, want add", mir.Symbols.Lookup(call.Imm.Symbol))
	}
}

func TestMIRParams(t *testing.T) {
	_, mir := lowerSource(t, "add :: proc (a: S64, b: S64) -> S64 { return a + b; }")
	fn := findMIRFunction(mir, "add")
	if fn == nil {
		t.Fatal("function add not found")
	}
	if len(fn.Params) != 2 || fn.Params[0] != 0 || fn.Params[1] != 1 {
		t.Errorf("params = %+v, want [0 1]", fn.Params)
	}
	if len(fn.Locals) != 2 {
		t.Errorf("locals = %+v, want two locals", fn.Locals)
	}
	if mir.Types.Lookup(fn.LocalTypes[0]).Name != "S64" {
		t.Errorf("param 0 type = %s, want S64", mir.Types.Lookup(fn.LocalTypes[0]).Name)
	}
}

func TestMIRGlobal(t *testing.T) {
	_, mir := lowerSource(t, "G :: 5;\nmain :: proc {\n    x := G;\n}")
	if len(mir.Globals) != 0 {
		t.Fatalf("compile-time globals survived into MIR: %+v", mir.Globals)
	}
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	if load := findInstr(fn, MIRLoadGlobal); load != nil {
		t.Fatalf("compile-time G was loaded at runtime: %+v", load)
	}
}

func TestMIRCompileTimeArithmeticIsFolded(t *testing.T) {
	_, mir := lowerSource(t, "#entry main :: proc -> S64 { answer :: 40 + 2; return answer; }")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("main function not found")
	}
	if len(fn.Locals) != 0 {
		t.Fatalf("compile-time binding allocated locals: %+v", fn.Locals)
	}
	if add := findInstr(fn, MIRAdd); add != nil {
		t.Fatalf("compile-time addition survived into MIR: %+v", add)
	}
	constant := findInstr(fn, MIRConst)
	if constant == nil || constant.Imm.Kind != MIRImmInt || constant.Imm.Str != "42" {
		t.Fatalf("folded constant = %+v, want exact integer 42", constant)
	}
}

func TestMIRVoidProcFallsOffEnd(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    x := 1;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	last := fn.Blocks[len(fn.Blocks)-1]
	if last.Term.Kind != MIRReturn {
		t.Errorf("final terminator = %v, want return", last.Term.Kind)
	}
}

func TestMIRUnary(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    flag := false;\n    magnitude := 5;\n    x := -magnitude;\n    y := !flag;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	neg := findInstr(fn, MIRNeg)
	if neg == nil || len(neg.Args) != 1 {
		t.Errorf("neg instruction = %+v, want one operand", neg)
	}
	not := findInstr(fn, MIRNot)
	if not == nil || len(not.Args) != 1 {
		t.Errorf("not instruction = %+v, want one operand", not)
	}
}

func TestMIRAssign(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    x := 1;\n    x = 2;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	stores := 0
	for _, ins := range fn.Blocks[0].Instrs {
		if ins.Op == MIRStoreLocal {
			stores++
		}
	}
	if stores != 2 {
		t.Errorf("store.local count = %d, want 2 (init and reassignment)", stores)
	}
}

func TestMIRGlobalAssign(t *testing.T) {
	_, mir := lowerSource(t, "G: S64 = 1;\nmain :: proc {\n    G = 2;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	store := findInstr(fn, MIRStoreGlobal)
	if store == nil {
		t.Fatal("store.global instruction not found")
	}
	if mir.Symbols.Lookup(store.Imm.Symbol) != "G" {
		t.Errorf("store.global symbol = %s, want G", mir.Symbols.Lookup(store.Imm.Symbol))
	}
}

func TestMIRLogicalOps(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    a := true;\n    b := false;\n    c := a && b;\n    d := a || b;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	and := findInstr(fn, MIRAnd)
	if and == nil || len(and.Args) != 2 {
		t.Errorf("and instruction = %+v, want two operands", and)
	}
	or := findInstr(fn, MIROr)
	if or == nil || len(or.Args) != 2 {
		t.Errorf("or instruction = %+v, want two operands", or)
	}
}

func TestMIRConstImmediates(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    a := 1.5;\n    b := «hi»;\n    c := true;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	bb := fn.Blocks[0]
	var sawFloat, sawString, sawBool bool
	for _, ins := range bb.Instrs {
		if ins.Op != MIRConst {
			continue
		}
		switch ins.Imm.Kind {
		case MIRImmFloat:
			sawFloat = true
		case MIRImmString:
			sawString = true
		case MIRImmBool:
			sawBool = true
		}
	}
	if !sawFloat || !sawString || !sawBool {
		t.Errorf("const immediates: float=%v string=%v bool=%v, want all true", sawFloat, sawString, sawBool)
	}
}

func TestMIRExitStatusOnly(t *testing.T) {
	_, mir := lowerSource(t, "main :: proc {\n    exit 1;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("function main not found")
	}
	term := fn.Blocks[0].Term
	if term.Kind != MIRTermExit || term.Status == NoValue || term.Message != NoValue {
		t.Errorf("exit terminator = %+v, want status only", term)
	}
}
