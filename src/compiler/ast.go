// AST node types for the Chaos language.
//
// The parser produces an unresolved syntax tree: identifiers are stored as
// plain strings (not SymbolIDs). Name resolution and type checking are
// separate passes that consume this tree.
//
// Every node carries a Span for source-location diagnostics.
package compiler

// Node is the common interface implemented by all AST nodes.
type Node interface {
	nodeSpan() Span
}

// Expr is implemented by all expression nodes.
type Expr interface {
	Node
	exprNode()
}

// Stmt is implemented by all statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Decl is implemented by top-level declaration nodes.
type Decl interface {
	Stmt
	declNode()
}

// IdentExpr is a reference to a named entity.
type IdentExpr struct {
	Span_ Span
	Name  string
}

func (e *IdentExpr) nodeSpan() Span {
	return e.Span_
}

func (e *IdentExpr) exprNode() {}

// IntExpr is an integer literal. Value is the raw source text (e.g. "42", "1_000").
type IntExpr struct {
	Span_ Span
	Value string
}

func (e *IntExpr) nodeSpan() Span { return e.Span_ }
func (e *IntExpr) exprNode()      {}

// FloatExpr is a float literal. Value is the raw source text (e.g. "3.14", ".5").
type FloatExpr struct {
	Span_ Span
	Value string
}

func (e *FloatExpr) nodeSpan() Span { return e.Span_ }
func (e *FloatExpr) exprNode()      {}

// StringExpr is a string literal. Value is the decoded inner text (without guillemets).
type StringExpr struct {
	Span_ Span
	Value string
}

func (e *StringExpr) nodeSpan() Span { return e.Span_ }
func (e *StringExpr) exprNode()      {}

// BoolExpr is a boolean literal (true or false).
type BoolExpr struct {
	Span_ Span
	Value bool
}

func (e *BoolExpr) nodeSpan() Span {
	return e.Span_
}

func (e *BoolExpr) exprNode() {}

// BinaryOp enumerates all binary operators.
type BinaryOp uint8

const (
	BinaryOpAdd BinaryOp = iota // +
	BinaryOpSub                 // -
	BinaryOpMul                 // *
	BinaryOpDiv                 // /
	BinaryOpMod                 // %
	BinaryOpLt                  // <
	BinaryOpGt                  // >
	BinaryOpLe                  // <=
	BinaryOpGe                  // >=
	BinaryOpEq                  // ==
	BinaryOpNeq                 // !=
	BinaryOpAnd                 // &&
	BinaryOpOr                  // ||
)

func (op BinaryOp) String() string {
	switch op {
	case BinaryOpAdd:
		return "+"
	case BinaryOpSub:
		return "-"
	case BinaryOpMul:
		return "*"
	case BinaryOpDiv:
		return "/"
	case BinaryOpMod:
		return "%"
	case BinaryOpLt:
		return "<"
	case BinaryOpGt:
		return ">"
	case BinaryOpLe:
		return "<="
	case BinaryOpGe:
		return ">="
	case BinaryOpEq:
		return "=="
	case BinaryOpNeq:
		return "!="
	case BinaryOpAnd:
		return "&&"
	case BinaryOpOr:
		return "||"
	default:
		return "?"
	}
}

// BinaryExpr is a binary operation: left op right.
type BinaryExpr struct {
	Span_ Span
	Op    BinaryOp
	Left  Expr
	Right Expr
}

func (e *BinaryExpr) nodeSpan() Span {
	return e.Span_
}

func (e *BinaryExpr) exprNode() {}

// UnaryOp enumerates unary operators.
type UnaryOp uint8

const (
	UnaryOpNeg UnaryOp = iota // -
	UnaryOpNot                // !
)

func (op UnaryOp) String() string {
	switch op {
	case UnaryOpNeg:
		return "-"
	case UnaryOpNot:
		return "!"
	default:
		return "?"
	}
}

// UnaryExpr is a unary operation: op operand.
type UnaryExpr struct {
	Span_   Span
	Op      UnaryOp
	Operand Expr
}

func (e *UnaryExpr) nodeSpan() Span {
	return e.Span_
}
func (e *UnaryExpr) exprNode() {}

// CallExpr is a function call: func(args...).
type CallExpr struct {
	Span_ Span
	Func  Expr
	Args  []Expr
}

func (e *CallExpr) nodeSpan() Span {
	return e.Span_
}

func (e *CallExpr) exprNode() {}

// ParenExpr is a parenthesized expression: (expr).
type ParenExpr struct {
	Span_ Span
	Inner Expr
}

func (e *ParenExpr) nodeSpan() Span {
	return e.Span_
}

func (e *ParenExpr) exprNode() {}

// StructInitExpr is a struct literal: TypeName.{field=value, ...} or .{...}.
// Type is nil for inferred literals (.{...}); the type comes from the
// surrounding declaration.
type StructInitExpr struct {
	Span_  Span
	Type   Expr // type name (nil for inferred)
	Fields []StructInitField
}

func (e *StructInitExpr) nodeSpan() Span {
	return e.Span_
}

func (e *StructInitExpr) exprNode() {}

// StructInitField is one entry in a struct literal. Name is empty for
// positional values.
type StructInitField struct {
	Span_ Span
	Name  string // field name (empty for positional)
	Value Expr
}

func (f StructInitField) nodeSpan() Span {
	return f.Span_
}

// ErrorExpr is a placeholder inserted when the parser encounters an error
// during expression parsing. It allows the parser to continue and collect
// additional diagnostics without crashing.
type ErrorExpr struct {
	Span_ Span
}

func (e *ErrorExpr) nodeSpan() Span {
	return e.Span_
}

func (e *ErrorExpr) exprNode() {}

// VarDecl is a variable or constant declaration.
//
//	CompileTime=true, Mutable=false  :  x :: expr
//	CompileTime=false, Mutable=true  :  x := expr
//	CompileTime=false, Mutable=true  :  x : T = expr
//	CompileTime=false, Mutable=true  :  x : T       (Init == nil)
type VarDecl struct {
	Span_       Span
	Name        string
	DeclType    Expr // type annotation (nil for inferred)
	Init        Expr // initializer (nil for uninitialized)
	Mutable     bool // can be reassigned
	CompileTime bool // compile-time constant (::)
}

func (d *VarDecl) nodeSpan() Span {
	return d.Span_
}

func (d *VarDecl) stmtNode() {}

func (d *VarDecl) declNode() {}

// AssignStmt is a reassignment: name = expr.
type AssignStmt struct {
	Span_ Span
	Name  string
	Value Expr
}

func (s *AssignStmt) nodeSpan() Span {
	return s.Span_
}

func (s *AssignStmt) stmtNode() {}

// ReturnStmt is a return statement. Value is nil for void returns.
type ReturnStmt struct {
	Span_ Span
	Value Expr // nil for bare "return;"
}

func (s *ReturnStmt) nodeSpan() Span {
	return s.Span_
}

func (s *ReturnStmt) stmtNode() {}

// ExitStmt is an exit statement. Message is nil when no message is provided.
type ExitStmt struct {
	Span_   Span
	Status  Expr
	Message Expr // nil if no message
}

func (s *ExitStmt) nodeSpan() Span {
	return s.Span_
}

func (s *ExitStmt) stmtNode() {}

// IfStmt is an if/elif/else chain.
//   - Elif holds the elif branches (each with its own Condition, Body, and nested Elif).
//   - ElseBody is nil when there is no else branch.
type IfStmt struct {
	Span_     Span
	Condition Expr
	Body      *BlockStmt
	Elif      []*IfStmt  // elif branches (each must have a Condition)
	ElseBody  *BlockStmt // optional else branch
}

func (s *IfStmt) nodeSpan() Span {
	return s.Span_
}

func (s *IfStmt) stmtNode() {}

// BlockStmt is a braced block of statements.
type BlockStmt struct {
	Span_ Span
	Stmts []Stmt
}

func (s *BlockStmt) nodeSpan() Span {
	return s.Span_
}

func (s *BlockStmt) stmtNode() {}

// ProcDecl is a procedure declaration.
//
//	Results is empty when the procedure specifies no return type (Void).
//	Params holds the parameter list. Each parameter has a type expression.
//	ErrorResult is the error type of a '<>' result spec ("-> Type <> ErrorType"
//	or "-> (Type <> ErrorType)"), or nil when the procedure cannot return an
//	error. The '<>' form declares that the procedure returns a value but can
//	also return an error; the two sides may be written in either order.
type ProcDecl struct {
	Span_       Span
	Name        string
	Params      []Param
	Results     []Expr // type expressions (empty for void)
	ErrorResult Expr   // error type expression (nil when no '<>')
	Body        *BlockStmt
}

func (d *ProcDecl) nodeSpan() Span {
	return d.Span_
}

func (d *ProcDecl) stmtNode() {}
func (d *ProcDecl) declNode() {}

// StructDecl is a struct type definition.
//
//	Something_New :: struct {
//	    field1: String;
//	    field2: U64;
//	}
//
// The declaration itself does not require a trailing semicolon; each field
// ends with its own semicolon.
type StructDecl struct {
	Span_  Span
	Name   string
	Fields []StructField
}

func (d *StructDecl) nodeSpan() Span {
	return d.Span_
}

func (d *StructDecl) stmtNode() {}
func (d *StructDecl) declNode() {}

// StructField is a single field in a struct definition: "name: type;".
type StructField struct {
	Span_ Span
	Name  string
	Type  Expr // type expression
}

func (f StructField) nodeSpan() Span {
	return f.Span_
}

// ErrorDecl is an error type definition.
//
//	Hash_Table_Error :: error {
//	    GENERIC;
//	    OUT_OF_MEMORY;
//	    NOT_FOUND;
//	    OUT_OF_BOUNDS;
//	}
//
// Each member is a distinct error value of this type, numbered sequentially
// from 0 in declaration order. Members cannot be assigned explicit values.
// The declaration itself does not require a trailing semicolon; each member
// ends with its own semicolon.
type ErrorDecl struct {
	Span_   Span
	Name    string
	Members []ErrorMember
}

func (d *ErrorDecl) nodeSpan() Span {
	return d.Span_
}

func (d *ErrorDecl) stmtNode() {}
func (d *ErrorDecl) declNode() {}

// ErrorMember is a single error value in an error type definition: "NAME;".
type ErrorMember struct {
	Span_ Span
	Name  string
}

func (m ErrorMember) nodeSpan() Span {
	return m.Span_
}

// ErrorMemberExpr is a reference to an error value: "Type.MEMBER!" or the
// bare ".MEMBER!" form. TypeName is empty for the bare form, whose type is
// inferred from the surrounding declaration. Bang is true when the reference
// is an error literal, which requires the trailing '!' (for example
// "Some_Error.GENERIC!"); variables holding error values do not use the bang.
type ErrorMemberExpr struct {
	Span_    Span
	TypeName string // error type name (empty for the bare ".MEMBER!" form)
	Name     string // member name
	Bang     bool   // error literal instantiation ('!' suffix)
}

func (e *ErrorMemberExpr) nodeSpan() Span {
	return e.Span_
}

func (e *ErrorMemberExpr) exprNode() {}

// Param is a single procedure parameter.
type Param struct {
	Span_ Span
	Name  string
	Type  Expr // type expression
}

func (p Param) nodeSpan() Span {
	return p.Span_
}

// Program is the root of the AST. It holds a list of top-level declarations.
// Entry is the name of the procedure selected by a '#entry' directive, or ""
// when the source declares no entry point.
type Program struct {
	Decls []Decl
	Entry string
}

func (p *Program) nodeSpan() Span {
	if len(p.Decls) == 0 {
		return Span{}
	}
	return spanUnion(p.Decls[0].nodeSpan(), p.Decls[len(p.Decls)-1].nodeSpan())
}

// ExprStmt is a statement that wraps an expression (e.g., a call used as a
// statement).
type ExprStmt struct {
	Span_ Span
	Expr  Expr
}

func (s *ExprStmt) nodeSpan() Span {
	return s.Span_
}

func (s *ExprStmt) stmtNode() {}
