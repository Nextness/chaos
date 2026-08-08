// Backend interface for the Chaos compiler.
//
// A backend consumes a verified MIR program and emits target assembly text.
// The MIR is target-independent, so multiple backends (fasm, gas, nasm, ...)
// can share the same lowering pipeline and differ only in how they render
// instructions, labels, and data.
package compiler

// Backend emits target assembly from a verified MIR program.
type Backend interface {
	// Name returns the backend's target name (for example "fasm").
	Name() string
	// Emit renders the MIR program as assembly text. Diagnostics report
	// constructs the backend does not yet support; when any diagnostic is an
	// error the returned text is incomplete and must not be assembled.
	Emit(prog *MIRProgram) (string, DiagnosticList)
}

// NewBackend returns a backend by name, or nil when the name is unknown.
func NewBackend(name string) Backend {
	switch name {
	case "fasm":
		return &FasmBackend{}
	}
	return nil
}
