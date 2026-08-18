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

// Len returns the number of declared symbols. Verifiers use it to reject
// invalid SymbolIDs without treating Lookup's empty-string fallback as real.
func (st *SymbolTable) Len() int { return len(st.names) }

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
	TypeKindEnum
	TypeKindTuple
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
// types carry their field list; array types carry their element type; enum
// types carry their underlying integer type.
type IRType struct {
	ID         TypeID
	Kind       TypeKind
	Name       string
	Fields     []TypeField
	Elem       TypeID // element type for TypeKindArray
	Underlying TypeID // underlying integer type for TypeKindEnum
}

// TypeTable interns types by name.
type TypeTable struct {
	types  []IRType
	byName map[string]TypeID
}

// NewTypeTable returns a type table pre-populated with the built-in types.
func NewTypeTable() *TypeTable {
	tt := &TypeTable{byName: make(map[string]TypeID)}
	for _, info := range BuiltinTypes() {
		kind := TypeKindUnknown
		switch info.Kind {
		case BuiltinVoid:
			kind = TypeKindVoid
		case BuiltinBool:
			kind = TypeKindBool
		case BuiltinString:
			kind = TypeKindString
		case BuiltinInteger:
			kind = TypeKindInt
		case BuiltinFloat:
			kind = TypeKindFloat
		}
		tt.intern(info.Name, kind)
	}
	tt.intern("", TypeKindUnknown)
	return tt
}

func (tt *TypeTable) intern(name string, kind TypeKind) TypeID {
	id := TypeID(len(tt.types))
	tt.types = append(tt.types, IRType{ID: id, Kind: kind, Name: name})
	tt.byName[name] = id
	return id
}

// InternStruct interns a struct type name with no fields yet. Fields are set
// later with SetStructFields once every type name is known.
func (tt *TypeTable) InternStruct(name string) TypeID {
	id, ok := tt.TryInternStruct(name)
	if !ok {
		return tt.Unknown()
	}
	return id
}

// TryInternStruct reports a cross-kind name collision instead of returning an
// unrelated existing type as though it were a struct.
func (tt *TypeTable) TryInternStruct(name string) (TypeID, bool) {
	if id, ok := tt.byName[name]; ok {
		return id, tt.types[id].Kind == TypeKindStruct
	}
	return tt.intern(name, TypeKindStruct), true
}

// InternScopedStruct always creates a fresh nominal type. Lexically shadowed
// declarations may share source spelling, but never type identity.
func (tt *TypeTable) InternScopedStruct(name string) TypeID {
	return tt.internUnindexed(name, TypeKindStruct)
}

func (tt *TypeTable) internUnindexed(name string, kind TypeKind) TypeID {
	id := TypeID(len(tt.types))
	tt.types = append(tt.types, IRType{ID: id, Kind: kind, Name: name})
	return id
}

// SetStructFields sets the field list of a struct type.
func (tt *TypeTable) SetStructFields(id TypeID, fields []TypeField) {
	if int(id) < len(tt.types) && (tt.types[id].Kind == TypeKindStruct || tt.types[id].Kind == TypeKindTuple) {
		tt.types[id].Fields = fields
	}
}

// InternError interns an error type name. Error values are nominal: each
// declared error type is its own type, backed by a 16-bit ordinal.
func (tt *TypeTable) InternError(name string) TypeID {
	id, ok := tt.TryInternError(name)
	if !ok {
		return tt.Unknown()
	}
	return id
}

func (tt *TypeTable) TryInternError(name string) (TypeID, bool) {
	if id, ok := tt.byName[name]; ok {
		return id, tt.types[id].Kind == TypeKindError
	}
	return tt.intern(name, TypeKindError), true
}

func (tt *TypeTable) InternScopedError(name string) TypeID {
	return tt.internUnindexed(name, TypeKindError)
}

// InternArray interns an array type "[]Elem" for the given element type.
func (tt *TypeTable) InternArray(elem TypeID) TypeID {
	name := "[]" + tt.Lookup(elem).Name
	if id, ok := tt.byName[name]; ok {
		if tt.types[id].Kind == TypeKindArray && tt.types[id].Elem == elem {
			return id
		}
		return tt.Unknown()
	}
	id := tt.intern(name, TypeKindArray)
	tt.types[id].Elem = elem
	return id
}

// InternEnum interns an enum type with the given name and underlying integer
// type.
func (tt *TypeTable) InternEnum(name string, underlying TypeID) TypeID {
	id, ok := tt.TryInternEnum(name, underlying)
	if !ok {
		return tt.Unknown()
	}
	return id
}

func (tt *TypeTable) TryInternEnum(name string, underlying TypeID) (TypeID, bool) {
	if id, ok := tt.byName[name]; ok {
		return id, tt.types[id].Kind == TypeKindEnum && tt.types[id].Underlying == underlying
	}
	id := tt.intern(name, TypeKindEnum)
	tt.types[id].Underlying = underlying
	return id, true
}

func (tt *TypeTable) InternScopedEnum(name string, underlying TypeID) TypeID {
	id := tt.internUnindexed(name, TypeKindEnum)
	tt.types[id].Underlying = underlying
	return id
}

// InternTuple interns the structural product used to carry multiple results.
// Field symbols are supplied by lowering so tuple construction and extraction
// use the same declaration-order identity as structs.
func (tt *TypeTable) InternTuple(name string, fields []TypeField) TypeID {
	id, ok := tt.TryInternTuple(name, fields)
	if !ok {
		return tt.Unknown()
	}
	return id
}

func (tt *TypeTable) TryInternTuple(name string, fields []TypeField) (TypeID, bool) {
	if id, ok := tt.byName[name]; ok {
		return id, tt.types[id].Kind == TypeKindTuple
	}
	id := tt.intern(name, TypeKindTuple)
	tt.types[id].Fields = fields
	return id, true
}

// Len returns the number of interned types. It is used by MIR verification to
// reject invalid TypeIDs without relying on Lookup's poison fallback.
func (tt *TypeTable) Len() int { return len(tt.types) }

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
