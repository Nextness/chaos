package main

import (
	"encoding/json"
	"sort"
	"strings"

	"chaos_compiler/compiler"
)

// Semantic token type indices into the legend declared in initialize.
const (
	semTypeKeyword   = 0
	semTypeNumber    = 1
	semTypeString    = 2
	semTypeComment   = 3
	semTypeFunction  = 4
	semTypeVariable  = 5
	semTypeParameter = 6
	semTypeType      = 7
	semTypeConstant  = 8
	semTypeDelimiter = 9
)

// semanticToken is one token to be delta-encoded. Positions and lengths are in
// UTF-16 code units.
type semanticToken struct {
	line      int
	startChar int
	length    int
	typeIndex int
	modifiers int
}

// semModBold is the bit for the "bold" semantic token modifier, matching the
// LSP standard modifier name declared in the legend.
const semModBold = 1

// boldTypeTokens emits type tokens with the bold modifier set.
func boldTypeTokens(span compiler.Span, sf *compiler.SourceFile) []semanticToken {
	toks := spanToTokenRows(span, sf, semTypeType)
	for i := range toks {
		toks[i].modifiers = semModBold
	}
	return toks
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
	if sf == nil {
		return nil
	}
	r := compiler.SpanToRange(span, sf)
	if r.Start.Line == r.End.Line {
		if r.End.Character <= r.Start.Character {
			return nil
		}
		return []semanticToken{{line: r.Start.Line, startChar: r.Start.Character, length: r.End.Character - r.Start.Character, typeIndex: typeIndex}}
	}
	var out []semanticToken
	firstLineEnd := utf16LineLength(sf.Source, sf.LineOffsets[r.Start.Line])
	if firstLineEnd > r.Start.Character {
		out = append(out, semanticToken{line: r.Start.Line, startChar: r.Start.Character, length: firstLineEnd - r.Start.Character, typeIndex: typeIndex})
	}
	for line := r.Start.Line + 1; line < r.End.Line; line++ {
		if length := utf16LineLength(sf.Source, sf.LineOffsets[line]); length > 0 {
			out = append(out, semanticToken{line: line, startChar: 0, length: length, typeIndex: typeIndex})
		}
	}
	if r.End.Character > 0 {
		out = append(out, semanticToken{line: r.End.Line, startChar: 0, length: r.End.Character, typeIndex: typeIndex})
	}
	return out
}

// tokenSemanticTokens collects keyword/number/string/comment tokens from the
// tokenizer output.
func tokenSemanticTokens(tokens compiler.TokenList, sf *compiler.SourceFile) []semanticToken {
	var out []semanticToken
	for _, tok := range tokens {
		idx := -1
		switch tok.Kind {
		case compiler.TkExit, compiler.TkIf, compiler.TkIfx, compiler.TkElif, compiler.TkElse,
			compiler.TkProc, compiler.TkThen, compiler.TkReturn, compiler.TkAs,
			compiler.TkStruct, compiler.TkErrorKw, compiler.TkEnum, compiler.TkUnless, compiler.TkCatch,
			compiler.TkFor, compiler.TkBreak, compiler.TkContinue, compiler.TkNull,
			compiler.TkSizeOf,
			compiler.TkTrue, compiler.TkFalse,
			compiler.TkHash, compiler.TkDirec,
			compiler.TkEllipsis:
			// TkEllipsis is the '...' marker in 'name : Type = ...;'
			// (initialize-later) declarations; it reads as a keyword.
			idx = semTypeKeyword
		case compiler.TkInt, compiler.TkFloat:
			idx = semTypeNumber
		case compiler.TkString:
			if strings.Contains(tok.Value, "{") {
				// Interpolated string: the AST walker emits the literal
				// segments and the inner expressions separately, so the
				// interpolation is not swallowed by the whole-string token.
				continue
			}
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

// knownTypeNames returns the set of type names that exist in the program:
// built-in primitive types plus declared struct and error type names.
func knownTypeNames(program *compiler.Program) map[string]bool {
	builtins := compiler.BuiltinTypeNames()
	types := make(map[string]bool, len(builtins))
	for _, name := range builtins {
		types[name] = true
	}
	for _, decl := range program.Decls {
		switch d := decl.(type) {
		case *compiler.StructDecl:
			types[d.Name] = true
			for _, tp := range d.TypeParams {
				types[tp.Name] = true
			}
		case *compiler.ErrorDecl:
			types[d.Name] = true
		case *compiler.EnumDecl:
			types[d.Name] = true
		case *compiler.ProcDecl:
			for _, tp := range d.TypeParams {
				types[tp.Name] = true
			}
		}
	}
	return types
}

// typeToken emits a type token for an identifier type expression, but only if
// the name is a known type (built-in or declared struct). Undeclared
// identifiers in type position are not highlighted.
func typeToken(expr compiler.Expr, sf *compiler.SourceFile, known map[string]bool, out *[]semanticToken) {
	switch e := expr.(type) {
	case *compiler.IdentExpr:
		if known[e.Name] {
			*out = append(*out, spanToTokenRows(e.Span_, sf, semTypeType)...)
		}
	case *compiler.ArrayTypeExpr:
		// The '[]' bracket pair and the element type both read as type
		// tokens so "[]String" renders as a unit.
		if e.Elem != nil {
			elemSpan := compiler.NodeSpan(e.Elem)
			if e.Span_.Start < elemSpan.Start {
				*out = append(*out, spanToTokenRows(compiler.Span{File: e.Span_.File, Start: e.Span_.Start, End: elemSpan.Start}, sf, semTypeType)...)
			}
			typeToken(e.Elem, sf, known, out)
		}
	case *compiler.PointerTypeExpr:
		// The '*' (and the '?' nullable marker) are part of the type; emit
		// them as type tokens so "*String?" renders as a unit.
		if e.Elem != nil {
			elemSpan := compiler.NodeSpan(e.Elem)
			if e.Span_.Start < elemSpan.Start {
				*out = append(*out, spanToTokenRows(compiler.Span{File: e.Span_.File, Start: e.Span_.Start, End: elemSpan.Start}, sf, semTypeType)...)
			}
			typeToken(e.Elem, sf, known, out)
			if e.Nullable && e.Span_.End > elemSpan.End {
				*out = append(*out, spanToTokenRows(compiler.Span{File: e.Span_.File, Start: elemSpan.End, End: e.Span_.End}, sf, semTypeType)...)
			}
		}
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
	var walkEnum func(ed *compiler.EnumDecl)

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
				if field.Name != "" {
					out = append(out, spanToTokenRows(field.NameSpan, sf, semTypeVariable)...)
				}
				if field.Value != nil {
					walkExpr(field.Value)
				}
			}
		case *compiler.ErrorMemberExpr:
			if e.TypeName != "" {
				out = append(out, spanToTokenRows(e.TypeNameSpan, sf, semTypeType)...)
			}
			out = append(out, spanToTokenRows(e.NameSpan, sf, semTypeConstant)...)
		case *compiler.EnumMemberExpr:
			if e.TypeName != "" {
				out = append(out, spanToTokenRows(e.TypeNameSpan, sf, semTypeType)...)
			}
			out = append(out, spanToTokenRows(e.NameSpan, sf, semTypeConstant)...)
		case *compiler.FieldAccessExpr:
			// Base: a type token when it names a known type (enum or struct
			// for a member reference), otherwise a variable. The field reads
			// as a constant on an enum type and as a variable otherwise.
			if e.Base != nil {
				if ident, ok := e.Base.(*compiler.IdentExpr); ok {
					if known[ident.Name] {
						out = append(out, spanToTokenRows(ident.Span_, sf, semTypeType)...)
						out = append(out, spanToTokenRows(e.FieldSpan, sf, semTypeConstant)...)
					} else {
						out = append(out, spanToTokenRows(ident.Span_, sf, semTypeVariable)...)
						out = append(out, spanToTokenRows(e.FieldSpan, sf, semTypeVariable)...)
					}
				} else {
					walkExpr(e.Base)
					out = append(out, spanToTokenRows(e.FieldSpan, sf, semTypeVariable)...)
				}
			}
		case *compiler.DerefExpr:
			walkExpr(e.Operand)
			// The '.*' dereference operator reads as a keyword.
			opStart := compiler.NodeSpan(e.Operand).End
			if opStart < e.Span_.End {
				out = append(out, spanToTokenRows(compiler.Span{File: e.Span_.File, Start: opStart, End: e.Span_.End}, sf, semTypeKeyword)...)
			}
		case *compiler.NullLitExpr:
			// Keyword literal; no highlight.
		case *compiler.PointerTypeExpr:
			typeToken(e, sf, known, &out)
		case *compiler.ArrayInitExpr:
			typeToken(e.Elem, sf, known, &out)
			for _, item := range e.Items {
				walkExpr(item)
			}
		case *compiler.IndexExpr:
			walkExpr(e.Base)
			walkExpr(e.Index)
		case *compiler.IfxExpr:
			walkExpr(e.Condition)
			walkExpr(e.Then)
			walkExpr(e.Else)
		case *compiler.LoopBuiltinExpr:
			// Builtin directive; no highlight.
		case *compiler.InterpolatedStringExpr:
			// Emit a string token for each literal segment (including the
			// opening « and closing ») and walk the inner expressions, so the
			// interpolation is highlighted as its real type instead of being
			// swallowed by the whole-string token.
			file := e.Span_.File
			// Opening « (2 bytes).
			out = append(out, spanToTokenRows(compiler.Span{File: file, Start: e.Span_.Start, End: e.Span_.Start + 2}, sf, semTypeString)...)
			cursor := e.Span_.Start + 2 // after the opening «
			for _, part := range e.Parts {
				if part.Expr != nil {
					exprSpan := compiler.NodeSpan(part.Expr)
					// The '{' before and '}' after the interpolation are
					// delimiters, colored like braces outside strings.
					out = append(out, spanToTokenRows(compiler.Span{File: file, Start: exprSpan.Start - 1, End: exprSpan.Start}, sf, semTypeDelimiter)...)
					walkExpr(part.Expr)
					out = append(out, spanToTokenRows(compiler.Span{File: file, Start: exprSpan.End, End: exprSpan.End + 1}, sf, semTypeDelimiter)...)
					// Skip the expression and its closing '}'.
					cursor = exprSpan.End + 1
				} else {
					end := cursor + len(part.Literal)
					out = append(out, spanToTokenRows(compiler.Span{File: file, Start: cursor, End: end}, sf, semTypeString)...)
					cursor = end
				}
			}
			// Closing » (2 bytes).
			out = append(out, spanToTokenRows(compiler.Span{File: file, Start: e.Span_.End - 2, End: e.Span_.End}, sf, semTypeString)...)
		case *compiler.AllocateExpr:
			walkExpr(e.Size)
		case *compiler.CastExpr:
			walkExpr(e.Value)
			// The whole '.(*T)' cast reads as a bold type token.
			castStart := compiler.NodeSpan(e.Value).End
			if castStart < e.Span_.End {
				out = append(out, boldTypeTokens(compiler.Span{File: e.Span_.File, Start: castStart, End: e.Span_.End}, sf)...)
			}
		case *compiler.ArrayTypeExpr:
			typeToken(e, sf, known, &out)
		case *compiler.ErrorExpr:
			// Parser recovery placeholder.
		}
	}

	walkStmt = func(stmt compiler.Stmt) {
		switch s := stmt.(type) {
		case *compiler.VarDecl:
			// A '::' declaration is a variable (compile-time constant), not a
			// type. Type names are highlighted only when they are known
			// (built-in or declared struct), via typeToken.
			out = append(out, spanToTokenRows(s.NameSpan, sf, semTypeVariable)...)
			typeToken(s.DeclType, sf, known, &out)
			if s.Init != nil {
				walkExpr(s.Init)
			}
		case *compiler.AssignStmt:
			if s.Target != nil {
				// A complex lvalue target (index, field, or deref) is walked
				// so its operators and members are highlighted.
				walkExpr(s.Target)
			} else {
				out = append(out, spanToTokenRows(s.NameSpan, sf, semTypeVariable)...)
			}
			if s.Value != nil {
				walkExpr(s.Value)
			}
		case *compiler.ReturnStmt:
			for _, value := range s.Values {
				walkExpr(value)
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
		case *compiler.EnumDecl:
			walkEnum(s)
		case *compiler.UnlessCatchStmt:
			for _, span := range s.TargetSpans {
				out = append(out, spanToTokenRows(span, sf, semTypeVariable)...)
			}
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
		case *compiler.ForStmt:
			if s.Init != nil {
				walkStmt(s.Init)
			}
			if s.Cond != nil {
				walkExpr(s.Cond)
			}
			if s.Range != nil {
				walkExpr(s.Range)
			}
			if s.IndexName != "" {
				out = append(out, spanToTokenRows(s.IndexNameSpan, sf, semTypeVariable)...)
			}
			if s.ElemName != "" {
				out = append(out, spanToTokenRows(s.ElemNameSpan, sf, semTypeVariable)...)
			}
			if s.After != nil {
				walkStmt(s.After)
			}
			walkBlock(s.Body)
		case *compiler.BreakStmt, *compiler.ContinueStmt:
			// No operands to highlight.
		case *compiler.DeallocateStmt:
			if s.Addr != nil {
				walkExpr(s.Addr)
			}
		case *compiler.CompoundAssignStmt:
			if s.Target != nil {
				walkExpr(s.Target)
			} else {
				out = append(out, spanToTokenRows(s.NameSpan, sf, semTypeVariable)...)
			}
			if s.Value != nil {
				walkExpr(s.Value)
			}
		case *compiler.IncDecStmt:
			out = append(out, spanToTokenRows(s.NameSpan, sf, semTypeVariable)...)
		case *compiler.MultiVarDecl:
			for _, span := range s.NameSpans {
				out = append(out, spanToTokenRows(span, sf, semTypeVariable)...)
			}
			walkExpr(s.Init)
		}
	}

	walkBlock = func(block *compiler.BlockStmt) {
		for _, stmt := range block.Stmts {
			walkStmt(stmt)
		}
	}

	walkProc = func(proc *compiler.ProcDecl) {
		out = append(out, spanToTokenRows(proc.NameSpan, sf, semTypeFunction)...)
		for _, tp := range proc.TypeParams {
			out = append(out, spanToTokenRows(tp.NameSpan, sf, semTypeType)...)
		}
		for _, param := range proc.Params {
			out = append(out, spanToTokenRows(param.NameSpan, sf, semTypeParameter)...)
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
		out = append(out, spanToTokenRows(st.NameSpan, sf, semTypeType)...)
		for _, tp := range st.TypeParams {
			out = append(out, spanToTokenRows(tp.NameSpan, sf, semTypeType)...)
		}
		for _, field := range st.Fields {
			out = append(out, spanToTokenRows(field.NameSpan, sf, semTypeVariable)...)
			typeToken(field.Type, sf, known, &out)
			if field.Default != nil {
				walkExpr(field.Default)
			}
		}
	}

	walkError = func(ed *compiler.ErrorDecl) {
		out = append(out, spanToTokenRows(ed.NameSpan, sf, semTypeType)...)
		for _, m := range ed.Members {
			out = append(out, spanToTokenRows(m.NameSpan, sf, semTypeConstant)...)
		}
	}

	walkEnum = func(ed *compiler.EnumDecl) {
		out = append(out, spanToTokenRows(ed.NameSpan, sf, semTypeType)...)
		for _, m := range ed.Members {
			out = append(out, spanToTokenRows(m.NameSpan, sf, semTypeConstant)...)
			typeToken(m.Type, sf, known, &out)
			if m.Value != nil {
				walkExpr(m.Value)
			}
		}
	}

	for _, decl := range program.Decls {
		// Only emit tokens for declarations in the current file. Imported
		// declarations have spans in other files and must not be rendered
		// against this document's source.
		if declSpanFile(decl) != sf.ID {
			continue
		}
		switch d := decl.(type) {
		case *compiler.ProcDecl:
			walkProc(d)
		case *compiler.VarDecl:
			walkStmt(d)
		case *compiler.StructDecl:
			walkStruct(d)
		case *compiler.ErrorDecl:
			walkError(d)
		case *compiler.EnumDecl:
			walkEnum(d)
		}
	}
	return out
}

// declSpanFile returns the source file ID of a declaration's span, or -1 when
// the declaration carries no span.
func declSpanFile(decl compiler.Decl) compiler.FileID {
	switch d := decl.(type) {
	case *compiler.ProcDecl:
		return d.Span_.File
	case *compiler.VarDecl:
		return d.Span_.File
	case *compiler.StructDecl:
		return d.Span_.File
	case *compiler.ErrorDecl:
		return d.Span_.File
	case *compiler.EnumDecl:
		return d.Span_.File
	case *compiler.ImportDecl:
		return d.Span_.File
	}
	return -1
}

// tokenPriority orders overlapping tokens so the more specific one wins. A
// type token (e.g. the "[dyn]" of an array type, or the ".(*T)" of a cast)
// must take precedence over the delimiter tokens it contains, so it sorts
// first and the contained delimiters are dropped by the overlap check.
func tokenPriority(t semanticToken) int {
	if t.typeIndex == semTypeType {
		return 0
	}
	return 1
}

// encodeSemanticTokens sorts tokens by position and delta-encodes them as
// [deltaLine, deltaStart, length, typeIndex, modifiers] rows, matching the LSP
// semantic tokens wire format of 5 integers per token.
func encodeSemanticTokens(tokens []semanticToken) []int {
	sort.SliceStable(tokens, func(i, j int) bool {
		if tokens[i].line != tokens[j].line {
			return tokens[i].line < tokens[j].line
		}
		if tokens[i].startChar != tokens[j].startChar {
			return tokens[i].startChar < tokens[j].startChar
		}
		// At the same position, the more specific token (a type) sorts first
		// so it wins the overlap check below.
		return tokenPriority(tokens[i]) < tokenPriority(tokens[j])
	})
	var out []int
	prevLine, prevStart := 0, 0
	lastLine, lastEnd := -1, 0
	for _, tok := range tokens {
		if tok.line < 0 || tok.startChar < 0 || tok.length <= 0 {
			continue
		}
		if tok.line == lastLine && tok.startChar < lastEnd {
			continue
		}
		deltaLine := tok.line - prevLine
		deltaStart := tok.startChar
		if deltaLine == 0 {
			deltaStart = tok.startChar - prevStart
		}
		out = append(out, deltaLine, deltaStart, tok.length, tok.typeIndex, tok.modifiers)
		prevLine = tok.line
		prevStart = tok.startChar
		lastLine = tok.line
		lastEnd = tok.startChar + tok.length
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
	out, err := json.Marshal(SemanticTokens{Data: doc.semantic})
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}
