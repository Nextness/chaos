// MIR (mid-level IR) node types for the Chaos compiler.
//
// The MIR is a three-address control-flow representation lowered from the
// HIR. Each function is a list of basic blocks; each block holds typed
// instructions and exactly one terminator. Values are produced by
// instructions and referenced by ValueID; locals and globals are referenced
// by LocalID and SymbolID respectively.
package compiler

// MIRProgram is the root of the control-flow IR. Entry is the SymbolID of
// the procedure selected by a '#entry' directive, or NoSymbol when the source
// declares no entry point.
type MIRProgram struct {
	Symbols   *SymbolTable
	Types     *TypeTable
	Functions []*MIRFunction
	Globals   []MIRGlobal
	Entry     SymbolID
}

// NoSymbol is the sentinel SymbolID used where no symbol is present (for
// example a program with no '#entry' procedure).
const NoSymbol SymbolID = ^SymbolID(0)

// MIRGlobal is a top-level variable or constant. HasInit reports whether the
// declaration carried an initializer.
type MIRGlobal struct {
	Symbol  SymbolID
	Name    string
	Type    TypeID
	Mutable bool
	HasInit bool
	Span    Span
}

// MIRFunction is a lowered procedure. Locals holds every local slot (params
// first, then declared locals); LocalTypes is indexed by LocalID.
type MIRFunction struct {
	Symbol     SymbolID
	Name       string
	Params     []LocalID
	Locals     []LocalID
	LocalTypes []TypeID
	Results    []TypeID
	Blocks     []*MIRBlock
	Span       Span
}

// MIRBlock is a basic block: a sequence of instructions and one terminator.
type MIRBlock struct {
	ID     BlockID
	Instrs []*MIRInstr
	Term   MIRTerminator
	Span   Span
}

// MIROpcode enumerates MIR instructions.
type MIROpcode uint8

const (
	MIRConst MIROpcode = iota
	MIRLoadLocal
	MIRStoreLocal
	MIRLoadGlobal
	MIRStoreGlobal
	MIRAdd
	MIRSub
	MIRMul
	MIRDiv
	MIRMod
	MIRCmpLt
	MIRCmpGt
	MIRCmpLe
	MIRCmpGe
	MIRCmpEq
	MIRCmpNeq
	MIRAnd
	MIROr
	MIRNot
	MIRNeg
	MIRCall
	MIRStructInit
	MIRExit
)

// String returns the textual name of an opcode.
func (op MIROpcode) String() string {
	switch op {
	case MIRConst:
		return "const"
	case MIRLoadLocal:
		return "load.local"
	case MIRStoreLocal:
		return "store.local"
	case MIRLoadGlobal:
		return "load.global"
	case MIRStoreGlobal:
		return "store.global"
	case MIRAdd:
		return "add"
	case MIRSub:
		return "sub"
	case MIRMul:
		return "mul"
	case MIRDiv:
		return "div"
	case MIRMod:
		return "mod"
	case MIRCmpLt:
		return "cmp.lt"
	case MIRCmpGt:
		return "cmp.gt"
	case MIRCmpLe:
		return "cmp.le"
	case MIRCmpGe:
		return "cmp.ge"
	case MIRCmpEq:
		return "cmp.eq"
	case MIRCmpNeq:
		return "cmp.neq"
	case MIRAnd:
		return "and"
	case MIROr:
		return "or"
	case MIRNot:
		return "not"
	case MIRNeg:
		return "neg"
	case MIRCall:
		return "call"
	case MIRStructInit:
		return "struct.init"
	case MIRExit:
		return "exit"
	}
	return "?"
}

// MIRImmediateKind classifies the immediate operand of an instruction.
type MIRImmediateKind uint8

const (
	MIRImmNone MIRImmediateKind = iota
	MIRImmInt
	MIRImmFloat
	MIRImmString
	MIRImmBool
	MIRImmSymbol
	MIRImmLocal
)

// MIRImmediate is the non-value operand of an instruction.
type MIRImmediate struct {
	Kind   MIRImmediateKind
	Int    int64
	Float  float64
	Str    string
	Bool   bool
	Symbol SymbolID
	Local  LocalID
}

// MIRInstr is a single three-address instruction. Result is NoValue for
// instructions that produce no value (for example store.local).
type MIRInstr struct {
	Result ValueID
	Op     MIROpcode
	Type   TypeID
	Args   []ValueID
	Imm    MIRImmediate
	Span   Span
}

// MIRTermKind enumerates block terminators.
type MIRTermKind uint8

const (
	MIRNoTerm MIRTermKind = iota
	MIRJump
	MIRBranch
	MIRReturn
	MIRTermExit
	MIRUnreachable
)

// String returns the textual name of a terminator kind.
func (k MIRTermKind) String() string {
	switch k {
	case MIRNoTerm:
		return "no-term"
	case MIRJump:
		return "jump"
	case MIRBranch:
		return "branch"
	case MIRReturn:
		return "return"
	case MIRTermExit:
		return "exit"
	case MIRUnreachable:
		return "unreachable"
	}
	return "?"
}

// MIRTerminator ends a basic block. The fields used depend on Kind: Target
// for MIRJump; Cond/Then/Else for MIRBranch; Value for MIRReturn (NoValue for
// void); Status/Message for MIRExit (NoValue when absent).
type MIRTerminator struct {
	Kind    MIRTermKind
	Target  BlockID
	Cond    ValueID
	Then    BlockID
	Else    BlockID
	Value   ValueID
	Status  ValueID
	Message ValueID
	Span    Span
}
