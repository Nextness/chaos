package compiler

import "testing"

func TestMIRPointerOps(t *testing.T) {
	_, mir := lowerSource(t, "#entry main :: proc -> S64 {\n    a := 10;\n    p: *S64 = *a;\n    v := p.*;\n    p.* = 11;\n    q := p + 1;\n    if v == q.* { return 0; }\n    return 1;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("proc main not found")
	}
	if findInstr(fn, MIRAddrOf) == nil {
		t.Error("expected an addr.of instruction for '*a'")
	}
	if findInstr(fn, MIRDerefLoad) == nil {
		t.Error("expected a deref.load instruction for 'p.*'")
	}
	if findInstr(fn, MIRDerefStore) == nil {
		t.Error("expected a deref.store instruction for 'p.* = 11'")
	}
	if findInstr(fn, MIRPtrAdd) == nil {
		t.Error("expected a ptr.add instruction for 'p + 1'")
	}
}

func TestMIRPointerNullConst(t *testing.T) {
	// The null literal lowers to a zero constant of the nullable pointer
	// type, and an initialized nullable binding stores it.
	_, mir := lowerSource(t, "#entry main :: proc -> S64 {\n\tp: *S64? = null;\n\tif p == null { return 0; }\n\treturn 1;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("proc main not found")
	}
	c := findInstr(fn, MIRConst)
	if c == nil {
		t.Fatal("no const instruction found")
	}
	if kind := mir.Types.Lookup(c.Type).Kind; kind != TypeKindPointer {
		t.Errorf("null const type kind = %v, want TypeKindPointer", kind)
	}
	if c.Imm.Kind != MIRImmInt || c.Imm.Int != 0 {
		t.Errorf("null const immediate = %#v, want integer 0", c.Imm)
	}
}

func TestMIRPointerFieldAddr(t *testing.T) {
	// "pp.*.age = 9" produces a field.addr over the pointer value.
	_, mir := lowerSource(t, "Person :: struct { name: String; age: S64; }\n#entry main :: proc -> S64 {\n    person := Person.{name=«x», age=7};\n    pp: *Person = *person;\n    pp.*.age = 9;\n    return person.age;\n}")
	fn := findMIRFunction(mir, "main")
	if fn == nil {
		t.Fatal("proc main not found")
	}
	if findInstr(fn, MIRFieldAddr) == nil {
		t.Error("expected a field.addr instruction for 'pp.*.age = 9'")
	}
	if findInstr(fn, MIRDerefStore) == nil {
		t.Error("expected a deref.store instruction for the field write")
	}
}
