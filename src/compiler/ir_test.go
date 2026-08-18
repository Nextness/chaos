package compiler

import "testing"

func TestSymbolTableDeclareAndLookup(t *testing.T) {
	st := NewSymbolTable()
	a := st.Declare("alpha")
	b := st.Declare("beta")
	if a == b {
		t.Errorf("distinct declarations share SymbolID %d", a)
	}
	if got := st.Lookup(a); got != "alpha" {
		t.Errorf("Lookup(%d) = %q, want %q", a, got, "alpha")
	}
	if got := st.Lookup(b); got != "beta" {
		t.Errorf("Lookup(%d) = %q, want %q", b, got, "beta")
	}
	if got := st.Lookup(SymbolID(999)); got != "" {
		t.Errorf("Lookup(out of range) = %q, want empty", got)
	}
}

func TestSymbolTableShadowing(t *testing.T) {
	st := NewSymbolTable()
	outer := st.Declare("x")
	inner := st.Declare("x")
	if outer == inner {
		t.Errorf("shadowed declarations share SymbolID %d", outer)
	}
	// ByName returns the most recent declaration.
	if got, ok := st.ByName("x"); !ok || got != inner {
		t.Errorf("ByName(x) = %d, %v; want %d, true", got, ok, inner)
	}
	if _, ok := st.ByName("missing"); ok {
		t.Errorf("ByName(missing) reported a match")
	}
}

func TestTypeTableBuiltins(t *testing.T) {
	tt := NewTypeTable()
	if tt.Lookup(tt.S64()).Kind != TypeKindInt {
		t.Errorf("S64 kind = %v, want int", tt.Lookup(tt.S64()).Kind)
	}
	if tt.Lookup(tt.F64()).Kind != TypeKindFloat {
		t.Errorf("F64 kind = %v, want float", tt.Lookup(tt.F64()).Kind)
	}
	if tt.Lookup(tt.String()).Kind != TypeKindString {
		t.Errorf("String kind = %v, want string", tt.Lookup(tt.String()).Kind)
	}
	if tt.Lookup(tt.Bool()).Kind != TypeKindBool {
		t.Errorf("Bool kind = %v, want bool", tt.Lookup(tt.Bool()).Kind)
	}
	if tt.Lookup(tt.Void()).Kind != TypeKindVoid {
		t.Errorf("Void kind = %v, want void", tt.Lookup(tt.Void()).Kind)
	}
	if tt.Lookup(tt.Unknown()).Kind != TypeKindUnknown {
		t.Errorf("Unknown kind = %v, want unknown", tt.Lookup(tt.Unknown()).Kind)
	}
}

func TestTypeTableStruct(t *testing.T) {
	tt := NewTypeTable()
	id := tt.InternStruct("Point")
	if tt.Lookup(id).Kind != TypeKindStruct {
		t.Errorf("struct kind = %v, want struct", tt.Lookup(id).Kind)
	}
	// Interning the same name returns the same ID.
	if again := tt.InternStruct("Point"); again != id {
		t.Errorf("InternStruct(Point) = %d, want %d", again, id)
	}
	tt.SetStructFields(id, []TypeField{
		{Name: "x", Type: tt.S64()},
		{Name: "y", Type: tt.S64()},
	})
	st := tt.Lookup(id)
	if len(st.Fields) != 2 || st.Fields[0].Name != "x" || st.Fields[1].Name != "y" {
		t.Errorf("struct fields = %+v, want x and y", st.Fields)
	}
	if tt.Lookup(TypeID(999)).Kind != TypeKindUnknown {
		t.Errorf("Lookup(out of range) kind = %v, want unknown", tt.Lookup(TypeID(999)).Kind)
	}
}

func TestTypeTableNominalInterningReportsCrossKindCollisions(t *testing.T) {
	tt := NewTypeTable()
	if id, ok := tt.TryInternStruct("S64"); ok || id != tt.S64() {
		t.Fatalf("TryInternStruct(S64) = %d, %v; want existing ID and collision", id, ok)
	}
	tt.SetStructFields(tt.S64(), []TypeField{{Name: "corruption", Type: tt.S64()}})
	if fields := tt.Lookup(tt.S64()).Fields; len(fields) != 0 {
		t.Fatalf("SetStructFields mutated integer metadata: %+v", fields)
	}
	structID, ok := tt.TryInternStruct("Nominal")
	if !ok {
		t.Fatal("first struct interning failed")
	}
	if id, ok := tt.TryInternEnum("Nominal", tt.S64()); ok || id != structID {
		t.Fatalf("TryInternEnum collision = %d, %v; want %d, false", id, ok, structID)
	}
}
