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
//
// F16 and F128 are deliberately NOT supported here. The front end accepts
// them as valid float types, but this backend has no correct code generation
// for either width (see checkSupportedTypes for the full rationale): F16 has
// no native x86-64 arithmetic (it needs F16C conversion instructions or
// software emulation), and F128 would require a full software floating-point
// library. Rather than silently emit F64 code with wrong precision, the
// backend rejects them with a diagnostic.
package compiler

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// FasmBackend emits flat ELF64 executables for fasm.
type FasmBackend struct{}

// Name returns the backend's target name.
func (b *FasmBackend) Name() string { return "fasm" }

// Emit renders a MIR program as fasm assembly text.
func (b *FasmBackend) Emit(prog *MIRProgram) (string, DiagnosticList) {
	if verifyDiags := VerifyMIR(prog); verifyDiags.HasErrors() {
		return "", verifyDiags
	}
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
	fn              *MIRFunction
	valueSlots      map[ValueID]int
	valueTypes      map[ValueID]TypeID
	localSlots      map[LocalID]int
	blockLabels     map[BlockID]string
	arrayBuffers    map[ValueID]int // stack buffer offset for array.init results
	nextOffset      int
	frameSize       int
	labelCount      int
	sretSlot        int // slot holding the sret pointer, -1 when unused
	calleeSaveSlots [5]int

	// Float and string constants emitted in the data section.
	floatConsts      []floatConst
	floatIndexes     map[floatConstKey]int
	strings          []string
	stringIndexes    map[string]int
	divisionMessages map[*MIRInstr]int
	boundsMessages   map[*MIRInstr]int
	newlineIndex     int // index of the "\n" constant in strings
	openFailIndex    int // index of the "cannot open file: " constant in strings
	readFailIndex    int // index of the "cannot read file: " constant in strings
	allocFailIndex   int // index of the allocation failure constant in strings
	globalLabels     map[SymbolID]string
	equalityTypes    map[TypeID]bool
	needsHeap        bool
	reachable        map[SymbolID]bool // functions reachable from the entry point
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

// checkSupportedTypes reports a diagnostic for float types the backend does
// not implement. F16 and F128 are accepted by the front end but have no
// x86-64 storage or arithmetic here; emitting them as F64 would silently
// change their semantics, so the backend rejects them instead.
//
// Why F16 and F128 are unsupported:
//
//   - Every float code path in this backend branches on "F32" vs everything
//     else (floatSuffix, floatMemSize, sizeOf, emitLoad/emitStore/emitCmp,
//     emitConst, emitNeg, and the argument ABI). F16 and F128 would fall
//     through to the F64 path, which is the wrong size: F16 stores in 2
//     bytes, F128 in 16 bytes.
//   - F16 has no native x86-64 arithmetic. SSE has single (addss) and double
//     (addsd) precision only. Half-precision needs the optional F16C
//     extension (vcvtph2ps/vcvtps2ph) plus conversion around every
//     operation, or software emulation on CPUs without F16C.
//   - F128 has no SSE support at all. The x87 long double is 80-bit, not
//     128-bit, so quad precision needs a full software floating-point
//     library: add/sub/mul/div, comparisons, rounding, and conversions over
//     a 128-bit payload. This is the same class of work as the 128-bit
//     integer support (emitArith128/emitDivMod128), but larger because of
//     IEEE rounding and special values (NaN, infinity, subnormals).
//
// Implementing F16 or F128 is therefore a significant project of its own.
// Until one of them is implemented, reject loudly instead of miscompiling:
// the front end keeps them as valid types, and this check is the
// implementation boundary.
func (fb *fasmEmitter) checkSupportedTypes() {
	reported := make(map[string]Span)
	visiting := make(map[TypeID]bool)
	var report func(TypeID, Span)
	report = func(tid TypeID, span Span) {
		t := fb.prog.Types.Lookup(tid)
		if visiting[tid] {
			key := "recursive:" + strconv.FormatUint(uint64(tid), 10)
			if _, ok := reported[key]; !ok {
				reported[key] = span
				fb.diags.Error(span, "recursive by-value type reached the fasm backend", "reject recursive storage during semantic analysis")
			}
			return
		}
		visiting[tid] = true
		defer delete(visiting, tid)
		switch t.Kind {
		case TypeKindUnknown:
			fb.diags.Error(span, "unresolved type reached the fasm backend", "fix semantic errors before code generation")
		case TypeKindInt:
			if info, ok := LookupBuiltinType(t.Name); !ok || info.Kind != BuiltinInteger {
				fb.diags.Error(span, "unsupported integer type reached the fasm backend", "use a compiler-defined integer type")
			}
		case TypeKindFloat:
			info, known := LookupBuiltinType(t.Name)
			if !known || info.Kind != BuiltinFloat {
				fb.diags.Error(span, "unsupported floating-point type reached the fasm backend", "use F32 or F64")
			} else if !info.FasmSupported {
				if _, ok := reported[t.Name]; !ok {
					reported[t.Name] = span
					fb.diags.Error(span, "float type "+t.Name+" is not yet supported by the fasm backend", "use F32 or F64")
				}
			}
		case TypeKindStruct, TypeKindTuple:
			for _, f := range t.Fields {
				report(f.Type, span)
			}
		case TypeKindArray:
			report(t.Elem, span)
		case TypeKindEnum:
			report(t.Underlying, span)
		}
	}
	for _, g := range fb.prog.Globals {
		report(g.Type, g.Span)
	}
	for _, fn := range fb.prog.Functions {
		for _, lid := range fn.Locals {
			report(fn.LocalTypes[lid], fn.Span)
		}
		for _, r := range fn.Results {
			report(r, fn.Span)
		}
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				report(ins.Type, ins.Span)
			}
		}
	}
}

func (fb *fasmEmitter) emit() {
	fb.checkSupportedTypes()
	entry := fb.findFunction(fb.prog.Entry)
	if fb.prog.Entry == NoSymbol || entry == nil {
		fb.diags.Error(Span{}, "program has no valid #entry procedure", "declare '#entry name :: proc -> S64'")
	} else if !fb.validEntrySignature(entry) {
		fb.diags.Error(entry.Span, "entry procedure does not have the required () -> S64 or (argc: S64, argv: *String) -> S64 target signature", "declare '#entry name :: proc -> S64' or '#entry name :: proc (argc: S64, argv: *String) -> S64'")
	}
	if fb.diags.HasErrors() {
		return
	}
	fb.reachable = fb.computeReachable()
	// The (argc, argv) entry builds its String array from the bump allocator,
	// so the heap symbols must be emitted even when no reachable function
	// allocates. collectStringConsts sets needsHeap for the other triggers.
	if entry := fb.findFunction(fb.prog.Entry); entry != nil && len(entry.Params) == 2 {
		fb.needsHeap = true
	}
	fb.prepareGlobalLabels()
	fb.out.WriteString("format ELF64 executable 3\n\n")
	fb.collectFloatConsts()
	fb.collectStringConsts()
	fb.emitData()
	fb.out.WriteString("segment readable executable\n\n")
	fb.emitEntry()
	if fb.needsHeap {
		fb.emitHeapAllocator()
	}
	for _, fn := range fb.prog.Functions {
		if fb.reachable[fn.Symbol] {
			fb.emitFunction(fn)
		}
	}
	fb.emitEqualityHelpers()
	if fb.diags.HasErrors() {
		fb.out.Reset()
	}
}

// computeReachable marks every function reachable from the entry procedure
// and the global initializer through direct calls. Unused imported functions
// are never reached and are therefore not emitted into the binary.
func (fb *fasmEmitter) computeReachable() map[SymbolID]bool {
	reachable := make(map[SymbolID]bool)
	queue := make([]SymbolID, 0, 2)
	if fb.prog.Entry != NoSymbol {
		queue = append(queue, fb.prog.Entry)
	}
	if fb.prog.GlobalInit != nil {
		queue = append(queue, fb.prog.GlobalInit.Symbol)
	}
	for len(queue) > 0 {
		sym := queue[0]
		queue = queue[1:]
		if reachable[sym] {
			continue
		}
		reachable[sym] = true
		fn := fb.findFunction(sym)
		if fn == nil {
			continue
		}
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				if ins.Op == MIRCall && ins.Imm.Kind == MIRImmSymbol && !reachable[ins.Imm.Symbol] {
					queue = append(queue, ins.Imm.Symbol)
				}
			}
		}
	}
	return reachable
}

// collectFloatConsts gathers every float constant so the data section can be
// emitted before the code that references it. The storage size (4 or 8) is
// taken from the constant's type.
func (fb *fasmEmitter) collectFloatConsts() {
	fb.floatIndexes = make(map[floatConstKey]int)
	for _, fn := range fb.prog.Functions {
		if !fb.reachable[fn.Symbol] {
			continue
		}
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
	fb.divisionMessages = make(map[*MIRInstr]int)
	fb.boundsMessages = make(map[*MIRInstr]int)
	fb.newlineIndex = -1
	fb.openFailIndex = -1
	fb.readFailIndex = -1
	fb.allocFailIndex = -1
	for _, fn := range fb.prog.Functions {
		if !fb.reachable[fn.Symbol] {
			continue
		}
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				if ins.Op == MIRArrayIndex || ins.Op == MIRArrayElemAddr {
					message := fb.boundsMessage(ins.Span)
					index, ok := fb.stringIndexes[message]
					if !ok {
						index = len(fb.strings)
						fb.stringIndexes[message] = index
						fb.strings = append(fb.strings, message)
					}
					fb.boundsMessages[ins] = index
				}
				if ins.Op == MIRConst && ins.Imm.Kind == MIRImmString {
					if _, ok := fb.stringIndexes[ins.Imm.Str]; !ok {
						fb.stringIndexes[ins.Imm.Str] = len(fb.strings)
						fb.strings = append(fb.strings, ins.Imm.Str)
					}
				}
				if ins.Op == MIRDiv || ins.Op == MIRMod {
					// Integer, enum, and floating-point division all share the
					// same source-located run-time trap. Collect its message before
					// emitting the data section; adding it lazily from the code
					// emitter would leave the referenced label undefined.
					message := fb.divisionMessage(ins.Span)
					index, ok := fb.stringIndexes[message]
					if !ok {
						index = len(fb.strings)
						fb.stringIndexes[message] = index
						fb.strings = append(fb.strings, message)
					}
					fb.divisionMessages[ins] = index
				}
				if ins.Op == MIRAllocate {
					fb.needsHeap = true
				}
				if ins.Op == MIRArrayInit {
					if t := fb.prog.Types.Lookup(ins.Type); t.Kind == TypeKindArray && t.ArrayKind == ArrayDynamic {
						fb.needsHeap = true
					}
				}
				if ins.Op == MIRInterpolate {
					fb.needsHeap = true
					for _, lit := range ins.Imm.Strs {
						if _, ok := fb.stringIndexes[lit]; !ok {
							fb.stringIndexes[lit] = len(fb.strings)
							fb.strings = append(fb.strings, lit)
						}
					}
				}
				if ins.Op == MIRPrint && ins.Imm.Bool {
					fb.ensureString("\n", &fb.newlineIndex)
				}
				if ins.Op == MIRReadFile || ins.Op == MIRFileExists {
					// The null-terminated path copy uses the bump allocator.
					fb.needsHeap = true
					fb.ensureString("\n", &fb.newlineIndex)
				}
				if ins.Op == MIRReadFile {
					fb.ensureString("cannot open file: ", &fb.openFailIndex)
					fb.ensureString("cannot read file: ", &fb.readFailIndex)
				}
			}
		}
	}
	if fb.needsHeap {
		fb.ensureString("allocation failed\n", &fb.allocFailIndex)
	}
}

// ensureString adds s to the string constant table if it is not already
// present and records its index in *index.
func (fb *fasmEmitter) ensureString(s string, index *int) {
	if *index >= 0 {
		return
	}
	if i, ok := fb.stringIndexes[s]; ok {
		*index = i
		return
	}
	i := len(fb.strings)
	fb.stringIndexes[s] = i
	fb.strings = append(fb.strings, s)
	*index = i
}

// newlineLabel returns the data-section label of the "\n" constant.
func (fb *fasmEmitter) newlineLabel() string {
	return fmt.Sprintf("str%d", fb.newlineIndex)
}

func (fb *fasmEmitter) divisionMessage(span Span) string {
	if sf, ok := fb.prog.Sources[span.File]; ok {
		clamped, source := ClampSpan(span, &sf)
		line, _ := OffsetToLineCol(clamped.Start, source.LineOffsets)
		lineStart := source.LineOffsets[line-1]
		column := utf8.RuneCount(source.Source[lineStart:clamped.Start]) + 1
		return fmt.Sprintf("division by zero at %s:%d:%d\n", source.Path, line, column)
	}
	return fmt.Sprintf("division by zero at file-%d:%d\n", span.File, span.Start)
}

func (fb *fasmEmitter) emitDivisionZeroExit(ins *MIRInstr) {
	index := fb.divisionMessages[ins]
	message := fb.strings[index]
	fb.out.WriteString("    mov rax, 1\n")
	fb.out.WriteString("    mov rdi, 2\n")
	fmt.Fprintf(&fb.out, "    mov rsi, str%d\n", index)
	fmt.Fprintf(&fb.out, "    mov rdx, %d\n", len(message))
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    mov rax, 60\n")
	fb.out.WriteString("    mov rdi, 1\n")
	fb.out.WriteString("    syscall\n")
}

// boundsMessage renders the runtime out-of-bounds message for an array index.
func (fb *fasmEmitter) boundsMessage(span Span) string {
	if sf, ok := fb.prog.Sources[span.File]; ok {
		clamped, source := ClampSpan(span, &sf)
		line, _ := OffsetToLineCol(clamped.Start, source.LineOffsets)
		lineStart := source.LineOffsets[line-1]
		column := utf8.RuneCount(source.Source[lineStart:clamped.Start]) + 1
		return fmt.Sprintf("array index out of bounds at %s:%d:%d\n", source.Path, line, column)
	}
	return fmt.Sprintf("array index out of bounds at file-%d:%d\n", span.File, span.Start)
}

// emitBoundsExit writes the runtime out-of-bounds error and exits.
func (fb *fasmEmitter) emitBoundsExit(ins *MIRInstr) {
	index := fb.boundsMessages[ins]
	message := fb.strings[index]
	fb.out.WriteString("    mov rax, 1\n")
	fb.out.WriteString("    mov rdi, 2\n")
	fmt.Fprintf(&fb.out, "    mov rsi, str%d\n", index)
	fmt.Fprintf(&fb.out, "    mov rdx, %d\n", len(message))
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    mov rax, 60\n")
	fb.out.WriteString("    mov rdi, 1\n")
	fb.out.WriteString("    syscall\n")
}

func (fb *fasmEmitter) emitData() {
	if len(fb.prog.Globals) == 0 && len(fb.floatConsts) == 0 && len(fb.strings) == 0 && !fb.needsHeap {
		return
	}
	fb.out.WriteString("segment readable writable\n")
	for _, g := range fb.prog.Globals {
		size := fb.sizeOf(fb.prog.Types.Lookup(g.Type))
		fmt.Fprintf(&fb.out, "%s:\n    rb %d\n", fb.globalLabel(g.Symbol), size)
	}
	if fb.needsHeap {
		// A bounded static bump allocator. chaos_alloc is the only code that
		// advances chaos_heap_ptr and rejects negative, overflowing, or
		// out-of-arena requests before returning an address.
		fb.out.WriteString("chaos_heap:\n    rb 1048576\n")
		fb.out.WriteString("chaos_heap_end:\n")
		fb.out.WriteString("chaos_heap_ptr:\n    dq chaos_heap\n")
	}
	for i, fc := range fb.floatConsts {
		if fc.size == 4 {
			fmt.Fprintf(&fb.out, "fc%d:\n    dd %s\n", i, fasmFloat(fc.value))
		} else {
			fmt.Fprintf(&fb.out, "fc%d:\n    dq %s\n", i, fasmFloat(fc.value))
		}
	}
	for i, s := range fb.strings {
		fmt.Fprintf(&fb.out, "str%d:\n", i)
		if len(s) == 0 {
			// fasm rejects an empty db list; reserve one byte so the label
			// still has a defined address.
			fb.out.WriteString("    db 0\n")
			continue
		}
		fb.out.WriteString("    db ")
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

// validEntrySignature reports whether an entry procedure has the supported
// target signature: no parameters or (argc: S64, argv: *String), returning S64.
func (fb *fasmEmitter) validEntrySignature(fn *MIRFunction) bool {
	if len(fn.Results) != 1 || fn.Results[0] != fb.prog.Types.S64() {
		return false
	}
	if len(fn.Params) == 0 {
		return true
	}
	if len(fn.Params) != 2 {
		return false
	}
	if fn.LocalTypes[fn.Params[0]] != fb.prog.Types.S64() {
		return false
	}
	argvType := fb.prog.Types.Lookup(fn.LocalTypes[fn.Params[1]])
	return argvType.Kind == TypeKindPointer && argvType.Elem == fb.prog.Types.String()
}

// emitEntry emits the process entry wrapper: it runs the global initializers,
// builds the argument vector when the entry procedure takes (argc, argv),
// calls the '#entry' procedure, and exits with its result.
func (fb *fasmEmitter) emitEntry() {
	fb.out.WriteString("entry _start\n\n")
	fb.out.WriteString("_start:\n")
	if fb.prog.GlobalInit != nil {
		fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(fb.prog.GlobalInit.Symbol))
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
		if isRecordKind(rt.Kind) {
			fmt.Fprintf(&fb.out, "    sub rsp, %d\n", align16(fb.sizeOf(rt)))
			fb.out.WriteString("    mov rdi, rsp\n")
		}
	}
	if len(entryFn.Params) == 2 {
		// The entry procedure takes (argc: S64, argv: *String). Build a
		// String array from the process argument vector: argc is at [rsp]
		// and argv (char**) at rsp+8. Each String is (data, count) with the
		// count computed by strlen.
		fb.needsHeap = true
		fb.out.WriteString("    mov r12, qword [rsp]\n")
		fb.out.WriteString("    lea r13, [rsp+8]\n")
		// Allocate argc*16 bytes. Reject the multiplication before it can
		// wrap into an apparently small allocation.
		fb.out.WriteString("    mov rax, r12\n")
		fb.out.WriteString("    mov rdx, 576460752303423487\n")
		fb.out.WriteString("    cmp rax, rdx\n")
		fb.out.WriteString("    ja chaos_alloc_fail\n")
		fb.out.WriteString("    shl rax, 4\n")
		fb.out.WriteString("    call chaos_alloc\n")
		fb.out.WriteString("    mov r14, rax\n")
		// Build the array: arr[i] = (argv[i], strlen(argv[i])).
		fb.out.WriteString("    xor r15, r15\n")
		argvLoop := fb.newLabel()
		argvDone := fb.newLabel()
		strlenLoop := fb.newLabel()
		strlenDone := fb.newLabel()
		fmt.Fprintf(&fb.out, "%s:\n", argvLoop)
		fb.out.WriteString("    cmp r15, r12\n")
		fmt.Fprintf(&fb.out, "    jge %s\n", argvDone)
		fb.out.WriteString("    mov rbx, qword [r13+r15*8]\n")
		fb.out.WriteString("    mov rdi, rbx\n")
		fb.out.WriteString("    xor rcx, rcx\n")
		fmt.Fprintf(&fb.out, "%s:\n", strlenLoop)
		fb.out.WriteString("    cmp byte [rdi+rcx], 0\n")
		fmt.Fprintf(&fb.out, "    je %s\n", strlenDone)
		fb.out.WriteString("    inc rcx\n")
		fmt.Fprintf(&fb.out, "    jmp %s\n", strlenLoop)
		fmt.Fprintf(&fb.out, "%s:\n", strlenDone)
		fb.out.WriteString("    lea rdi, [r14+r15*8]\n")
		fb.out.WriteString("    lea rdi, [rdi+r15*8]\n")
		fb.out.WriteString("    mov qword [rdi], rbx\n")
		fb.out.WriteString("    mov qword [rdi+8], rcx\n")
		fb.out.WriteString("    inc r15\n")
		fmt.Fprintf(&fb.out, "    jmp %s\n", argvLoop)
		fmt.Fprintf(&fb.out, "%s:\n", argvDone)
		fb.out.WriteString("    mov rdi, r12\n")
		fb.out.WriteString("    mov rsi, r14\n")
	} else {
		// Zero the integer argument registers the entry procedure uses.
		intCount := 0
		for _, lid := range entryFn.Params {
			intCount += fb.intRegCount(fb.prog.Types.Lookup(entryFn.LocalTypes[lid]))
		}
		for i := 0; i < intCount && i < 6; i++ {
			fmt.Fprintf(&fb.out, "    xor %s, %s\n", intArgReg(i), intArgReg(i))
		}
	}
	fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(entryFn.Symbol))
	if len(entryFn.Results) > 0 {
		rt := fb.prog.Types.Lookup(entryFn.Results[0])
		switch {
		case isRecordKind(rt.Kind):
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

// emitHeapAllocator emits the single checked allocation path used by every
// compiler-generated heap request. The requested signed byte count arrives in
// rax and the allocation address is returned in rax. Allocation failure is a
// deterministic stderr diagnostic followed by exit status 1.
func (fb *fasmEmitter) emitHeapAllocator() {
	message := fb.strings[fb.allocFailIndex]
	fb.out.WriteString("chaos_alloc:\n")
	fb.out.WriteString("    test rax, rax\n")
	fb.out.WriteString("    js chaos_alloc_fail\n")
	fb.out.WriteString("    add rax, 7\n")
	fb.out.WriteString("    jc chaos_alloc_fail\n")
	fb.out.WriteString("    and rax, -8\n")
	fb.out.WriteString("    mov rcx, qword [chaos_heap_ptr]\n")
	fb.out.WriteString("    mov rdx, rcx\n")
	fb.out.WriteString("    add rdx, rax\n")
	fb.out.WriteString("    jc chaos_alloc_fail\n")
	fb.out.WriteString("    cmp rdx, chaos_heap_end\n")
	fb.out.WriteString("    ja chaos_alloc_fail\n")
	fb.out.WriteString("    mov qword [chaos_heap_ptr], rdx\n")
	fb.out.WriteString("    mov rax, rcx\n")
	fb.out.WriteString("    ret\n")
	fb.out.WriteString("chaos_alloc_fail:\n")
	fb.out.WriteString("    mov rax, 1\n")
	fb.out.WriteString("    mov rdi, 2\n")
	fmt.Fprintf(&fb.out, "    mov rsi, str%d\n", fb.allocFailIndex)
	fmt.Fprintf(&fb.out, "    mov rdx, %d\n", len(message))
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    mov rax, 60\n")
	fb.out.WriteString("    mov rdi, 1\n")
	fb.out.WriteString("    syscall\n\n")
}

func (fb *fasmEmitter) emitFunction(fn *MIRFunction) {
	fb.fn = fn
	fb.assignSlots(fn)
	fb.blockLabels = make(map[BlockID]string)
	for _, b := range fn.Blocks {
		fb.blockLabels[b.ID] = fb.newLabel()
	}
	fmt.Fprintf(&fb.out, "%s:\n", fb.funcLabel(fn.Symbol))
	fb.out.WriteString("    push rbp\n")
	fb.out.WriteString("    mov rbp, rsp\n")
	if fb.frameSize > 0 {
		fmt.Fprintf(&fb.out, "    sub rsp, %d\n", fb.frameSize)
	}
	for i, reg := range []string{"rbx", "r12", "r13", "r14", "r15"} {
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], %s\n", fb.calleeSaveSlots[i], reg)
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
	fb.arrayBuffers = make(map[ValueID]int)
	fb.nextOffset = 0
	fb.sretSlot = -1
	for i := range fb.calleeSaveSlots {
		fb.calleeSaveSlots[i] = fb.allocSlot(8, 8)
	}
	if len(fn.Results) > 0 && isRecordKind(fb.prog.Types.Lookup(fn.Results[0]).Kind) {
		fb.sretSlot = fb.allocSlot(8, 8)
	}
	for _, lid := range fn.Locals {
		t := fb.prog.Types.Lookup(fn.LocalTypes[lid])
		layout := fb.layoutOf(t)
		fb.localSlots[lid] = fb.allocSlot(layout.Size, layout.Align)
	}
	for _, b := range fn.Blocks {
		for _, ins := range b.Instrs {
			if ins.Result != NoValue {
				t := fb.prog.Types.Lookup(ins.Type)
				fb.valueTypes[ins.Result] = ins.Type
				layout := fb.layoutOf(t)
				fb.valueSlots[ins.Result] = fb.allocSlot(layout.Size, layout.Align)
				if ins.Op == MIRArrayInit && t.ArrayKind != ArrayDynamic {
					// Allocate the element buffer for a static/runtime array
					// literal. Dynamic arrays heap-allocate their buffer.
					et := fb.prog.Types.Lookup(t.Elem)
					elemSize := fb.sizeOf(et)
					count := len(ins.Args)
					if count == 0 {
						count = 1
					}
					fb.arrayBuffers[ins.Result] = fb.allocSlot(elemSize*count, fb.layoutOf(et).Align)
				}
			}
		}
	}
	fb.frameSize = align16(fb.nextOffset)
}

func (fb *fasmEmitter) allocSlot(size, alignment int) int {
	if size <= 0 {
		return fb.nextOffset
	}
	if alignment < 1 {
		alignment = 1
	}
	fb.nextOffset = align(fb.nextOffset, alignment)
	fb.nextOffset += size
	return fb.nextOffset
}

// emitParamMoves stores incoming arguments into their local slots. String
// and 128-bit parameters arrive in two integer registers (low/high or
// pointer/length); struct parameters arrive as an address and are copied.
// A struct-returning function receives its sret pointer in RDI.
func (fb *fasmEmitter) emitParamMoves(fn *MIRFunction) {
	if fb.sretSlot >= 0 {
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rdi\n", fb.sretSlot)
	}
	types := make([]IRType, len(fn.Params))
	for i, lid := range fn.Params {
		types[i] = fb.prog.Types.Lookup(fn.LocalTypes[lid])
	}
	locations, _ := fb.classifyABI(types, fb.sretSlot >= 0)
	for i, lid := range fn.Params {
		slot := fb.localSlots[lid]
		t := types[i]
		loc := locations[i]
		if loc.stackOffset >= 0 {
			source := fmt.Sprintf("[rbp+%d]", 16+loc.stackOffset)
			switch loc.kind {
			case argFloat:
				if t.Name == "F32" {
					fmt.Fprintf(&fb.out, "    movss xmm0, dword %s\n", source)
					fmt.Fprintf(&fb.out, "    movss dword [rbp-%d], xmm0\n", slot)
				} else {
					fmt.Fprintf(&fb.out, "    movsd xmm0, qword %s\n", source)
					fmt.Fprintf(&fb.out, "    movsd qword [rbp-%d], xmm0\n", slot)
				}
			case argString, argInt128:
				fmt.Fprintf(&fb.out, "    mov rax, qword %s\n", source)
				fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
				fmt.Fprintf(&fb.out, "    mov rax, qword [rbp+%d]\n", 24+loc.stackOffset)
				fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
			case argStruct:
				fmt.Fprintf(&fb.out, "    mov r10, qword %s\n", source)
				fb.emitCopyFromR10(slot, fb.sizeOf(t))
			default:
				fmt.Fprintf(&fb.out, "    mov rax, qword %s\n", source)
				fb.emitStore(t, slot)
			}
			continue
		}

		switch loc.kind {
		case argFloat:
			if t.Name == "F32" {
				fmt.Fprintf(&fb.out, "    movss dword [rbp-%d], xmm%d\n", slot, loc.floatReg)
			} else {
				fmt.Fprintf(&fb.out, "    movsd qword [rbp-%d], xmm%d\n", slot, loc.floatReg)
			}
		case argString:
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(loc.intReg))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(loc.intReg+1))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
		case argStruct:
			fmt.Fprintf(&fb.out, "    mov r10, %s\n", intArgReg(loc.intReg))
			fb.emitCopyFromR10(slot, fb.sizeOf(t))
		case argInt128:
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(loc.intReg))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(loc.intReg+1))
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
		default:
			fmt.Fprintf(&fb.out, "    mov rax, %s\n", intArgReg(loc.intReg))
			fb.emitStore(t, slot)
		}
	}
}

// emitCopyFromR10 copies size bytes from the address in R10 to a local slot
// without clobbering any System V argument registers.
func (fb *fasmEmitter) emitCopyFromR10(slot, size int) {
	offset := 0
	for size-offset >= 8 {
		fmt.Fprintf(&fb.out, "    mov rax, qword [r10+%d]\n", offset)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+%d], rax\n", slot, offset)
		offset += 8
	}
	if size-offset >= 4 {
		fmt.Fprintf(&fb.out, "    mov eax, dword [r10+%d]\n", offset)
		fmt.Fprintf(&fb.out, "    mov dword [rbp-%d+%d], eax\n", slot, offset)
		offset += 4
	}
	if size-offset >= 2 {
		fmt.Fprintf(&fb.out, "    mov ax, word [r10+%d]\n", offset)
		fmt.Fprintf(&fb.out, "    mov word [rbp-%d+%d], ax\n", slot, offset)
		offset += 2
	}
	if size-offset == 1 {
		fmt.Fprintf(&fb.out, "    mov al, byte [r10+%d]\n", offset)
		fmt.Fprintf(&fb.out, "    mov byte [rbp-%d+%d], al\n", slot, offset)
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
	case MIRZero:
		fb.emitZero(ins)
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
	case MIRFieldLoad:
		fb.emitFieldLoad(ins)
	case MIRArrayInit:
		fb.emitArrayInit(ins)
	case MIRArrayLen:
		fb.emitArrayLen(ins)
	case MIRArrayIndex:
		fb.emitArrayIndex(ins)
	case MIRAddrOf:
		fb.emitAddrOf(ins)
	case MIRDerefLoad:
		fb.emitDerefLoad(ins)
	case MIRDerefStore:
		fb.emitDerefStore(ins)
	case MIRArrayElemAddr:
		fb.emitArrayElemAddr(ins)
	case MIRFieldAddr:
		fb.emitFieldAddr(ins)
	case MIRPtrAdd:
		fb.emitPtrArith(ins, false, false)
	case MIRPtrSub:
		fb.emitPtrArith(ins, true, false)
	case MIRPtrDiff:
		fb.emitPtrArith(ins, true, true)
	case MIRAllocate:
		fb.emitAllocate(ins)
	case MIRDeallocate:
		fb.emitDeallocate(ins)
	case MIRCast:
		fb.emitCast(ins)
	case MIRConvert:
		fb.emitConvert(ins)
	case MIRInterpolate:
		fb.emitInterpolate(ins)
	case MIRPrint:
		fb.emitPrint(ins)
	case MIRReadFile:
		fb.emitReadFile(ins)
	case MIRFileExists:
		fb.emitFileExists(ins)
	default:
		fb.diags.Error(ins.Span, "unsupported MIR opcode reached the fasm backend", "verify MIR before code generation")
	}
}

func (fb *fasmEmitter) emitZero(ins *MIRInstr) {
	slot := fb.valueSlots[ins.Result]
	size := fb.sizeOf(fb.prog.Types.Lookup(ins.Type))
	if size <= 0 {
		return
	}
	fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", slot)
	fb.out.WriteString("    xor eax, eax\n")
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", size)
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    rep stosb\n")
}

func (fb *fasmEmitter) emitConst(ins *MIRInstr) {
	slot := fb.valueSlots[ins.Result]
	t := fb.prog.Types.Lookup(ins.Type)
	switch ins.Imm.Kind {
	case MIRImmInt:
		if fb.isInt128Storage(t) {
			// Preserve the compact legacy sequence for ordinary 128-bit literals
			// that fit int64. Enum constants and genuinely wide literals need the
			// exact two-word path below.
			if t.Kind == TypeKindInt {
				raw := strings.ReplaceAll(ins.Imm.Str, "_", "")
				if raw == "" {
					raw = strconv.FormatInt(ins.Imm.Int, 10)
				}
				if _, err := strconv.ParseInt(raw, 10, 64); err == nil {
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
			}
			lo, hi := integerImmediateWords(ins.Imm.Str, ins.Imm.Int, fb.sizeOf(t)*8)
			// Store the complete fixed-width two's-complement representation.
			fmt.Fprintf(&fb.out, "    mov rax, 0x%x\n", lo)
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", slot)
			fmt.Fprintf(&fb.out, "    mov rax, 0x%x\n", hi)
			fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", slot)
			return
		}
		literal := ins.Imm.Str
		if literal == "" {
			literal = strconv.FormatInt(ins.Imm.Int, 10)
		}
		fmt.Fprintf(&fb.out, "    mov rax, %s\n", literal)
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
	if fb.isAggregate(t) {
		fb.emitAggCopy(fb.localSlots[ins.Imm.Local], fb.valueSlots[ins.Result], fb.sizeOf(t))
		return
	}
	fb.emitLoad(t, fb.localSlots[ins.Imm.Local])
	fb.emitStore(t, fb.valueSlots[ins.Result])
}

func (fb *fasmEmitter) emitStoreLocal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	if fb.isAggregate(t) {
		fb.emitAggCopy(fb.valueSlots[ins.Args[0]], fb.localSlots[ins.Imm.Local], fb.sizeOf(t))
		return
	}
	fb.emitLoad(t, fb.valueSlots[ins.Args[0]])
	fb.emitStore(t, fb.localSlots[ins.Imm.Local])
}

func (fb *fasmEmitter) emitLoadGlobal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	name := fb.globalLabel(ins.Imm.Symbol)
	if fb.isAggregate(t) {
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
		size := fb.sizeOf(t)
		switch size {
		case 1:
			if fb.isSignedStorage(t) {
				fmt.Fprintf(&fb.out, "    movsx rax, byte [%s]\n", name)
			} else {
				fmt.Fprintf(&fb.out, "    movzx rax, byte [%s]\n", name)
			}
		case 2:
			if fb.isSignedStorage(t) {
				fmt.Fprintf(&fb.out, "    movsx rax, word [%s]\n", name)
			} else {
				fmt.Fprintf(&fb.out, "    movzx rax, word [%s]\n", name)
			}
		case 4:
			if fb.isSignedStorage(t) {
				fmt.Fprintf(&fb.out, "    movsxd rax, dword [%s]\n", name)
			} else {
				fmt.Fprintf(&fb.out, "    mov eax, dword [%s]\n", name)
			}
		default:
			fmt.Fprintf(&fb.out, "    mov rax, qword [%s]\n", name)
		}
	}
	fb.emitStore(t, fb.valueSlots[ins.Result])
}

func (fb *fasmEmitter) emitStoreGlobal(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	name := fb.globalLabel(ins.Imm.Symbol)
	if fb.isAggregate(t) {
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
		size := fb.sizeOf(t)
		fmt.Fprintf(&fb.out, "    mov %s [%s], %s\n", memSize(size), name, widthReg(size))
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
		if ins.Op == MIRDiv {
			nonzero := fb.newLabel()
			if t.Name == "F32" {
				fmt.Fprintf(&fb.out, "    movss xmm1, dword [rbp-%d]\n", rslot)
				fb.out.WriteString("    xorps xmm2, xmm2\n")
				fb.out.WriteString("    ucomiss xmm1, xmm2\n")
			} else {
				fmt.Fprintf(&fb.out, "    movsd xmm1, qword [rbp-%d]\n", rslot)
				fb.out.WriteString("    xorpd xmm2, xmm2\n")
				fb.out.WriteString("    ucomisd xmm1, xmm2\n")
			}
			fmt.Fprintf(&fb.out, "    jp %s\n", nonzero)
			fmt.Fprintf(&fb.out, "    jne %s\n", nonzero)
			fb.emitDivisionZeroExit(ins)
			fmt.Fprintf(&fb.out, "%s:\n", nonzero)
			fb.emitLoad(t, lslot)
		}
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
	nonzero := fb.newLabel()
	fb.out.WriteString("    mov rax, r10\n")
	fb.out.WriteString("    or rax, r11\n")
	fmt.Fprintf(&fb.out, "    jnz %s\n", nonzero)
	fb.emitDivisionZeroExit(ins)
	fmt.Fprintf(&fb.out, "%s:\n", nonzero)
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
	nonzero := fb.newLabel()
	fb.out.WriteString("    test rcx, rcx\n")
	fmt.Fprintf(&fb.out, "    jnz %s\n", nonzero)
	fb.emitDivisionZeroExit(ins)
	fmt.Fprintf(&fb.out, "%s:\n", nonzero)
	if size == 8 && isSignedInt(t.Name) {
		normal := fb.newLabel()
		fmt.Fprintf(&fb.out, "    mov r8, 0x8000000000000000\n")
		fb.out.WriteString("    cmp rax, r8\n")
		fmt.Fprintf(&fb.out, "    jne %s\n", normal)
		fb.out.WriteString("    cmp rcx, -1\n")
		fmt.Fprintf(&fb.out, "    jne %s\n", normal)
		if ins.Op == MIRMod {
			fb.out.WriteString("    xor eax, eax\n")
		} else {
			fb.out.WriteString("    mov rax, r8\n")
		}
		fb.emitStore(t, resSlot)
		returnLabel := fb.newLabel()
		fmt.Fprintf(&fb.out, "    jmp %s\n", returnLabel)
		fmt.Fprintf(&fb.out, "%s:\n", normal)
		if isSignedInt(t.Name) {
			fb.out.WriteString("    cqo\n")
			fb.out.WriteString("    idiv rcx\n")
		}
		if ins.Op == MIRMod {
			fb.out.WriteString("    mov rax, rdx\n")
		}
		fb.emitStore(t, resSlot)
		fmt.Fprintf(&fb.out, "%s:\n", returnLabel)
		return
	}
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
	if isRecordKind(lt.Kind) || lt.Kind == TypeKindArray {
		if ins.Op != MIRCmpEq && ins.Op != MIRCmpNeq {
			fb.diags.Error(ins.Span, "aggregate ordering reached the fasm backend", "use == or != for aggregate values")
			return
		}
		fb.requestEqualityHelper(lt.ID)
		fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", lslot)
		fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", rslot)
		fmt.Fprintf(&fb.out, "    call %s\n", fb.equalityLabel(lt.ID))
		if ins.Op == MIRCmpNeq {
			fb.out.WriteString("    xor al, 1\n")
		}
		fmt.Fprintf(&fb.out, "    mov byte [rbp-%d], al\n", resSlot)
		return
	}
	if fb.isInt128Storage(lt) {
		fb.emitCmp128(ins, lt, lslot, rslot, resSlot)
		return
	}
	if lt.Kind == TypeKindFloat {
		fb.emitLoad(lt, lslot)
		if lt.Name == "F32" {
			fmt.Fprintf(&fb.out, "    ucomiss xmm0, dword [rbp-%d]\n", rslot)
		} else {
			fmt.Fprintf(&fb.out, "    ucomisd xmm0, qword [rbp-%d]\n", rslot)
		}
		fb.emitFloatSetcc(ins.Op, resSlot)
		return
	}
	size := fb.sizeOf(lt)
	fb.emitLoad(lt, lslot)
	fmt.Fprintf(&fb.out, "    cmp %s, %s [rbp-%d]\n", widthReg(size), memSize(size), rslot)
	fb.emitSetcc(ins.Op, fb.isSignedStorage(lt), resSlot)
}

// emitCmp128 emits a 128-bit signed or unsigned comparison by comparing the
// high halves first, then the low halves when they are equal.
func (fb *fasmEmitter) emitCmp128(ins *MIRInstr, t IRType, lslot, rslot, resSlot int) {
	fmt.Fprintf(&fb.out, "    mov r8, qword [rbp-%d]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov r9, qword [rbp-%d+8]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov r10, qword [rbp-%d]\n", rslot)
	fmt.Fprintf(&fb.out, "    mov r11, qword [rbp-%d+8]\n", rslot)
	fb.out.WriteString("    cmp r9, r11\n")
	less, greater, lessEq, greaterEq := cmpBranches(fb.isSignedStorage(t))
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

// emitStringCmp compares two String values (pointer and length pairs)
// byte-wise, then by length when the common prefix is equal. The final
// comparison flags describe the unsigned ordering (CF set when the left side
// is less, ZF set when equal), so emitSetcc can compute any of the six
// comparison operators.
func (fb *fasmEmitter) emitStringCmp(ins *MIRInstr, lslot, rslot, resSlot int) {
	loopLabel := fb.newLabel()
	lenLabel := fb.newLabel()
	doneLabel := fb.newLabel()
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d+8]\n", lslot)
	fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d]\n", rslot)
	fmt.Fprintf(&fb.out, "    mov r8, qword [rbp-%d+8]\n", rslot)
	fb.out.WriteString("    xor r9, r9\n")
	fmt.Fprintf(&fb.out, "%s:\n", loopLabel)
	fb.out.WriteString("    cmp r9, rcx\n")
	fmt.Fprintf(&fb.out, "    jae %s\n", lenLabel)
	fb.out.WriteString("    cmp r9, r8\n")
	fmt.Fprintf(&fb.out, "    jae %s\n", lenLabel)
	fb.out.WriteString("    mov r10b, byte [rax+r9]\n")
	fb.out.WriteString("    cmp r10b, byte [rdx+r9]\n")
	fmt.Fprintf(&fb.out, "    jne %s\n", doneLabel)
	fb.out.WriteString("    inc r9\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", loopLabel)
	fmt.Fprintf(&fb.out, "%s:\n", lenLabel)
	fb.out.WriteString("    cmp rcx, r8\n")
	fmt.Fprintf(&fb.out, "%s:\n", doneLabel)
	fb.emitSetcc(ins.Op, false, resSlot)
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

// emitFloatSetcc implements ordered IEEE comparisons. Every ordering and
// equality comparison involving NaN is false; inequality involving NaN is
// true. UCOMIS* exposes the unordered state through PF.
func (fb *fasmEmitter) emitFloatSetcc(op MIROpcode, resSlot int) {
	if op == MIRCmpNeq {
		fb.out.WriteString("    setp dl\n")
		fb.out.WriteString("    setne al\n")
		fb.out.WriteString("    or al, dl\n")
		fmt.Fprintf(&fb.out, "    mov byte [rbp-%d], al\n", resSlot)
		return
	}
	cc := "sete"
	switch op {
	case MIRCmpLt:
		cc = "setb"
	case MIRCmpGt:
		cc = "seta"
	case MIRCmpLe:
		cc = "setbe"
	case MIRCmpGe:
		cc = "setae"
	}
	fb.out.WriteString("    setnp dl\n")
	fmt.Fprintf(&fb.out, "    %s al\n", cc)
	fb.out.WriteString("    and al, dl\n")
	fmt.Fprintf(&fb.out, "    mov byte [rbp-%d], al\n", resSlot)
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
	if isInt128(t.Name) {
		// 128-bit two's complement negation: negate the low half, propagate
		// the borrow into the high half, then negate the high half.
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
		fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d+8]\n", slot)
		fb.out.WriteString("    neg rax\n")
		fb.out.WriteString("    adc rdx, 0\n")
		fb.out.WriteString("    neg rdx\n")
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rdx\n", resSlot)
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
	returnsStruct := isRecordKind(resultType.Kind)
	argTypes := make([]IRType, len(ins.Args))
	for i, arg := range ins.Args {
		argTypes[i] = fb.prog.Types.Lookup(fb.valueTypes[arg])
	}
	locations, stackSize := fb.classifyABI(argTypes, returnsStruct)
	if stackSize > 0 {
		fmt.Fprintf(&fb.out, "    sub rsp, %d\n", stackSize)
	}

	// Materialize stack arguments before assigning registers. Source values
	// have stable RBP-relative addresses, so reserving outgoing stack space
	// cannot invalidate them.
	for i, arg := range ins.Args {
		loc := locations[i]
		if loc.stackOffset < 0 {
			continue
		}
		t := argTypes[i]
		slot := fb.valueSlots[arg]
		switch loc.kind {
		case argFloat:
			fb.emitLoad(t, slot)
			if t.Name == "F32" {
				fmt.Fprintf(&fb.out, "    movss dword [rsp+%d], xmm0\n", loc.stackOffset)
			} else {
				fmt.Fprintf(&fb.out, "    movsd qword [rsp+%d], xmm0\n", loc.stackOffset)
			}
		case argString, argInt128:
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov qword [rsp+%d], rax\n", loc.stackOffset)
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", slot)
			fmt.Fprintf(&fb.out, "    mov qword [rsp+%d], rax\n", loc.stackOffset+8)
		case argStruct:
			fmt.Fprintf(&fb.out, "    lea rax, [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov qword [rsp+%d], rax\n", loc.stackOffset)
		default:
			fb.emitLoad(t, slot)
			fmt.Fprintf(&fb.out, "    mov qword [rsp+%d], rax\n", loc.stackOffset)
		}
	}

	// Pass the result slot address for struct-returning callees.
	if returnsStruct {
		fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", fb.valueSlots[ins.Result])
	}

	for i := len(ins.Args) - 1; i >= 0; i-- {
		loc := locations[i]
		if loc.stackOffset >= 0 {
			continue
		}
		arg := ins.Args[i]
		t := argTypes[i]
		slot := fb.valueSlots[arg]
		switch loc.kind {
		case argFloat:
			fb.emitLoad(t, slot)
			if t.Name == "F32" {
				fmt.Fprintf(&fb.out, "    movss xmm%d, xmm0\n", loc.floatReg)
			} else {
				fmt.Fprintf(&fb.out, "    movsd xmm%d, xmm0\n", loc.floatReg)
			}
		case argString:
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(loc.intReg))
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(loc.intReg+1))
		case argStruct:
			fmt.Fprintf(&fb.out, "    lea rax, [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(loc.intReg))
		case argInt128:
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(loc.intReg))
			fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(loc.intReg+1))
		case argInt:
			fb.emitLoad(t, slot)
			fmt.Fprintf(&fb.out, "    mov %s, rax\n", intArgReg(loc.intReg))
		}
	}
	fmt.Fprintf(&fb.out, "    call %s\n", fb.funcLabel(callee.Symbol))
	if stackSize > 0 {
		fmt.Fprintf(&fb.out, "    add rsp, %d\n", stackSize)
	}

	// Store the result. Struct results were written into the sret slot by the
	// callee; String and 128-bit results arrive in RAX:RDX.
	if ins.Type != fb.prog.Types.Void() {
		slot := fb.valueSlots[ins.Result]
		switch {
		case isRecordKind(resultType.Kind):
			// Already in the sret slot.
		case resultType.Kind == TypeKindString || resultType.Kind == TypeKindArray || fb.isInt128Storage(resultType):
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

type abiArgLocation struct {
	kind        argKind
	intReg      int
	floatReg    int
	stackOffset int
}

func (fb *fasmEmitter) equalityLabel(id TypeID) string {
	return "chaos_eq_" + strconv.FormatUint(uint64(id), 10)
}

func (fb *fasmEmitter) requestEqualityHelper(id TypeID) {
	if fb.equalityTypes == nil {
		fb.equalityTypes = make(map[TypeID]bool)
	}
	if fb.equalityTypes[id] {
		return
	}
	fb.equalityTypes[id] = true
	t := fb.prog.Types.Lookup(id)
	switch t.Kind {
	case TypeKindStruct, TypeKindTuple:
		for _, field := range t.Fields {
			fb.requestEqualityHelper(field.Type)
		}
	case TypeKindArray:
		fb.requestEqualityHelper(t.Elem)
	}
}

func (fb *fasmEmitter) emitEqualityHelpers() {
	ids := make([]int, 0, len(fb.equalityTypes))
	for id := range fb.equalityTypes {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, raw := range ids {
		id := TypeID(raw)
		t := fb.prog.Types.Lookup(id)
		fmt.Fprintf(&fb.out, "%s:\n", fb.equalityLabel(id))
		switch t.Kind {
		case TypeKindString:
			fb.emitStringEqualityHelper()
		case TypeKindArray:
			fb.emitArrayEqualityHelper(t)
		case TypeKindStruct, TypeKindTuple:
			fb.emitRecordEqualityHelper(t)
		case TypeKindFloat:
			if t.Name == "F32" {
				fb.out.WriteString("    movss xmm0, dword [rdi]\n")
				fb.out.WriteString("    ucomiss xmm0, dword [rsi]\n")
			} else {
				fb.out.WriteString("    movsd xmm0, qword [rdi]\n")
				fb.out.WriteString("    ucomisd xmm0, qword [rsi]\n")
			}
			fb.out.WriteString("    setnp dl\n")
			fb.out.WriteString("    sete al\n")
			fb.out.WriteString("    and al, dl\n")
			fb.out.WriteString("    ret\n\n")
		default:
			storage := fb.storageType(t)
			if fb.isInt128Storage(t) {
				falseLabel := fb.newLabel()
				doneLabel := fb.newLabel()
				fb.out.WriteString("    mov rax, qword [rdi]\n")
				fb.out.WriteString("    cmp rax, qword [rsi]\n")
				fmt.Fprintf(&fb.out, "    jne %s\n", falseLabel)
				fb.out.WriteString("    mov rax, qword [rdi+8]\n")
				fb.out.WriteString("    cmp rax, qword [rsi+8]\n")
				fmt.Fprintf(&fb.out, "    jne %s\n", falseLabel)
				fb.out.WriteString("    mov al, 1\n")
				fmt.Fprintf(&fb.out, "    jmp %s\n", doneLabel)
				fmt.Fprintf(&fb.out, "%s:\n", falseLabel)
				fb.out.WriteString("    xor eax, eax\n")
				fmt.Fprintf(&fb.out, "%s:\n", doneLabel)
				fb.out.WriteString("    ret\n\n")
				continue
			}
			size := fb.sizeOf(storage)
			fmt.Fprintf(&fb.out, "    mov %s, %s [rdi]\n", widthReg(size), memSize(size))
			fmt.Fprintf(&fb.out, "    cmp %s, %s [rsi]\n", widthReg(size), memSize(size))
			fb.out.WriteString("    sete al\n")
			fb.out.WriteString("    ret\n\n")
		}
	}
}

func (fb *fasmEmitter) emitStringEqualityHelper() {
	loopLabel := fb.newLabel()
	falseLabel := fb.newLabel()
	trueLabel := fb.newLabel()
	fb.out.WriteString("    mov rcx, qword [rdi+8]\n")
	fb.out.WriteString("    cmp rcx, qword [rsi+8]\n")
	fmt.Fprintf(&fb.out, "    jne %s\n", falseLabel)
	fb.out.WriteString("    mov r8, qword [rdi]\n")
	fb.out.WriteString("    mov r9, qword [rsi]\n")
	fb.out.WriteString("    xor rdx, rdx\n")
	fmt.Fprintf(&fb.out, "%s:\n", loopLabel)
	fb.out.WriteString("    cmp rdx, rcx\n")
	fmt.Fprintf(&fb.out, "    jae %s\n", trueLabel)
	fb.out.WriteString("    mov al, byte [r8+rdx]\n")
	fb.out.WriteString("    cmp al, byte [r9+rdx]\n")
	fmt.Fprintf(&fb.out, "    jne %s\n", falseLabel)
	fb.out.WriteString("    inc rdx\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", loopLabel)
	fmt.Fprintf(&fb.out, "%s:\n", trueLabel)
	fb.out.WriteString("    mov al, 1\n")
	fb.out.WriteString("    ret\n")
	fmt.Fprintf(&fb.out, "%s:\n", falseLabel)
	fb.out.WriteString("    xor eax, eax\n")
	fb.out.WriteString("    ret\n\n")
}

func (fb *fasmEmitter) emitRecordEqualityHelper(t IRType) {
	falseLabel := fb.newLabel()
	doneLabel := fb.newLabel()
	fb.out.WriteString("    push rbp\n")
	fb.out.WriteString("    mov rbp, rsp\n")
	fb.out.WriteString("    push rbx\n")
	fb.out.WriteString("    push r12\n")
	fb.out.WriteString("    mov rbx, rdi\n")
	fb.out.WriteString("    mov r12, rsi\n")
	offsets := fb.structFieldOffsets(t)
	for i, field := range t.Fields {
		fmt.Fprintf(&fb.out, "    lea rdi, [rbx+%d]\n", offsets[i])
		fmt.Fprintf(&fb.out, "    lea rsi, [r12+%d]\n", offsets[i])
		fmt.Fprintf(&fb.out, "    call %s\n", fb.equalityLabel(field.Type))
		fb.out.WriteString("    test al, al\n")
		fmt.Fprintf(&fb.out, "    jz %s\n", falseLabel)
	}
	fb.out.WriteString("    mov al, 1\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", doneLabel)
	fmt.Fprintf(&fb.out, "%s:\n", falseLabel)
	fb.out.WriteString("    xor eax, eax\n")
	fmt.Fprintf(&fb.out, "%s:\n", doneLabel)
	fb.out.WriteString("    pop r12\n")
	fb.out.WriteString("    pop rbx\n")
	fb.out.WriteString("    leave\n")
	fb.out.WriteString("    ret\n\n")
}

func (fb *fasmEmitter) emitArrayEqualityHelper(t IRType) {
	loopLabel := fb.newLabel()
	falseLabel := fb.newLabel()
	trueLabel := fb.newLabel()
	doneLabel := fb.newLabel()
	elemSize := fb.sizeOf(fb.prog.Types.Lookup(t.Elem))
	if elemSize <= 0 {
		fb.diags.Error(Span{}, "array equality reached an element type with no storage", "use a storable array element type")
		elemSize = 1
	}
	fb.out.WriteString("    push rbp\n")
	fb.out.WriteString("    mov rbp, rsp\n")
	fb.out.WriteString("    push rbx\n")
	fb.out.WriteString("    push r12\n")
	fb.out.WriteString("    push r13\n")
	fb.out.WriteString("    push r14\n")
	fb.out.WriteString("    mov r13, qword [rdi+8]\n")
	fb.out.WriteString("    cmp r13, qword [rsi+8]\n")
	fmt.Fprintf(&fb.out, "    jne %s\n", falseLabel)
	fb.out.WriteString("    mov rbx, qword [rdi]\n")
	fb.out.WriteString("    mov r12, qword [rsi]\n")
	fb.out.WriteString("    xor r14, r14\n")
	fmt.Fprintf(&fb.out, "%s:\n", loopLabel)
	fb.out.WriteString("    test r13, r13\n")
	fmt.Fprintf(&fb.out, "    jz %s\n", trueLabel)
	fb.out.WriteString("    lea rdi, [rbx+r14]\n")
	fb.out.WriteString("    lea rsi, [r12+r14]\n")
	fmt.Fprintf(&fb.out, "    call %s\n", fb.equalityLabel(t.Elem))
	fb.out.WriteString("    test al, al\n")
	fmt.Fprintf(&fb.out, "    jz %s\n", falseLabel)
	fmt.Fprintf(&fb.out, "    add r14, %d\n", elemSize)
	fb.out.WriteString("    dec r13\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", loopLabel)
	fmt.Fprintf(&fb.out, "%s:\n", trueLabel)
	fb.out.WriteString("    mov al, 1\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", doneLabel)
	fmt.Fprintf(&fb.out, "%s:\n", falseLabel)
	fb.out.WriteString("    xor eax, eax\n")
	fmt.Fprintf(&fb.out, "%s:\n", doneLabel)
	fb.out.WriteString("    pop r14\n")
	fb.out.WriteString("    pop r13\n")
	fb.out.WriteString("    pop r12\n")
	fb.out.WriteString("    pop rbx\n")
	fb.out.WriteString("    leave\n")
	fb.out.WriteString("    ret\n\n")
}

// classifyABI gives callers and callees one authoritative description of the
// compiler's internal x86-64 calling convention. Multi-register values remain
// whole: if all of their registers are unavailable, the complete value is
// placed in an aligned outgoing stack slot.
func (fb *fasmEmitter) classifyABI(types []IRType, sret bool) ([]abiArgLocation, int) {
	locations := make([]abiArgLocation, len(types))
	intIdx, floatIdx, stackSize := 0, 0, 0
	if sret {
		intIdx = 1
	}
	for i, t := range types {
		loc := abiArgLocation{intReg: -1, floatReg: -1, stackOffset: -1}
		intUnits, stackBytes := 1, 8
		switch {
		case t.Kind == TypeKindFloat:
			loc.kind = argFloat
			if floatIdx < 8 {
				loc.floatReg = floatIdx
				floatIdx++
			} else {
				loc.stackOffset = stackSize
				stackSize += 8
			}
			locations[i] = loc
			continue
		case t.Kind == TypeKindArray && t.ArrayKind == ArrayDynamic:
			// A dynamic array is three words. Pass it by address like other
			// aggregates so caller and callee copy the complete value.
			loc.kind = argStruct
		case t.Kind == TypeKindString || t.Kind == TypeKindArray:
			loc.kind, intUnits, stackBytes = argString, 2, 16
		case isRecordKind(t.Kind):
			loc.kind = argStruct
		case fb.isInt128Storage(t):
			loc.kind, intUnits, stackBytes = argInt128, 2, 16
		default:
			loc.kind = argInt
		}
		if intIdx+intUnits <= 6 {
			loc.intReg = intIdx
			intIdx += intUnits
		} else {
			stackSize = align(stackSize, 8)
			loc.stackOffset = stackSize
			stackSize += stackBytes
		}
		locations[i] = loc
	}
	return locations, align16(stackSize)
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
			slot := fb.valueSlots[t.Value]
			switch {
			case isRecordKind(vt.Kind):
				// Copy the struct value to the sret address.
				fb.emitAggCopyToAddrSlot(slot, fb.sretSlot, fb.sizeOf(vt))
			case vt.Kind == TypeKindString || vt.Kind == TypeKindArray || fb.isInt128Storage(vt):
				fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", slot)
				fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d+8]\n", slot)
			default:
				fb.emitLoad(vt, slot)
			}
		}
		for i, reg := range []string{"rbx", "r12", "r13", "r14", "r15"} {
			fmt.Fprintf(&fb.out, "    mov %s, qword [rbp-%d]\n", reg, fb.calleeSaveSlots[i])
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
	default:
		fb.diags.Error(t.Span, "unsupported MIR terminator reached the fasm backend", "verify MIR before code generation")
	}
}

func (fb *fasmEmitter) layoutOf(t IRType) Layout {
	return fb.prog.Types.LayoutOf(t.ID)
}

func (fb *fasmEmitter) sizeOf(t IRType) int { return fb.layoutOf(t).Size }

// structFieldOffsets returns the byte offset of each field in a struct type.
func (fb *fasmEmitter) structFieldOffsets(t IRType) []int {
	return fb.layoutOf(t).FieldOffsets
}

// storageType resolves a nominal enum to the fixed-width integer type that
// backs it. Other types are returned unchanged.
func (fb *fasmEmitter) storageType(t IRType) IRType {
	for t.Kind == TypeKindEnum {
		t = fb.prog.Types.Lookup(t.Underlying)
	}
	return t
}

func (fb *fasmEmitter) isSignedStorage(t IRType) bool {
	return isSignedInt(fb.storageType(t).Name)
}

func (fb *fasmEmitter) isInt128Storage(t IRType) bool {
	return isInt128(fb.storageType(t).Name)
}

// isAggregate reports whether a type occupies more than one register-sized
// piece and must be copied as a byte range.
func (fb *fasmEmitter) isAggregate(t IRType) bool {
	switch {
	case t.Kind == TypeKindString:
		return true
	case t.Kind == TypeKindArray:
		return true
	case isRecordKind(t.Kind):
		return true
	case fb.isInt128Storage(t):
		return true
	}
	return false
}

func isRecordKind(kind TypeKind) bool { return kind == TypeKindStruct || kind == TypeKindTuple }

// isInt128 reports whether a type name is a 128-bit integer.
func isInt128(name string) bool {
	return name == "S128" || name == "U128"
}

// integerImmediateWords converts an already type-checked decimal literal to
// its fixed-width two's-complement representation. Arbitrary precision is
// used only here at compile time; the emitted value still has exactly bits
// bits of storage.
func integerImmediateWords(raw string, fallback int64, bits int) (uint64, uint64) {
	var value *big.Int
	if raw != "" {
		value, _ = new(big.Int).SetString(strings.ReplaceAll(raw, "_", ""), 10)
	}
	if value == nil {
		value = big.NewInt(fallback)
	}
	if bits <= 0 {
		bits = 64
	}
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	encoded := new(big.Int).Mod(new(big.Int).Set(value), modulus)
	low := encoded.Uint64()
	high := new(big.Int).Rsh(encoded, 64).Uint64()
	return low, high
}

// intRegCount returns the number of integer argument registers a type
// consumes when passed by value.
func (fb *fasmEmitter) intRegCount(t IRType) int {
	switch {
	case t.Kind == TypeKindString:
		return 2
	case t.Kind == TypeKindArray:
		if t.ArrayKind == ArrayDynamic {
			return 1 // passed by address
		}
		return 2
	case isRecordKind(t.Kind):
		return 1 // passed by address
	case fb.isInt128Storage(t):
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
		if fb.isAggregate(ft) {
			fb.emitAggCopy(fb.valueSlots[a], dstOffset, fb.sizeOf(ft))
		} else {
			fb.emitLoad(ft, fb.valueSlots[a])
			fb.emitStore(ft, dstOffset)
		}
	}
}

// emitFieldLoad loads one field of a struct value into the result slot.
func (fb *fasmEmitter) emitFieldLoad(ins *MIRInstr) {
	baseType := fb.prog.Types.Lookup(fb.valueTypes[ins.Args[0]])
	offsets := fb.structFieldOffsets(baseType)
	fieldIdx := int(ins.Imm.Int)
	fieldCount := len(baseType.Fields)
	if baseType.Kind == TypeKindArray {
		if baseType.ArrayKind == ArrayDynamic {
			fieldCount = 3
		} else {
			fieldCount = 2
		}
	}
	if fieldIdx < 0 || fieldIdx >= fieldCount {
		return
	}
	var ft IRType
	if baseType.Kind == TypeKindArray {
		switch fieldIdx {
		case 0:
			ft = fb.prog.Types.Lookup(fb.prog.Types.InternPointer(baseType.Elem, false))
		case 1, 2:
			ft = fb.prog.Types.Lookup(fb.prog.Types.Size())
		}
	} else {
		ft = fb.prog.Types.Lookup(baseType.Fields[fieldIdx].Type)
	}
	resSlot := fb.valueSlots[ins.Result]
	srcSlot := fb.valueSlots[ins.Args[0]]
	if fb.isAggregate(ft) {
		// Copy the field's bytes as data (for example a string's pointer and
		// length pair), not the value they point to.
		fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", srcSlot-offsets[fieldIdx])
		fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov rcx, %d\n", fb.sizeOf(ft))
		fb.out.WriteString("    cld\n")
		fb.out.WriteString("    rep movsb\n")
		return
	}
	fb.emitLoad(ft, srcSlot-offsets[fieldIdx])
	fb.emitStore(ft, resSlot)
}

// emitArrayInit builds an array value from its element values. The element
// buffer was allocated in assignSlots; each element is stored at its offset
// and the value slot receives the buffer address and element count. Dynamic
// arrays heap-allocate their buffer and set data/count/capacity.
func (fb *fasmEmitter) emitArrayInit(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	et := fb.prog.Types.Lookup(t.Elem)
	elemSize := fb.sizeOf(et)
	resSlot := fb.valueSlots[ins.Result]
	if t.ArrayKind == ArrayDynamic {
		count := len(ins.Args)
		if count == 0 {
			count = 1
		}
		// Allocate a heap buffer of count*elemSize bytes; keep its address in
		// r10 so element stores do not clobber it.
		fmt.Fprintf(&fb.out, "    mov rax, %d\n", count*elemSize)
		fb.out.WriteString("    call chaos_alloc\n")
		fb.out.WriteString("    mov r10, rax\n")
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot) // data
		for i, a := range ins.Args {
			dst := i * elemSize
			if fb.isAggregate(et) {
				fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", fb.valueSlots[a])
				fmt.Fprintf(&fb.out, "    lea rdi, [r10+%d]\n", dst)
				fmt.Fprintf(&fb.out, "    mov rcx, %d\n", elemSize)
				fb.out.WriteString("    cld\n")
				fb.out.WriteString("    rep movsb\n")
			} else {
				fb.emitLoad(et, fb.valueSlots[a])
				fb.emitStoreToReg(et, fmt.Sprintf("r10+%d", dst))
			}
		}
		fmt.Fprintf(&fb.out, "    mov rax, %d\n", len(ins.Args))
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", resSlot) // count
		fmt.Fprintf(&fb.out, "    mov rax, %d\n", count)
		fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+16], rax\n", resSlot) // capacity
		return
	}
	buf := fb.arrayBuffers[ins.Result]
	for i, a := range ins.Args {
		dst := buf - i*elemSize
		if fb.isAggregate(et) {
			fb.emitAggCopy(fb.valueSlots[a], dst, elemSize)
		} else {
			fb.emitLoad(et, fb.valueSlots[a])
			fb.emitStore(et, dst)
		}
	}
	fmt.Fprintf(&fb.out, "    lea rax, [rbp-%d]\n", buf)
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
	fmt.Fprintf(&fb.out, "    mov rax, %d\n", len(ins.Args))
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rax\n", resSlot)
}

// emitArrayLen loads the length half of an array value.
func (fb *fasmEmitter) emitArrayLen(ins *MIRInstr) {
	slot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", slot)
	fb.emitStore(fb.prog.Types.Lookup(ins.Type), resSlot)
}

// emitArrayIndex loads the element at the given index of an array value. The
// element address is base + index*elemSize; aggregate elements are copied as a
// byte range, others are loaded with the element type's width.
func (fb *fasmEmitter) emitArrayIndex(ins *MIRInstr) {
	arrSlot := fb.valueSlots[ins.Args[0]]
	idxSlot := fb.valueSlots[ins.Args[1]]
	t := fb.prog.Types.Lookup(ins.Type)
	elemSize := fb.sizeOf(t)
	resSlot := fb.valueSlots[ins.Result]
	fb.emitArrayBoundsCheck(ins, arrSlot, idxSlot)
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", arrSlot)
	fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d]\n", idxSlot)
	if elemSize != 1 {
		fmt.Fprintf(&fb.out, "    imul rcx, %d\n", elemSize)
	}
	fb.out.WriteString("    add rax, rcx\n")
	if fb.isAggregate(t) {
		fmt.Fprintf(&fb.out, "    mov rsi, rax\n")
		fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov rcx, %d\n", elemSize)
		fb.out.WriteString("    cld\n")
		fb.out.WriteString("    rep movsb\n")
		return
	}
	fb.emitLoadIndirect(t, "rax")
	fb.emitStore(t, resSlot)
}

// emitArrayBoundsCheck enforces the shared array indexing contract before a
// value load or an element address can reach the backing buffer.
func (fb *fasmEmitter) emitArrayBoundsCheck(ins *MIRInstr, arrSlot, idxSlot int) {
	errLabel := fb.newLabel()
	okLabel := fb.newLabel()
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", idxSlot)
	fb.out.WriteString("    test rax, rax\n")
	fmt.Fprintf(&fb.out, "    js %s\n", errLabel)
	fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d+8]\n", arrSlot)
	fb.out.WriteString("    cmp rax, rcx\n")
	fmt.Fprintf(&fb.out, "    jae %s\n", errLabel)
	fmt.Fprintf(&fb.out, "    jmp %s\n", okLabel)
	fmt.Fprintf(&fb.out, "%s:\n", errLabel)
	fb.emitBoundsExit(ins)
	fmt.Fprintf(&fb.out, "%s:\n", okLabel)
}

// emitAddrOf materializes the address of a local slot or a global.
func (fb *fasmEmitter) emitAddrOf(ins *MIRInstr) {
	resSlot := fb.valueSlots[ins.Result]
	switch ins.Imm.Kind {
	case MIRImmLocal:
		fmt.Fprintf(&fb.out, "    lea rax, [rbp-%d]\n", fb.localSlots[ins.Imm.Local])
	case MIRImmSymbol:
		fmt.Fprintf(&fb.out, "    lea rax, [%s]\n", fb.globalLabel(ins.Imm.Symbol))
	}
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
}

// emitDerefLoad loads a value through the address in Args[0].
func (fb *fasmEmitter) emitDerefLoad(ins *MIRInstr) {
	addrSlot := fb.valueSlots[ins.Args[0]]
	t := fb.prog.Types.Lookup(ins.Type)
	size := fb.sizeOf(t)
	resSlot := fb.valueSlots[ins.Result]
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", addrSlot)
	if fb.isAggregate(t) {
		fmt.Fprintf(&fb.out, "    mov rsi, rax\n")
		fmt.Fprintf(&fb.out, "    lea rdi, [rbp-%d]\n", resSlot)
		fmt.Fprintf(&fb.out, "    mov rcx, %d\n", size)
		fb.out.WriteString("    cld\n")
		fb.out.WriteString("    rep movsb\n")
		return
	}
	fb.emitLoadIndirect(t, "rax")
	fb.emitStore(t, resSlot)
}

// emitDerefStore stores Args[1] through the address in Args[0].
func (fb *fasmEmitter) emitDerefStore(ins *MIRInstr) {
	addrSlot := fb.valueSlots[ins.Args[0]]
	valSlot := fb.valueSlots[ins.Args[1]]
	t := fb.prog.Types.Lookup(ins.Type)
	if Debug && (t.Name == "Diagnostic" || t.Name == "Token") {
		fmt.Printf("DBG emitDerefStore type=%s kind=%d agg=%v\n", t.Name, t.Kind, fb.isAggregate(t))
	}
	size := fb.sizeOf(t)
	// Preserve the address in r10 so a scalar value never clobbers it.
	fmt.Fprintf(&fb.out, "    mov r10, qword [rbp-%d]\n", addrSlot)
	if fb.isAggregate(t) {
		fmt.Fprintf(&fb.out, "    lea rsi, [rbp-%d]\n", valSlot)
		fmt.Fprintf(&fb.out, "    mov rdi, r10\n")
		fmt.Fprintf(&fb.out, "    mov rcx, %d\n", size)
		fb.out.WriteString("    cld\n")
		fb.out.WriteString("    rep movsb\n")
		return
	}
	fb.emitLoad(t, valSlot)
	fb.emitStoreToReg(t, "r10")
}

// emitStoreToReg stores the value currently in rax (integers) or xmm0
// (floats) at the address in reg, using the type's width.
func (fb *fasmEmitter) emitStoreToReg(t IRType, reg string) {
	size := fb.sizeOf(t)
	if t.Kind == TypeKindFloat {
		if t.Name == "F32" {
			fmt.Fprintf(&fb.out, "    movss dword [%s], xmm0\n", reg)
		} else {
			fmt.Fprintf(&fb.out, "    movsd qword [%s], xmm0\n", reg)
		}
		return
	}
	switch size {
	case 1:
		fmt.Fprintf(&fb.out, "    mov byte [%s], al\n", reg)
	case 2:
		fmt.Fprintf(&fb.out, "    mov word [%s], ax\n", reg)
	case 4:
		fmt.Fprintf(&fb.out, "    mov dword [%s], eax\n", reg)
	default:
		fmt.Fprintf(&fb.out, "    mov qword [%s], rax\n", reg)
	}
}

// emitArrayElemAddr computes the address "data + index * sizeof(elem)" of an
// array element. The array value in Args[0] holds the data pointer at offset
// 0 (16-byte fat value), and the result is the element address.
func (fb *fasmEmitter) emitArrayElemAddr(ins *MIRInstr) {
	arrSlot := fb.valueSlots[ins.Args[0]]
	idxSlot := fb.valueSlots[ins.Args[1]]
	ptrType := fb.prog.Types.Lookup(ins.Type)
	elemType := fb.prog.Types.Lookup(ptrType.Elem)
	elemSize := fb.sizeOf(elemType)
	resSlot := fb.valueSlots[ins.Result]
	fb.emitArrayBoundsCheck(ins, arrSlot, idxSlot)
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", arrSlot)
	fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d]\n", idxSlot)
	if elemSize != 1 {
		fmt.Fprintf(&fb.out, "    imul rcx, %d\n", elemSize)
	}
	fb.out.WriteString("    add rax, rcx\n")
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
}

// emitFieldAddr computes "base + offset(field)" for the struct at a pointer.
// The pointed-to struct type comes from the address operand's type.
func (fb *fasmEmitter) emitFieldAddr(ins *MIRInstr) {
	addrSlot := fb.valueSlots[ins.Args[0]]
	addrType := fb.prog.Types.Lookup(fb.valueTypes[ins.Args[0]])
	structType := fb.prog.Types.Lookup(addrType.Elem)
	fieldIdx := int(ins.Imm.Int)
	resSlot := fb.valueSlots[ins.Result]
	fieldCount := len(structType.Fields)
	if structType.Kind == TypeKindArray {
		if structType.ArrayKind == ArrayDynamic {
			fieldCount = 3
		} else {
			fieldCount = 2
		}
	}
	if fieldIdx < 0 || fieldIdx >= fieldCount {
		return
	}
	offsets := fb.structFieldOffsets(structType)
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", addrSlot)
	if offsets[fieldIdx] != 0 {
		fmt.Fprintf(&fb.out, "    add rax, %d\n", offsets[fieldIdx])
	}
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
}

// emitPtrArith implements pointer +- integer scaled by the pointed-to element
// size, and pointer difference. subtract selects '-' and scale selects the
// pointer-difference form (byte difference divided by the element size, with
// C-like truncation toward zero).
func (fb *fasmEmitter) emitPtrArith(ins *MIRInstr, subtract, scale bool) {
	ptrSlot := fb.valueSlots[ins.Args[0]]
	otherSlot := fb.valueSlots[ins.Args[1]]
	ptrType := fb.prog.Types.Lookup(fb.valueTypes[ins.Args[0]])
	elemSize := fb.sizeOf(fb.prog.Types.Lookup(ptrType.Elem))
	resSlot := fb.valueSlots[ins.Result]
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", ptrSlot)
	fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d]\n", otherSlot)
	if scale {
		// p - q: byte difference, then divide by the element size.
		fb.out.WriteString("    sub rax, rcx\n")
		if elemSize != 1 {
			fmt.Fprintf(&fb.out, "    mov rcx, %d\n", elemSize)
			fb.out.WriteString("    cqo\n")
			fb.out.WriteString("    idiv rcx\n")
		}
	} else if subtract {
		// p - n: subtract n * size.
		if elemSize != 1 {
			fmt.Fprintf(&fb.out, "    imul rcx, %d\n", elemSize)
		}
		fb.out.WriteString("    sub rax, rcx\n")
	} else {
		// p + n: add n * size.
		if elemSize != 1 {
			fmt.Fprintf(&fb.out, "    imul rcx, %d\n", elemSize)
		}
		fb.out.WriteString("    add rax, rcx\n")
	}
	fb.emitStore(fb.prog.Types.Lookup(ins.Type), resSlot)
}

// emitAllocate implements '#allocate <size>'. It returns the current bump
// pointer and advances it by the size aligned up to 8 bytes. The heap is a
// static arena (chaos_heap) tracked by chaos_heap_ptr.
func (fb *fasmEmitter) emitAllocate(ins *MIRInstr) {
	sizeSlot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d]\n", sizeSlot)
	fb.out.WriteString("    call chaos_alloc\n")
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
}

// emitDeallocate implements '#deallocate <addr>'. A bump allocator cannot
// reclaim individual blocks, so this is a no-op for now.
func (fb *fasmEmitter) emitDeallocate(ins *MIRInstr) {}

// emitCast copies the value into a new slot typed as the target type. Pointer
// and Addr casts are representation-preserving, so this is a plain copy.
func (fb *fasmEmitter) emitCast(ins *MIRInstr) {
	t := fb.prog.Types.Lookup(ins.Type)
	srcSlot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	if fb.isAggregate(t) {
		fb.emitAggCopy(srcSlot, resSlot, fb.sizeOf(t))
		return
	}
	fb.emitLoad(t, srcSlot)
	fb.emitStore(t, resSlot)
}

// emitConvert implements a numeric conversion between integer and
// floating-point types. The value is preserved, not reinterpreted: integer
// loads sign/zero-extend to 64 bits, stores truncate to the target width, and
// int/float crossings use the SSE conversion instructions.
func (fb *fasmEmitter) emitConvert(ins *MIRInstr) {
	srcT := fb.prog.Types.Lookup(fb.valueTypes[ins.Args[0]])
	dstT := fb.prog.Types.Lookup(ins.Type)
	srcSlot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	srcFloat := srcT.Kind == TypeKindFloat
	dstFloat := dstT.Kind == TypeKindFloat
	switch {
	case srcFloat && dstFloat:
		fb.emitLoad(srcT, srcSlot)
		if srcT.Name == "F32" && dstT.Name == "F64" {
			fb.out.WriteString("    cvtss2sd xmm0, xmm0\n")
		} else if srcT.Name == "F64" && dstT.Name == "F32" {
			fb.out.WriteString("    cvtsd2ss xmm0, xmm0\n")
		}
		fb.emitStore(dstT, resSlot)
	case srcFloat && !dstFloat:
		fb.emitLoad(srcT, srcSlot)
		if srcT.Name == "F32" {
			fb.out.WriteString("    cvttss2si rax, xmm0\n")
		} else {
			fb.out.WriteString("    cvttsd2si rax, xmm0\n")
		}
		fb.emitStore(dstT, resSlot)
	case !srcFloat && dstFloat:
		fb.emitLoad(srcT, srcSlot)
		if dstT.Name == "F32" {
			fb.out.WriteString("    cvtsi2ss xmm0, rax\n")
		} else {
			fb.out.WriteString("    cvtsi2sd xmm0, rax\n")
		}
		fb.emitStore(dstT, resSlot)
	default:
		// Integer to integer: the load extends to 64 bits, the store
		// truncates to the target width.
		fb.emitLoad(srcT, srcSlot)
		fb.emitStore(dstT, resSlot)
	}
}

// emitInterpolate builds a String at runtime from literal runs and interpolated
// String values. The literal parts (ins.Imm.Strs) and the interpolated values
// (ins.Args) interleave as literal, value, literal, value, ..., literal. A
// fresh heap buffer holds the concatenated bytes.
func (fb *fasmEmitter) emitInterpolate(ins *MIRInstr) {
	resSlot := fb.valueSlots[ins.Result]
	// Compute the total byte count in rbx.
	fb.out.WriteString("    xor ebx, ebx\n")
	for _, lit := range ins.Imm.Strs {
		fmt.Fprintf(&fb.out, "    mov rax, %d\n", len(lit))
		fb.out.WriteString("    add rbx, rax\n")
		fb.out.WriteString("    jc chaos_alloc_fail\n")
	}
	for _, a := range ins.Args {
		fmt.Fprintf(&fb.out, "    mov rax, qword [rbp-%d+8]\n", fb.valueSlots[a])
		fb.out.WriteString("    add rbx, rax\n")
		fb.out.WriteString("    jc chaos_alloc_fail\n")
	}
	// Allocate a buffer through the checked shared allocator.
	fb.out.WriteString("    mov rax, rbx\n")
	fb.out.WriteString("    call chaos_alloc\n")
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], rax\n", resSlot)
	// Copy each part into the buffer.
	fmt.Fprintf(&fb.out, "    mov rdi, qword [rbp-%d]\n", resSlot)
	fb.out.WriteString("    cld\n")
	litIdx := 0
	for _, a := range ins.Args {
		// Copy the literal before this interpolated value.
		fb.emitInterpCopy(ins.Imm.Strs[litIdx])
		litIdx++
		// Copy the interpolated value's bytes.
		slot := fb.valueSlots[a]
		fmt.Fprintf(&fb.out, "    mov rsi, qword [rbp-%d]\n", slot)
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d+8]\n", slot)
		fb.out.WriteString("    rep movsb\n")
	}
	// Copy the trailing literal.
	fb.emitInterpCopy(ins.Imm.Strs[litIdx])
	// Set the result count.
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], rbx\n", resSlot)
}

// emitInterpCopy copies a literal string from the data section into the
// destination cursor (rdi), advancing it.
func (fb *fasmEmitter) emitInterpCopy(lit string) {
	idx := fb.stringIndexes[lit]
	fmt.Fprintf(&fb.out, "    mov rsi, str%d\n", idx)
	fmt.Fprintf(&fb.out, "    mov rcx, %d\n", len(lit))
	fb.out.WriteString("    rep movsb\n")
}

// emitPrint writes a String to stdout (fd 1). The newline flag appends a
// newline after the value.
func (fb *fasmEmitter) emitPrint(ins *MIRInstr) {
	slot := fb.valueSlots[ins.Args[0]]
	fb.out.WriteString("    mov rax, 1\n")
	fb.out.WriteString("    mov rdi, 1\n")
	fmt.Fprintf(&fb.out, "    mov rsi, qword [rbp-%d]\n", slot)
	fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d+8]\n", slot)
	fb.out.WriteString("    syscall\n")
	if ins.Imm.Bool {
		fb.out.WriteString("    mov rax, 1\n")
		fb.out.WriteString("    mov rdi, 1\n")
		fmt.Fprintf(&fb.out, "    lea rsi, [%s]\n", fb.newlineLabel())
		fb.out.WriteString("    mov rdx, 1\n")
		fb.out.WriteString("    syscall\n")
	}
}

// emitReadFile opens the path, reads the whole file into a heap buffer, and
// returns it as a String. A failure to open or read terminates the program
// with a message on stderr.
func (fb *fasmEmitter) emitReadFile(ins *MIRInstr) {
	pathSlot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	openFail := fb.newLabel()
	readFail := fb.newLabel()
	readFailNoClose := fb.newLabel()
	// This block is emitted inline into a function that may also use the
	// callee-saved registers r12-r15 for live values, so save and restore
	// them around the syscall sequence.
	fb.out.WriteString("    push r12\n")
	fb.out.WriteString("    push r13\n")
	fb.out.WriteString("    push r14\n")
	fb.out.WriteString("    push r15\n")
	// The open syscall requires a null-terminated path; Chaos strings carry
	// (data, count) with no terminator, so copy the path to a heap buffer.
	fb.emitNullTerminatedPath(pathSlot)
	// open(buf, O_RDONLY) -> fd in rax.
	fb.out.WriteString("    mov rax, 2\n")
	fb.out.WriteString("    xor rsi, rsi\n")
	fb.out.WriteString("    xor rdx, rdx\n")
	fb.out.WriteString("    mov rdi, r14\n")
	fb.out.WriteString("    syscall\n")
	// If fd < 0, report the failure and exit.
	fb.out.WriteString("    test rax, rax\n")
	fmt.Fprintf(&fb.out, "    js %s\n", openFail)
	// Save the fd in r12.
	fb.out.WriteString("    mov r12, rax\n")
	// fstat(fd, buf) to learn the file size; st_size is at offset 48.
	fb.out.WriteString("    sub rsp, 144\n")
	fb.out.WriteString("    mov rax, 5\n")
	fb.out.WriteString("    mov rdi, r12\n")
	fb.out.WriteString("    mov rsi, rsp\n")
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    mov r15, rax\n")
	fb.out.WriteString("    mov r13, qword [rsp+48]\n")
	fb.out.WriteString("    add rsp, 144\n")
	fb.out.WriteString("    test r15, r15\n")
	fmt.Fprintf(&fb.out, "    js %s\n", readFail)
	// Allocate r13 bytes through the checked shared allocator.
	fb.out.WriteString("    mov rax, r13\n")
	fb.out.WriteString("    call chaos_alloc\n")
	fb.out.WriteString("    mov r14, rax\n")
	// read(fd, buf, size) in a loop until all bytes are read.
	fb.out.WriteString("    xor r15, r15\n")
	readLoop := fb.newLabel()
	readDone := fb.newLabel()
	fmt.Fprintf(&fb.out, "%s:\n", readLoop)
	fb.out.WriteString("    cmp r15, r13\n")
	fmt.Fprintf(&fb.out, "    jae %s\n", readDone)
	fb.out.WriteString("    mov rax, 0\n")
	fb.out.WriteString("    mov rdi, r12\n")
	fb.out.WriteString("    lea rsi, [r14+r15]\n")
	fb.out.WriteString("    mov rdx, r13\n")
	fb.out.WriteString("    sub rdx, r15\n")
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    cmp rax, -4\n")
	fmt.Fprintf(&fb.out, "    je %s\n", readLoop)
	fb.out.WriteString("    test rax, rax\n")
	fmt.Fprintf(&fb.out, "    js %s\n", readFail)
	fmt.Fprintf(&fb.out, "    jz %s\n", readFail)
	fb.out.WriteString("    add r15, rax\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", readLoop)
	fmt.Fprintf(&fb.out, "%s:\n", readDone)
	// close(fd).
	fb.out.WriteString("    mov rax, 3\n")
	fb.out.WriteString("    mov rdi, r12\n")
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    test rax, rax\n")
	fmt.Fprintf(&fb.out, "    js %s\n", readFailNoClose)
	// Store the result String (data, count).
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d], r14\n", resSlot)
	fmt.Fprintf(&fb.out, "    mov qword [rbp-%d+8], r15\n", resSlot)
	fb.out.WriteString("    pop r15\n")
	fb.out.WriteString("    pop r14\n")
	fb.out.WriteString("    pop r13\n")
	fb.out.WriteString("    pop r12\n")
	done := fb.newLabel()
	fmt.Fprintf(&fb.out, "    jmp %s\n", done)
	// The open failed: write "cannot open file: <path>" to stderr and exit 1.
	fmt.Fprintf(&fb.out, "%s:\n", openFail)
	fb.out.WriteString("    pop r15\n")
	fb.out.WriteString("    pop r14\n")
	fb.out.WriteString("    pop r13\n")
	fb.out.WriteString("    pop r12\n")
	fb.emitWriteStderr(fb.openFailIndex, pathSlot)
	fb.out.WriteString("    mov rax, 60\n")
	fb.out.WriteString("    mov rdi, 1\n")
	fb.out.WriteString("    syscall\n")
	// A stat, read, short-read, or close failure is deterministic and never
	// returns partial file contents. Retry only an interrupted read.
	fmt.Fprintf(&fb.out, "%s:\n", readFail)
	fb.out.WriteString("    mov rax, 3\n")
	fb.out.WriteString("    mov rdi, r12\n")
	fb.out.WriteString("    syscall\n")
	fmt.Fprintf(&fb.out, "%s:\n", readFailNoClose)
	fb.out.WriteString("    pop r15\n")
	fb.out.WriteString("    pop r14\n")
	fb.out.WriteString("    pop r13\n")
	fb.out.WriteString("    pop r12\n")
	fb.emitWriteStderr(fb.readFailIndex, pathSlot)
	fb.out.WriteString("    mov rax, 60\n")
	fb.out.WriteString("    mov rdi, 1\n")
	fb.out.WriteString("    syscall\n")
	fmt.Fprintf(&fb.out, "%s:\n", done)
}

// emitFileExists opens the path read-only and reports whether it succeeded.
func (fb *fasmEmitter) emitFileExists(ins *MIRInstr) {
	pathSlot := fb.valueSlots[ins.Args[0]]
	resSlot := fb.valueSlots[ins.Result]
	openFail := fb.newLabel()
	done := fb.newLabel()
	// Save the callee-saved registers this block clobbers (see emitReadFile).
	fb.out.WriteString("    push r12\n")
	fb.out.WriteString("    push r13\n")
	fb.out.WriteString("    push r14\n")
	fb.out.WriteString("    push r15\n")
	fb.emitNullTerminatedPath(pathSlot)
	fb.out.WriteString("    mov rax, 2\n")
	fb.out.WriteString("    xor rsi, rsi\n")
	fb.out.WriteString("    xor rdx, rdx\n")
	fb.out.WriteString("    mov rdi, r14\n")
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    test rax, rax\n")
	fmt.Fprintf(&fb.out, "    js %s\n", openFail)
	// The file opened; close it and report true.
	fb.out.WriteString("    mov rdi, rax\n")
	fb.out.WriteString("    mov rax, 3\n")
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    mov al, 1\n")
	fmt.Fprintf(&fb.out, "    mov byte [rbp-%d], al\n", resSlot)
	fb.out.WriteString("    pop r15\n")
	fb.out.WriteString("    pop r14\n")
	fb.out.WriteString("    pop r13\n")
	fb.out.WriteString("    pop r12\n")
	fmt.Fprintf(&fb.out, "    jmp %s\n", done)
	// The open failed; report false.
	fmt.Fprintf(&fb.out, "%s:\n", openFail)
	fb.out.WriteString("    xor al, al\n")
	fmt.Fprintf(&fb.out, "    mov byte [rbp-%d], al\n", resSlot)
	fb.out.WriteString("    pop r15\n")
	fb.out.WriteString("    pop r14\n")
	fb.out.WriteString("    pop r13\n")
	fb.out.WriteString("    pop r12\n")
	fmt.Fprintf(&fb.out, "%s:\n", done)
}

// emitNullTerminatedPath copies the String in pathSlot to a fresh heap buffer
// with a trailing null byte and leaves the buffer address in r14. The open
// syscall requires a null-terminated path; Chaos strings carry (data, count)
// with no terminator.
func (fb *fasmEmitter) emitNullTerminatedPath(pathSlot int) {
	fmt.Fprintf(&fb.out, "    mov r13, qword [rbp-%d+8]\n", pathSlot)
	// Allocate count+1 bytes, aligned up to 8.
	fb.out.WriteString("    mov rax, r13\n")
	fb.out.WriteString("    inc rax\n")
	fb.out.WriteString("    jc chaos_alloc_fail\n")
	fb.out.WriteString("    call chaos_alloc\n")
	fb.out.WriteString("    mov r14, rax\n")
	// Copy the path bytes and null-terminate.
	fmt.Fprintf(&fb.out, "    mov rsi, qword [rbp-%d]\n", pathSlot)
	fb.out.WriteString("    mov rdi, r14\n")
	fb.out.WriteString("    mov rcx, r13\n")
	fb.out.WriteString("    cld\n")
	fb.out.WriteString("    rep movsb\n")
	fb.out.WriteString("    mov byte [r14+r13], 0\n")
}

// emitWriteStderr writes the message at string index msgIndex followed by the
// String value in pathSlot to stderr (fd 2).
func (fb *fasmEmitter) emitWriteStderr(msgIndex int, pathSlot int) {
	fb.out.WriteString("    mov rax, 1\n")
	fb.out.WriteString("    mov rdi, 2\n")
	fmt.Fprintf(&fb.out, "    mov rsi, str%d\n", msgIndex)
	fmt.Fprintf(&fb.out, "    mov rdx, %d\n", len(fb.strings[msgIndex]))
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    mov rax, 1\n")
	fb.out.WriteString("    mov rdi, 2\n")
	fmt.Fprintf(&fb.out, "    mov rsi, qword [rbp-%d]\n", pathSlot)
	fmt.Fprintf(&fb.out, "    mov rdx, qword [rbp-%d+8]\n", pathSlot)
	fb.out.WriteString("    syscall\n")
	fb.out.WriteString("    mov rax, 1\n")
	fb.out.WriteString("    mov rdi, 2\n")
	fmt.Fprintf(&fb.out, "    mov rsi, str%d\n", fb.newlineIndex)
	fb.out.WriteString("    mov rdx, 1\n")
	fb.out.WriteString("    syscall\n")
}

// emitLoadIndirect loads the value at [addrReg] into rax (integers) or xmm0
// (floats), using the type's width and sign extension.
func (fb *fasmEmitter) emitLoadIndirect(t IRType, addrReg string) {
	size := fb.sizeOf(t)
	if t.Kind == TypeKindFloat {
		if t.Name == "F32" {
			fmt.Fprintf(&fb.out, "    movss xmm0, dword [%s]\n", addrReg)
		} else {
			fmt.Fprintf(&fb.out, "    movsd xmm0, qword [%s]\n", addrReg)
		}
		return
	}
	switch size {
	case 1:
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsx rax, byte [%s]\n", addrReg)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rax, byte [%s]\n", addrReg)
		}
	case 2:
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsx rax, word [%s]\n", addrReg)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rax, word [%s]\n", addrReg)
		}
	case 4:
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsxd rax, dword [%s]\n", addrReg)
		} else {
			fmt.Fprintf(&fb.out, "    mov eax, dword [%s]\n", addrReg)
		}
	default:
		fmt.Fprintf(&fb.out, "    mov rax, qword [%s]\n", addrReg)
	}
}

// emitLoad loads the value at [rbp-offset] into rax (integers, sign or zero
// extended to 64 bits) or xmm0 (floats). Zero-size types (Void) load nothing.
func (fb *fasmEmitter) emitLoad(t IRType, offset int) {
	size := fb.sizeOf(t)
	if size == 0 {
		return
	}
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
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsx rax, byte [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rax, byte [rbp-%d]\n", offset)
		}
	case 2:
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsx rax, word [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rax, word [rbp-%d]\n", offset)
		}
	case 4:
		if fb.isSignedStorage(t) {
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
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsx rcx, byte [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rcx, byte [rbp-%d]\n", offset)
		}
	case 2:
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsx rcx, word [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    movzx rcx, word [rbp-%d]\n", offset)
		}
	case 4:
		if fb.isSignedStorage(t) {
			fmt.Fprintf(&fb.out, "    movsxd rcx, dword [rbp-%d]\n", offset)
		} else {
			fmt.Fprintf(&fb.out, "    mov ecx, dword [rbp-%d]\n", offset)
		}
	default:
		fmt.Fprintf(&fb.out, "    mov rcx, qword [rbp-%d]\n", offset)
	}
}

// emitStore stores rax (integers) or xmm0 (floats) into the slot at
// [rbp-offset], using the type's width. Zero-size types (Void) store nothing.
func (fb *fasmEmitter) emitStore(t IRType, offset int) {
	size := fb.sizeOf(t)
	if size == 0 {
		return
	}
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

func (fb *fasmEmitter) funcLabel(sym SymbolID) string {
	return "chaos_fn_" + strconv.FormatUint(uint64(sym), 10)
}

func (fb *fasmEmitter) globalLabel(sym SymbolID) string {
	if label, ok := fb.globalLabels[sym]; ok {
		return label
	}
	return "chaos_global_" + strconv.FormatUint(uint64(sym), 10)
}

func (fb *fasmEmitter) prepareGlobalLabels() {
	fb.globalLabels = make(map[SymbolID]string, len(fb.prog.Globals))
	for _, global := range fb.prog.Globals {
		fb.globalLabels[global.Symbol] = "chaos_global_" + strconv.FormatUint(uint64(global.Symbol), 10)
	}
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
	info, ok := LookupBuiltinType(name)
	return ok && info.Kind == BuiltinInteger && info.Signed
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
	return ""
}

func align(n, a int) int {
	if a <= 1 {
		return n
	}
	return (n + a - 1) / a * a
}

func align16(n int) int {
	return align(n, 16)
}
