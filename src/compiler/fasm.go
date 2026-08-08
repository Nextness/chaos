// FASM backend for the Chaos compiler.
//
// FasmBackend renders a verified MIR program as a flat ELF64 executable for
// the fasm assembler. It uses the System V AMD64 calling convention: integer
// arguments in RDI/RSI/RDX/RCX/R8/R9, float arguments in XMM0-XMM7, integer
// results in RAX, float results in XMM0, and a 16-byte aligned stack at
// calls. Locals and MIR values live in stack slots addressed from RBP.
//
// Supported types: S8-S64, U8-U64, Size, Byte, Bool, and F64. F32, String,
// structs, and 128-bit integers are reported as unsupported. Global
// initializers are not yet emitted; globals are zero-initialized.
package compiler

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FasmBackend emits flat ELF64 executables for fasm.
type FasmBackend struct{}

// Name returns the backend's target name.
func (b *FasmBackend) Name() string { return "fasm" }

// Emit renders a MIR program as fasm assembly text.
func (b *FasmBackend) Emit(prog *MIRProgram) (string, DiagnosticList) {
	fb := &fasmEmitter{prog: prog}
	fb.emit()
	return fb.out.String(), fb.diags
}

// fasmEmitter holds the per-program and per-function state of the emitter.
type fasmEmitter struct {
	prog  *MIRProgram
	diags DiagnosticList
	out   strings.Builder

	// Per-function state.
	fn          *MIRFunction
	valueSlots  map[ValueID]int
	valueTypes  map[ValueID]TypeID
	localSlots  map[LocalID]int
	blockLabels map[BlockID]string
	nextOffset  int
	frameSize   int
	labelCount  int

	// Float constants emitted in the data section.
	floatConsts  []float64
	floatIndexes map[uint64]int
}

func (fb *fasmEmitter) emit() {
	fb.out.WriteString("format ELF64 executable 3\n\n")
	fb.collectFloatConsts()
	fb.emitData()
	fb.out.WriteString("segment readable executable\n\n")
	fb.emitEntry()
	for _, fn := range fb.prog.Functions {
		fb.emitFunction(fn)
	}
}

// collectFloatConsts gathers every float constant so the data section can be
// emitted before the code that references it.
func (fb *fasmEmitter) collectFloatConsts() {
	fb.floatIndexes = make(map[uint64]int)
	for _, fn := range fb.prog.Functions {
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				if ins.Op == MIRConst && ins.Imm.Kind == MIRImmFloat {
					if _, ok := fb.floatIndexes[math.Float64bits(ins.Imm.Float)]; !ok {
						fb.floatIndexes[math.Float64bits(ins.Imm.Float)] = len(fb.floatConsts)
						fb.floatConsts = append(fb.floatConsts, ins.Imm.Float)
					}
				}
			}
		}
	}
}

func (fb *fasmEmitter) emitData() {
	if len(fb.prog.Globals) == 0 && len(fb.floatConsts) == 0 {
		return
	}
	fb.out.WriteString("segment readable writable\n")
	for _, g := range fb.prog.Globals {
		t := fb.prog.Types.Lookup(g.Type)
		fb.checkType(t, g.Span)
		if g.HasInit {
			fb.diags.Error(g.Span, "global initializers are not yet supported by the fasm backend", "initialize the global at runtime or remove the initializer")
		}
		fmt.Fprintf(&fb.out, "%s:\n    dq 0\n", fb.globalLabel(g.Symbol))
	}
	for i, v := range fb.floatConsts {
		fmt.Fprintf(&fb.out, "fc%d:\n    dq %s\n", i, fasmFloat(v))
	}
	fb.out.WriteString("\n")
}

// fasmFloat formats a float constant so fasm treats it as a floating-point
// literal. Integer-looking values (for example 3.0) must carry a decimal
// point; otherwise fasm stores the integer bit pattern instead of the double.
func fasmFloat(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// emitEntry emits the process entry wrapper: it calls the '#entry' procedure
// and exits with its result.
func (fb *fasmEmitter) emitEntry() {
	fb.out.WriteString("entry _start\n\n")
	fb.out.WriteString("_start:\n")
	entryFn := fb.findFunction(fb.prog.Entry)
	if entryFn == nil {
		fb.out.WriteString("    xor edi, edi\n")
		fb.out.WriteString("    mov rax, 60\n")
		fb.out.WriteString("    syscall\n\n")
		return
	}
	// Zero the integer argument registers the entry procedure uses.
	for i := 0; i < len(entryFn.Params) && i < 6; i++ {
		fmt.Fprintf(&fb.out, "    xor %s, %s\n", intArgReg(i), intArgReg(i))
	}
	fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(entryFn.Name))
	if len(entryFn.Results) > 0 {
		rt := fb.prog.Types.Lookup(entryFn.Results[0])
		if rt.Kind == TypeKindFloat {
			fb.out.WriteString("    cvttsd2si rdi, xmm0\n")
		} else {
			fb.out.WriteString("    mov rdi, rax\n")
		}
	} else {
		fb.out.WriteString("    xor edi, edi\n")
	}
	fb.out.WriteString("    mov rax, 60\n")
	fb.out.WriteString("    syscall\n\n")
}

func (fb *fasmEmitter) emitFunction(fn *MIRFunction) {
	fb.fn = fn
	fb.assignSlots(fn)
	fb.blockLabels = make(map[BlockID]string)
	for _, b := range fn.Blocks {
		fb.blockLabels[b.ID] = fmt.Sprintf("L%d", fb.labelCount)
		fb.labelCount++
	}
	fmt.Fprintf(&fb.out, "%s:\n", fb.funcLabel(fn.Name))
	fb.out.WriteString("    push rbp\n")
	fb.out.WriteString("    mov rbp, rsp\n")
	if fb.frameSize > 0 {
		fmt.Fprintf(&fb.out, "    sub rsp, %d\n", fb.frameSize)
	}
	fb.emitParamMoves(fn)
	for _, b := range fn.Blocks {
		fb.emitBlock(b)
	}
	fb.out.WriteString("\n")
}

// assignSlots allocates a stack slot for every local and MIR value in the
// function and computes the frame size.
func (fb *fasmEmitter) assignSlots(fn *MIRFunction) {
	fb.valueSlots = make(map[ValueID]int)
	fb.valueTypes = make(map[ValueID]TypeID)
	fb.localSlots = make(map[LocalID]int)
	fb.nextOffset = 0
	for _, lid := range fn.Locals {
		t := fb.prog.Types.Lookup(fn.LocalTypes[lid])
		fb.checkType(t, fn.Span)
		fb.localSlots[lid] = fb.allocSlot(fb.sizeOf(t))
	}
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			if ins.Result != NoValue {
				t := fb.prog.Types.Lookup(ins.Type)
				fb.checkType(t, ins.Span)
				fb.valueTypes[ins.Result] = ins.Type
				fb.valueSlots[ins.Result] = fb.allocSlot(fb.sizeOf(t))
			}
		}
	}
	fb.frameSize = align16(fb.nextOffset)
}

func (fb *fasmEmitter) allocSlot(size int) int {
	fb.nextOffset = align(fb.nextOffset, size)
	fb.nextOffset += size
	return fb.nextOffset
}

func (fb *fasmEmitter) emitParamMoves(fn *MIRFunction) {
	for i, lid := range fn.Params {
		slot := fb.localSlots[lid]
		t := fb.prog.Types.Lookup(fn.LocalTypes[lid])
		if t.Kind == TypeKindFloat {
			fmt.Fprintf(&fb.out, "    movsd qword [rbp-%d], xmm%d\n", slot, i)
			continue
		}
		fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(i))
		fb.emitStore(t, slot)
	}
}

func (fb *fasmEmitter) emitBlock(b *MIRBlock) {
	fmt.Fprintf(&fb.out, "%s:\n", fb.blockLabels[b.ID])
	for _, ins := range b.Instrs {
		fb.emitInstr(ins)
	}
	fb.emitTerminator(b)
}

func (fb *fasmEmitter) emitInstr(ins *MIRInstr) {
	switch ins.Op {
	case MIRConst:
		fb.emitConst(ins)
	case MIRLoadLocal:
		fb.emitLoadLocal(ins)
	case MIRStoreLocal:
		fb.emitStoreLocal(ins)
	case MIRLoadGlobal:
		fb.emitLoadGlobal(ins)
	case MIRStoreGlobal:
		fb.emitStoreGlobal(ins)
	case MIRAdd, MIRSub, MIRMul, MIRDiv, MIRMod:
		fb.emitArith(ins)
	case MIRCmpLt, MIRCmpGt, MIRCmpLe, MIRCmpGe, MIRCmpEq, MIRCmpNeq:
		fb.emitCmp(ins)
	case MIRAnd, MIROr:
		fb.emitLogical(ins)
	case MIRNot:
		fb.emitNot(ins)
	case MIRNeg:
		fb.emitNeg(ins)
	case MIRCall:
		fb.emitCall(ins)
	case MIRStructInit:
		fb.diags.Error(ins.Span, "struct codegen is not yet supported by the fasm backend", "defer struct literals until the backend supports them")
	case MIRExit:
		// exit is a terminator, not an instruction.
	}
}

func (fb *fasmEmitter) emitConst(ins *MIRInstr) {
	slot := fb.valueSlots[ins.Result]
	t := fb.prog.Types.Lookup(ins.Type)
	switch ins.Imm.Kind {
	case MIRImmInt:
		fmt.Fprintf(&fb.out, "    mov rax, %d\n", ins.Imm.Int)
		fb.emitStore(t, slot)
	case MIRImmBool:
		v := 0
		if ins.Imm.Bool {
			v = 1
		}
		fmt.Fprintf(&fb.out, "    mov rax, %d\n", v)
		fb.emitStore(t, slot)
	case MIRImmFloat:
		idx := fb.floatIndexes[math.Float64bits(ins.Imm.Float)]
		fmt.Fprintf(&fb.out, "    movsd xmm0, qword [fc%d]\n", idx)
		fb.emitStore(t, slot)
	case MIRImmString:
		// String codegen is reported as unsupported by checkType.
	}
}

func (fb *fasmEmitter) emitLoadLocal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	fb.emitLoad(t, fb.localSlots[ins.Imm.Local])
	fb.emitStore(t, fb.valueSlots[ins.Result])
}

func (fb *fasmEmitter) emitStoreLocal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	fb.emitLoad(t, fb.valueSlots[ins.Args[0]])
	fb.emitStore(t, fb.localSlots[ins.Imm.Local])
}

func (fb *fasmEmitter) emitLoadGlobal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	name := fb.globalLabel(ins.Imm.Symbol)
	if t.Kind == TypeKindFloat {
		fmt.Fprintf(&fb.out, "    movsd xmm0, qword [%s]\n", name)
	} else {
		fmt.Fprintf(&fb.out, "    mov rax, qword [%s]\n", name)
	}
	fb.emitStore(t, fb.valueSlots[ins.Result])
}

func (fb *fasmEmitter) emitStoreGlobal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	fb.emitLoad(t, fb.valueSlots[ins.Args[0]])
	name := fb.globalLabel(ins.Imm.Symbol)
	if t.Kind == TypeKindFloat {
		fmt.Fprintf(&fb.out, "    movsd qword [%s], xmm0\n", name)
	} else {
		fmt.Fprintf(&fb.out, "    mov qword [%s], rax\n", name)
	}
}

func (fb *fasmEmitter) emitArith(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	lslot := fb.valueSlots[ins.Args[0]]
	rslot := fb.valueSlots[ins.Args[1]]
	resSlot := fb.valueSlots[ins.Result]
	if t.Kind == TypeKindFloat {
		fb.emitLoad(t, lslot)
		switch ins.Op {
		case MIRAdd:
			fmt.Fprintf(&fb.out, "    addsd xmm0, qword [rbp-%d]\n", rslot)
		case MIRSub:
			fmt.Fprintf(&fb.out, "    subsd xmm0, qword [rbp-%d]\n", rslot)
		case MIRMul:
			fmt.Fprintf(&fb.out, "    mulsd xmm0, qword [rbp-%d]\n", rslot)
		case MIRDiv:
			fmt.Fprintf(&fb.out, "    divsd xmm0, qword [rbp-%d]\n", rslot)
		}
		fb.emitStore(t, resSlot)
		return
	}
	size := fb.sizeOf(t)
	fb.emitLoad(t, lslot)
	switch ins.Op {
	case MIRAdd:
		fmt.Fprintf(&fb.out, "    add %s, %s [rbp-%d]\n", widthReg(size), memSize(size), rslot)
	case MIRSub:
		fmt.Fprintf(&fb.out, "    sub %s, %s [rbp-%d]\n", widthReg(size), memSize(size), rslot)
	case MIRMul:
		fb.emitLoadRight(t, rslot)
		fb.out.WriteString("    imul rax, rcx\n")
	case MIRDiv, MIRMod:
		fb.emitDivMod(ins, t, size, rslot, resSlot)
		return
	}
	fb.emitStore(t, resSlot)
}

// emitDivMod emits signed or unsigned division. The dividend is already in
// rax (sign or zero extended by emitLoad); the divisor is loaded into rcx
// with the same extension and a 64-bit divide is used, which is correct for
// every supported width.
func (fb *fasmEmitter) emitDivMod(ins *MIRInstr, t IRType, size int, rslot, resSlot int) {
	fb.emitLoadRight(t, rslot)
	if isSignedInt(t.Name) {
		fb.out.WriteString("    cqo\n")
		fb.out.WriteString("    idiv rcx\n")
	} else {
		fb.out.WriteString("    xor edx, edx\n")
		fb.out.WriteString("    div rcx\n")
	}
	if ins.Op == MIRMod {
		fb.out.WriteString("    mov rax, rdx\n")
	}
	fb.emitStore(t, resSlot)
}

func (fb *fasmEmitter) emitCmp(ins *MIRInstr) {
	lt := fb.prog.Types.Lookup(fb.valueTypes[ins.Args[0]])
	lslot := fb.valueSlots[ins.Args[0]]
	rslot := fb.valueSlots[ins.Args[1]]
	resSlot := fb.valueSlots[ins.Result]
	if lt.Kind == TypeKindFloat {
		fb.emitLoad(lt, lslot)
		fmt.Fprintf(&fb.out, "    comisd xmm0, qword [rbp-%d]\n", rslot)
		fb.emitSetcc(ins.Op, false, resSlot)
		return
	}
	size := fb.sizeOf(lt)
	fb.emitLoad(lt, lslot)
	fmt.Fprintf(&fb.out, "    cmp %s, %s [rbp-%d]\n", widthReg(size), memSize(size), rslot)
	fb.emitSetcc(ins.Op, isSignedInt(lt.Name), resSlot)
}

func (fb *fasmEmitter) emitSetcc(op MIROpcode, signed bool, resSlot int) {
	var cc string
	switch op {
	case MIRCmpLt:
		if signed {
			cc = "setl"
		} else {
			cc = "setb"
		}
	case MIRCmpGt:
		if signed {
			cc = "setg"
		} else {
			cc = "seta"
		}
	case MIRCmpLe:
		if signed {
			cc = "setle"
		} else {
			cc = "setbe"
		}
	case MIRCmpGe:
		if signed {
			cc = "setge"
		} else {
			cc = "setae"
		}
	case MIRCmpEq:
		cc = "sete"
	case MIRCmpNeq:
		cc = "setne"
	}
	fmt.Fprintf(&fb.out, "    %s al\n", cc)
	fb.emitStore(fb.prog.Types.Lookup(fb.prog.Types.Bool()), resSlot)
}

func (fb *fasmEmitter) emitLogical(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	lslot := fb.valueSlots[ins.Args[0]]
	rslot := fb.valueSlots[ins.Args[1]]
	resSlot := fb.valueSlots[ins.Result]
	size := fb.sizeOf(t)
	fb.emitLoad(t, lslot)
	if ins.Op == MIRAnd {
		fmt.Fprintf(&fb.out, "    and %s, %s [rbp-%d]\n", widthReg(size), memSize(size), rslot)
	} else {
		fmt.Fprintf(&fb.out, "    or %s, %s [rbp-%d]\n", widthReg(size), memSize(size), rslot)
	}
	fb.emitStore(t, resSlot)
}

func (fb *fasmEmitter) emitNot(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	slot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	fb.emitLoad(t, slot)
	fb.out.WriteString("    xor al, 1\n")
	fb.emitStore(t, resSlot)
}

func (fb *fasmEmitter) emitNeg(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	slot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	if t.Kind == TypeKindFloat {
		fb.emitLoad(t, slot)
		fb.out.WriteString("    xorpd xmm1, xmm1\n")
		fb.out.WriteString("    subsd xmm1, xmm0\n")
		fb.out.WriteString("    movsd xmm0, xmm1\n")
		fb.emitStore(t, resSlot)
		return
	}
	size := fb.sizeOf(t)
	fb.emitLoad(t, slot)
	fmt.Fprintf(&fb.out, "    neg %s\n", widthReg(size))
	fb.emitStore(t, resSlot)
}

func (fb *fasmEmitter) emitCall(ins *MIRInstr) {
	callee := fb.findFunction(ins.Imm.Symbol)
	if callee == nil {
		fb.diags.Error(ins.Span, "call to unknown procedure", "declare the procedure before calling it")
		return
	}
	// Assign each argument to its System V register. Arguments are then
	// loaded in reverse order so that loading a later argument cannot
	// clobber an earlier one (the load scratch registers rax and xmm0 are
	// also the first argument registers).
	type argReg struct {
		arg     ValueID
		isFloat bool
		idx     int
	}
	regs := make([]argReg, 0, len(ins.Args))
	intIdx, floatIdx := 0, 0
	for _, a := range ins.Args {
		t := fb.prog.Types.Lookup(fb.valueTypes[a])
		if t.Kind == TypeKindFloat {
			regs = append(regs, argReg{arg: a, isFloat: true, idx: floatIdx})
			floatIdx++
		} else {
			regs = append(regs, argReg{arg: a, isFloat: false, idx: intIdx})
			intIdx++
		}
	}
	for i := len(regs) - 1; i >= 0; i-- {
		r := regs[i]
		t := fb.prog.Types.Lookup(fb.valueTypes[r.arg])
		slot := fb.valueSlots[r.arg]
		if r.isFloat {
			if r.idx >= 8 {
				fb.diags.Error(ins.Span, "calls with more than 8 float arguments are not yet supported", "reduce the number of float arguments")
				continue
			}
			fb.emitLoad(t, slot)
			fmt.Fprintf(&fb.out, "    movsd xmm%d, xmm0\n", r.idx)
		} else {
			if r.idx >= 6 {
				fb.diags.Error(ins.Span, "calls with more than 6 integer arguments are not yet supported", "reduce the number of integer arguments")
				continue
			}
			fb.emitLoad(t, slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(r.idx))
		}
	}
	fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(callee.Name))
	if ins.Type != fb.prog.Types.Void() {
		t := fb.prog.Types.Lookup(ins.Type)
		fb.emitStore(t, fb.valueSlots[ins.Result])
	}
}

func (fb *fasmEmitter) emitTerminator(b *MIRBlock) {
	t := b.Term
	switch t.Kind {
	case MIRJump:
		fmt.Fprintf(&fb.out, "    jmp %s\n", fb.blockLabels[t.Target])
	case MIRBranch:
		condSlot := fb.valueSlots[t.Cond]
		fb.emitLoad(fb.prog.Types.Lookup(fb.prog.Types.Bool()), condSlot)
		fb.out.WriteString("    test al, al\n")
		fmt.Fprintf(&fb.out, "    jnz %s\n", fb.blockLabels[t.Then])
		fmt.Fprintf(&fb.out, "    jmp %s\n", fb.blockLabels[t.Else])
	case MIRReturn:
		if t.Value != NoValue {
			vt := fb.prog.Types.Lookup(fb.valueTypes[t.Value])
			fb.emitLoad(vt, fb.valueSlots[t.Value])
		}
		fb.out.WriteString("    leave\n")
		fb.out.WriteString("    ret\n")
	case MIRTermExit:
		if t.Status != NoValue {
			st := fb.prog.Types.Lookup(fb.valueTypes[t.Status])
			fb.emitLoad(st, fb.valueSlots[t.Status])
			fb.out.WriteString("    mov rdi, rax\n")
		} else {
			fb.out.WriteString("    xor edi, edi\n")
		}
		if t.Message != NoValue {
			fb.diags.Warn(t.Span, "exit messages are not yet printed by the fasm backend", "the exit status is used; the message is ignored")
		}
		fb.out.WriteString("    mov rax, 60\n")
		fb.out.WriteString("    syscall\n")
	case MIRUnreachable:
		fb.out.WriteString("    ud2\n")
	}
}

// checkType reports whether a type is supported by the backend, emitting a
// diagnostic for unsupported types.
func (fb *fasmEmitter) checkType(t IRType, span Span) bool {
	switch t.Kind {
	case TypeKindVoid, TypeKindBool:
		return true
	case TypeKindInt:
		switch t.Name {
		case "S8", "S16", "S32", "S64", "U8", "U16", "U32", "U64", "Size", "Byte":
			return true
		}
		fb.diags.Error(span, "integer type "+t.Name+" is not yet supported by the fasm backend", "use a supported integer type")
		return false
	case TypeKindFloat:
		if t.Name == "F64" {
			return true
		}
		fb.diags.Error(span, "float type "+t.Name+" is not yet supported by the fasm backend", "use F64 or an integer type")
		return false
	case TypeKindString:
		fb.diags.Error(span, "String codegen is not yet supported by the fasm backend", "defer string values until the backend supports them")
		return false
	case TypeKindStruct:
		fb.diags.Error(span, "struct codegen is not yet supported by the fasm backend", "defer struct values until the backend supports them")
		return false
	}
	return true
}

// sizeOf returns the storage size in bytes for a type.
func (fb *fasmEmitter) sizeOf(t IRType) int {
	switch t.Kind {
	case TypeKindBool:
		return 1
	case TypeKindInt:
		switch t.Name {
		case "S8", "U8", "Byte":
			return 1
		case "S16", "U16":
			return 2
		case "S32", "U32":
			return 4
		case "S64", "U64", "Size":
			return 8
		}
	case TypeKindFloat:
		switch t.Name {
		case "F32":
			return 4
		case "F64":
			return 8
		}
	}
	return 8
}

// emitLoad loads the value at [rbp-offset] into rax (integers, sign or zero
// extended to 64 bits) or xmm0 (floats).
func (fb *fasmEmitter) emitLoad(t IRType, offset int) {
	size := fb.sizeOf(t)
	if t.Kind == TypeKindFloat {
		fmt.Fprintf(&fb.out, "    movsd xmm0, qword [rbp-%d]\n", offset)
		return
	}
	switch size {
	case 1:
		if t.Kind == TypeKindInt && isSignedInt(t.Name) {
			fmt.Fprintf(&fb.out, "    movsx rax, byte [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rax, byte [rbp-%d]\n", offset)
		}
	case 2:
		if t.Kind == TypeKindInt && isSignedInt(t.Name) {
			fmt.Fprintf(&fb.out, "    movsx rax, word [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rax, word [rbp-%d]\n", offset)
		}
	case 4:
		if t.Kind == TypeKindInt && isSignedInt(t.Name) {
			fmt.Fprintf(&fb.out, "    movsxd rax, dword [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    mov eax, dword [rbp-%d]\n", offset)
		}
	default:
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", offset)
	}
}

// emitLoadRight loads the value at [rbp-offset] into rcx with the same sign
// or zero extension as emitLoad uses for rax.
func (fb *fasmEmitter) emitLoadRight(t IRType, offset int) {
	size := fb.sizeOf(t)
	switch size {
	case 1:
		if t.Kind == TypeKindInt && isSignedInt(t.Name) {
			fmt.Fprintf(&fb.out, "    movsx rcx, byte [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rcx, byte [rbp-%d]\n", offset)
		}
	case 2:
		if t.Kind == TypeKindInt && isSignedInt(t.Name) {
			fmt.Fprintf(&fb.out, "    movsx rcx, word [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rcx, word [rbp-%d]\n", offset)
		}
	case 4:
		if t.Kind == TypeKindInt && isSignedInt(t.Name) {
			fmt.Fprintf(&fb.out, "    movsxd rcx, dword [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    mov ecx, dword [rbp-%d]\n", offset)
		}
	default:
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d]\n", offset)
	}
}

// emitStore stores rax (integers) or xmm0 (floats) into the slot at
// [rbp-offset], using the type's width.
func (fb *fasmEmitter) emitStore(t IRType, offset int) {
	size := fb.sizeOf(t)
	if t.Kind == TypeKindFloat {
		fmt.Fprintf(&fb.out, "    movsd qword [rbp-%d], xmm0\n", offset)
		return
	}
	fmt.Fprintf(&fb.out, "    mov %s [rbp-%d], %s\n", memSize(size), offset, widthReg(size))
}

func (fb *fasmEmitter) funcLabel(name string) string {
	return "f_" + name
}

func (fb *fasmEmitter) globalLabel(sym SymbolID) string {
	return "g_" + fb.prog.Symbols.Lookup(sym)
}

func (fb *fasmEmitter) findFunction(sym SymbolID) *MIRFunction {
	for _, fn := range fb.prog.Functions {
		if fn.Symbol == sym {
			return fn
		}
	}
	return nil
}

func isSignedInt(name string) bool {
	switch name {
	case "S8", "S16", "S32", "S64":
		return true
	}
	return false
}

func memSize(size int) string {
	switch size {
	case 1:
		return "byte"
	case 2:
		return "word"
	case 4:
		return "dword"
	default:
		return "qword"
	}
}

func widthReg(size int) string {
	switch size {
	case 1:
		return "al"
	case 2:
		return "ax"
	case 4:
		return "eax"
	default:
		return "rax"
	}
}

func intArgReg(i int) string {
	switch i {
	case 0:
		return "rdi"
	case 1:
		return "rsi"
	case 2:
		return "rdx"
	case 3:
		return "rcx"
	case 4:
		return "r8"
	case 5:
		return "r9"
	}
	return "rdi"
}

func align(n, a int) int {
	return (n + a - 1) / a * a
}

func align16(n int) int {
	return align(n, 16)
}
