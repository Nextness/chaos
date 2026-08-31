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

func TestVerifyUnknownLocal(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID:     0,
			Instrs: []*MIRInstr{{Result: 0, Op: MIRLoadLocal, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmLocal, Local: 7}}},
			Term:   MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	diags := VerifyMIR(&MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}})
	if !hasError(diags, "references unknown local l7") {
		t.Errorf("expected unknown-local error, got %v", diags)
	}
}

func TestVerifyUnknownGlobal(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	missing := st.Declare("missing")
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID:     0,
			Instrs: []*MIRInstr{{Result: 0, Op: MIRLoadGlobal, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmSymbol, Symbol: missing}}},
			Term:   MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	diags := VerifyMIR(&MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}})
	if !hasError(diags, "references unknown global missing") {
		t.Errorf("expected unknown-global error, got %v", diags)
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

func TestVerifyAddrOfRequiresExistingMatchingEntity(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	ptrByte := tt.InternPointer(tt.Byte(), false)
	fn := &MIRFunction{
		Symbol:       st.Declare("main"),
		Name:         "main",
		Locals:       []LocalID{0},
		LocalTypes:   []TypeID{tt.S64()},
		LocalMutable: []bool{true},
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRAddrOf, Type: ptrByte, Imm: MIRImmediate{Kind: MIRImmLocal, Local: 0}},
				{Result: 1, Op: MIRAddrOf, Type: ptrByte, Imm: MIRImmediate{Kind: MIRImmLocal, Local: 7}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	diags := VerifyMIR(&MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}})
	for _, message := range []string{"pointee type does not match", "references an unknown local"} {
		if !hasError(diags, message) {
			t.Errorf("expected %q error, got %v", message, diags)
		}
	}
}

func TestVerifyArrayElemAddrContract(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	array := tt.InternArray(tt.S64())
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRZero, Type: array},
				{Result: 1, Op: MIRConst, Type: tt.String(), Imm: MIRImmediate{Kind: MIRImmString, Str: "bad"}},
				{Result: 2, Op: MIRArrayElemAddr, Type: tt.S64(), Args: []ValueID{0, 1}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	diags := VerifyMIR(&MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}})
	for _, message := range []string{"index must be an integer", "result must be a pointer"} {
		if !hasError(diags, message) {
			t.Errorf("expected %q error, got %v", message, diags)
		}
	}
}

func TestVerifyFieldAddrRequiresFieldImmediate(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	field := st.Declare("value")
	record := tt.InternStruct("Record")
	tt.SetStructFields(record, []TypeField{{Symbol: field, Name: "value", Type: tt.S64()}})
	ptrRecord := tt.InternPointer(record, false)
	ptrS64 := tt.InternPointer(tt.S64(), false)
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRZero, Type: ptrRecord},
				{Result: 1, Op: MIRFieldAddr, Type: ptrS64, Args: []ValueID{0}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	diags := VerifyMIR(&MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}})
	if !hasError(diags, "requires a field operand") {
		t.Fatalf("expected field immediate error, got %v", diags)
	}
}

func TestVerifyDerefStoreInstructionType(t *testing.T) {
	tt := NewTypeTable()
	st := NewSymbolTable()
	ptrS64 := tt.InternPointer(tt.S64(), false)
	fn := &MIRFunction{
		Symbol: st.Declare("main"),
		Name:   "main",
		Blocks: []*MIRBlock{{
			ID: 0,
			Instrs: []*MIRInstr{
				{Result: 0, Op: MIRZero, Type: ptrS64},
				{Result: 1, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt, Int: 1}},
				{Result: NoValue, Op: MIRDerefStore, Type: tt.Byte(), Args: []ValueID{0, 1}},
			},
			Term: MIRTerminator{Kind: MIRReturn, Value: NoValue},
		}},
	}
	diags := VerifyMIR(&MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}})
	if !hasError(diags, "instruction type Byte does not match pointed-to type S64") {
		t.Fatalf("expected deref.store type error, got %v", diags)
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

func TestVerifyRejectsMalformedContracts(t *testing.T) {
	tests := []struct {
		name string
		want string
		make func(*TypeTable, *SymbolTable) *MIRProgram
	}{
		{
			name: "duplicate block",
			want: "basic block bb0 is defined more than once",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}, {ID: 0, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "duplicate value",
			want: "value v0 is defined more than once",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt}}, {Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "use before definition",
			want: "uses v1 before it is defined",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRNeg, Type: tt.S64(), Args: []ValueID{1}}, {Result: 1, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "non dominating definition",
			want: "does not dominate its use",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{
					{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: tt.Bool(), Imm: MIRImmediate{Kind: MIRImmBool}}}, Term: MIRTerminator{Kind: MIRBranch, Cond: 0, Then: 1, Else: 2}},
					{ID: 1, Instrs: []*MIRInstr{{Result: 1, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt}}}, Term: MIRTerminator{Kind: MIRJump, Target: 2}},
					{ID: 2, Instrs: []*MIRInstr{{Result: 2, Op: MIRNeg, Type: tt.S64(), Args: []ValueID{1}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}},
				}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "unknown type",
			want: "instruction references an invalid or unresolved type ID",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: TypeID(999), Imm: MIRImmediate{Kind: MIRImmInt}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "wrong immediate",
			want: "const immediate kind does not match S64",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmString, Str: "bad"}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "invalid arithmetic type",
			want: "add is not defined for String",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: tt.String(), Imm: MIRImmediate{Kind: MIRImmString}}, {Result: 1, Op: MIRAdd, Type: tt.String(), Args: []ValueID{0, 0}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "immutable store",
			want: "store.local mutates an immutable local",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Locals: []LocalID{0}, LocalTypes: []TypeID{tt.S64()}, LocalMutable: []bool{false}, Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: tt.S64(), Imm: MIRImmediate{Kind: MIRImmInt}}, {Result: NoValue, Op: MIRStoreLocal, Type: tt.S64(), Args: []ValueID{0}, Imm: MIRImmediate{Kind: MIRImmLocal, Local: 0}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "negative field",
			want: "field.load field index out of range",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				record := tt.InternStruct("R")
				tt.SetStructFields(record, []TypeField{{Name: "x", Type: tt.S64()}})
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRZero, Type: record}, {Result: 1, Op: MIRFieldLoad, Type: tt.S64(), Args: []ValueID{0}, Imm: MIRImmediate{Kind: MIRImmField, Int: -1}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "non integer index",
			want: "array.index index must be an integer",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				array := tt.InternArray(tt.S64())
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRArrayInit, Type: array}, {Result: 1, Op: MIRConst, Type: tt.Bool(), Imm: MIRImmediate{Kind: MIRImmBool}}, {Result: 2, Op: MIRArrayIndex, Type: tt.S64(), Args: []ValueID{0, 1}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "missing callee",
			want: "call to unknown procedure",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				missing := st.Declare("missing")
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: NoValue, Op: MIRCall, Type: tt.Void(), Imm: MIRImmediate{Kind: MIRImmSymbol, Symbol: missing}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "unknown opcode",
			want: "unknown MIR opcode",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIROpcode(255), Type: tt.S64()}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "unknown terminator",
			want: "unknown MIR terminator",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Term: MIRTerminator{Kind: MIRTermKind(255)}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "out of range const",
			want: "integer const does not fit S8",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				s8, _ := tt.ByName("S8")
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Instrs: []*MIRInstr{{Result: 0, Op: MIRConst, Type: s8, Imm: MIRImmediate{Kind: MIRImmInt, Int: 128}}}, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "missing exit status",
			want: "exit terminator requires a status value",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Term: MIRTerminator{Kind: MIRTermExit, Status: NoValue, Message: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
		{
			name: "disconnected block",
			want: "is unreachable from bb0",
			make: func(tt *TypeTable, st *SymbolTable) *MIRProgram {
				fn := &MIRFunction{Symbol: st.Declare("f"), Name: "f", ResultType: tt.Void(), Blocks: []*MIRBlock{{ID: 0, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}, {ID: 1, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
				return &MIRProgram{Symbols: st, Types: tt, Functions: []*MIRFunction{fn}}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types := NewTypeTable()
			symbols := NewSymbolTable()
			diags := VerifyMIR(tt.make(types, symbols))
			if !hasError(diags, tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, diags)
			}
		})
	}
}

func TestVerifyRejectsMalformedTypeTable(t *testing.T) {
	tests := []struct {
		name string
		want string
		edit func(*TypeTable, *SymbolTable)
	}{
		{
			name: "enum underlying",
			want: "has a non-integer underlying type",
			edit: func(types *TypeTable, _ *SymbolTable) {
				types.InternEnum("BadEnum", types.String())
			},
		},
		{
			name: "invalid field metadata",
			want: "has an invalid field type",
			edit: func(types *TypeTable, symbols *SymbolTable) {
				record := types.InternStruct("BadRecord")
				types.SetStructFields(record, []TypeField{{Symbol: symbols.Declare("field"), Name: "field", Type: TypeID(999)}})
			},
		},
		{
			name: "recursive record",
			want: "recursive by-value record layout",
			edit: func(types *TypeTable, symbols *SymbolTable) {
				record := types.InternStruct("Recursive")
				types.SetStructFields(record, []TypeField{{Symbol: symbols.Declare("self"), Name: "self", Type: record}})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types := NewTypeTable()
			symbols := NewSymbolTable()
			tt.edit(types, symbols)
			fn := &MIRFunction{Symbol: symbols.Declare("main"), Name: "main", ResultType: types.Void(), Blocks: []*MIRBlock{{ID: 0, Term: MIRTerminator{Kind: MIRReturn, Value: NoValue}}}}
			diags := VerifyMIR(&MIRProgram{Symbols: symbols, Types: types, Functions: []*MIRFunction{fn}})
			if !hasError(diags, tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, diags)
			}
		})
	}
}
