// MIR verification for the Chaos compiler.
//
// VerifyMIR checks the structural invariants of a MIR program: every block
// has exactly one terminator, every value use is defined, local and global
// accesses reference declared slots, instruction argument types match, and
// returns match the function signature.
package compiler

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

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
	localMut   map[LocalID]bool
	functions  map[SymbolID]*MIRFunction
	globals    map[SymbolID]MIRGlobal
	blocks     map[BlockID]*MIRBlock
	defSites   map[ValueID]mirDefSite
	dominators map[BlockID]map[BlockID]bool
}

type mirDefSite struct {
	block BlockID
	index int
}

func (v *MIRVerifier) verify() {
	if v.prog == nil {
		v.diags.Error(Span{}, "cannot verify a nil MIR program", "report this compiler bug")
		return
	}
	if v.prog.Types == nil || v.prog.Symbols == nil {
		v.diags.Error(Span{}, "MIR program is missing its symbol or type table", "report this compiler bug")
		return
	}
	v.verifyTypeTable()
	v.functions = make(map[SymbolID]*MIRFunction, len(v.prog.Functions))
	for _, fn := range v.prog.Functions {
		if fn == nil {
			v.diags.Error(Span{}, "MIR program contains a nil function", "report this compiler bug")
			continue
		}
		if int(fn.Symbol) >= v.prog.Symbols.Len() {
			v.diags.Error(fn.Span, "function references an invalid symbol ID", "report this compiler bug")
		}
		if _, exists := v.functions[fn.Symbol]; exists {
			v.diags.Error(fn.Span, "function symbol is defined more than once", "give every function a unique symbol")
			continue
		}
		v.functions[fn.Symbol] = fn
	}
	v.globals = make(map[SymbolID]MIRGlobal, len(v.prog.Globals))
	for _, global := range v.prog.Globals {
		if int(global.Symbol) >= v.prog.Symbols.Len() {
			v.diags.Error(global.Span, "global references an invalid symbol ID", "report this compiler bug")
		}
		if _, exists := v.globals[global.Symbol]; exists {
			v.diags.Error(global.Span, "global symbol is defined more than once", "give every global a unique symbol")
			continue
		}
		if _, exists := v.functions[global.Symbol]; exists {
			v.diags.Error(global.Span, "a global and function share the same symbol ID", "give every declaration a unique symbol")
		}
		v.requireType(global.Type, global.Span, "global")
		if global.CompileTime {
			v.diags.Error(global.Span, "compile-time binding survived into executable MIR", "substitute compile-time values before MIR lowering")
		}
		v.globals[global.Symbol] = global
	}
	v.verifyGlobalInit()
	if v.prog.Entry != NoSymbol {
		if _, ok := v.functions[v.prog.Entry]; !ok {
			v.diags.Error(Span{}, "MIR entry references an unknown function", "select a declared entry procedure")
		}
	}
	for _, fn := range v.prog.Functions {
		if fn != nil {
			v.verifyFunction(fn)
		}
	}
}

func (v *MIRVerifier) verifyTypeTable() {
	types := v.prog.Types
	for i, typ := range types.types {
		if typ.ID != TypeID(i) {
			v.diags.Error(Span{}, "type table entry has an inconsistent TypeID", "rebuild the type table with canonical IDs")
		}
		if typ.Kind > TypeKindUnknown {
			v.diags.Error(Span{}, "type table contains an unknown type kind", "use a declared IR type kind")
			continue
		}
		if info, builtin := LookupBuiltinType(typ.Name); builtin {
			want := TypeKindUnknown
			switch info.Kind {
			case BuiltinVoid:
				want = TypeKindVoid
			case BuiltinBool:
				want = TypeKindBool
			case BuiltinString:
				want = TypeKindString
			case BuiltinInteger:
				want = TypeKindInt
			case BuiltinFloat:
				want = TypeKindFloat
			case BuiltinAddr:
				want = TypeKindAddr
			}
			if typ.Kind != want {
				v.diags.Error(Span{}, "built-in type "+typ.Name+" has inconsistent metadata", "preserve the compiler built-in type registry")
			}
		} else if typ.Kind == TypeKindInt || typ.Kind == TypeKindFloat || typ.Kind == TypeKindBool || typ.Kind == TypeKindString || typ.Kind == TypeKindVoid || typ.Kind == TypeKindAddr {
			v.diags.Error(Span{}, "primitive type metadata uses unknown name '"+typ.Name+"'", "use a compiler-defined primitive type")
		}
		switch typ.Kind {
		case TypeKindArray:
			if !v.validResolvedTypeID(typ.Elem) {
				v.diags.Error(Span{}, "array type "+typ.Name+" has an invalid element type", "use a resolved element type")
			}
		case TypeKindPointer:
			if !v.validResolvedTypeID(typ.Elem) {
				v.diags.Error(Span{}, "pointer type "+typ.Name+" has an invalid pointed-to type", "use a resolved element type")
			}
		case TypeKindEnum:
			if !v.validResolvedTypeID(typ.Underlying) || types.Lookup(typ.Underlying).Kind != TypeKindInt {
				v.diags.Error(Span{}, "enum type "+typ.Name+" has a non-integer underlying type", "use a resolved integer underlying type")
			}
		case TypeKindStruct, TypeKindTuple:
			seenFields := make(map[string]bool, len(typ.Fields))
			for _, field := range typ.Fields {
				if field.Name == "" || seenFields[field.Name] {
					v.diags.Error(Span{}, "record type "+typ.Name+" has invalid or duplicate field metadata", "give every field a non-empty unique name")
				}
				seenFields[field.Name] = true
				if int(field.Symbol) >= v.prog.Symbols.Len() {
					v.diags.Error(Span{}, "record type "+typ.Name+" references an invalid field symbol", "declare every field symbol")
				}
				if !v.validResolvedTypeID(field.Type) {
					v.diags.Error(Span{}, "record type "+typ.Name+" has an invalid field type", "resolve every field type")
				}
			}
		}
	}

	// Arrays are pointer/length values and intentionally break storage cycles.
	state := make(map[TypeID]uint8, types.Len())
	var visit func(TypeID) bool
	visit = func(id TypeID) bool {
		if state[id] == 1 {
			return true
		}
		if state[id] == 2 || !v.validResolvedTypeID(id) {
			return false
		}
		typ := types.Lookup(id)
		if typ.Kind != TypeKindStruct && typ.Kind != TypeKindTuple {
			return false
		}
		state[id] = 1
		for _, field := range typ.Fields {
			if visit(field.Type) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for i, typ := range types.types {
		if (typ.Kind == TypeKindStruct || typ.Kind == TypeKindTuple) && visit(TypeID(i)) {
			v.diags.Error(Span{}, "type table contains a recursive by-value record layout involving "+typ.Name, "break the cycle with an indirection")
			state[TypeID(i)] = 2
		}
	}
}

func (v *MIRVerifier) validResolvedTypeID(id TypeID) bool {
	return int(id) < v.prog.Types.Len() && v.prog.Types.Lookup(id).Kind != TypeKindUnknown
}

// pointerAssignCompatible reports whether a value of type vt can be stored
// where a value of type target is expected. Non-nullable pointer values are
// compatible with nullable pointer slots of the same pointed-to type (the
// non-null-to-nullable widening).
func (v *MIRVerifier) pointerAssignCompatible(vt, target TypeID) bool {
	vtT, ftT := v.prog.Types.Lookup(vt), v.prog.Types.Lookup(target)
	return vtT.Kind == TypeKindPointer && ftT.Kind == TypeKindPointer &&
		vtT.Elem == ftT.Elem && strings.HasSuffix(ftT.Name, "?")
}

func (v *MIRVerifier) verifyGlobalInit() {
	if v.prog.GlobalInit == nil {
		return
	}
	count := 0
	for _, fn := range v.prog.Functions {
		if fn == v.prog.GlobalInit {
			count++
		}
	}
	if count != 1 {
		v.diags.Error(v.prog.GlobalInit.Span, "global initializer must appear exactly once in the function list", "retain one shared global initializer function")
	}
	fn := v.prog.GlobalInit
	if fn.Name != "__chaos_global_init" || len(fn.Params) != 0 || len(fn.Results) != 0 || fn.ResultType != v.prog.Types.Void() {
		v.diags.Error(fn.Span, "global initializer has an invalid signature", "use a reserved, parameterless Void initializer")
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
	v.localMut = make(map[LocalID]bool)
	v.blocks = make(map[BlockID]*MIRBlock, len(fn.Blocks))
	v.defSites = make(map[ValueID]mirDefSite)
	if len(fn.LocalTypes) != len(fn.Locals) || len(fn.LocalMutable) != len(fn.Locals) {
		v.diags.Error(fn.Span, "function "+fn.Name+" has inconsistent local metadata", "provide one type and mutability flag for every local")
	}
	for _, lid := range fn.Locals {
		if _, exists := v.localTypes[lid]; exists {
			v.diags.Error(fn.Span, "local l"+strconv.Itoa(int(lid))+" is declared more than once", "give every local a unique ID")
			continue
		}
		if int(lid) >= len(fn.LocalTypes) {
			v.diags.Error(fn.Span, "local l"+strconv.Itoa(int(lid))+" has no type metadata", "index local metadata by LocalID")
			continue
		}
		v.localTypes[lid] = fn.LocalTypes[lid]
		v.requireType(fn.LocalTypes[lid], fn.Span, "local")
		if int(lid) < len(fn.LocalMutable) {
			v.localMut[lid] = fn.LocalMutable[lid]
		}
	}
	seenParams := make(map[LocalID]bool, len(fn.Params))
	for _, lid := range fn.Params {
		if seenParams[lid] {
			v.diags.Error(fn.Span, "parameter local l"+strconv.Itoa(int(lid))+" is listed more than once", "list each parameter once")
		}
		seenParams[lid] = true
		if _, ok := v.localTypes[lid]; !ok {
			v.diags.Error(fn.Span, "parameter references unknown local l"+strconv.Itoa(int(lid)), "declare the parameter local")
		}
	}
	if len(fn.Results) > 1 {
		v.diags.Error(fn.Span, "MIR function has more than one physical result", "aggregate source-level multiple results into one tuple type")
	}
	wantResult := v.prog.Types.Void()
	if len(fn.Results) == 1 {
		wantResult = fn.Results[0]
		v.requireType(wantResult, fn.Span, "function result")
	}
	if fn.ResultType != wantResult {
		v.diags.Error(fn.Span, "function result metadata is inconsistent", "make ResultType match the physical MIR result")
	}

	for _, b := range fn.Blocks {
		if b == nil {
			v.diags.Error(fn.Span, "function "+fn.Name+" contains a nil basic block", "report this compiler bug")
			continue
		}
		if _, exists := v.blocks[b.ID]; exists {
			v.diags.Error(b.Span, "basic block bb"+strconv.Itoa(int(b.ID))+" is defined more than once", "give every block a unique ID")
			continue
		}
		v.blocks[b.ID] = b
		for index, ins := range b.Instrs {
			if ins == nil {
				v.diags.Error(b.Span, "basic block contains a nil instruction", "report this compiler bug")
				continue
			}
			if ins.Result != NoValue {
				if _, exists := v.defSites[ins.Result]; exists {
					v.diags.Error(ins.Span, "value v"+strconv.Itoa(int(ins.Result))+" is defined more than once", "give every instruction result a unique ValueID")
					continue
				}
				v.defined[ins.Result] = true
				v.valueTypes[ins.Result] = ins.Type
				v.defSites[ins.Result] = mirDefSite{block: b.ID, index: index}
			}
		}
	}
	if _, ok := v.blocks[0]; !ok || fn.Blocks[0] == nil || fn.Blocks[0].ID != 0 {
		v.diags.Error(fn.Span, "function "+fn.Name+" has no canonical bb0 entry block", "make bb0 the first block")
	}
	v.computeDominators(fn)

	for _, b := range fn.Blocks {
		if b != nil {
			v.verifyBlock(fn, b)
		}
	}
}

func (v *MIRVerifier) verifyBlock(fn *MIRFunction, b *MIRBlock) {
	if b.Term.Kind == MIRNoTerm {
		v.diags.Error(b.Span, "block bb"+strconv.Itoa(int(b.ID))+" in "+fn.Name+" has no terminator", "add a jump, branch, return, exit, or unreachable terminator")
	}
	for index, ins := range b.Instrs {
		if ins != nil {
			v.verifyInstr(fn, b, index, ins)
		}
	}
	v.verifyTerminator(fn, b)
}

func (v *MIRVerifier) verifyInstr(fn *MIRFunction, b *MIRBlock, index int, ins *MIRInstr) {
	for _, arg := range ins.Args {
		v.checkUse(arg, b.ID, index, ins.Span, "instruction")
	}
	if int(ins.Op) > int(MIROpcodeMax) {
		v.diags.Error(ins.Span, "unknown MIR opcode "+strconv.Itoa(int(ins.Op)), "report this compiler bug")
		return
	}
	v.requireType(ins.Type, ins.Span, "instruction")
	produces := ins.Op != MIRStoreLocal && ins.Op != MIRStoreGlobal && ins.Op != MIRDerefStore && ins.Op != MIRDeallocate
	if ins.Op == MIRCall && ins.Type == v.prog.Types.Void() {
		produces = false
	}
	if produces && ins.Result == NoValue {
		v.diags.Error(ins.Span, ins.Op.String()+" must produce a result", "assign a fresh ValueID")
	}
	if !produces && ins.Result != NoValue {
		v.diags.Error(ins.Span, ins.Op.String()+" must not produce a result", "use NoValue for side-effect-only instructions")
	}
	if ins.Initializing && ins.Op != MIRStoreLocal && ins.Op != MIRStoreGlobal {
		v.diags.Error(ins.Span, "only a store may be marked as initialization", "remove the initialization marker")
	}
	switch ins.Op {
	case MIRConst:
		if len(ins.Args) != 0 {
			v.diags.Error(ins.Span, "const takes no value arguments", "remove the arguments")
		}
		v.checkConst(ins)
	case MIRLoadLocal:
		if len(ins.Args) != 0 {
			v.diags.Error(ins.Span, "load.local takes no value arguments", "remove the arguments")
		}
		if ins.Imm.Kind != MIRImmLocal {
			v.diags.Error(ins.Span, "load.local requires a local operand", "add a local operand")
		} else if lt, ok := v.localTypes[ins.Imm.Local]; !ok {
			v.diags.Error(ins.Span, "load.local references unknown local l"+strconv.Itoa(int(ins.Imm.Local)), "use a declared local")
		} else if ins.Type != lt {
			v.diags.Error(ins.Span, "load.local type "+v.typeName(ins.Type)+" does not match local type "+v.typeName(lt), "use the local's type")
		}
	case MIRStoreLocal:
		if ins.Imm.Kind != MIRImmLocal {
			v.diags.Error(ins.Span, "store.local requires a local operand", "add a local operand")
		} else if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "store.local requires one value argument", "add the stored value")
		} else if lt, ok := v.localTypes[ins.Imm.Local]; !ok {
			v.diags.Error(ins.Span, "store.local references unknown local l"+strconv.Itoa(int(ins.Imm.Local)), "use a declared local")
		} else {
			vt := v.valueTypes[ins.Args[0]]
			if vt != lt && vt != v.prog.Types.Unknown() && !v.pointerAssignCompatible(vt, lt) {
				v.diags.Error(ins.Span, "store.local value type "+v.typeName(vt)+" does not match local type "+v.typeName(lt), "store a value of the local's type")
			}
			if ins.Type != lt {
				v.diags.Error(ins.Span, "store.local instruction type does not match local type", "use the local's type")
			}
			if !v.localMut[ins.Imm.Local] && !ins.Initializing {
				v.diags.Error(ins.Span, "store.local mutates an immutable local", "only initialize the binding once or declare it mutable")
			}
		}
	case MIRLoadGlobal:
		if len(ins.Args) != 0 {
			v.diags.Error(ins.Span, "load.global takes no value arguments", "remove the arguments")
		}
		if ins.Imm.Kind != MIRImmSymbol {
			v.diags.Error(ins.Span, ins.Op.String()+" requires a symbol operand", "add a symbol operand")
		} else if global, ok := v.globals[ins.Imm.Symbol]; !ok {
			v.diags.Error(ins.Span, "load.global references unknown global "+v.prog.Symbols.Lookup(ins.Imm.Symbol), "use a declared global")
		} else if ins.Type != global.Type {
			v.diags.Error(ins.Span, "load.global type "+v.typeName(ins.Type)+" does not match global type "+v.typeName(global.Type), "use the global's type")
		}
	case MIRStoreGlobal:
		if ins.Imm.Kind != MIRImmSymbol {
			v.diags.Error(ins.Span, ins.Op.String()+" requires a symbol operand", "add a symbol operand")
		} else if global, ok := v.globals[ins.Imm.Symbol]; !ok {
			v.diags.Error(ins.Span, "store.global references unknown global "+v.prog.Symbols.Lookup(ins.Imm.Symbol), "use a declared global")
		} else if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "store.global requires one value argument", "add the stored value")
		} else {
			if vt := v.valueTypes[ins.Args[0]]; vt != global.Type && vt != v.prog.Types.Unknown() && !v.pointerAssignCompatible(vt, global.Type) {
				v.diags.Error(ins.Span, "store.global value type "+v.typeName(vt)+" does not match global type "+v.typeName(global.Type), "store a value of the global's type")
			}
			if ins.Type != global.Type {
				v.diags.Error(ins.Span, "store.global instruction type does not match global type", "use the global's type")
			}
			if !global.Mutable && !ins.Initializing {
				v.diags.Error(ins.Span, "store.global mutates an immutable global", "only initialize the binding once or declare it mutable")
			}
			if ins.Initializing && fn != v.prog.GlobalInit {
				v.diags.Error(ins.Span, "global initialization occurs outside the global initializer", "initialize globals in the reserved initializer function")
			}
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
		if ins.Type != v.prog.Types.Bool() || ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "not must produce Bool and take no immediate", "use the Bool result contract")
		}
	case MIRNeg:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "neg requires one value argument", "add the operand")
		} else if vt := v.valueTypes[ins.Args[0]]; vt != ins.Type && vt != v.prog.Types.Unknown() {
			v.diags.Error(ins.Span, "neg operand type "+v.typeName(vt)+" does not match result type "+v.typeName(ins.Type), "use matching types")
		} else if !v.isNumeric(ins.Type) {
			v.diags.Error(ins.Span, "neg requires a numeric type, got "+v.typeName(ins.Type), "use an integer or floating-point operand")
		}
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "neg takes no immediate operand", "remove the immediate")
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
			if !isRecordKind(bt.Kind) && bt.Kind != TypeKindString {
				v.diags.Error(ins.Span, "field.load base type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not a record", "use a struct or tuple value")
			} else if ins.Imm.Int < 0 || int(ins.Imm.Int) >= len(bt.Fields) {
				v.diags.Error(ins.Span, "field.load field index out of range", "use a declared field")
			} else if ft := bt.Fields[ins.Imm.Int].Type; ins.Type != ft {
				v.diags.Error(ins.Span, "field.load result type "+v.typeName(ins.Type)+" does not match field type "+v.typeName(ft), "use the field's type")
			}
		}
	case MIRZero:
		if len(ins.Args) != 0 || ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "zero takes no operands", "remove the operands")
		}
	case MIRArrayInit:
		t := v.prog.Types.Lookup(ins.Type)
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "array.init takes no immediate operand", "remove the immediate")
		}
		if t.Kind != TypeKindArray {
			v.diags.Error(ins.Span, "array.init result type "+v.typeName(ins.Type)+" is not an array", "use an array type")
		} else {
			et := t.Elem
			for i, a := range ins.Args {
				if vt := v.valueTypes[a]; vt != et && vt != v.prog.Types.Unknown() {
					v.diags.Error(ins.Span, "array.init element "+strconv.Itoa(i)+" type "+v.typeName(vt)+" does not match element type "+v.typeName(et), "use the element type")
				}
			}
		}
	case MIRArrayLen:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "array.len requires one value argument", "add the array value")
		} else if at := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]]); at.Kind != TypeKindArray && at.Kind != TypeKindUnknown {
			v.diags.Error(ins.Span, "array.len operand type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not an array", "use an array value")
		}
		if v.prog.Types.Lookup(ins.Type).Kind != TypeKindInt || ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "array.len must produce an integer and take no immediate", "use the array length contract")
		}
	case MIRArrayIndex:
		if len(ins.Args) != 2 {
			v.diags.Error(ins.Span, "array.index requires two value arguments", "add the array and index values")
		} else if at := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]]); at.Kind != TypeKindArray && at.Kind != TypeKindUnknown {
			v.diags.Error(ins.Span, "array.index base type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not an array", "use an array value")
		} else if at.Kind == TypeKindArray && ins.Type != at.Elem {
			v.diags.Error(ins.Span, "array.index result type "+v.typeName(ins.Type)+" does not match element type "+v.typeName(at.Elem), "use the element type")
		}
		if len(ins.Args) == 2 && v.prog.Types.Lookup(v.valueTypes[ins.Args[1]]).Kind != TypeKindInt {
			v.diags.Error(ins.Span, "array.index index must be an integer", "use an integer index")
		}
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "array.index takes no immediate operand", "remove the immediate")
		}
	case MIRAddrOf:
		if len(ins.Args) != 0 {
			v.diags.Error(ins.Span, "addr.of takes no value arguments", "remove the arguments")
		}
		if ins.Imm.Kind != MIRImmLocal && ins.Imm.Kind != MIRImmSymbol {
			v.diags.Error(ins.Span, "addr.of requires a local or symbol operand", "add the addressed entity")
		} else if v.prog.Types.Lookup(ins.Type).Kind != TypeKindPointer {
			v.diags.Error(ins.Span, "addr.of result must be a pointer type", "use the pointer to the addressed entity")
		}
	case MIRDerefLoad:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "deref.load requires one value argument", "add the pointer value")
		} else if pt := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]]); pt.Kind == TypeKindPointer {
			if ins.Type != pt.Elem {
				v.diags.Error(ins.Span, "deref.load result type "+v.typeName(ins.Type)+" does not match pointed-to type "+v.typeName(pt.Elem), "use the pointed-to type")
			}
		} else if pt.Kind != TypeKindUnknown {
			v.diags.Error(ins.Span, "deref.load operand type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not a pointer", "use a pointer value")
		}
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "deref.load takes no immediate operand", "remove the immediate")
		}
	case MIRDerefStore:
		if len(ins.Args) != 2 {
			v.diags.Error(ins.Span, "deref.store requires two value arguments", "add the address and the stored value")
		} else if pt := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]]); pt.Kind == TypeKindPointer {
			if vt := v.valueTypes[ins.Args[1]]; vt != pt.Elem && vt != v.prog.Types.Unknown() {
				v.diags.Error(ins.Span, "deref.store value type "+v.typeName(vt)+" does not match pointed-to type "+v.typeName(pt.Elem), "store a value of the pointed-to type")
			}
		} else if pt.Kind != TypeKindUnknown {
			v.diags.Error(ins.Span, "deref.store first argument type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not a pointer", "use a pointer value")
		}
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "deref.store takes no immediate operand", "remove the immediate")
		}
	case MIRArrayElemAddr:
		if len(ins.Args) != 2 {
			v.diags.Error(ins.Span, "array.elem.addr requires two value arguments", "add the array and index values")
		} else if at := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]]); at.Kind != TypeKindArray && at.Kind != TypeKindUnknown {
			v.diags.Error(ins.Span, "array.elem.addr base type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not an array", "use an array value")
		} else if pt := v.prog.Types.Lookup(ins.Type); pt.Kind == TypeKindPointer && at.Kind == TypeKindArray && pt.Elem != at.Elem {
			v.diags.Error(ins.Span, "array.elem.addr result type does not point at the array element type", "use the pointer-to-element type")
		}
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "array.elem.addr takes no immediate operand", "remove the immediate")
		}
	case MIRFieldAddr:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "field.addr requires one value argument", "add the base address")
		} else if at := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]]); at.Kind == TypeKindPointer {
			st := v.prog.Types.Lookup(at.Elem)
			fi := int(ins.Imm.Int)
			if st.Kind != TypeKindStruct && st.Kind != TypeKindTuple && st.Kind != TypeKindString {
				v.diags.Error(ins.Span, "field.addr base does not point at a record type", "use the address of a struct")
			} else if fi < 0 || fi >= len(st.Fields) {
				v.diags.Error(ins.Span, "field.addr references an unknown field index", "use a declared field index")
			} else if pt := v.prog.Types.Lookup(ins.Type); pt.Kind != TypeKindPointer || pt.Elem != st.Fields[fi].Type {
				v.diags.Error(ins.Span, "field.addr result does not point at the field type", "use the pointer-to-field type")
			}
		} else if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "field.addr takes no immediate operand", "remove the immediate")
		}
	case MIRPtrAdd, MIRPtrSub, MIRPtrDiff:
		if len(ins.Args) != 2 {
			v.diags.Error(ins.Span, ins.Op.String()+" requires two value arguments", "add both operands")
		} else {
			lt := v.valueTypes[ins.Args[0]]
			rt := v.valueTypes[ins.Args[1]]
			ltKind := v.prog.Types.Lookup(lt).Kind
			rtKind := v.prog.Types.Lookup(rt).Kind
			if ins.Op == MIRPtrDiff {
				if (ltKind != TypeKindPointer && lt != v.prog.Types.Unknown()) || (rtKind != TypeKindPointer && rt != v.prog.Types.Unknown()) || (ltKind == TypeKindPointer && lt != rt) {
					v.diags.Error(ins.Span, "ptr.diff requires two pointers of the same type", "subtract pointers of the same type")
				}
				if ins.Type != v.prog.Types.S64() {
					v.diags.Error(ins.Span, "ptr.diff result must be S64", "use the S64 result contract")
				}
			} else {
				if (ltKind != TypeKindPointer && lt != v.prog.Types.Unknown()) || (rtKind != TypeKindInt && rt != v.prog.Types.Unknown()) {
					v.diags.Error(ins.Span, ins.Op.String()+" requires a pointer and an integer offset", "use a pointer plus an integer")
				}
				if ins.Type != lt {
					v.diags.Error(ins.Span, ins.Op.String()+" result type must match the pointer type", "use the pointer result type")
				}
			}
		}
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, ins.Op.String()+" takes no immediate operand", "remove the immediate")
		}
	case MIRAllocate:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "allocate requires one value argument", "add the size")
		} else if v.prog.Types.Lookup(v.valueTypes[ins.Args[0]]).Kind != TypeKindInt {
			v.diags.Error(ins.Span, "allocate size must be an integer", "use an integer size")
		}
		if v.prog.Types.Lookup(ins.Type).Kind != TypeKindAddr || ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "allocate must produce Addr and take no immediate", "use the Addr result contract")
		}
	case MIRDeallocate:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "deallocate requires one value argument", "add the address")
		} else {
			at := v.prog.Types.Lookup(v.valueTypes[ins.Args[0]])
			if at.Kind != TypeKindAddr && at.Kind != TypeKindPointer && at.Kind != TypeKindUnknown {
				v.diags.Error(ins.Span, "deallocate operand type "+v.typeName(v.valueTypes[ins.Args[0]])+" is not an address", "pass an Addr or pointer value")
			}
		}
		if ins.Type != v.prog.Types.Void() || ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "deallocate must produce Void and take no immediate", "use the Void result contract")
		}
	case MIRCast:
		if len(ins.Args) != 1 {
			v.diags.Error(ins.Span, "cast requires one value argument", "add the value to cast")
		}
		if ins.Imm.Kind != MIRImmNone {
			v.diags.Error(ins.Span, "cast takes no immediate operand", "remove the immediate")
		}
	case MIRInterpolate:
		if ins.Type != v.prog.Types.String() {
			v.diags.Error(ins.Span, "interpolate must produce String", "use the String result contract")
		}
		if ins.Imm.Kind != MIRImmStringList {
			v.diags.Error(ins.Span, "interpolate requires a literal-parts operand", "add the literal segments")
		} else if len(ins.Imm.Strs) != len(ins.Args)+1 {
			v.diags.Error(ins.Span, "interpolate literal count must be one more than the value count", "provide one literal per gap")
		}
		for _, a := range ins.Args {
			if vt := v.valueTypes[a]; vt != v.prog.Types.String() && vt != v.prog.Types.Unknown() {
				v.diags.Error(ins.Span, "interpolate value type "+v.typeName(vt)+" is not String", "interpolate a String value")
			}
		}
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
	if !v.isNumeric(lt) || (ins.Op == MIRMod && v.prog.Types.Lookup(lt).Kind == TypeKindFloat) {
		v.diags.Error(ins.Span, ins.Op.String()+" is not defined for "+v.typeName(lt), "use an allowed numeric operator/type pair")
	}
	if ins.Imm.Kind != MIRImmNone {
		v.diags.Error(ins.Span, ins.Op.String()+" takes no immediate operand", "remove the immediate")
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
	kind := v.prog.Types.Lookup(lt).Kind
	ordering := ins.Op == MIRCmpLt || ins.Op == MIRCmpGt || ins.Op == MIRCmpLe || ins.Op == MIRCmpGe
	if ordering && kind != TypeKindInt && kind != TypeKindFloat && kind != TypeKindString && kind != TypeKindPointer {
		v.diags.Error(ins.Span, "ordering comparison is not defined for "+v.typeName(lt), "use a numeric type or ==/!=")
	}
	if !ordering && !v.isEqualityComparable(kind) {
		v.diags.Error(ins.Span, "equality comparison is not defined for "+v.typeName(lt), "use a comparable type")
	}
	if ins.Imm.Kind != MIRImmNone {
		v.diags.Error(ins.Span, ins.Op.String()+" takes no immediate operand", "remove the immediate")
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
	if ins.Imm.Kind != MIRImmNone {
		v.diags.Error(ins.Span, ins.Op.String()+" takes no immediate operand", "remove the immediate")
	}
}

func (v *MIRVerifier) checkConst(ins *MIRInstr) {
	t := v.prog.Types.Lookup(ins.Type)
	want := MIRImmNone
	switch t.Kind {
	case TypeKindBool:
		want = MIRImmBool
	case TypeKindString:
		want = MIRImmString
	case TypeKindInt, TypeKindEnum, TypeKindError, TypeKindPointer:
		// Pointers are addresses stored as machine words; the null pointer is
		// the zero constant.
		want = MIRImmInt
	case TypeKindFloat:
		want = MIRImmFloat
	default:
		v.diags.Error(ins.Span, "const cannot construct "+v.typeName(ins.Type), "use zero or an aggregate initializer")
		return
	}
	if ins.Imm.Kind != want {
		v.diags.Error(ins.Span, "const immediate kind does not match "+v.typeName(ins.Type), "use the immediate representation for the result type")
		return
	}
	if want == MIRImmFloat && (math.IsInf(ins.Imm.Float, 0) || math.IsNaN(ins.Imm.Float)) {
		v.diags.Error(ins.Span, "floating-point const is not finite", "use a finite representable value")
	}
	if want == MIRImmInt {
		raw := strings.ReplaceAll(ins.Imm.Str, "_", "")
		if raw == "" {
			raw = strconv.FormatInt(ins.Imm.Int, 10)
		}
		value, ok := new(big.Int).SetString(raw, 10)
		if !ok {
			v.diags.Error(ins.Span, "integer const has an invalid exact representation", "store a base-10 integer spelling")
			return
		}
		rangeType := Type(t.Name)
		if t.Kind == TypeKindEnum {
			rangeType = Type(v.prog.Types.Lookup(t.Underlying).Name)
		}
		if t.Kind == TypeKindError {
			if value.Sign() < 0 || value.BitLen() > 16 {
				v.diags.Error(ins.Span, "error const is outside its 16-bit ordinal representation", "use a declared error member")
			}
		} else if t.Kind == TypeKindPointer {
			if value.Sign() != 0 {
				v.diags.Error(ins.Span, "pointer const must be null", "store the null pointer as zero")
			}
		} else if !fitsIntegerType(rangeType, value) {
			v.diags.Error(ins.Span, "integer const does not fit "+string(rangeType), "use a value representable by its MIR type")
		}
	}
}

func (v *MIRVerifier) isNumeric(id TypeID) bool {
	k := v.prog.Types.Lookup(id).Kind
	return k == TypeKindInt || k == TypeKindFloat
}

func (v *MIRVerifier) isEqualityComparable(kind TypeKind) bool {
	switch kind {
	case TypeKindBool, TypeKindString, TypeKindInt, TypeKindFloat, TypeKindStruct, TypeKindTuple, TypeKindError, TypeKindArray, TypeKindEnum, TypeKindPointer:
		return true
	}
	return false
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
			lid := callee.Params[i]
			if int(lid) >= len(callee.LocalTypes) {
				v.diags.Error(ins.Span, "callee parameter has no type metadata", "fix the callee's local table")
				continue
			}
			pt := callee.LocalTypes[lid]
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
	if ins.Imm.Kind != MIRImmSymbol {
		return
	}
	if want == v.prog.Types.Void() && ins.Result != NoValue {
		v.diags.Error(ins.Span, "void call must not define a value", "use NoValue for the call result")
	}
	if want != v.prog.Types.Void() && ins.Result == NoValue {
		v.diags.Error(ins.Span, "non-void call must define a value", "assign a fresh ValueID")
	}
}

func (v *MIRVerifier) checkStructInit(ins *MIRInstr) {
	st := v.prog.Types.Lookup(ins.Type)
	if !isRecordKind(st.Kind) {
		v.diags.Error(ins.Span, "struct.init result type "+v.typeName(ins.Type)+" is not a record", "use a struct or tuple type")
		return
	}
	if ins.Imm.Kind != MIRImmNone {
		v.diags.Error(ins.Span, "struct.init takes no immediate operand", "remove the immediate")
	}
	if len(ins.Args) != len(st.Fields) {
		v.diags.Error(ins.Span, "struct.init for "+st.Name+" expects "+strconv.Itoa(len(st.Fields))+" field values, got "+strconv.Itoa(len(ins.Args)), "provide every field value")
		return
	}
	for i, a := range ins.Args {
		ft := st.Fields[i].Type
		if vt := v.valueTypes[a]; vt != ft && vt != v.prog.Types.Unknown() && !v.pointerAssignCompatible(vt, ft) {
			v.diags.Error(ins.Span, "struct.init field "+st.Fields[i].Name+" type "+v.typeName(vt)+" does not match field type "+v.typeName(ft), "use the field's type")
		}
	}
}

func (v *MIRVerifier) verifyTerminator(fn *MIRFunction, b *MIRBlock) {
	t := b.Term
	useIndex := len(b.Instrs)
	switch t.Kind {
	case MIRNoTerm:
		return
	case MIRJump:
		if !v.validBlock(fn, t.Target) {
			v.diags.Error(t.Span, "jump to undefined block bb"+strconv.Itoa(int(t.Target)), "jump to a declared block")
		}
	case MIRBranch:
		if !v.validBlock(fn, t.Then) || !v.validBlock(fn, t.Else) {
			v.diags.Error(t.Span, "branch to undefined block", "branch to declared blocks")
		}
		v.checkUse(t.Cond, b.ID, useIndex, t.Span, "branch condition")
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
				// A bare return is valid when the result type is Void.
				if v.prog.Types.Lookup(fn.Results[0]).Kind != TypeKindVoid {
					v.diags.Error(t.Span, "return without a value in a procedure returning "+v.typeName(fn.Results[0]), "return a value")
				}
			} else {
				v.checkUse(t.Value, b.ID, useIndex, t.Span, "return")
				if vt := v.valueTypes[t.Value]; vt != fn.Results[0] && vt != v.prog.Types.Unknown() && !v.pointerAssignCompatible(vt, fn.Results[0]) {
					v.diags.Error(t.Span, "return value type "+v.typeName(vt)+" does not match result type "+v.typeName(fn.Results[0]), "return a value of the result type")
				}
			}
		}
	case MIRTermExit:
		if t.Status == NoValue {
			v.diags.Error(t.Span, "exit terminator requires a status value", "provide an integer, enum, or error status")
		} else {
			v.checkUse(t.Status, b.ID, useIndex, t.Span, "exit status")
			kind := v.prog.Types.Lookup(v.valueTypes[t.Status]).Kind
			if kind != TypeKindInt && kind != TypeKindEnum && kind != TypeKindError && kind != TypeKindUnknown {
				v.diags.Error(t.Span, "exit status must be an integer, enum, or error value", "use an integer-compatible status")
			}
		}
		if t.Message != NoValue {
			v.checkUse(t.Message, b.ID, useIndex, t.Span, "exit message")
			if mt := v.valueTypes[t.Message]; mt != v.prog.Types.String() && mt != v.prog.Types.Unknown() {
				v.diags.Error(t.Span, "exit message must be String", "use a string message")
			}
		}
	case MIRUnreachable:
		// No operands to check.
	default:
		v.diags.Error(t.Span, "unknown MIR terminator "+strconv.Itoa(int(t.Kind)), "report this compiler bug")
	}
}

func (v *MIRVerifier) validBlock(fn *MIRFunction, id BlockID) bool {
	_, ok := v.blocks[id]
	return ok
}

func (v *MIRVerifier) typeName(id TypeID) string {
	t := v.prog.Types.Lookup(id)
	if t.Kind == TypeKindUnknown {
		return "<unknown>"
	}
	return t.Name
}

func (v *MIRVerifier) requireType(id TypeID, span Span, owner string) bool {
	if int(id) >= v.prog.Types.Len() || v.prog.Types.Lookup(id).Kind == TypeKindUnknown {
		v.diags.Error(span, owner+" references an invalid or unresolved type ID", "resolve every executable MIR type")
		return false
	}
	return true
}

func (v *MIRVerifier) checkUse(value ValueID, block BlockID, index int, span Span, owner string) {
	if value == NoValue {
		v.diags.Error(span, owner+" uses NoValue as an operand", "provide a defined value")
		return
	}
	def, ok := v.defSites[value]
	if !ok {
		v.diags.Error(span, owner+" uses undefined value v"+strconv.Itoa(int(value)), "define the value before using it")
		return
	}
	if def.block == block {
		if def.index >= index {
			v.diags.Error(span, owner+" uses v"+strconv.Itoa(int(value))+" before it is defined", "move the definition before the use")
		}
		return
	}
	if !v.dominators[block][def.block] {
		v.diags.Error(span, "definition of v"+strconv.Itoa(int(value))+" does not dominate its use", "merge control-flow values explicitly")
	}
}

func (v *MIRVerifier) computeDominators(fn *MIRFunction) {
	preds := make(map[BlockID][]BlockID, len(v.blocks))
	reachable := make(map[BlockID]bool, len(v.blocks))
	queue := []BlockID{0}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if reachable[id] {
			continue
		}
		b, ok := v.blocks[id]
		if !ok {
			continue
		}
		reachable[id] = true
		for _, succ := range v.successors(b.Term) {
			if _, ok := v.blocks[succ]; ok {
				preds[succ] = append(preds[succ], id)
				queue = append(queue, succ)
			}
		}
	}
	for id, block := range v.blocks {
		if !reachable[id] {
			v.diags.Error(block.Span, "basic block bb"+strconv.Itoa(int(id))+" in "+fn.Name+" is unreachable from bb0", "remove the disconnected block or add a valid control-flow edge")
		}
	}
	v.dominators = make(map[BlockID]map[BlockID]bool, len(v.blocks))
	for id := range v.blocks {
		set := make(map[BlockID]bool)
		if id == 0 {
			set[0] = true
		} else if reachable[id] {
			for candidate := range reachable {
				set[candidate] = true
			}
		} else {
			set[id] = true
		}
		v.dominators[id] = set
	}
	changed := true
	for changed {
		changed = false
		for id := range v.blocks {
			if id == 0 || !reachable[id] {
				continue
			}
			newSet := make(map[BlockID]bool)
			first := true
			for _, pred := range preds[id] {
				if first {
					for d := range v.dominators[pred] {
						newSet[d] = true
					}
					first = false
					continue
				}
				for d := range newSet {
					if !v.dominators[pred][d] {
						delete(newSet, d)
					}
				}
			}
			newSet[id] = true
			if !sameBlockSet(newSet, v.dominators[id]) {
				v.dominators[id] = newSet
				changed = true
			}
		}
	}
}

func (v *MIRVerifier) successors(term MIRTerminator) []BlockID {
	switch term.Kind {
	case MIRJump:
		return []BlockID{term.Target}
	case MIRBranch:
		if term.Then == term.Else {
			return []BlockID{term.Then}
		}
		return []BlockID{term.Then, term.Else}
	}
	return nil
}

func sameBlockSet(a, b map[BlockID]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}
