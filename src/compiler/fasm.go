// FASM backend for the Chaos compiler.
//
// FasmBackend renders a verified MIR program as a flat ELF64 executable for
// the fasm assembler. It uses the System V AMD64 calling convention: integer
// arguments in RDI/RSI/RDX/RCX/R8/R9, float arguments in XMM0-XMM7, integer
// results in RAX, float results in XMM0, and a 16-byte aligned stack at
// calls. Locals and MIR values live in stack slots addressed from RBP.
//
// Supported types: S8-S128, U8-U128, Size, Byte, Bool, F32, F64, String, and
// structs. String values are a pointer and length pair (16 bytes); 128-bit
// integers are a low and high half pair (16 bytes); structs are aggregates of
// their fields. Struct parameters are passed by address and struct results
// are returned through a hidden pointer in RDI (sret). Global initializers
// run in a synthetic function called before the entry procedure.
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
	sretSlot    int // slot holding the sret pointer, -1 when unused

	// Float and string constants emitted in the data section.
	floatConsts   []floatConst
	floatIndexes  map[floatConstKey]int
	strings       []string
	stringIndexes map[string]int
}

// floatConst is a float constant with its storage size (4 for F32, 8 for
// F64), so the data section can emit it with the right width.
type floatConst struct {
	value float64
	size  int
}

// floatConstKey identifies a float constant by bit pattern and size.
type floatConstKey struct {
	bits uint64
	size int
}

func (fb *fasmEmitter) emit() {
	fb.out.WriteString("format ELF64 executable 3\n\n")
	fb.collectFloatConsts()
	fb.collectStringConsts()
	fb.emitData()
	fb.out.WriteString("segment readable executable\n\n")
	fb.emitEntry()
	for _, fn := range fb.prog.Functions {
		fb.emitFunction(fn)
	}
}

// collectFloatConsts gathers every float constant so the data section can be
// emitted before the code that references it. The storage size (4 or 8) is
// taken from the constant's type.
func (fb *fasmEmitter) collectFloatConsts() {
	fb.floatIndexes = make(map[floatConstKey]int)
	for _, fn := range fb.prog.Functions {
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				if ins.Op == MIRConst && ins.Imm.Kind == MIRImmFloat {
					size := fb.sizeOf(fb.prog.Types.Lookup(ins.Type))
					key := floatConstKey{bits: math.Float64bits(ins.Imm.Float), size: size}
					if _, ok := fb.floatIndexes[key]; !ok {
						fb.floatIndexes[key] = len(fb.floatConsts)
						fb.floatConsts = append(fb.floatConsts, floatConst{value: ins.Imm.Float, size: size})
					}
				}
			}
		}
	}
}

// collectStringConsts gathers every string constant so the data section can
// be emitted before the code that references it.
func (fb *fasmEmitter) collectStringConsts() {
	fb.stringIndexes = make(map[string]int)
	for _, fn := range fb.prog.Functions {
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				if ins.Op == MIRConst && ins.Imm.Kind == MIRImmString {
					if _, ok := fb.stringIndexes[ins.Imm.Str]; !ok {
						fb.stringIndexes[ins.Imm.Str] = len(fb.strings)
						fb.strings = append(fb.strings, ins.Imm.Str)
					}
				}
			}
		}
	}
}

func (fb *fasmEmitter) emitData() {
	if len(fb.prog.Globals) == 0 && len(fb.floatConsts) == 0 && len(fb.strings) == 0 {
		return
	}
	fb.out.WriteString("segment readable writable\n")
	for _, g := range fb.prog.Globals {
		size := fb.sizeOf(fb.prog.Types.Lookup(g.Type))
		fmt.Fprintf(&fb.out, "%s:\n    rb %d\n", fb.globalLabel(g.Symbol), size)
	}
	for i, fc := range fb.floatConsts {
		if fc.size == 4 {
			fmt.Fprintf(&fb.out, "fc%d:\n    dd %s\n", i, fasmFloat(fc.value))
		} else {
			fmt.Fprintf(&fb.out, "fc%d:\n    dq %s\n", i, fasmFloat(fc.value))
		}
	}
	for i, s := range fb.strings {
		fmt.Fprintf(&fb.out, "str%d:\n    db ", i)
		for j, b := range []byte(s) {
			if j > 0 {
				fb.out.WriteString(",")
			}
			fmt.Fprintf(&fb.out, "0x%02x", b)
		}
		fb.out.WriteString("\n")
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

// emitEntry emits the process entry wrapper: it runs the global initializers,
// calls the '#entry' procedure, and exits with its result.
func (fb *fasmEmitter) emitEntry() {
	fb.out.WriteString("entry _start\n\n")
	fb.out.WriteString("_start:\n")
	if fb.prog.GlobalInit != nil {
		fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(fb.prog.GlobalInit.Name))
	}
	entryFn := fb.findFunction(fb.prog.Entry)
	if entryFn == nil {
		fb.out.WriteString("    xor edi, edi\n")
		fb.out.WriteString("    mov rax, 60\n")
		fb.out.WriteString("    syscall\n\n")
		return
	}
	// A struct-returning entry procedure receives the result slot address in
	// RDI (sret); reserve room on the stack for the result.
	if len(entryFn.Results) > 0 {
		rt := fb.prog.Types.Lookup(entryFn.Results[0])
		if rt.Kind == TypeKindStruct {
			fmt.Fprintf(&fb.out, "    sub rsp, %d\n", align16(fb.sizeOf(rt)))
			fb.out.WriteString("    mov rdi, rsp\n")
		}
	}
	// Zero the integer argument registers the entry procedure uses.
	intCount := 0
	for _, lid := range entryFn.Params {
		intCount += intRegCount(fb.prog.Types.Lookup(entryFn.LocalTypes[lid]))
	}
	for i := 0; i < intCount && i < 6; i++ {
		fmt.Fprintf(&fb.out, "    xor %s, %s\n", intArgReg(i), intArgReg(i))
	}
	fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(entryFn.Name))
	if len(entryFn.Results) > 0 {
		rt := fb.prog.Types.Lookup(entryFn.Results[0])
		switch {
		case rt.Kind == TypeKindStruct:
			fb.out.WriteString("    xor edi, edi\n")
		case rt.Kind == TypeKindFloat:
			if rt.Name == "F32" {
				fb.out.WriteString("    cvttss2si rdi, xmm0\n")
			} else {
				fb.out.WriteString("    cvttsd2si rdi, xmm0\n")
			}
		default:
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
		fb.blockLabels[b.ID] = fb.newLabel()
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
// function and computes the frame size. A function returning a struct also
// gets a slot for the sret pointer.
func (fb *fasmEmitter) assignSlots(fn *MIRFunction) {
	fb.valueSlots = make(map[ValueID]int)
	fb.valueTypes = make(map[ValueID]TypeID)
	fb.localSlots = make(map[LocalID]int)
	fb.nextOffset = 0
	fb.sretSlot = -1
	if len(fn.Results) > 0 && fb.prog.Types.Lookup(fn.Results[0]).Kind == TypeKindStruct {
		fb.sretSlot = fb.allocSlot(8)
	}
	for _, lid := range fn.Locals {
		t := fb.prog.Types.Lookup(fn.LocalTypes[lid])
		fb.localSlots[lid] = fb.allocSlot(fb.sizeOf(t))
	}
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			if ins.Result != NoValue {
				t := fb.prog.Types.Lookup(ins.Type)
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

// emitParamMoves stores incoming arguments into their local slots. String
// and 128-bit parameters arrive in two integer registers (low/high or
// pointer/length); struct parameters arrive as an address and are copied.
// A struct-returning function receives its sret pointer in RDI.
func (fb *fasmEmitter) emitParamMoves(fn *MIRFunction) {
	intIdx, floatIdx := 0, 0
	if fb.sretSlot >= 0 {
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rdi\n", fb.sretSlot)
		intIdx = 1
	}
	for _, lid := range fn.Params {
		slot := fb.localSlots[lid]
		t := fb.prog.Types.Lookup(fn.LocalTypes[lid])
		switch {
		case t.Kind == TypeKindFloat:
			if t.Name == "F32" {
				fmt.Fprintf(&fb.out, "    movss dword [rbp-%d], xmm%d\n", slot, floatIdx)
			} else {
				fmt.Fprintf(&fb.out, "    movsd qword [rbp-%d], xmm%d\n", slot, floatIdx)
			}
			floatIdx++
		case t.Kind == TypeKindString:
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(intIdx))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(intIdx+1))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
			intIdx += 2
		case t.Kind == TypeKindStruct:
			fmt.Fprintf(&fb.out, "    mov rsi, %s\n", intArgReg(intIdx))
			fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov rcx, %d\n", fb.sizeOf(t))
			fb.out.WriteString("    cld\n")
			fb.out.WriteString("    rep movsb\n")
			intIdx++
		case isInt128(t.Name):
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(intIdx))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(intIdx+1))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
			intIdx += 2
		default:
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(intIdx))
			fb.emitStore(t, slot)
			intIdx++
		}
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
		fb.emitStructInit(ins)
	case MIRExit:
		// exit is a terminator, not an instruction.
	}
}

func (fb *fasmEmitter) emitConst(ins *MIRInstr) {
	slot := fb.valueSlots[ins.Result]
	t := fb.prog.Types.Lookup(ins.Type)
	switch ins.Imm.Kind {
	case MIRImmInt:
		if isInt128(t.Name) {
			// Low half holds the value; the high half is sign or zero
			// extended.
			fmt.Fprintf(&fb.out, "    mov rax, %d\n", ins.Imm.Int)
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
			if isSignedInt(t.Name) {
				fb.out.WriteString("    sar rax, 63\n")
			} else {
				fb.out.WriteString("    xor eax, eax\n")
			}
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
			return
		}
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
		size := fb.sizeOf(t)
		idx := fb.floatIndexes[floatConstKey{bits: math.Float64bits(ins.Imm.Float), size: size}]
		if size == 4 {
			fmt.Fprintf(&fb.out, "    movss xmm0, dword [fc%d]\n", idx)
		} else {
			fmt.Fprintf(&fb.out, "    movsd xmm0, qword [fc%d]\n", idx)
		}
		fb.emitStore(t, slot)
	case MIRImmString:
		idx := fb.stringIndexes[ins.Imm.Str]
		fmt.Fprintf(&fb.out, "    mov rax, str%d\n", idx)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
		fmt.Fprintf(&fb.out, "    mov rax, %d\n", len(ins.Imm.Str))
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
	}
}

func (fb *fasmEmitter) emitLoadLocal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	if isAggregate(t) {
		fb.emitAggCopy(fb.localSlots[ins.Imm.Local], fb.valueSlots[ins.Result], fb.sizeOf(t))
		return
	}
	fb.emitLoad(t, fb.localSlots[ins.Imm.Local])
	fb.emitStore(t, fb.valueSlots[ins.Result])
}

func (fb *fasmEmitter) emitStoreLocal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	if isAggregate(t) {
		fb.emitAggCopy(fb.valueSlots[ins.Args[0]], fb.localSlots[ins.Imm.Local], fb.sizeOf(t))
		return
	}
	fb.emitLoad(t, fb.valueSlots[ins.Args[0]])
	fb.emitStore(t, fb.localSlots[ins.Imm.Local])
}

func (fb *fasmEmitter) emitLoadGlobal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	name := fb.globalLabel(ins.Imm.Symbol)
	if isAggregate(t) {
		fb.emitAggCopyFromAddr(name, fb.valueSlots[ins.Result], fb.sizeOf(t))
		return
	}
	if t.Kind == TypeKindFloat {
		if t.Name == "F32" {
			fmt.Fprintf(&fb.out, "    movss xmm0, dword [%s]\n", name)
		} else {
			fmt.Fprintf(&fb.out, "    movsd xmm0, qword [%s]\n", name)
		}
	} else {
		fmt.Fprintf(&fb.out, "    mov rax, qword [%s]\n", name)
	}
	fb.emitStore(t, fb.valueSlots[ins.Result])
}

func (fb *fasmEmitter) emitStoreGlobal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	name := fb.globalLabel(ins.Imm.Symbol)
	if isAggregate(t) {
		fb.emitAggCopyToAddr(fb.valueSlots[ins.Args[0]], name, fb.sizeOf(t))
		return
	}
	fb.emitLoad(t, fb.valueSlots[ins.Args[0]])
	if t.Kind == TypeKindFloat {
		if t.Name == "F32" {
			fmt.Fprintf(&fb.out, "    movss dword [%s], xmm0\n", name)
		} else {
			fmt.Fprintf(&fb.out, "    movsd qword [%s], xmm0\n", name)
		}
	} else {
		fmt.Fprintf(&fb.out, "    mov qword [%s], rax\n", name)
	}
}

func (fb *fasmEmitter) emitArith(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	lslot := fb.valueSlots[ins.Args[0]]
	rslot := fb.valueSlots[ins.Args[1]]
	resSlot := fb.valueSlots[ins.Result]
	if isInt128(t.Name) {
		fb.emitArith128(ins, t, lslot, rslot, resSlot)
		return
	}
	if t.Kind == TypeKindFloat {
		fb.emitLoad(t, lslot)
		switch ins.Op {
		case MIRAdd:
			fmt.Fprintf(&fb.out, "    add%s xmm0, %s [rbp-%d]\n", floatSuffix(t), floatMemSize(t), rslot)
		case MIRSub:
			fmt.Fprintf(&fb.out, "    sub%s xmm0, %s [rbp-%d]\n", floatSuffix(t), floatMemSize(t), rslot)
		case MIRMul:
			fmt.Fprintf(&fb.out, "    mul%s xmm0, %s [rbp-%d]\n", floatSuffix(t), floatMemSize(t), rslot)
		case MIRDiv:
			fmt.Fprintf(&fb.out, "    div%s xmm0, %s [rbp-%d]\n", floatSuffix(t), floatMemSize(t), rslot)
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

// emitArith128 emits 128-bit add, sub, mul, div, and mod. Values are a low
// half at [rbp-offset] and a high half at [rbp-offset+8].
func (fb *fasmEmitter) emitArith128(ins *MIRInstr, t IRType, lslot, rslot, resSlot int) {
	switch ins.Op {
	case MIRAdd:
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", lslot)
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d+8]\n", lslot)
		fmt.Fprintf(&fb.out, "    add rax, qword [rbp-%d]\n", rslot)
		fmt.Fprintf(&fb.out, "    adc rcx, qword [rbp-%d+8]\n", rslot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rcx\n", resSlot)
	case MIRSub:
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", lslot)
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d+8]\n", lslot)
		fmt.Fprintf(&fb.out, "    sub rax, qword [rbp-%d]\n", rslot)
		fmt.Fprintf(&fb.out, "    sbb rcx, qword [rbp-%d+8]\n", rslot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rcx\n", resSlot)
	case MIRMul:
		// a*b = a0*b0 + (a0*b1 + a1*b0)<<64 (mod 2^128).
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", lslot)
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d]\n", rslot)
		fb.out.WriteString("    mul rcx\n") // rdx:rax = a0*b0
		fb.out.WriteString("    mov r8, rax\n")
		fb.out.WriteString("    mov r9, rdx\n")
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", lslot)
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d+8]\n", rslot)
		fb.out.WriteString("    mul rcx\n") // rdx:rax = a0*b1
		fb.out.WriteString("    mov r10, rax\n")
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", lslot)
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d]\n", rslot)
		fb.out.WriteString("    mul rcx\n") // rdx:rax = a1*b0
		fmt.Fprintf(&fb.out, "    mov r11, rax\n")
		fb.out.WriteString("    mov rax, r9\n")
		fb.out.WriteString("    add rax, r10\n")
		fb.out.WriteString("    add rax, r11\n")
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], r8\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", resSlot)
	case MIRDiv, MIRMod:
		fb.emitDivMod128(ins, t, lslot, rslot, resSlot)
	}
}

// emitDivMod128 emits 128-bit signed or unsigned division using the binary
// long-division algorithm. The quotient is stored for MIRDiv and the
// remainder for MIRMod.
func (fb *fasmEmitter) emitDivMod128(ins *MIRInstr, t IRType, lslot, rslot, resSlot int) {
	loop := fb.newLabel()
	sub := fb.newLabel()
	skip := fb.newLabel()
	fmt.Fprintf(&fb.out, "    mov r8, qword [rbp-%d]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov r9, qword [rbp-%d+8]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov r10, qword [rbp-%d]\n", rslot)
	fmt.Fprintf(&fb.out, "    mov r11, qword [rbp-%d+8]\n", rslot)
	if isSignedInt(t.Name) {
		// Track the dividend sign in ebx; negate it if negative.
		fb.out.WriteString("    xor ebx, ebx\n")
		fb.out.WriteString("    bt r9, 63\n")
		fb.out.WriteString("    jnc L" + loop + "_dv\n")
		fb.out.WriteString("    not r8\n")
		fb.out.WriteString("    not r9\n")
		fb.out.WriteString("    add r8, 1\n")
		fb.out.WriteString("    adc r9, 0\n")
		fb.out.WriteString("    mov ebx, 1\n")
		fmt.Fprintf(&fb.out, "L%s_dv:\n", loop)
		// Track the result sign in edi; negate the divisor if negative.
		fb.out.WriteString("    xor edi, edi\n")
		fb.out.WriteString("    bt r11, 63\n")
		fmt.Fprintf(&fb.out, "    jnc L%s_ds\n", loop)
		fb.out.WriteString("    not r10\n")
		fb.out.WriteString("    not r11\n")
		fb.out.WriteString("    add r10, 1\n")
		fb.out.WriteString("    adc r11, 0\n")
		fb.out.WriteString("    mov edi, 1\n")
		fmt.Fprintf(&fb.out, "L%s_ds:\n", loop)
		fb.out.WriteString("    xor edi, ebx\n")
	}
	fb.out.WriteString("    xor r12, r12\n")
	fb.out.WriteString("    xor r13, r13\n")
	fb.out.WriteString("    xor r14, r14\n")
	fb.out.WriteString("    xor r15, r15\n")
	fb.out.WriteString("    mov rcx, 128\n")
	fmt.Fprintf(&fb.out, "L%s:\n", loop)
	fb.out.WriteString("    shl r14, 1\n")
	fb.out.WriteString("    adc r15, r15\n")
	fb.out.WriteString("    bt r9, 63\n")
	fb.out.WriteString("    adc r14, 0\n")
	fb.out.WriteString("    shl r8, 1\n")
	fb.out.WriteString("    adc r9, r9\n")
	fb.out.WriteString("    shl r12, 1\n")
	fb.out.WriteString("    adc r13, r13\n")
	fb.out.WriteString("    cmp r15, r11\n")
	fmt.Fprintf(&fb.out, "    jb L%s\n", skip)
	fmt.Fprintf(&fb.out, "    ja L%s\n", sub)
	fb.out.WriteString("    cmp r14, r10\n")
	fmt.Fprintf(&fb.out, "    jb L%s\n", skip)
	fmt.Fprintf(&fb.out, "L%s:\n", sub)
	fb.out.WriteString("    sub r14, r10\n")
	fb.out.WriteString("    sbb r15, r11\n")
	fb.out.WriteString("    or r12, 1\n")
	fmt.Fprintf(&fb.out, "L%s:\n", skip)
	fb.out.WriteString("    dec rcx\n")
	fmt.Fprintf(&fb.out, "    jnz L%s\n", loop)
	if isSignedInt(t.Name) {
		// Negate the quotient when the signs differ; the remainder takes
		// the dividend's sign.
		fb.out.WriteString("    test edi, edi\n")
		fmt.Fprintf(&fb.out, "    jz L%s_q\n", loop)
		fb.out.WriteString("    not r12\n")
		fb.out.WriteString("    not r13\n")
		fb.out.WriteString("    add r12, 1\n")
		fb.out.WriteString("    adc r13, 0\n")
		fmt.Fprintf(&fb.out, "L%s_q:\n", loop)
		fb.out.WriteString("    test ebx, ebx\n")
		fmt.Fprintf(&fb.out, "    jz L%s_r\n", loop)
		fb.out.WriteString("    not r14\n")
		fb.out.WriteString("    not r15\n")
		fb.out.WriteString("    add r14, 1\n")
		fb.out.WriteString("    adc r15, 0\n")
		fmt.Fprintf(&fb.out, "L%s_r:\n", loop)
	}
	if ins.Op == MIRMod {
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], r14\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], r15\n", resSlot)
	} else {
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], r12\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], r13\n", resSlot)
	}
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
	if lt.Kind == TypeKindString {
		fb.emitStringCmp(ins, lslot, rslot, resSlot)
		return
	}
	if lt.Kind == TypeKindStruct {
		fb.emitStructCmp(ins, lt, lslot, rslot, resSlot)
		return
	}
	if isInt128(lt.Name) {
		fb.emitCmp128(ins, lt, lslot, rslot, resSlot)
		return
	}
	if lt.Kind == TypeKindFloat {
		fb.emitLoad(lt, lslot)
		if lt.Name == "F32" {
			fmt.Fprintf(&fb.out, "    comiss xmm0, dword [rbp-%d]\n", rslot)
		} else {
			fmt.Fprintf(&fb.out, "    comisd xmm0, qword [rbp-%d]\n", rslot)
		}
		fb.emitSetcc(ins.Op, false, resSlot)
		return
	}
	size := fb.sizeOf(lt)
	fb.emitLoad(lt, lslot)
	fmt.Fprintf(&fb.out, "    cmp %s, %s [rbp-%d]\n", widthReg(size), memSize(size), rslot)
	fb.emitSetcc(ins.Op, isSignedInt(lt.Name), resSlot)
}

// emitStructCmp emits a byte-wise comparison of two struct values using
// repe cmpsb. The flags afterwards describe the lexicographic byte order.
func (fb *fasmEmitter) emitStructCmp(ins *MIRInstr, t IRType, lslot, rslot, resSlot int) {
	fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", lslot)
	fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", rslot)
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", fb.sizeOf(t))
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    repe cmpsb\n")
	cc := "sete"
	switch ins.Op {
	case MIRCmpNeq:
		cc = "setne"
	case MIRCmpLt:
		cc = "setb"
	case MIRCmpGt:
		cc = "seta"
	case MIRCmpLe:
		cc = "setbe"
	case MIRCmpGe:
		cc = "setae"
	}
	fmt.Fprintf(&fb.out, "    %s al\n", cc)
	fmt.Fprintf(&fb.out, "    mov byte [rbp-%d], al\n", resSlot)
}

// emitCmp128 emits a 128-bit signed or unsigned comparison by comparing the
// high halves first, then the low halves when they are equal.
func (fb *fasmEmitter) emitCmp128(ins *MIRInstr, t IRType, lslot, rslot, resSlot int) {
	fmt.Fprintf(&fb.out, "    mov r8, qword [rbp-%d]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov r9, qword [rbp-%d+8]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov r10, qword [rbp-%d]\n", rslot)
	fmt.Fprintf(&fb.out, "    mov r11, qword [rbp-%d+8]\n", rslot)
	fb.out.WriteString("    cmp r9, r11\n")
	less, greater, lessEq, greaterEq := cmpBranches(isSignedInt(t.Name))
	trueLabel := fb.newLabel()
	falseLabel := fb.newLabel()
	doneLabel := fb.newLabel()
	switch ins.Op {
	case MIRCmpEq:
		fmt.Fprintf(&fb.out, "    jne %s\n", falseLabel)
		fb.out.WriteString("    cmp r8, r10\n")
		fmt.Fprintf(&fb.out, "    jne %s\n", falseLabel)
		fmt.Fprintf(&fb.out, "    jmp %s\n", trueLabel)
	case MIRCmpNeq:
		fmt.Fprintf(&fb.out, "    jne %s\n", trueLabel)
		fb.out.WriteString("    cmp r8, r10\n")
		fmt.Fprintf(&fb.out, "    jne %s\n", trueLabel)
		fmt.Fprintf(&fb.out, "    jmp %s\n", falseLabel)
	case MIRCmpLt:
		fmt.Fprintf(&fb.out, "    %s %s\n", less, trueLabel)
		fmt.Fprintf(&fb.out, "    %s %s\n", greater, falseLabel)
		fb.out.WriteString("    cmp r8, r10\n")
		fmt.Fprintf(&fb.out, "    %s %s\n", less, trueLabel)
		fmt.Fprintf(&fb.out, "    jmp %s\n", falseLabel)
	case MIRCmpGt:
		fmt.Fprintf(&fb.out, "    %s %s\n", greater, trueLabel)
		fmt.Fprintf(&fb.out, "    %s %s\n", less, falseLabel)
		fb.out.WriteString("    cmp r8, r10\n")
		fmt.Fprintf(&fb.out, "    %s %s\n", greater, trueLabel)
		fmt.Fprintf(&fb.out, "    jmp %s\n", falseLabel)
	case MIRCmpLe:
		fmt.Fprintf(&fb.out, "    %s %s\n", less, trueLabel)
		fmt.Fprintf(&fb.out, "    %s %s\n", greater, falseLabel)
		fb.out.WriteString("    cmp r8, r10\n")
		fmt.Fprintf(&fb.out, "    %s %s\n", lessEq, trueLabel)
		fmt.Fprintf(&fb.out, "    jmp %s\n", falseLabel)
	case MIRCmpGe:
		fmt.Fprintf(&fb.out, "    %s %s\n", greater, trueLabel)
		fmt.Fprintf(&fb.out, "    %s %s\n", less, falseLabel)
		fb.out.WriteString("    cmp r8, r10\n")
		fmt.Fprintf(&fb.out, "    %s %s\n", greaterEq, trueLabel)
		fmt.Fprintf(&fb.out, "    jmp %s\n", falseLabel)
	}
	fmt.Fprintf(&fb.out, "%s:\n", trueLabel)
	fb.out.WriteString("    mov rax, 1\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", doneLabel)
	fmt.Fprintf(&fb.out, "%s:\n", falseLabel)
	fb.out.WriteString("    mov rax, 0\n")
	fmt.Fprintf(&fb.out, "%s:\n", doneLabel)
	fmt.Fprintf(&fb.out, "    mov byte [rbp-%d], al\n", resSlot)
}

// emitStringCmp emits a byte-wise comparison of two String values (pointer
// and length pairs). The result is stored for MIRCmpEq and inverted for
// MIRCmpNeq.
func (fb *fasmEmitter) emitStringCmp(ins *MIRInstr, lslot, rslot, resSlot int) {
	eqLabel := fb.newLabel()
	neLabel := fb.newLabel()
	loopLabel := fb.newLabel()
	doneLabel := fb.newLabel()
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d+8]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d]\n", rslot)
	fmt.Fprintf(&fb.out, "    mov r8, qword [rbp-%d+8]\n", rslot)
	fb.out.WriteString("    cmp rcx, r8\n")
	fmt.Fprintf(&fb.out, "    jne %s\n", neLabel)
	fb.out.WriteString("    xor r9, r9\n")
	fmt.Fprintf(&fb.out, "%s:\n", loopLabel)
	fb.out.WriteString("    cmp r9, rcx\n")
	fmt.Fprintf(&fb.out, "    jae %s\n", eqLabel)
	fb.out.WriteString("    mov r10b, byte [rax+r9]\n")
	fb.out.WriteString("    cmp r10b, byte [rdx+r9]\n")
	fmt.Fprintf(&fb.out, "    jne %s\n", neLabel)
	fb.out.WriteString("    inc r9\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", loopLabel)
	fmt.Fprintf(&fb.out, "%s:\n", eqLabel)
	fb.out.WriteString("    mov rax, 1\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", doneLabel)
	fmt.Fprintf(&fb.out, "%s:\n", neLabel)
	fb.out.WriteString("    mov rax, 0\n")
	fmt.Fprintf(&fb.out, "%s:\n", doneLabel)
	if ins.Op == MIRCmpNeq {
		fb.out.WriteString("    xor al, 1\n")
	}
	fb.out.WriteString("    mov byte [rbp-" + strconv.Itoa(resSlot) + "], al\n")
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
		if t.Name == "F32" {
			fb.out.WriteString("    xorps xmm1, xmm1\n")
			fb.out.WriteString("    subss xmm1, xmm0\n")
			fb.out.WriteString("    movss xmm0, xmm1\n")
		} else {
			fb.out.WriteString("    xorpd xmm1, xmm1\n")
			fb.out.WriteString("    subsd xmm1, xmm0\n")
			fb.out.WriteString("    movsd xmm0, xmm1\n")
		}
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
	resultType := fb.prog.Types.Lookup(ins.Type)
	returnsStruct := resultType.Kind == TypeKindStruct

	// Assign each argument to its System V register or registers. String and
	// 128-bit arguments take two integer registers; struct arguments pass an
	// address. Arguments are loaded in reverse order so that loading a later
	// argument cannot clobber an earlier one.
	type argReg struct {
		arg  ValueID
		kind argKind
		idx  int
	}
	regs := make([]argReg, 0, len(ins.Args))
	intIdx, floatIdx := 0, 0
	if returnsStruct {
		intIdx = 1 // RDI is reserved for the sret pointer.
	}
	for _, a := range ins.Args {
		t := fb.prog.Types.Lookup(fb.valueTypes[a])
		switch {
		case t.Kind == TypeKindFloat:
			regs = append(regs, argReg{arg: a, kind: argFloat, idx: floatIdx})
			floatIdx++
		case t.Kind == TypeKindString:
			regs = append(regs, argReg{arg: a, kind: argString, idx: intIdx})
			intIdx += 2
		case t.Kind == TypeKindStruct:
			regs = append(regs, argReg{arg: a, kind: argStruct, idx: intIdx})
			intIdx++
		case isInt128(t.Name):
			regs = append(regs, argReg{arg: a, kind: argInt128, idx: intIdx})
			intIdx += 2
		default:
			regs = append(regs, argReg{arg: a, kind: argInt, idx: intIdx})
			intIdx++
		}
	}

	// Pass the result slot address for struct-returning callees.
	if returnsStruct {
		fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", fb.valueSlots[ins.Result])
	}

	for i := len(regs) - 1; i >= 0; i-- {
		r := regs[i]
		t := fb.prog.Types.Lookup(fb.valueTypes[r.arg])
		slot := fb.valueSlots[r.arg]
		switch r.kind {
		case argFloat:
			if r.idx >= 8 {
				fb.diags.Error(ins.Span, "calls with more than 8 float arguments are not yet supported", "reduce the number of float arguments")
				continue
			}
			fb.emitLoad(t, slot)
			if t.Name == "F32" {
				fmt.Fprintf(&fb.out, "    movss xmm%d, xmm0\n", r.idx)
			} else {
				fmt.Fprintf(&fb.out, "    movsd xmm%d, xmm0\n", r.idx)
			}
		case argString:
			if r.idx+1 >= 6 {
				fb.diags.Error(ins.Span, "calls with too many integer arguments are not yet supported", "reduce the number of arguments")
				continue
			}
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(r.idx))
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(r.idx+1))
		case argStruct:
			if r.idx >= 6 {
				fb.diags.Error(ins.Span, "calls with too many integer arguments are not yet supported", "reduce the number of arguments")
				continue
			}
			fmt.Fprintf(&fb.out, "    lea rax, [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(r.idx))
		case argInt128:
			if r.idx+1 >= 6 {
				fb.diags.Error(ins.Span, "calls with too many integer arguments are not yet supported", "reduce the number of arguments")
				continue
			}
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(r.idx))
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(r.idx+1))
		case argInt:
			if r.idx >= 6 {
				fb.diags.Error(ins.Span, "calls with more than 6 integer arguments are not yet supported", "reduce the number of integer arguments")
				continue
			}
			fb.emitLoad(t, slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(r.idx))
		}
	}
	fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(callee.Name))

	// Store the result. Struct results were written into the sret slot by the
	// callee; String and 128-bit results arrive in RAX:RDX.
	if ins.Type != fb.prog.Types.Void() {
		slot := fb.valueSlots[ins.Result]
		switch {
		case resultType.Kind == TypeKindStruct:
			// Already in the sret slot.
		case resultType.Kind == TypeKindString || isInt128(resultType.Name):
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rdx\n", slot)
		default:
			fb.emitStore(resultType, slot)
		}
	}
}

// argKind classifies how an argument is passed.
type argKind uint8

const (
	argInt argKind = iota
	argFloat
	argString
	argStruct
	argInt128
)

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
			slot := fb.valueSlots[t.Value]
			switch {
			case vt.Kind == TypeKindStruct:
				// Copy the struct value to the sret address.
				fb.emitAggCopyToAddrSlot(slot, fb.sretSlot, fb.sizeOf(vt))
			case vt.Kind == TypeKindString || isInt128(vt.Name):
				fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
				fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d+8]\n", slot)
			default:
				fb.emitLoad(vt, slot)
			}
		}
		fb.out.WriteString("    leave\n")
		fb.out.WriteString("    ret\n")
	case MIRTermExit:
		if t.Message != NoValue {
			msgSlot := fb.valueSlots[t.Message]
			fb.out.WriteString("    mov rax, 1\n")
			fb.out.WriteString("    mov rdi, 2\n")
			fmt.Fprintf(&fb.out, "    mov rsi, qword [rbp-%d]\n", msgSlot)
			fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d+8]\n", msgSlot)
			fb.out.WriteString("    syscall\n")
		}
		if t.Status != NoValue {
			st := fb.prog.Types.Lookup(fb.valueTypes[t.Status])
			fb.emitLoad(st, fb.valueSlots[t.Status])
			fb.out.WriteString("    mov rdi, rax\n")
		} else {
			fb.out.WriteString("    xor edi, edi\n")
		}
		fb.out.WriteString("    mov rax, 60\n")
		fb.out.WriteString("    syscall\n")
	case MIRUnreachable:
		fb.out.WriteString("    ud2\n")
	}
}

// sizeOf returns the storage size in bytes for a type. String values are a
// pointer and length pair, 128-bit integers a low and high half pair, and
// structs the sum of their aligned fields.
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
		case "S128", "U128":
			return 16
		}
	case TypeKindFloat:
		switch t.Name {
		case "F32":
			return 4
		case "F64":
			return 8
		}
	case TypeKindString:
		return 16
	case TypeKindStruct:
		size := 0
		for _, f := range t.Fields {
			ft := fb.prog.Types.Lookup(f.Type)
			size = align(size, fb.sizeOf(ft))
			size += fb.sizeOf(ft)
		}
		return size
	}
	return 8
}

// structFieldOffsets returns the byte offset of each field in a struct type.
func (fb *fasmEmitter) structFieldOffsets(t IRType) []int {
	offsets := make([]int, len(t.Fields))
	off := 0
	for i, f := range t.Fields {
		ft := fb.prog.Types.Lookup(f.Type)
		off = align(off, fb.sizeOf(ft))
		offsets[i] = off
		off += fb.sizeOf(ft)
	}
	return offsets
}

// isAggregate reports whether a type occupies more than one register-sized
// piece and must be copied as a byte range.
func isAggregate(t IRType) bool {
	switch {
	case t.Kind == TypeKindString:
		return true
	case t.Kind == TypeKindStruct:
		return true
	case t.Kind == TypeKindInt && isInt128(t.Name):
		return true
	}
	return false
}

// isInt128 reports whether a type name is a 128-bit integer.
func isInt128(name string) bool {
	return name == "S128" || name == "U128"
}

// intRegCount returns the number of integer argument registers a type
// consumes when passed by value.
func intRegCount(t IRType) int {
	switch {
	case t.Kind == TypeKindString:
		return 2
	case t.Kind == TypeKindStruct:
		return 1 // passed by address
	case t.Kind == TypeKindInt && isInt128(t.Name):
		return 2
	default:
		return 1
	}
}

// floatSuffix returns the SSE operand-size suffix for a float type.
func floatSuffix(t IRType) string {
	if t.Name == "F32" {
		return "ss"
	}
	return "sd"
}

// floatMemSize returns the memory operand size for a float type.
func floatMemSize(t IRType) string {
	if t.Name == "F32" {
		return "dword"
	}
	return "qword"
}

// cmpBranches returns the branch mnemonics for 128-bit comparisons.
func cmpBranches(signed bool) (less, greater, lessEq, greaterEq string) {
	if signed {
		return "jl", "jg", "jle", "jge"
	}
	return "jb", "ja", "jbe", "jae"
}

// newLabel returns a fresh, unique label name.
func (fb *fasmEmitter) newLabel() string {
	l := fmt.Sprintf("L%d", fb.labelCount)
	fb.labelCount++
	return l
}

// emitAggCopy copies size bytes between two stack slots.
func (fb *fasmEmitter) emitAggCopy(srcSlot, dstSlot, size int) {
	fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", srcSlot)
	fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", dstSlot)
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", size)
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    rep movsb\n")
}

// emitAggCopyFromAddr copies size bytes from an absolute address into a
// stack slot.
func (fb *fasmEmitter) emitAggCopyFromAddr(addr string, dstSlot, size int) {
	fmt.Fprintf(&fb.out, "    mov rsi, %s\n", addr)
	fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", dstSlot)
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", size)
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    rep movsb\n")
}

// emitAggCopyToAddr copies size bytes from a stack slot to an absolute
// address.
func (fb *fasmEmitter) emitAggCopyToAddr(srcSlot int, addr string, size int) {
	fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", srcSlot)
	fmt.Fprintf(&fb.out, "    mov rdi, %s\n", addr)
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", size)
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    rep movsb\n")
}

// emitAggCopyToAddrSlot copies size bytes from a stack slot to the address
// held in another stack slot (used for sret returns).
func (fb *fasmEmitter) emitAggCopyToAddrSlot(srcSlot, addrSlot, size int) {
	fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", srcSlot)
	fmt.Fprintf(&fb.out, "    mov rdi, qword [rbp-%d]\n", addrSlot)
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", size)
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    rep movsb\n")
}

// emitStructInit zeroes the result slot (so padding bytes are defined) and
// stores each struct literal field at its byte offset.
func (fb *fasmEmitter) emitStructInit(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	resSlot := fb.valueSlots[ins.Result]
	fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", resSlot)
	fb.out.WriteString("    xor eax, eax\n")
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", fb.sizeOf(t))
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    rep stosb\n")
	offsets := fb.structFieldOffsets(t)
	for i, a := range ins.Args {
		ft := fb.prog.Types.Lookup(t.Fields[i].Type)
		dstOffset := resSlot - offsets[i]
		if isAggregate(ft) {
			fb.emitAggCopy(fb.valueSlots[a], dstOffset, fb.sizeOf(ft))
		} else {
			fb.emitLoad(ft, fb.valueSlots[a])
			fb.emitStore(ft, dstOffset)
		}
	}
}

// emitLoad loads the value at [rbp-offset] into rax (integers, sign or zero
// extended to 64 bits) or xmm0 (floats).
func (fb *fasmEmitter) emitLoad(t IRType, offset int) {
	size := fb.sizeOf(t)
	if t.Kind == TypeKindFloat {
		if t.Name == "F32" {
			fmt.Fprintf(&fb.out, "    movss xmm0, dword [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movsd xmm0, qword [rbp-%d]\n", offset)
		}
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
		if t.Name == "F32" {
			fmt.Fprintf(&fb.out, "    movss dword [rbp-%d], xmm0\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movsd qword [rbp-%d], xmm0\n", offset)
		}
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
	case "S8", "S16", "S32", "S64", "S128":
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
