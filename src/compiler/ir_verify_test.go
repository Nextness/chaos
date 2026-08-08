package compiler

import "testing"

// verifySource lowers a source string and returns the verifier diagnostics.
func verifySource(t *testing.T, source string) DiagnosticList {
	t.Helper()
	_, mir := lowerSource(t, source)
	return VerifyMIR(mir)
}

func TestVerifyValidProgram(t *testing.T) {
	diags := verifySource(t, "add :: proc (a: S64, b: S64) -> S64 { return a + b; }\nmain :: proc {\n    x := add(1, 2);\n    if x > 0 { } else { }\n}")
	if diags.HasErrors() {
		t.Errorf("unexpected verify errors: %v", diags)
	}
}

func TestVerifyNoBlocks(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{Symbol: st.Declare("main"), Name: "main"}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "has no basic blocks") {
		t.Errorf("expected no-blocks error, got %v", diags)
	}
}

func TestVerifyMissingTerminator(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{ID: 0}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "has no terminator") {
		t.Errorf("expected missing-terminator error, got %v", diags)
	}
}

func TestVerifyUndefinedValue(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID:     0,
			Instrs: []*MIRInstr{{Result: 0, Op: MIRAdd, Type: tt.S64(), Args: []ValueID{1, 2}}},
			Term:   MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "undefined value") {
		t.Errorf("expected undefined-value error, got %v", diags)
	}
}

func TestVerifyStoreTypeMismatch(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol:     st.Declare("main"),
		Name:       "main",
		Locals:     []LocalID{0},
		LocalTypes: []TypeID{tt.S64()},
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRConst, Type: tt.String(), Imm: MIRImmediate{Kind: MIRImmString, Str: "x"}},
				{Result: NoValue, Op: MIRStoreLocal, Type: tt.String(), Args: []ValueID{0}, Imm: MIRImmediate{Kind: MIRImmLocal, Local: 0}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "does not match local type") {
		t.Errorf("expected store type-mismatch error, got %v", diags)
	}
}

func TestVerifyReturnMismatch(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol:  st.Declare("main"),
		Name:    "main",
		Results: []TypeID{tt.S64()},
		Blocks: []*MIRBlock{{
			ID:     0,
			Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: tt.String(), Imm: MIRImmediate{Kind: MIRImmString, Str: "x"}}},
			Term:   MIRTerminator{Kind: MIRReturn, Value: 0},
		}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "does not match result type") {
		t.Errorf("expected return type-mismatch error, got %v", diags)
	}
}

func TestVerifyBranchCondition(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{
			{
				ID:     0,
				Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt, Int: 1}}},
				Term:   MIRTerminator{Kind: MIRBranch, Cond: 0, Then: 1, Else: 2},
			},
			{ID: 1, Term: MIRTerminator{Kind: MIRJump, Target: 2}},
			{ID: 2, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}},
		},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "branch condition must be Bool") {
		t.Errorf("expected branch-condition error, got %v", diags)
	}
}

func TestVerifyCallArity(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	callee := &MIRFunction{
		Symbol:     st.Declare("add"),
		Name:       "add",
		Params:     []LocalID{0, 1},
		Locals:     []LocalID{0, 1},
		LocalTypes: []TypeID{tt.S64(), tt.S64()},
		Results:    []TypeID{tt.S64()},
		Blocks:     []*MIRBlock{{ID: 0, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}},
	}
	caller := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt, Int: 1}},
				{Result: 1, Op: MIRCall, Type: tt.S64(), Args: []ValueID{0}, Imm: MIRImmediate{Kind: MIRImmSymbol, Symbol: callee.Symbol}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{callee, caller}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "expects 2 arguments, got 1") {
		t.Errorf("expected call-arity error, got %v", diags)
	}
}

func TestVerifyBoolArgs(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt, Int: 1}},
				{Result: 1, Op: MIRAnd, Type: tt.Bool(), Args: []ValueID{0, 0}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "requires Bool operands") {
		t.Errorf("expected bool-operand error, got %v", diags)
	}
}

func TestVerifyStructInitArity(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	point := tt.InternStruct("Point")
	tt.SetStructFields(point, []TypeField{{Name: "x", Type: tt.S64()}, {Name: "y", Type: tt.S64()}})
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt, Int: 1}},
				{Result: 1, Op: MIRStructInit, Type: point, Args: []ValueID{0}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "expects 2 field values, got 1") {
		t.Errorf("expected struct-init arity error, got %v", diags)
	}
}

func TestVerifyJumpUndefinedBlock(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{ID: 0, Term: MIRTerminator{Kind: MIRJump, Target: 5}}},
	}
	prog := &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
	diags := VerifyMIR(prog)
	if !hasError(diags, "jump to undefined block") {
		t.Errorf("expected undefined-block error, got %v", diags)
	}
}
