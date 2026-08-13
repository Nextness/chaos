// HIR (high-level IR) node types for the Chaos compiler.
//
// The HIR is a typed, source-close representation produced by lowering a
// parsed AST. Names are resolved to SymbolIDs, type expressions to TypeIDs,
// and every expression carries its inferred type. Control flow remains
// structured (blocks, if/elif/else) and every node keeps its source Span.
package compiler

// HIR is the root of the typed, source-close IR. Entry is the name of the
// procedure selected by a '#entry' directive, or "" when the source declares
// no entry point.
type HIR struct {
	Symbols *SymbolTable
	Types   *TypeTable
	Structs []*HIRStruct
	Globals []*HIRGlobal
	Procs   []*HIRProc
	Entry   string
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
	Symbol SymbolID
	Name   string
	Type   TypeID
	Span   Span
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
	Symbol  SymbolID
	Name    string
	Params  []HIRParam
	Results []TypeID
	Body    *HIRBlock
	Span    Span
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

// ConstKind classifies an HIRConst literal.
type ConstKind uint8

const (
	ConstInt ConstKind = iota
	ConstFloat
	ConstString
	ConstBool
	ConstError
	ConstUnknown
)

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
	Span_ Span
	Op    BinaryOp
	Left  HIRExpr
	Right HIRExpr
	Type  TypeID
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
