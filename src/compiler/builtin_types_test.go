package compiler

import "testing"

func TestBuiltinTypeRegistryIsCompleteAndConsistent(t *testing.T) {
	seen := make(map[string]bool)
	for _, info := range BuiltinTypes() {
		if info.Name == "" || seen[info.Name] {
			t.Fatalf("invalid or duplicate built-in metadata: %+v", info)
		}
		seen[info.Name] = true
		lookup, ok := LookupBuiltinType(info.Name)
		if !ok || lookup != info {
			t.Fatalf("LookupBuiltinType(%q) = %+v, %v; want %+v", info.Name, lookup, ok, info)
		}
		if (info.Kind == BuiltinInteger || info.Kind == BuiltinFloat) && info.Bits == 0 {
			t.Errorf("numeric built-in %s has no width", info.Name)
		}
	}
	if len(BuiltinTypeNames()) != len(seen) {
		t.Fatalf("BuiltinTypeNames length = %d, registry length = %d", len(BuiltinTypeNames()), len(seen))
	}
	for _, name := range []string{"S8", "U128", "Size", "Byte", "F32", "F64", "Bool", "String", "Void"} {
		if !seen[name] {
			t.Errorf("missing built-in %s", name)
		}
	}
	for _, name := range []string{"F16", "F128"} {
		info, _ := LookupBuiltinType(name)
		if info.FasmSupported {
			t.Errorf("%s unexpectedly marked as fasm-supported", name)
		}
	}
}
