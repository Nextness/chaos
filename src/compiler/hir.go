// HIR (high-level IR) node types for the Chaos compiler.
//
// The HIR is a typed, source-close representation produced by lowering a
// parsed AST. Names are resolved to SymbolIDs, type expressions to TypeIDs,
// and every expression carries its inferred type. Control flow remains
// structured (blocks, if/elif/else) and every node keeps its source Span.
package compiler

// HIR is the root of the typed, source-close IR. Entry is the stable symbol of
// the selected procedure, never a spelling-based lookup.
type HIR struct {
	Symbols *SymbolTable
	Types   *TypeTable
	Structs []*HIRStruct
	Globals []*HIRGlobal
	Procs   []*HIRProc
	Entry   SymbolID
	Sources map[FileID]SourceFile
}

// HIRStruct is a resolved struct type declaration.
type HIRStruct struct {
	Symbol SymbolID
	Name   string
	Type   TypeID
	Fields []HIRField
	Span   Span
}

// HIRField is a resolved struct field.
type HIRField struct {
	Symbol  SymbolID
	Name    string
	Type    TypeID
	Default HIRExpr
	Span    Span
}

// HIRGlobal is a top-level variable or constant declaration.
type HIRGlobal struct {
	Symbol      SymbolID
	Name        string
	Type        TypeID
	Init        HIRExpr
	Mutable     bool
	CompileTime bool
	Span        Span
}

// HIRProc is a resolved procedure declaration.
type HIRProc struct {
	Symbol     SymbolID
	Name       string
	Params     []HIRParam
	Results    []TypeID
	ResultType TypeID
	Body       *HIRBlock
	Span       Span
}

// HIRParam is a resolved procedure parameter.
type HIRParam struct {
	Symbol SymbolID
	Name   string
	Type   TypeID
	Span   Span
}

// HIRStmt is implemented by all HIR statement nodes.
type HIRStmt interface {
	hirStmtNode()
	hirSpan() Span
}

// HIRExpr is implemented by all HIR expression nodes.
type HIRExpr interface {
	hirExprNode()
	hirSpan() Span
	hirType() TypeID
}

// HIRBlock is a braced block of HIR statements.
type HIRBlock struct {
	Span_ Span
	Stmts []HIRStmt
}

func (b *HIRBlock) hirStmtNode()  {}
func (b *HIRBlock) hirSpan() Span { return b.Span_ }

// HIRVarDecl is a local variable or constant declaration.
type HIRVarDecl struct {
	Span_       Span
	Symbol      SymbolID
	Name        string
	Type        TypeID
	Init        HIRExpr
	Mutable     bool
	CompileTime bool
}

func (s *HIRVarDecl) hirStmtNode()  {}
func (s *HIRVarDecl) hirSpan() Span { return s.Span_ }

// HIRAssign is a reassignment: target = value.
type HIRAssign struct {
	Span_  Span
	Target SymbolID
	Value  HIRExpr
}

func (s *HIRAssign) hirStmtNode()  {}
func (s *HIRAssign) hirSpan() Span { return s.Span_ }

// HIRReturn is a return statement. Value is nil for void returns.
type HIRReturn struct {
	Span_ Span
	Value HIRExpr
}

func (s *HIRReturn) hirStmtNode()  {}
func (s *HIRReturn) hirSpan() Span { return s.Span_ }

// HIRExit is an exit statement. Status and Message are nil when absent.
type HIRExit struct {
	Span_   Span
	Status  HIRExpr
	Message HIRExpr
}

func (s *HIRExit) hirStmtNode()  {}
func (s *HIRExit) hirSpan() Span { return s.Span_ }

// HIRIf is an if/elif/else chain. Elif holds the elif branches (each with its
// own Condition and Then); Else is nil when there is no else branch.
type HIRIf struct {
	Span_     Span
	Condition HIRExpr
	Then      *HIRBlock
	Elif      []*HIRIf
	Else      *HIRBlock
}

func (s *HIRIf) hirStmtNode()  {}
func (s *HIRIf) hirSpan() Span { return s.Span_ }

// HIRExprStmt wraps an expression used as a statement (for example a bare
// call).
type HIRExprStmt struct {
	Span_ Span
	Expr  HIRExpr
}

func (s *HIRExprStmt) hirStmtNode()  {}
func (s *HIRExprStmt) hirSpan() Span { return s.Span_ }

// HIRIfCatch is an error check: when Cond holds an error, the catch body runs
// (with CatchSym bound to the error when it is not NoSymbol) and must return
// or exit; otherwise execution continues. UnionType is the value-or-error
// pair type of Cond.
type HIRIfCatch struct {
	Span_     Span
	Cond      HIRExpr
	CatchSym  SymbolID // error binding (NoSymbol when unnamed)
	CatchBody *HIRBlock
	UnionType TypeID
}

func (s *HIRIfCatch) hirStmtNode()  {}
func (s *HIRIfCatch) hirSpan() Span { return s.Span_ }

// HIRFor is a loop. Init, Cond, and After are the c-style header; the range
// form is desugared into this shape during lowering (a hidden index variable,
// a length comparison, and an increment). Break and continue jump to the
// loop's exit and after blocks.
type HIRFor struct {
	Span_ Span
	Init  HIRStmt
	Cond  HIRExpr
	After HIRStmt
	Body  *HIRBlock
}

func (s *HIRFor) hirStmtNode()  {}
func (s *HIRFor) hirSpan() Span { return s.Span_ }

// HIRBreak exits the innermost enclosing loop.
type HIRBreak struct {
	Span_ Span
}

func (s *HIRBreak) hirStmtNode()  {}
func (s *HIRBreak) hirSpan() Span { return s.Span_ }

// HIRContinue jumps to the after-statement of the innermost enclosing loop.
type HIRContinue struct {
	Span_ Span
}

func (s *HIRContinue) hirStmtNode()  {}
func (s *HIRContinue) hirSpan() Span { return s.Span_ }

// HIRArrayInit is an array literal. Type is the array type "[]T"; Items holds
// the element values.
type HIRArrayInit struct {
	Span_ Span
	Type  TypeID
	Items []HIRExpr
}

func (e *HIRArrayInit) hirExprNode()    {}
func (e *HIRArrayInit) hirSpan() Span   { return e.Span_ }
func (e *HIRArrayInit) hirType() TypeID { return e.Type }

// HIRArrayLen is the length of an array value. Type is S64.
type HIRArrayLen struct {
	Span_ Span
	Array HIRExpr
	Type  TypeID
}

func (e *HIRArrayLen) hirExprNode()    {}
func (e *HIRArrayLen) hirSpan() Span   { return e.Span_ }
func (e *HIRArrayLen) hirType() TypeID { return e.Type }

// HIRIndex is an array element access. Type is the element type.
type HIRIndex struct {
	Span_ Span
	Base  HIRExpr
	Index HIRExpr
	Type  TypeID
}

func (e *HIRIndex) hirExprNode()    {}
func (e *HIRIndex) hirSpan() Span   { return e.Span_ }
func (e *HIRIndex) hirType() TypeID { return e.Type }

// HIRFieldLoad reads one field of a struct value. Field is the field index
// into the base's struct type; Type is the field type.
type HIRFieldLoad struct {
	Span_ Span
	Base  HIRExpr
	Field int
	Type  TypeID
}

func (e *HIRFieldLoad) hirExprNode()    {}
func (e *HIRFieldLoad) hirSpan() Span   { return e.Span_ }
func (e *HIRFieldLoad) hirType() TypeID { return e.Type }

// HIRDeref loads a value through a pointer: "p.*". Type is the pointed-to
// element type. The pointer must be null-checked before this node may reach
// lowering (enforced during type checking).
type HIRDeref struct {
	Span_   Span
	Operand HIRExpr
	Type    TypeID
}

func (e *HIRDeref) hirExprNode()    {}
func (e *HIRDeref) hirSpan() Span   { return e.Span_ }
func (e *HIRDeref) hirType() TypeID { return e.Type }

// HIRAddrOf computes the address of an lvalue (a local, global, or the value
// behind a dereference). Type is the resulting pointer type "*Elem".
type HIRAddrOf struct {
	Span_   Span
	Operand HIRExpr
	Type    TypeID
}

func (e *HIRAddrOf) hirExprNode()    {}
func (e *HIRAddrOf) hirSpan() Span   { return e.Span_ }
func (e *HIRAddrOf) hirType() TypeID { return e.Type }

// HIRFieldAddr computes the address of a field inside the struct at a
// pointer: "addr + offset(field)". Addr is an address HIRExpr; Type is the
// pointed-to field type (a pointer type).
type HIRFieldAddr struct {
	Span_ Span
	Addr  HIRExpr
	Field int
	Type  TypeID
}

func (e *HIRFieldAddr) hirExprNode()    {}
func (e *HIRFieldAddr) hirSpan() Span   { return e.Span_ }
func (e *HIRFieldAddr) hirType() TypeID { return e.Type }

// HIRArrayElemAddr computes the address of the element at Index inside the
// array value Array. Type is the pointer-to-element type.
type HIRArrayElemAddr struct {
	Span_ Span
	Array HIRExpr
	Index HIRExpr
	Type  TypeID
}

func (e *HIRArrayElemAddr) hirExprNode()    {}
func (e *HIRArrayElemAddr) hirSpan() Span   { return e.Span_ }
func (e *HIRArrayElemAddr) hirType() TypeID { return e.Type }

// HIRAddrStore stores a value through an address expression (the target of an
// assignment to an array element, struct field, or dereferenced pointer).
// Type is the stored value's type.
type HIRAddrStore struct {
	Span_ Span
	Addr  HIRExpr
	Value HIRExpr
	Type  TypeID
}

func (s *HIRAddrStore) hirStmtNode()  {}
func (s *HIRAddrStore) hirSpan() Span { return s.Span_ }

// ConstKind classifies an HIRConst literal.
type ConstKind uint8

const (
	ConstInt ConstKind = iota
	ConstFloat
	ConstString
	ConstBool
	ConstError
)

// HIRZero is the recursive zero value of a resolved type.
type HIRZero struct {
	Span_ Span
	Type  TypeID
}

func (e *HIRZero) hirExprNode()    {}
func (e *HIRZero) hirSpan() Span   { return e.Span_ }
func (e *HIRZero) hirType() TypeID { return e.Type }

// HIRPoison represents an internal lowering failure. It permits diagnostics to
// accumulate, but MIR lowering rejects it so poison can never be emitted.
type HIRPoison struct {
	Span_ Span
	Type  TypeID
}

func (e *HIRPoison) hirExprNode()    {}
func (e *HIRPoison) hirSpan() Span   { return e.Span_ }
func (e *HIRPoison) hirType() TypeID { return e.Type }

// HIRConst is a literal constant. Int, Float, Str, and Bool hold the value
// for the matching Kind; Str also keeps the raw source text for integer and
// float literals.
type HIRConst struct {
	Span_ Span
	Type  TypeID
	Kind  ConstKind
	Int   int64
	Float float64
	Str   string
	Bool  bool
}

func (e *HIRConst) hirExprNode()    {}
func (e *HIRConst) hirSpan() Span   { return e.Span_ }
func (e *HIRConst) hirType() TypeID { return e.Type }

// HIRRef is a reference to a named entity (local, parameter, or global).
type HIRRef struct {
	Span_  Span
	Symbol SymbolID
	Type   TypeID
}

func (e *HIRRef) hirExprNode()    {}
func (e *HIRRef) hirSpan() Span   { return e.Span_ }
func (e *HIRRef) hirType() TypeID { return e.Type }

// HIRBinary is a binary operation.
type HIRBinary struct {
	Span_  Span
	OpSpan Span
	Op     BinaryOp
	Left   HIRExpr
	Right  HIRExpr
	Type   TypeID
}

func (e *HIRBinary) hirExprNode()    {}
func (e *HIRBinary) hirSpan() Span   { return e.Span_ }
func (e *HIRBinary) hirType() TypeID { return e.Type }

// HIRUnary is a unary operation.
type HIRUnary struct {
	Span_   Span
	Op      UnaryOp
	Operand HIRExpr
	Type    TypeID
}

func (e *HIRUnary) hirExprNode()    {}
func (e *HIRUnary) hirSpan() Span   { return e.Span_ }
func (e *HIRUnary) hirType() TypeID { return e.Type }

// HIRIfx is a ternary expression: ifx cond then then else else. All three
// parts are expressions and the node produces a value of Type.
type HIRIfx struct {
	Span_ Span
	Cond  HIRExpr
	Then  HIRExpr
	Else  HIRExpr
	Type  TypeID
}

func (e *HIRIfx) hirExprNode()    {}
func (e *HIRIfx) hirSpan() Span   { return e.Span_ }
func (e *HIRIfx) hirType() TypeID { return e.Type }

// HIRCall is a call to a named procedure.
type HIRCall struct {
	Span_ Span
	Func  SymbolID
	Args  []HIRExpr
	Type  TypeID
}

func (e *HIRCall) hirExprNode()    {}
func (e *HIRCall) hirSpan() Span   { return e.Span_ }
func (e *HIRCall) hirType() TypeID { return e.Type }

// HIRAllocate is the '#allocate <size>' intrinsic. It allocates a heap block
// of the given size and yields an Addr to it.
type HIRAllocate struct {
	Span_ Span
	Size  HIRExpr
	Type  TypeID
}

func (e *HIRAllocate) hirExprNode()    {}
func (e *HIRAllocate) hirSpan() Span   { return e.Span_ }
func (e *HIRAllocate) hirType() TypeID { return e.Type }

// HIRCast reinterprets a value as another type. Pointer and Addr casts are
// representation-preserving no-ops; the node exists so the type change is
// explicit in the IR.
type HIRCast struct {
	Span_ Span
	Value HIRExpr
	Type  TypeID
}

func (e *HIRCast) hirExprNode()    {}
func (e *HIRCast) hirSpan() Span   { return e.Span_ }
func (e *HIRCast) hirType() TypeID { return e.Type }

// HIRInterpolate builds a String at runtime from literal runs and interpolated
// String values. Literals holds the literal parts (one more than the number of
// interpolated values); Values holds the interpolated String expressions.
type HIRInterpolate struct {
	Span_    Span
	Literals []string
	Values   []HIRExpr
	Type     TypeID
}

func (e *HIRInterpolate) hirExprNode()    {}
func (e *HIRInterpolate) hirSpan() Span   { return e.Span_ }
func (e *HIRInterpolate) hirType() TypeID { return e.Type }

// HIRConvert is a numeric conversion between integer and floating-point types
// (for example S64 to F64, or Byte to S64). Unlike HIRCast it preserves the
// numeric value rather than the bit representation.
type HIRConvert struct {
	Span_ Span
	Value HIRExpr
	Type  TypeID
}

func (e *HIRConvert) hirExprNode()    {}
func (e *HIRConvert) hirSpan() Span   { return e.Span_ }
func (e *HIRConvert) hirType() TypeID { return e.Type }

// HIRPrint writes a String to stdout. Newline selects the println form, which
// appends a newline after the value.
type HIRPrint struct {
	Span_   Span
	Value   HIRExpr
	Newline bool
	Type    TypeID
}

func (e *HIRPrint) hirExprNode()    {}
func (e *HIRPrint) hirSpan() Span   { return e.Span_ }
func (e *HIRPrint) hirType() TypeID { return e.Type }

// HIRReadFile reads a file into a String. The file must exist and be
// readable; a failure terminates the program with a diagnostic.
type HIRReadFile struct {
	Span_ Span
	Path  HIRExpr
	Type  TypeID
}

func (e *HIRReadFile) hirExprNode()    {}
func (e *HIRReadFile) hirSpan() Span   { return e.Span_ }
func (e *HIRReadFile) hirType() TypeID { return e.Type }

// HIRFileExists reports whether a file exists and is readable.
type HIRFileExists struct {
	Span_ Span
	Path  HIRExpr
	Type  TypeID
}

func (e *HIRFileExists) hirExprNode()    {}
func (e *HIRFileExists) hirSpan() Span   { return e.Span_ }
func (e *HIRFileExists) hirType() TypeID { return e.Type }

// HIRDeallocate is the '#deallocate <addr>' intrinsic. It frees a heap block
// previously returned by '#allocate'.
type HIRDeallocate struct {
	Span_ Span
	Addr  HIRExpr
}

func (s *HIRDeallocate) hirStmtNode()  {}
func (s *HIRDeallocate) hirSpan() Span { return s.Span_ }

// HIRStructInit is a struct literal. Fields are kept in source order; each
// carries the resolved field symbol.
type HIRStructInit struct {
	Span_  Span
	Struct SymbolID
	Type   TypeID
	Fields []HIRStructInitField
}

func (e *HIRStructInit) hirExprNode()    {}
func (e *HIRStructInit) hirSpan() Span   { return e.Span_ }
func (e *HIRStructInit) hirType() TypeID { return e.Type }

// HIRStructInitField is one entry in a struct literal.
type HIRStructInitField struct {
	Span_ Span
	Field SymbolID
	Value HIRExpr
}

func (f HIRStructInitField) hirSpan() Span { return f.Span_ }
