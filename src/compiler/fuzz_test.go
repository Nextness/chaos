package compiler

import (
	"bytes"
	"testing"
)

func FuzzFrontendNoPanic(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("#entry main :: proc -> S64 { return 0; }"),
		[]byte("E :: enum { A: U8 = 255; B; }"),
		{0xff, 'x', ':', ':', 0xe2, 0x82},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		if len(source) > 64*1024 {
			t.Skip()
		}
		tokens, _ := Tokenize(source, 0)
		strict := ParseProgram(tokens)
		_ = DumpTokens(tokens)
		_ = DumpParseResult(strict)
		tolerant := ParseProgramTolerant(tokens)
		_ = DumpParseResult(tolerant)
		if tolerant.Program != nil {
			_, _ = AnalyzeProgram(tolerant.Program)
		}
	})
}

func FuzzSpanAndDiagnosticNoPanic(f *testing.F) {
	f.Add([]byte("alpha\nβeta\n"), -1, 200)
	f.Add([]byte{0xff, '\n', 'x'}, 2, 1)
	f.Fuzz(func(t *testing.T, source []byte, start, end int) {
		if len(source) > 64*1024 {
			t.Skip()
		}
		sm := &SourceManager{}
		id := sm.Register("fuzz.chaos", source)
		sf := sm.Lookup(id)
		span := Span{File: id, Start: start, End: end}
		_, _ = ClampSpan(span, sf)
		_ = SpanToRange(span, sf)
		var output bytes.Buffer
		Diagnostic{Severity: SeverityError, Span: span, Message: "fuzz"}.Render(&output, sf)
	})
}

func FuzzVerifyMIRNoPanic(f *testing.F) {
	f.Add(byte(MIRConst), byte(MIRReturn), int16(0), int16(0))
	f.Add(byte(255), byte(255), int16(-1), int16(999))
	f.Fuzz(func(t *testing.T, opcode, terminator byte, result, typeID int16) {
		types := NewTypeTable()
		symbols := NewSymbolTable()
		fn := &MIRFunction{
			Symbol:     symbols.Declare("f"),
			Name:       "f",
			ResultType: types.Void(),
			Blocks: []*MIRBlock{{
				ID: 0,
				Instrs: []*MIRInstr{{
					Result: ValueID(result), Op: MIROpcode(opcode), Type: TypeID(typeID),
				}},
				Term: MIRTerminator{Kind: MIRTermKind(terminator)},
			}},
		}
		_ = VerifyMIR(&MIRProgram{Symbols: symbols, Types: types, Functions: []*MIRFunction{fn}})
	})
}
