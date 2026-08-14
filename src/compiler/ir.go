// Shared IR infrastructure for the Chaos compiler.
//
// The IR is built in two levels:
//
//   - HIR (hir.go): a typed, source-close form with resolved symbols and
//     types, structured control flow, and source spans. It is suitable for
//     diagnostics and compile-time evaluation.
//   - MIR (mir.go): a three-address control-flow form with basic blocks,
//     typed values, and explicit terminators. It is suitable for lowering to
//     assembly and for verification.
//
// This file defines the tables shared by both levels: symbols are interned
// into stable SymbolIDs and types into TypeIDs.
package compiler

// SymbolID identifies a declared entity (procedure, struct, variable,
// parameter, or field) in the IR. Every declaration receives a fresh ID, so
// identity is stable across shadowing and lowering.
type SymbolID uint32

// TypeID identifies a type in the TypeTable.
type TypeID uint32

// ValueID identifies a value produced by a MIR instruction within a function.
type ValueID uint32

// BlockID identifies a basic block within a MIR function.
type BlockID uint32

// LocalID identifies a local slot (parameter or declared variable) within a
// MIR function.
type LocalID uint32

// NoValue is the sentinel ValueID used where a MIR terminator or instruction
// has no value (for example a void return).
const NoValue ValueID = ^ValueID(0)

// SymbolTable interns declaration names into stable SymbolIDs.
type SymbolTable struct {
	names  []string
	byName map[string]SymbolID
}

// NewSymbolTable returns an empty symbol table.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{byName: make(map[string]SymbolID)}
}

// Declare interns a new declaration name and returns a fresh SymbolID. Every
// call returns a distinct ID even for the same name, so shadowed declarations
// keep separate identities.
func (st *SymbolTable) Declare(name string) SymbolID {
	id := SymbolID(len(st.names))
	st.names = append(st.names, name)
	st.byName[name] = id
	return id
}

// Lookup returns the name for a SymbolID, or "" for an out-of-range ID.
func (st *SymbolTable) Lookup(id SymbolID) string {
	if int(id) < len(st.names) {
		return st.names[id]
	}
	return ""
}

// ByName returns the most recently declared SymbolID for a name, or false if
// the name was never declared.
func (st *SymbolTable) ByName(name string) (SymbolID, bool) {
	id, ok := st.byName[name]
	return id, ok
}

// TypeKind classifies a resolved IR type.
type TypeKind uint8

const (
	TypeKindVoid TypeKind = iota
	TypeKindBool
	TypeKindString
	TypeKindInt
	TypeKindFloat
	TypeKindStruct
	TypeKindError
	TypeKindArray
	TypeKindUnknown
)

// TypeField is a single field of a struct type. Symbol is the field's
// SymbolID, assigned when the struct declaration is lowered.
type TypeField struct {
	Symbol SymbolID
	Name   string
	Type   TypeID
}

// IRType is a resolved type in the IR. Primitives carry only a name; struct
// types carry their field list; array types carry their element type.
type IRType struct {
	Kind   TypeKind
	Name   string
	Fields []TypeField
	Elem   TypeID // element type for TypeKindArray
}

// TypeTable interns types by name.
type TypeTable struct {
	types  []IRType
	byName map[string]TypeID
}

// NewTypeTable returns a type table pre-populated with the built-in types.
func NewTypeTable() *TypeTable {
	tt := &TypeTable{byName: make(map[string]TypeID)}
	tt.intern("Void", TypeKindVoid)
	tt.intern("Bool", TypeKindBool)
	tt.intern("String", TypeKindString)
	for _, n := range []string{"S8", "S16", "S32", "S64", "S128", "U8", "U16", "U32", "U64", "U128", "Size", "Byte"} {
		tt.intern(n, TypeKindInt)
	}
	for _, n := range []string{"F16", "F32", "F64", "F128"} {
		tt.intern(n, TypeKindFloat)
	}
	tt.intern("", TypeKindUnknown)
	return tt
}

func (tt *TypeTable) intern(name string, kind TypeKind) TypeID {
	id := TypeID(len(tt.types))
	tt.types = append(tt.types, IRType{Kind: kind, Name: name})
	tt.byName[name] = id
	return id
}

// InternStruct interns a struct type name with no fields yet. Fields are set
// later with SetStructFields once every type name is known.
func (tt *TypeTable) InternStruct(name string) TypeID {
	if id, ok := tt.byName[name]; ok {
		return id
	}
	return tt.intern(name, TypeKindStruct)
}

// SetStructFields sets the field list of a struct type.
func (tt *TypeTable) SetStructFields(id TypeID, fields []TypeField) {
	if int(id) < len(tt.types) {
		tt.types[id].Fields = fields
	}
}

// InternError interns an error type name. Error values are nominal: each
// declared error type is its own type, backed by a 16-bit ordinal.
func (tt *TypeTable) InternError(name string) TypeID {
	if id, ok := tt.byName[name]; ok {
		return id
	}
	return tt.intern(name, TypeKindError)
}

// InternArray interns an array type "[]Elem" for the given element type.
func (tt *TypeTable) InternArray(elem TypeID) TypeID {
	name := "[]" + tt.Lookup(elem).Name
	if id, ok := tt.byName[name]; ok {
		return id
	}
	id := tt.intern(name, TypeKindArray)
	tt.types[id].Elem = elem
	return id
}

// Lookup returns the IRType for a TypeID, or an unknown type for an
// out-of-range ID.
func (tt *TypeTable) Lookup(id TypeID) IRType {
	if int(id) < len(tt.types) {
		return tt.types[id]
	}
	return IRType{Kind: TypeKindUnknown}
}

// ByName returns the TypeID for a type name, or false if the name is not a
// known type.
func (tt *TypeTable) ByName(name string) (TypeID, bool) {
	id, ok := tt.byName[name]
	return id, ok
}

// Void returns the TypeID of the Void type.
func (tt *TypeTable) Void() TypeID { return tt.byName["Void"] }

// Bool returns the TypeID of the Bool type.
func (tt *TypeTable) Bool() TypeID { return tt.byName["Bool"] }

// String returns the TypeID of the String type.
func (tt *TypeTable) String() TypeID { return tt.byName["String"] }

// S64 returns the TypeID of the S64 type.
func (tt *TypeTable) S64() TypeID { return tt.byName["S64"] }

// F64 returns the TypeID of the F64 type.
func (tt *TypeTable) F64() TypeID { return tt.byName["F64"] }

// Unknown returns the TypeID of the unknown type used for unresolved values.
func (tt *TypeTable) Unknown() TypeID { return tt.byName[""] }
