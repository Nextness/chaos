package main

import (
	"encoding/json"
	"sort"

	"chaos_new/compiler"
)

// Semantic token type indices into the legend declared in initialize.
const (
	semTypeKeyword    = 0
	semTypeNumber     = 1
	semTypeString     = 2
	semTypeComment    = 3
	semTypeFunction   = 4
	semTypeVariable   = 5
	semTypeParameter  = 6
	semTypeType       = 7
	semTypeConstant   = 8
	semTypeDelimiter  = 9
)

// semanticToken is one token to be delta-encoded. Positions and lengths are in
// UTF-16 code units.
type semanticToken struct {
	line      int
	startChar int
	length    int
	typeIndex int
}

// utf16LineLength returns the UTF-16 length of the line starting at lineStart,
// excluding the trailing newline and carriage return.
func utf16LineLength(source []byte, lineStart int) int {
	lineEnd := lineStart
	for lineEnd < len(source) && source[lineEnd] != '\n' && source[lineEnd] != '\r' {
		lineEnd++
	}
	return compiler.ByteOffsetToUTF16(source, lineStart, lineEnd)
}

// spanToTokenRows converts a span to one or more semantic token rows. A
// multiline span is split into per-line rows so each row stays on one line.
func spanToTokenRows(span compiler.Span, sf *compiler.SourceFile, typeIndex int) []semanticToken {
	r := compiler.SpanToRange(span, sf)
	if r.Start.Line == r.End.Line {
		return []semanticToken{{line: r.Start.Line, startChar: r.Start.Character, length: r.End.Character - r.Start.Character, typeIndex: typeIndex}}
	}
	var out []semanticToken
	firstLineEnd := utf16LineLength(sf.Source, sf.LineOffsets[r.Start.Line])
	out = append(out, semanticToken{line: r.Start.Line, startChar: r.Start.Character, length: firstLineEnd - r.Start.Character, typeIndex: typeIndex})
	for line := r.Start.Line + 1; line < r.End.Line; line++ {
		out = append(out, semanticToken{line: line, startChar: 0, length: utf16LineLength(sf.Source, sf.LineOffsets[line]), typeIndex: typeIndex})
	}
	out = append(out, semanticToken{line: r.End.Line, startChar: 0, length: r.End.Character, typeIndex: typeIndex})
	return out
}

// tokenSemanticTokens collects keyword/number/string/comment tokens from the
// tokenizer output.
func tokenSemanticTokens(tokens compiler.TokenList, sf *compiler.SourceFile) []semanticToken {
	var out []semanticToken
	for _, tok := range tokens {
		idx := -1
		switch tok.Kind {
		case compiler.TkExit, compiler.TkIf, compiler.TkElif, compiler.TkElse,
			compiler.TkProc, compiler.TkThen, compiler.TkReturn, compiler.TkAs,
			compiler.TkStruct, compiler.TkErrorKw, compiler.TkUnless, compiler.TkCatch,
			compiler.TkTrue, compiler.TkFalse,
			compiler.TkHash, compiler.TkDirec:
			idx = semTypeKeyword
		case compiler.TkInt, compiler.TkFloat:
			idx = semTypeNumber
		case compiler.TkString:
			idx = semTypeString
		case compiler.TkComment:
			idx = semTypeComment
		case compiler.TkLBrace, compiler.TkRBrace,
			compiler.TkLParen, compiler.TkRParen,
			compiler.TkLBracket, compiler.TkRBracket,
			compiler.TkErrorReturn:
			idx = semTypeDelimiter
		}
		if idx < 0 {
			continue
		}
		out = append(out, spanToTokenRows(tok.Span, sf, idx)...)
	}
	return out
}

// builtinTypes are the primitive type names that exist in the language. Only
// these and declared struct names are highlighted as types; undeclared
// identifiers in type position are left unhighlighted. Array and Map are
// intentionally absent (handled later).
var builtinTypes = map[string]bool{
	"S8": true, "S16": true, "S32": true, "S64": true, "S128": true,
	"U8": true, "U16": true, "U32": true, "U64": true, "U128": true,
	"Size": true,
	"F16": true, "F32": true, "F64": true, "F128": true,
	"C64": true, "C128": true,
	"Q128": true, "Q256": true,
	"String": true, "Byte": true, "Void": true, "Addr": true, "Bool": true,
	"Error": true, "Any": true, "Self": true, "Type": true, "Named_Scope": true,
}

// knownTypeNames returns the set of type names that exist in the program:
// built-in primitive types plus declared struct and error type names.
func knownTypeNames(program *compiler.Program) map[string]bool {
	types := make(map[string]bool, len(builtinTypes))
	for name := range builtinTypes {
		types[name] = true
	}
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *compiler.StructDecl:
			types[d.Name] = true
		case *compiler.ErrorDecl:
			types[d.Name] = true
		}
	}
	return types
}

// typeToken emits a type token for an identifier type expression, but only if
// the name is a known type (built-in or declared struct). Undeclared
// identifiers in type position are not highlighted.
func typeToken(expr compiler.Expr, sf *compiler.SourceFile, known map[string]bool, out *[]semanticToken) {
	if ident, ok := expr.(*compiler.IdentExpr); ok && known[ident.Name] {
		*out = append(*out, spanToTokenRows(ident.Span_, sf, semTypeType)...)
	}
}

// astSemanticTokens collects function/variable/parameter/type/constant tokens
// by walking the AST.
func astSemanticTokens(program *compiler.Program, sf *compiler.SourceFile) []semanticToken {
	var out []semanticToken
	known := knownTypeNames(program)

	var walkExpr func(expr compiler.Expr)
	var walkStmt func(stmt compiler.Stmt)
	var walkBlock func(block *compiler.BlockStmt)
	var walkProc func(proc *compiler.ProcDecl)
	var walkStruct func(st *compiler.StructDecl)
	var walkError func(ed *compiler.ErrorDecl)

	walkExpr = func(expr compiler.Expr) {
		switch e := expr.(type) {
		case *compiler.IdentExpr:
			out = append(out, spanToTokenRows(e.Span_, sf, semTypeVariable)...)
		case *compiler.BinaryExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *compiler.UnaryExpr:
			walkExpr(e.Operand)
		case *compiler.CallExpr:
			if ident, ok := e.Func.(*compiler.IdentExpr); ok {
				out = append(out, spanToTokenRows(ident.Span_, sf, semTypeFunction)...)
			} else {
				walkExpr(e.Func)
			}
			for _, arg := range e.Args {
				walkExpr(arg)
			}
		case *compiler.ParenExpr:
			walkExpr(e.Inner)
		case *compiler.StructInitExpr:
			if e.Type != nil {
				typeToken(e.Type, sf, known, &out)
			}
			for _, field := range e.Fields {
				if field.Value != nil {
					walkExpr(field.Value)
				}
			}
		case *compiler.ErrorMemberExpr:
			if e.TypeName != "" {
				typeSpan := compiler.Span{File: e.Span_.File, Start: e.Span_.Start, End: e.Span_.Start + len(e.TypeName)}
				out = append(out, spanToTokenRows(typeSpan, sf, semTypeType)...)
			}
			bangLen := 0
			if e.Bang {
				bangLen = 1
			}
			// The constant highlight covers the member name and its '!'.
			memberSpan := compiler.Span{File: e.Span_.File, Start: e.Span_.End - len(e.Name) - bangLen, End: e.Span_.End}
			out = append(out, spanToTokenRows(memberSpan, sf, semTypeConstant)...)
		}
	}

	walkStmt = func(stmt compiler.Stmt) {
		switch s := stmt.(type) {
		case *compiler.VarDecl:
			// A '::' declaration is a variable (compile-time constant), not a
			// type. Type names are highlighted only when they are known
			// (built-in or declared struct), via typeToken.
			out = append(out, spanToTokenRows(nameSpan(s.Span_, s.Name), sf, semTypeVariable)...)
			typeToken(s.DeclType, sf, known, &out)
			if s.Init != nil {
				walkExpr(s.Init)
			}
		case *compiler.AssignStmt:
			out = append(out, spanToTokenRows(nameSpan(s.Span_, s.Name), sf, semTypeVariable)...)
			if s.Value != nil {
				walkExpr(s.Value)
			}
		case *compiler.ReturnStmt:
			if s.Value != nil {
				walkExpr(s.Value)
			}
		case *compiler.ExitStmt:
			if s.Status != nil {
				walkExpr(s.Status)
			}
			if s.Message != nil {
				walkExpr(s.Message)
			}
		case *compiler.IfStmt:
			walkExpr(s.Condition)
			walkBlock(s.Body)
			for _, elif := range s.Elif {
				walkExpr(elif.Condition)
				walkBlock(elif.Body)
			}
			if s.ElseBody != nil {
				walkBlock(s.ElseBody)
			}
		case *compiler.BlockStmt:
			walkBlock(s)
		case *compiler.ExprStmt:
			walkExpr(s.Expr)
		case *compiler.ProcDecl:
			walkProc(s)
		case *compiler.StructDecl:
			walkStruct(s)
		case *compiler.ErrorDecl:
			walkError(s)
		case *compiler.UnlessCatchStmt:
			walkExpr(s.Init)
			if s.CatchName != "" {
				out = append(out, spanToTokenRows(s.CatchNameSpan, sf, semTypeVariable)...)
			}
			walkBlock(s.CatchBody)
		case *compiler.IfCatchStmt:
			walkExpr(s.Cond)
			if s.CatchName != "" {
				out = append(out, spanToTokenRows(s.CatchNameSpan, sf, semTypeVariable)...)
			}
			walkBlock(s.CatchBody)
		}
	}

	walkBlock = func(block *compiler.BlockStmt) {
		for _, stmt := range block.Stmts {
			walkStmt(stmt)
		}
	}

	walkProc = func(proc *compiler.ProcDecl) {
		out = append(out, spanToTokenRows(nameSpan(proc.Span_, proc.Name), sf, semTypeFunction)...)
		for _, param := range proc.Params {
			out = append(out, spanToTokenRows(nameSpan(param.Span_, param.Name), sf, semTypeParameter)...)
			typeToken(param.Type, sf, known, &out)
		}
		for _, res := range proc.Results {
			typeToken(res, sf, known, &out)
		}
		if proc.ErrorResult != nil {
			typeToken(proc.ErrorResult, sf, known, &out)
		}
		if proc.Body != nil {
			walkBlock(proc.Body)
		}
	}

	walkStruct = func(st *compiler.StructDecl) {
		out = append(out, spanToTokenRows(nameSpan(st.Span_, st.Name), sf, semTypeType)...)
		for _, field := range st.Fields {
			out = append(out, spanToTokenRows(nameSpan(field.Span_, field.Name), sf, semTypeVariable)...)
			typeToken(field.Type, sf, known, &out)
		}
	}

	walkError = func(ed *compiler.ErrorDecl) {
		out = append(out, spanToTokenRows(nameSpan(ed.Span_, ed.Name), sf, semTypeType)...)
		for _, m := range ed.Members {
			out = append(out, spanToTokenRows(nameSpan(m.Span_, m.Name), sf, semTypeConstant)...)
		}
	}

	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *compiler.ProcDecl:
			walkProc(d)
		case *compiler.VarDecl:
			walkStmt(d)
		case *compiler.StructDecl:
			walkStruct(d)
		case *compiler.ErrorDecl:
			walkError(d)
		}
	}
	return out
}

// encodeSemanticTokens sorts tokens by position and delta-encodes them as
// [deltaLine, deltaStart, length, typeIndex, modifiers] rows. The modifiers
// field is always 0 (no modifiers), matching the LSP semantic tokens wire
// format of 5 integers per token.
func encodeSemanticTokens(tokens []semanticToken) []int {
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].line != tokens[j].line {
			return tokens[i].line < tokens[j].line
		}
		return tokens[i].startChar < tokens[j].startChar
	})
	var out []int
	prevLine, prevStart := 0, 0
	for _, tok := range tokens {
		deltaLine := tok.line - prevLine
		deltaStart := tok.startChar
		if deltaLine == 0 {
			deltaStart = tok.startChar - prevStart
		}
		out = append(out, deltaLine, deltaStart, tok.length, tok.typeIndex, 0)
		prevLine = tok.line
		prevStart = tok.startChar
	}
	if out == nil {
		out = []int{}
	}
	return out
}

// handleSemanticTokens handles textDocument/semanticTokens/full.
func (s *Server) handleSemanticTokens(msg message) Response {
	var params SemanticTokensParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return errorResponse(msg.ID, -32602, "invalid params")
	}
	s.mu.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		return errorResponse(msg.ID, -32603, "document not open")
	}
	tokens, _ := compiler.Tokenize(doc.sf.Source, doc.sf.ID)
	result := compiler.ParseProgramTolerant(tokens)
	all := append(tokenSemanticTokens(tokens, doc.sf), astSemanticTokens(result.Program, doc.sf)...)
	out, err := json.Marshal(SemanticTokens{Data: encodeSemanticTokens(all)})
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}
