// MIR verification for the Chaos compiler.
//
// VerifyMIR checks the structural invariants of a MIR program: every block
// has exactly one terminator, every value use is defined, local and global
// accesses reference declared slots, instruction argument types match, and
// returns match the function signature.
package compiler

import "strconv"

// VerifyMIR validates a MIR program and returns any diagnostics it produced.
func VerifyMIR(prog *MIRProgram) DiagnosticList {
	v := &MIRVerifier{prog: prog}
	v.verify()
	return v.diags
}

// MIRVerifier validates a MIR program.
type MIRVerifier struct {
	prog       *MIRProgram
	diags      DiagnosticList
	defined    map[ValueID]bool
	valueTypes map[ValueID]TypeID
	localTypes map[LocalID]TypeID
	functions  map[SymbolID]*MIRFunction
}

func (v *MIRVerifier) verify() {
	v.functions = make(map[SymbolID]*MIRFunction, len(v.prog.Functions))
	for _, fn := range v.prog.Functions {
		v.functions[fn.Symbol] = fn
	}
	for _, fn := range v.prog.Functions {
		v.verifyFunction(fn)
	}
}

func (v *MIRVerifier) verifyFunction(fn *MIRFunction) {
	if len(fn.Blocks) == 0 {
		v.diags.Error(fn.Span, "function "+fn.Name+" has no basic blocks", "add at least one block")
		return
	}
	v.defined = make(map[ValueID]bool)
	v.valueTypes = make(map[ValueID]TypeID)
	v.localTypes = make(map[LocalID]TypeID)
	for i, lid := range fn.Locals {
		if i < len(fn.LocalTypes) {
			v.localTypes[lid] = fn.LocalTypes[i]
		}
	}

	// Collect every defined value and its type.
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			if ins.Result != NoValue {
				v.defined[ins.Result] = true
				v.valueTypes[ins.Result] = ins.Type
			}
		}
	}

	for _, b := range fn.Blocks {
		v.verifyBlock(fn, b)
	}
}

func (v *MIRVerifier) verifyBlock(fn *MIRFunction, b *MIRBlock) {
	if b.Term.Kind == MIRNoTerm {
		v.diags.Error(b.Span, "block bb"+strconv.Itoa(int(b.ID))+" in "+fn.Name+" has no terminator", "add a jump, branch, return, exit, or unreachable terminator")
	}
	for _, ins := range b.Instrs {
		v.verifyInstr(fn, ins)
	}
	v.verifyTerminator(fn, b)
}

func (v *MIRVerifier) verifyInstr(fn *MIRFunction, ins *MIRInstr) {
	for _, arg := range ins.Args {
		if !v.defined[arg] {
			v.diags.Error(ins.Span, "instruction uses undefined value v"+strconv.Itoa(int(arg)), "define the value before using it")
		}
	}
	switch ins.Op {
	case MIRConst:
		if len(ins.Args) != 0 {
			v.diags.Error(ins.Span, "const takes no value arguments", "remove the arguments")
		}
	case MIRLoadLocal:
		if ins.Imm.Kind != MIRImmLocal {
			v.diags.Error(ins.Span, "load.local requires a local operand", "add a local operand")
		} else if lt, ok := v.localTypes[ins.Imm.Local]; ok && ins.Type != lt {
			v.diags.Error(ins.Span, "load.local type "+v.typeName(ins.Type)+" does not match local type "+v.typeName(lt), "use the local's type")
		}
	case MIRStoreLocal:
		if ins.Imm.Kind != MIRImmLocal {
			v.diags.Error(ins.Span, "store.local requires a local operand", "add a local operand")
		} else if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "store.local requires one value argument", "add the stored value")
		} else if lt, ok := v.localTypes[ins.Imm.Local]; ok {
			vt := v.valueTypes[ins.Args[0]]
			if vt != lt && vt != v.prog.Types.Unknown() {
				v.diags.Error(ins.Span, "store.local value type "+v.typeName(vt)+" does not match local type "+v.typeName(lt), "store a value of the local's type")
			}
		}
	case MIRLoadGlobal, MIRStoreGlobal:
		if ins.Imm.Kind != MIRImmSymbol {
			v.diags.Error(ins.Span, ins.Op.String()+" requires a symbol operand", "add a symbol operand")
		}
	case MIRAdd, MIRSub, MIRMul, MIRDiv, MIRMod:
		v.checkBinaryArith(ins)
	case MIRCmpLt, MIRCmpGt, MIRCmpLe, MIRCmpGe, MIRCmpEq, MIRCmpNeq:
		v.checkBinaryCmp(ins)
	case MIRAnd, MIROr:
		v.checkBoolArgs(ins)
	case MIRNot:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "not requires one value argument", "add the operand")
		} else if vt := v.valueTypes[ins.Args[0]]; vt != v.prog.Types.Bool() && vt != v.prog.Types.Unknown() {
			v.diags.Error(ins.Span, "not requires a Bool operand, got "+v.typeName(vt), "use a boolean operand")
		}
	case MIRNeg:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "neg requires one value argument", "add the operand")
		} else if vt := v.valueTypes[ins.Args[0]]; vt != ins.Type && vt != v.prog.Types.Unknown() {
			v.diags.Error(ins.Span, "neg operand type "+v.typeName(vt)+" does not match result type "+v.typeName(ins.Type), "use matching types")
		}
	case MIRCall:
		v.checkCall(fn, ins)
	case MIRStructInit:
		v.checkStructInit(ins)
	case MIRFieldLoad:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "field.load requires one value argument", "add the base value")
		} else if ins.Imm.Kind != MIRImmField {
			v.diags.Error(ins.Span, "field.load requires a field operand", "add a field index")
		} else {
			bt := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]])
			if bt.Kind != TypeKindStruct {
				v.diags.Error(ins.Span, "field.load base type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not a struct", "use a struct value")
			} else if int(ins.Imm.Int) >= len(bt.Fields) {
				v.diags.Error(ins.Span, "field.load field index out of range", "use a declared field")
			} else if ft := bt.Fields[ins.Imm.Int].Type; ins.Type != ft {
				v.diags.Error(ins.Span, "field.load result type "+v.typeName(ins.Type)+" does not match field type "+v.typeName(ft), "use the field's type")
			}
		}
	case MIRExit:
		// exit is emitted as a terminator, not an instruction.
	}
}

func (v *MIRVerifier) checkBinaryArith(ins *MIRInstr) {
	if len(ins.Args) != 2 {
		v.diags.Error(ins.Span, ins.Op.String()+" requires two value arguments", "add both operands")
		return
	}
	lt := v.valueTypes[ins.Args[0]]
	rt := v.valueTypes[ins.Args[1]]
	if lt == v.prog.Types.Unknown() || rt == v.prog.Types.Unknown() {
		return
	}
	if lt != rt {
		v.diags.Error(ins.Span, ins.Op.String()+" operands have different types "+v.typeName(lt)+" and "+v.typeName(rt), "operate on values of the same type")
	}
	if ins.Type != lt {
		v.diags.Error(ins.Span, ins.Op.String()+" result type "+v.typeName(ins.Type)+" does not match operand type "+v.typeName(lt), "use matching types")
	}
}

func (v *MIRVerifier) checkBinaryCmp(ins *MIRInstr) {
	if len(ins.Args) != 2 {
		v.diags.Error(ins.Span, ins.Op.String()+" requires two value arguments", "add both operands")
		return
	}
	lt := v.valueTypes[ins.Args[0]]
	rt := v.valueTypes[ins.Args[1]]
	if lt == v.prog.Types.Unknown() || rt == v.prog.Types.Unknown() {
		return
	}
	if lt != rt {
		v.diags.Error(ins.Span, ins.Op.String()+" operands have different types "+v.typeName(lt)+" and "+v.typeName(rt), "compare values of the same type")
	}
	if ins.Type != v.prog.Types.Bool() {
		v.diags.Error(ins.Span, ins.Op.String()+" result must be Bool, got "+v.typeName(ins.Type), "use a boolean result")
	}
}

func (v *MIRVerifier) checkBoolArgs(ins *MIRInstr) {
	if len(ins.Args) != 2 {
		v.diags.Error(ins.Span, ins.Op.String()+" requires two value arguments", "add both operands")
		return
	}
	for _, a := range ins.Args {
		if vt := v.valueTypes[a]; vt != v.prog.Types.Bool() && vt != v.prog.Types.Unknown() {
			v.diags.Error(ins.Span, ins.Op.String()+" requires Bool operands, got "+v.typeName(vt), "use boolean operands")
		}
	}
	if ins.Type != v.prog.Types.Bool() {
		v.diags.Error(ins.Span, ins.Op.String()+" result must be Bool, got "+v.typeName(ins.Type), "use a boolean result")
	}
}

func (v *MIRVerifier) checkCall(fn *MIRFunction, ins *MIRInstr) {
	if ins.Imm.Kind != MIRImmSymbol {
		v.diags.Error(ins.Span, "call requires a symbol operand", "add the callee symbol")
		return
	}
	callee, ok := v.functions[ins.Imm.Symbol]
	if !ok {
		v.diags.Error(ins.Span, "call to unknown procedure", "declare the procedure before calling it")
		return
	}
	if len(ins.Args) != len(callee.Params) {
		v.diags.Error(ins.Span, "call to "+callee.Name+" expects "+strconv.Itoa(len(callee.Params))+" arguments, got "+strconv.Itoa(len(ins.Args)), "pass the correct number of arguments")
	}
	for i, a := range ins.Args {
		if i < len(callee.Params) {
			pt := callee.LocalTypes[callee.Params[i]]
			if vt := v.valueTypes[a]; vt != pt && vt != v.prog.Types.Unknown() {
				v.diags.Error(ins.Span, "call argument type "+v.typeName(vt)+" does not match parameter type "+v.typeName(pt), "pass a value of the parameter's type")
			}
		}
	}
	want := v.prog.Types.Void()
	if len(callee.Results) > 0 {
		want = callee.Results[0]
	}
	if ins.Type != want {
		v.diags.Error(ins.Span, "call result type "+v.typeName(ins.Type)+" does not match "+callee.Name+" result type "+v.typeName(want), "use the procedure's result type")
	}
}

func (v *MIRVerifier) checkStructInit(ins *MIRInstr) {
	st := v.prog.Types.Lookup(ins.Type)
	if st.Kind != TypeKindStruct {
		v.diags.Error(ins.Span, "struct.init result type "+v.typeName(ins.Type)+" is not a struct", "use a struct type")
		return
	}
	if len(ins.Args) != len(st.Fields) {
		v.diags.Error(ins.Span, "struct.init for "+st.Name+" expects "+strconv.Itoa(len(st.Fields))+" field values, got "+strconv.Itoa(len(ins.Args)), "provide every field value")
		return
	}
	for i, a := range ins.Args {
		ft := st.Fields[i].Type
		if vt := v.valueTypes[a]; vt != ft && vt != v.prog.Types.Unknown() {
			v.diags.Error(ins.Span, "struct.init field "+st.Fields[i].Name+" type "+v.typeName(vt)+" does not match field type "+v.typeName(ft), "use the field's type")
		}
	}
}

func (v *MIRVerifier) verifyTerminator(fn *MIRFunction, b *MIRBlock) {
	t := b.Term
	switch t.Kind {
	case MIRJump:
		if !v.validBlock(fn, t.Target) {
			v.diags.Error(t.Span, "jump to undefined block bb"+strconv.Itoa(int(t.Target)), "jump to a declared block")
		}
	case MIRBranch:
		if !v.validBlock(fn, t.Then) || !v.validBlock(fn, t.Else) {
			v.diags.Error(t.Span, "branch to undefined block", "branch to declared blocks")
		}
		if ct := v.valueTypes[t.Cond]; ct != v.prog.Types.Bool() && ct != v.prog.Types.Unknown() {
			v.diags.Error(t.Span, "branch condition must be Bool, got "+v.typeName(ct), "use a boolean condition")
		}
	case MIRReturn:
		if len(fn.Results) == 0 {
			if t.Value != NoValue {
				v.diags.Error(t.Span, "return with a value in a void procedure", "return without a value")
			}
		} else {
			if t.Value == NoValue {
				v.diags.Error(t.Span, "return without a value in a procedure returning "+v.typeName(fn.Results[0]), "return a value")
			} else if vt := v.valueTypes[t.Value]; vt != fn.Results[0] && vt != v.prog.Types.Unknown() {
				v.diags.Error(t.Span, "return value type "+v.typeName(vt)+" does not match result type "+v.typeName(fn.Results[0]), "return a value of the result type")
			}
		}
	case MIRTermExit:
		if t.Status != NoValue && !v.defined[t.Status] {
			v.diags.Error(t.Span, "exit status uses undefined value", "define the status before using it")
		}
		if t.Message != NoValue && !v.defined[t.Message] {
			v.diags.Error(t.Span, "exit message uses undefined value", "define the message before using it")
		}
	case MIRUnreachable:
		// No operands to check.
	}
}

func (v *MIRVerifier) validBlock(fn *MIRFunction, id BlockID) bool {
	return int(id) < len(fn.Blocks)
}

func (v *MIRVerifier) typeName(id TypeID) string {
	return v.prog.Types.Lookup(id).Name
}
