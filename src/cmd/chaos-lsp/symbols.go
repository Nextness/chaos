package main

import (
	"encoding/json"

	"chaos_compiler/compiler"
)

// LSP DocumentSymbol kinds used by the server.
const (
	symbolKindFunction   = 12
	symbolKindVariable   = 13
	symbolKindConstant   = 14
	symbolKindEnum       = 10
	symbolKindEnumMember = 21
	symbolKindStruct     = 23
)

// nameSpan derives the byte span of a declaration or parameter name. Every
// decl/param span starts at its name token, so the name is
// [Span_.Start, Span_.Start + len(Name)).
func nameSpan(span compiler.Span, name string) compiler.Span {
	return compiler.Span{File: span.File, Start: span.Start, End: span.Start + len(name)}
}

// toLSPRange converts a compiler range to an LSP range.
func toLSPRange(r compiler.Range) Range {
	return Range{
		Start: Position{Line: r.Start.Line, Character: r.Start.Character},
		End:   Position{Line: r.End.Line, Character: r.End.Character},
	}
}

// documentSymbols walks the top-level declarations and produces the document
// outline.
func documentSymbols(program *compiler.Program, sf *compiler.SourceFile) []DocumentSymbol {
	var out []DocumentSymbol
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *compiler.ProcDecl:
			out = append(out, DocumentSymbol{
				Name:           d.Name,
				Kind:           symbolKindFunction,
				Range:          toLSPRange(compiler.SpanToRange(d.Span_, sf)),
				SelectionRange: toLSPRange(compiler.SpanToRange(d.NameSpan, sf)),
			})
		case *compiler.VarDecl:
			kind := symbolKindVariable
			if d.CompileTime {
				kind = symbolKindConstant
			}
			out = append(out, DocumentSymbol{
				Name:           d.Name,
				Kind:           kind,
				Range:          toLSPRange(compiler.SpanToRange(d.Span_, sf)),
				SelectionRange: toLSPRange(compiler.SpanToRange(d.NameSpan, sf)),
			})
		case *compiler.StructDecl:
			var children []DocumentSymbol
			for _, field := range d.Fields {
				children = append(children, DocumentSymbol{
					Name:           field.Name,
					Kind:           symbolKindVariable,
					Range:          toLSPRange(compiler.SpanToRange(field.Span_, sf)),
					SelectionRange: toLSPRange(compiler.SpanToRange(field.NameSpan, sf)),
				})
			}
			out = append(out, DocumentSymbol{
				Name:           d.Name,
				Kind:           symbolKindStruct,
				Range:          toLSPRange(compiler.SpanToRange(d.Span_, sf)),
				SelectionRange: toLSPRange(compiler.SpanToRange(d.NameSpan, sf)),
				Children:       children,
			})
		case *compiler.ErrorDecl:
			var children []DocumentSymbol
			for _, member := range d.Members {
				children = append(children, DocumentSymbol{
					Name:           member.Name,
					Kind:           symbolKindEnumMember,
					Range:          toLSPRange(compiler.SpanToRange(member.Span_, sf)),
					SelectionRange: toLSPRange(compiler.SpanToRange(member.NameSpan, sf)),
				})
			}
			out = append(out, DocumentSymbol{
				Name:           d.Name,
				Kind:           symbolKindEnum,
				Range:          toLSPRange(compiler.SpanToRange(d.Span_, sf)),
				SelectionRange: toLSPRange(compiler.SpanToRange(d.NameSpan, sf)),
				Children:       children,
			})
		case *compiler.EnumDecl:
			var children []DocumentSymbol
			for _, member := range d.Members {
				children = append(children, DocumentSymbol{
					Name:           member.Name,
					Kind:           symbolKindEnumMember,
					Range:          toLSPRange(compiler.SpanToRange(member.Span_, sf)),
					SelectionRange: toLSPRange(compiler.SpanToRange(member.NameSpan, sf)),
				})
			}
			out = append(out, DocumentSymbol{
				Name:           d.Name,
				Kind:           symbolKindEnum,
				Range:          toLSPRange(compiler.SpanToRange(d.Span_, sf)),
				SelectionRange: toLSPRange(compiler.SpanToRange(d.NameSpan, sf)),
				Children:       children,
			})
		}
	}
	return out
}

// handleDocumentSymbol handles textDocument/documentSymbol.
func (s *Server) handleDocumentSymbol(msg message) Response {
	var params DocumentSymbolParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return errorResponse(msg.ID, -32602, "invalid params")
	}
	s.mu.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		return errorResponse(msg.ID, -32603, "document not open")
	}
	out, err := json.Marshal(doc.symbols)
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}
