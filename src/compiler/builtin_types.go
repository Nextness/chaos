package compiler

// BuiltinTypeKind is the target-independent semantic category of a built-in
// Chaos type.
type BuiltinTypeKind uint8

const (
	BuiltinVoid BuiltinTypeKind = iota
	BuiltinBool
	BuiltinString
	BuiltinInteger
	BuiltinFloat
	BuiltinAddr
)

// BuiltinTypeInfo is the compiler-owned source of truth shared by semantic
// analysis, IR construction, target validation, layout, and editor tooling.
// Bits and Signed describe numeric types; they are zero/false otherwise.
type BuiltinTypeInfo struct {
	Name          string
	Kind          BuiltinTypeKind
	Bits          int
	Signed        bool
	FasmSupported bool
}

var builtinTypes = []BuiltinTypeInfo{
	{Name: "Void", Kind: BuiltinVoid, FasmSupported: true},
	{Name: "Bool", Kind: BuiltinBool, Bits: 8, FasmSupported: true},
	{Name: "String", Kind: BuiltinString, FasmSupported: true},
	{Name: "S8", Kind: BuiltinInteger, Bits: 8, Signed: true, FasmSupported: true},
	{Name: "S16", Kind: BuiltinInteger, Bits: 16, Signed: true, FasmSupported: true},
	{Name: "S32", Kind: BuiltinInteger, Bits: 32, Signed: true, FasmSupported: true},
	{Name: "S64", Kind: BuiltinInteger, Bits: 64, Signed: true, FasmSupported: true},
	{Name: "S128", Kind: BuiltinInteger, Bits: 128, Signed: true, FasmSupported: true},
	{Name: "U8", Kind: BuiltinInteger, Bits: 8, FasmSupported: true},
	{Name: "U16", Kind: BuiltinInteger, Bits: 16, FasmSupported: true},
	{Name: "U32", Kind: BuiltinInteger, Bits: 32, FasmSupported: true},
	{Name: "U64", Kind: BuiltinInteger, Bits: 64, FasmSupported: true},
	{Name: "U128", Kind: BuiltinInteger, Bits: 128, FasmSupported: true},
	{Name: "Size", Kind: BuiltinInteger, Bits: 64, FasmSupported: true},
	{Name: "Byte", Kind: BuiltinInteger, Bits: 8, FasmSupported: true},
	{Name: "F16", Kind: BuiltinFloat, Bits: 16},
	{Name: "F32", Kind: BuiltinFloat, Bits: 32, FasmSupported: true},
	{Name: "F64", Kind: BuiltinFloat, Bits: 64, FasmSupported: true},
	{Name: "F128", Kind: BuiltinFloat, Bits: 128},
	{Name: "Addr", Kind: BuiltinAddr, FasmSupported: true},
}

var builtinTypeByName = func() map[string]BuiltinTypeInfo {
	result := make(map[string]BuiltinTypeInfo, len(builtinTypes))
	for _, info := range builtinTypes {
		result[info.Name] = info
	}
	return result
}()

// BuiltinTypes returns the canonical ordered built-in type registry.
func BuiltinTypes() []BuiltinTypeInfo {
	return append([]BuiltinTypeInfo(nil), builtinTypes...)
}

// LookupBuiltinType returns metadata for a built-in type name.
func LookupBuiltinType(name string) (BuiltinTypeInfo, bool) {
	info, ok := builtinTypeByName[name]
	return info, ok
}

// BuiltinTypeNames returns the canonical compiler-owned names for editor and
// tooling consumers.
func BuiltinTypeNames() []string {
	names := make([]string, len(builtinTypes))
	for i, info := range builtinTypes {
		names[i] = info.Name
	}
	return names
}

func isBuiltinTypeName(name string) bool {
	_, ok := builtinTypeByName[name]
	return ok
}

type builtinProcKind uint8

const (
	builtinPrint builtinProcKind = iota
	builtinPrintln
	builtinReadFile
	builtinFileExists
)

// BuiltinProcInfo is the shared signature contract for a compiler-provided
// procedure. Semantic checking and lowering dispatch use this registry.
type BuiltinProcInfo struct {
	Name   string
	Params []string
	Result string
	kind   builtinProcKind
}

var builtinProcs = []BuiltinProcInfo{
	{Name: "print", Params: []string{"String"}, Result: "Void", kind: builtinPrint},
	{Name: "println", Params: []string{"String"}, Result: "Void", kind: builtinPrintln},
	{Name: "read_file", Params: []string{"String"}, Result: "String", kind: builtinReadFile},
	{Name: "file_exists", Params: []string{"String"}, Result: "Bool", kind: builtinFileExists},
}

var builtinProcByName = func() map[string]BuiltinProcInfo {
	result := make(map[string]BuiltinProcInfo, len(builtinProcs))
	for _, info := range builtinProcs {
		result[info.Name] = info
	}
	return result
}()

func BuiltinProcedures() []BuiltinProcInfo {
	result := make([]BuiltinProcInfo, len(builtinProcs))
	for i, info := range builtinProcs {
		result[i] = info
		result[i].Params = append([]string(nil), info.Params...)
	}
	return result
}

func LookupBuiltinProcedure(name string) (BuiltinProcInfo, bool) {
	info, ok := builtinProcByName[name]
	if ok {
		info.Params = append([]string(nil), info.Params...)
	}
	return info, ok
}
