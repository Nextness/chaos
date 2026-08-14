package main

import (
	"encoding/json"
	"sort"
	"strings"

	"chaos_new/compiler"
)

// LSP CompletionItemKind values used by the server.
const (
	completionKindFunction   = 3
	completionKindVariable   = 6
	completionKindEnum       = 13
	completionKindConstant   = 14
	completionKindEnumMember = 20
	completionKindStruct     = 22
)

// symbol is a declared name (proc, var, const, struct, error type, error
// member, param, local).
type symbol struct {
	name        string
	kind        int           // LSP CompletionItemKind
	span        compiler.Span // name span
	proc        *compiler.ProcDecl
	varDecl     *compiler.VarDecl
	structDecl  *compiler.StructDecl
	errorDecl   *compiler.ErrorDecl
	errorMember *compiler.ErrorMember
	errorType   string // owning error type name for an error member
	param       *compiler.Param
	scope       *scope
}

// scope is a lexical scope with a parent and nested children.
type scope struct {
	parent   *scope
	children []*scope
	start    int // byte offset of the scope start
	end      int // byte offset of the scope end
	symbols  []*symbol
}

// occurrence is one name occurrence in the document (decl, param, or
// identifier reference).
type occurrence struct {
	name string
	span compiler.Span
	sym  *symbol // non-nil if this occurrence is a declaration or param name
}

// resolver is a per-document symbol index built from the tolerant parse.
type resolver struct {
	sf           *compiler.SourceFile
	uri          string
	program      *compiler.Program
	tokens       compiler.TokenList
	global       *scope
	decls        []*symbol
	occurrences  []*occurrence
	errorMembers map[string]map[string]*symbol // error type name -> member name -> symbol
}

// buildResolver tokenizes and parses a document in tolerant mode and builds
// its symbol index.
func buildResolver(doc *Document) *resolver {
	tokens, _ := compiler.Tokenize(doc.sf.Source, doc.sf.ID)
	result := compiler.ParseProgramTolerant(tokens)
	r := &resolver{sf: doc.sf, uri: doc.URI, program: result.Program, tokens: tokens, errorMembers: make(map[string]map[string]*symbol)}
	r.buildScopes()
	r.collectOccurrences()
	return r
}

// buildScopes builds the lexical scope tree and the top-level symbol list.
func (r *resolver) buildScopes() {
	r.global = &scope{start: 0, end: len(r.sf.Source)}
	for _, decl := range r.program.Decls {
		switch d := decl.(type) {
		case *compiler.ProcDecl:
			sym := &symbol{name: d.Name, kind: completionKindFunction, span: nameSpan(d.Span_, d.Name), proc: d, scope: r.global}
			r.global.symbols = append(r.global.symbols, sym)
			r.decls = append(r.decls, sym)
			procScope := &scope{parent: r.global, start: d.Span_.Start, end: d.Span_.End}
			r.global.children = append(r.global.children, procScope)
			for i := range d.Params {
				p := &d.Params[i]
				psym := &symbol{name: p.Name, kind: completionKindVariable, span: nameSpan(p.Span_, p.Name), param: p, scope: procScope}
				procScope.symbols = append(procScope.symbols, psym)
				r.decls = append(r.decls, psym)
			}
			if d.Body != nil {
				r.buildBlockScope(d.Body, procScope)
			}
		case *compiler.VarDecl:
			kind := completionKindVariable
			if d.CompileTime {
				kind = completionKindConstant
			}
			sym := &symbol{name: d.Name, kind: kind, span: nameSpan(d.Span_, d.Name), varDecl: d, scope: r.global}
			r.global.symbols = append(r.global.symbols, sym)
			r.decls = append(r.decls, sym)
		case *compiler.StructDecl:
			sym := &symbol{name: d.Name, kind: completionKindStruct, span: nameSpan(d.Span_, d.Name), structDecl: d, scope: r.global}
			r.global.symbols = append(r.global.symbols, sym)
			r.decls = append(r.decls, sym)
		case *compiler.ErrorDecl:
			sym := &symbol{name: d.Name, kind: completionKindEnum, span: nameSpan(d.Span_, d.Name), errorDecl: d, scope: r.global}
			r.global.symbols = append(r.global.symbols, sym)
			r.decls = append(r.decls, sym)
			r.collectErrorMembers(d)
		}
	}
}

// collectErrorMembers records each error member as an occurrence and indexes
// it by error type name so member references (Type.MEMBER and .MEMBER) can
// resolve to their declaration. Members are not added to any scope's symbol
// list: they are not accessible by bare name in expressions.
func (r *resolver) collectErrorMembers(ed *compiler.ErrorDecl) {
	if r.errorMembers[ed.Name] == nil {
		r.errorMembers[ed.Name] = make(map[string]*symbol)
	}
	for i := range ed.Members {
		m := &ed.Members[i]
		msym := &symbol{name: m.Name, kind: completionKindEnumMember, span: nameSpan(m.Span_, m.Name), errorMember: m, errorType: ed.Name, scope: r.global}
		r.errorMembers[ed.Name][m.Name] = msym
		r.occurrences = append(r.occurrences, &occurrence{name: m.Name, span: msym.span, sym: msym})
	}
}

// buildBlockScope builds a nested scope for a block and its child blocks.
func (r *resolver) buildBlockScope(block *compiler.BlockStmt, parent *scope) {
	s := &scope{parent: parent, start: block.Span_.Start, end: block.Span_.End}
	parent.children = append(parent.children, s)
	for _, stmt := range block.Stmts {
		switch st := stmt.(type) {
		case *compiler.VarDecl:
			r.addVarDecl(s, st)
		case *compiler.IfStmt:
			r.buildBlockScope(st.Body, s)
			for _, elif := range st.Elif {
				r.buildBlockScope(elif.Body, s)
			}
			if st.ElseBody != nil {
				r.buildBlockScope(st.ElseBody, s)
			}
		case *compiler.UnlessCatchStmt:
			if st.Target != "" {
				r.addBinding(s, st.Target, nameSpan(st.Span_, st.Target))
			}
			r.buildCatchScope(st.CatchBody, st.CatchName, st.CatchNameSpan, s)
		case *compiler.IfCatchStmt:
			r.buildCatchScope(st.CatchBody, st.CatchName, st.CatchNameSpan, s)
		case *compiler.BlockStmt:
			r.buildBlockScope(st, s)
		case *compiler.ProcDecl:
			procScope := &scope{parent: s, start: st.Span_.Start, end: st.Span_.End}
			s.children = append(s.children, procScope)
			for i := range st.Params {
				p := &st.Params[i]
				psym := &symbol{name: p.Name, kind: completionKindVariable, span: nameSpan(p.Span_, p.Name), param: p, scope: procScope}
				procScope.symbols = append(procScope.symbols, psym)
				r.decls = append(r.decls, psym)
			}
			if st.Body != nil {
				r.buildBlockScope(st.Body, procScope)
			}
		case *compiler.StructDecl:
			sym := &symbol{name: st.Name, kind: completionKindStruct, span: nameSpan(st.Span_, st.Name), structDecl: st, scope: s}
			s.symbols = append(s.symbols, sym)
			r.decls = append(r.decls, sym)
		case *compiler.ErrorDecl:
			sym := &symbol{name: st.Name, kind: completionKindEnum, span: nameSpan(st.Span_, st.Name), errorDecl: st, scope: s}
			s.symbols = append(s.symbols, sym)
			r.decls = append(r.decls, sym)
			r.collectErrorMembers(st)
		case *compiler.ForStmt:
			r.buildForScope(st, s)
		}
	}
}

func (r *resolver) addVarDecl(sc *scope, decl *compiler.VarDecl) {
	kind := completionKindVariable
	if decl.CompileTime {
		kind = completionKindConstant
	}
	sym := &symbol{name: decl.Name, kind: kind, span: nameSpan(decl.Span_, decl.Name), varDecl: decl, scope: sc}
	sc.symbols = append(sc.symbols, sym)
	r.decls = append(r.decls, sym)
}

func (r *resolver) addBinding(sc *scope, name string, span compiler.Span) {
	sym := &symbol{name: name, kind: completionKindVariable, span: span, scope: sc}
	sc.symbols = append(sc.symbols, sym)
	r.decls = append(r.decls, sym)
}

// buildForScope indexes loop declarations using the same visibility as the
// compiler. C-style initializers live in the surrounding scope. Range
// bindings are visible only in the loop body and do not shadow names in the
// iterable expression.
func (r *resolver) buildForScope(loop *compiler.ForStmt, parent *scope) {
	if init, ok := loop.Init.(*compiler.VarDecl); ok {
		r.addVarDecl(parent, init)
	}
	if loop.Range == nil {
		r.buildBlockScope(loop.Body, parent)
		return
	}
	loopScope := &scope{parent: parent, start: loop.Body.Span_.Start, end: loop.Body.Span_.End}
	parent.children = append(parent.children, loopScope)
	if loop.IndexName != "" {
		r.addBinding(loopScope, loop.IndexName, loop.IndexNameSpan)
	}
	if loop.ElemName != "" {
		r.addBinding(loopScope, loop.ElemName, loop.ElemNameSpan)
	}
	r.buildBlockScope(loop.Body, loopScope)
}

// uniqueErrorMember returns the member symbol for a bare '.MEMBER' reference
// when the member name is unique across all error types, or nil when it is
// ambiguous or unknown.
func (r *resolver) uniqueErrorMember(name string) *symbol {
	var found *symbol
	for _, members := range r.errorMembers {
		if msym, ok := members[name]; ok {
			if found != nil {
				return nil // ambiguous
			}
			found = msym
		}
	}
	return found
}

// buildCatchScope builds the scope of a catch body, declaring the optional
// error binding in it.
func (r *resolver) buildCatchScope(body *compiler.BlockStmt, catchName string, catchNameSpan compiler.Span, parent *scope) {
	s := &scope{parent: parent, start: body.Span_.Start, end: body.Span_.End}
	parent.children = append(parent.children, s)
	if catchName != "" {
		sym := &symbol{name: catchName, kind: completionKindVariable, span: catchNameSpan, scope: s}
		s.symbols = append(s.symbols, sym)
		r.decls = append(r.decls, sym)
	}
	r.buildBlockScope(body, s)
}

// collectStructFields records each struct field name as an occurrence so
// definition, references, and documentHighlight work on field names. Fields
// are not added to any scope's symbol list: they are not accessible by bare
// name in expressions.
func (r *resolver) collectStructFields(st *compiler.StructDecl) {
	for _, field := range st.Fields {
		fsym := &symbol{name: field.Name, kind: completionKindVariable, span: nameSpan(field.Span_, field.Name), scope: r.global}
		r.occurrences = append(r.occurrences, &occurrence{name: field.Name, span: fsym.span, sym: fsym})
	}
}

// collectOccurrences gathers every name occurrence (decl names, param names,
// assignment targets, and identifier references) for references and
// documentHighlight.
func (r *resolver) collectOccurrences() {
	for _, sym := range r.decls {
		r.occurrences = append(r.occurrences, &occurrence{name: sym.name, span: sym.span, sym: sym})
	}

	var walkExpr func(expr compiler.Expr)
	var walkStmt func(stmt compiler.Stmt)
	var walkBlock func(block *compiler.BlockStmt)

	walkExpr = func(expr compiler.Expr) {
		switch e := expr.(type) {
		case *compiler.IdentExpr:
			r.occurrences = append(r.occurrences, &occurrence{name: e.Name, span: e.Span_})
		case *compiler.BinaryExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *compiler.UnaryExpr:
			walkExpr(e.Operand)
		case *compiler.CallExpr:
			walkExpr(e.Func)
			for _, arg := range e.Args {
				walkExpr(arg)
			}
		case *compiler.ParenExpr:
			walkExpr(e.Inner)
		case *compiler.StructInitExpr:
			if e.Type != nil {
				walkExpr(e.Type)
			}
			for _, field := range e.Fields {
				if field.Value != nil {
					walkExpr(field.Value)
				}
			}
		case *compiler.ErrorMemberExpr:
			if e.TypeName != "" {
				typeSpan := compiler.Span{File: e.Span_.File, Start: e.Span_.Start, End: e.Span_.Start + len(e.TypeName)}
				r.occurrences = append(r.occurrences, &occurrence{name: e.TypeName, span: typeSpan})
			}
			bangLen := 0
			if e.Bang {
				bangLen = 1
			}
			// The occurrence covers the member name only, not its '!'.
			memberSpan := compiler.Span{File: e.Span_.File, Start: e.Span_.End - len(e.Name) - bangLen, End: e.Span_.End - bangLen}
			var msym *symbol
			if e.TypeName != "" {
				msym = r.errorMembers[e.TypeName][e.Name]
			} else {
				msym = r.uniqueErrorMember(e.Name)
			}
			r.occurrences = append(r.occurrences, &occurrence{name: e.Name, span: memberSpan, sym: msym})
		case *compiler.ArrayInitExpr:
			if e.Elem != nil {
				walkExpr(e.Elem)
			}
			for _, item := range e.Items {
				walkExpr(item)
			}
		case *compiler.IndexExpr:
			walkExpr(e.Base)
			walkExpr(e.Index)
		case *compiler.LoopBuiltinExpr:
			// Builtin directive; no symbol.
		}
	}

	walkStmt = func(stmt compiler.Stmt) {
		switch s := stmt.(type) {
		case *compiler.VarDecl:
			if s.Init != nil {
				walkExpr(s.Init)
			}
		case *compiler.AssignStmt:
			r.occurrences = append(r.occurrences, &occurrence{name: s.Name, span: nameSpan(s.Span_, s.Name)})
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
			if s.Body != nil {
				walkBlock(s.Body)
			}
		case *compiler.StructDecl:
			r.collectStructFields(s)
		case *compiler.UnlessCatchStmt:
			walkExpr(s.Init)
			walkBlock(s.CatchBody)
		case *compiler.IfCatchStmt:
			walkExpr(s.Cond)
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
			if s.After != nil {
				walkStmt(s.After)
			}
			walkBlock(s.Body)
		case *compiler.BreakStmt, *compiler.ContinueStmt:
			// No operands.
		case *compiler.CompoundAssignStmt:
			r.occurrences = append(r.occurrences, &occurrence{name: s.Name, span: nameSpan(s.Span_, s.Name)})
			if s.Value != nil {
				walkExpr(s.Value)
			}
		case *compiler.IncDecStmt:
			r.occurrences = append(r.occurrences, &occurrence{name: s.Name, span: nameSpan(s.Span_, s.Name)})
		}
	}

	walkBlock = func(block *compiler.BlockStmt) {
		for _, stmt := range block.Stmts {
			walkStmt(stmt)
		}
	}

	for _, decl := range r.program.Decls {
		switch d := decl.(type) {
		case *compiler.ProcDecl:
			if d.Body != nil {
				walkBlock(d.Body)
			}
		case *compiler.VarDecl:
			walkStmt(d)
		case *compiler.StructDecl:
			r.collectStructFields(d)
		}
	}
}

// scopeAt returns the deepest scope containing the byte offset.
func (r *resolver) scopeAt(offset int) *scope {
	var best *scope
	var visit func(s *scope)
	visit = func(s *scope) {
		if offset < s.start || offset > s.end {
			return
		}
		if best == nil || s.start >= best.start {
			best = s
		}
		for _, child := range s.children {
			visit(child)
		}
	}
	visit(r.global)
	return best
}

// resolveName resolves a name to the deepest scope containing offset. Local
// names must be declared before the offset; globals are visible regardless of
// source order, matching the compiler's global declaration pass.
func (r *resolver) resolveName(name string, offset int) *symbol {
	sc := r.scopeAt(offset)
	for sc != nil {
		for i := len(sc.symbols) - 1; i >= 0; i-- {
			sym := sc.symbols[i]
			if sym.name == name && (sc == r.global || sym.span.End <= offset) {
				return sym
			}
		}
		sc = sc.parent
	}
	return nil
}

// occurrenceAt returns the occurrence whose span contains the byte offset.
func (r *resolver) occurrenceAt(offset int) *occurrence {
	for _, occ := range r.occurrences {
		if offset >= occ.span.Start && offset < occ.span.End {
			return occ
		}
	}
	return nil
}

// definitionAt resolves the identifier under the cursor to its declaration.
// Definition on a declaration or param name resolves to itself.
func (r *resolver) definitionAt(offset int) *symbol {
	occ := r.occurrenceAt(offset)
	if occ == nil {
		return nil
	}
	if occ.sym != nil {
		return occ.sym
	}
	return r.resolveName(occ.name, offset)
}

func (r *resolver) resolveOccurrence(occ *occurrence) *symbol {
	if occ == nil {
		return nil
	}
	if occ.sym != nil {
		return occ.sym
	}
	return r.resolveName(occ.name, occ.span.Start)
}

// referencesAt returns all occurrences that resolve to the same declaration
// as the name under the cursor.
func (r *resolver) referencesAt(offset int) []*occurrence {
	occ := r.occurrenceAt(offset)
	target := r.resolveOccurrence(occ)
	if target == nil {
		return nil
	}
	var out []*occurrence
	for _, o := range r.occurrences {
		if r.resolveOccurrence(o) == target {
			out = append(out, o)
		}
	}
	return out
}

// hoverAt returns markdown content for the identifier or literal under the
// cursor, or "" when nothing is under the cursor.
func (r *resolver) hoverAt(offset int) string {
	occ := r.occurrenceAt(offset)
	if occ != nil {
		if occ.sym != nil {
			return symbolHover(occ.sym)
		}
		if sym := r.resolveName(occ.name, offset); sym != nil {
			return symbolHover(sym)
		}
	}
	// Literal: return the raw token source text.
	for _, tok := range r.tokens {
		if offset >= tok.Span.Start && offset < tok.Span.End {
			switch tok.Kind {
			case compiler.TkInt, compiler.TkFloat, compiler.TkString, compiler.TkTrue, compiler.TkFalse:
				return "```chaos\n" + tok.Text() + "\n```"
			}
		}
	}
	return ""
}

// completionAt returns the names in scope at the offset, excluding names
// declared after the offset. When the cursor follows 'TypeName.', it returns
// the members of that error type instead.
func (r *resolver) completionAt(offset int) []CompletionItem {
	if items := r.memberCompletion(offset); items != nil {
		return items
	}
	sc := r.scopeAt(offset)
	seen := make(map[string]bool)
	out := []CompletionItem{}
	for sc != nil {
		for i := len(sc.symbols) - 1; i >= 0; i-- {
			sym := sc.symbols[i]
			if (sc == r.global || sym.span.End <= offset) && !seen[sym.name] {
				seen[sym.name] = true
				item := CompletionItem{Label: sym.name, Kind: sym.kind}
				if sym.proc != nil {
					item.Detail = procSignature(sym.proc)
				}
				out = append(out, item)
			}
		}
		sc = sc.parent
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// memberCompletion returns the members of the error type named just before a
// trailing '.' at the offset, or nil when the cursor is not after 'TypeName.'.
func (r *resolver) memberCompletion(offset int) []CompletionItem {
	if offset <= 0 {
		return nil
	}
	src := r.sf.Source
	i := offset - 1
	for i >= 0 && (src[i] == ' ' || src[i] == '\t') {
		i--
	}
	if i < 0 || src[i] != '.' {
		return nil
	}
	j := i - 1
	for j >= 0 && !isIdentStop(src[j]) {
		j--
	}
	name := string(src[j+1 : i])
	members, ok := r.errorMembers[name]
	if !ok {
		return nil
	}
	out := []CompletionItem{}
	for _, msym := range members {
		out = append(out, CompletionItem{Label: msym.name, Kind: msym.kind})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// isIdentStop reports whether a byte cannot appear inside an identifier.
func isIdentStop(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '.', '(', ')', '{', '}', '[', ']', ';', ',',
		'=', ':', '+', '-', '*', '/', '%', '<', '>', '!', '&', '|', '?', '@', '#', '"':
		return true
	}
	return false
}

// exprText returns the source text of an identifier type expression.
func exprText(e compiler.Expr) string {
	if ident, ok := e.(*compiler.IdentExpr); ok {
		return ident.Name
	}
	return ""
}

// procSignature renders a procedure signature, e.g. "add(a: S64, b: S64) -> S64".
func procSignature(proc *compiler.ProcDecl) string {
	var b strings.Builder
	b.WriteString(proc.Name)
	b.WriteString("(")
	for i, param := range proc.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(param.Name)
		b.WriteString(": ")
		b.WriteString(exprText(param.Type))
	}
	b.WriteString(")")
	if len(proc.Results) == 0 {
		b.WriteString(" -> void")
	} else {
		b.WriteString(" -> ")
		for i, res := range proc.Results {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprText(res))
		}
	}
	return b.String()
}

// structSignature renders a struct definition, e.g.
// "Something_New :: struct {\n\tfield1: String;\n\tfield2: U64;\n}".
func structSignature(st *compiler.StructDecl) string {
	var b strings.Builder
	b.WriteString(st.Name)
	b.WriteString(" :: struct {\n")
	for _, field := range st.Fields {
		b.WriteString("\t")
		b.WriteString(field.Name)
		b.WriteString(": ")
		b.WriteString(exprText(field.Type))
		b.WriteString(";\n")
	}
	b.WriteString("}")
	return b.String()
}

// errorSignature renders an error type definition, e.g.
// "Hash_Table_Error :: error {\n\tGENERIC;\n\tOUT_OF_MEMORY;\n}".
func errorSignature(ed *compiler.ErrorDecl) string {
	var b strings.Builder
	b.WriteString(ed.Name)
	b.WriteString(" :: error {\n")
	for _, m := range ed.Members {
		b.WriteString("\t")
		b.WriteString(m.Name)
		b.WriteString(";\n")
	}
	b.WriteString("}")
	return b.String()
}

// symbolHover renders markdown content for a symbol.
func symbolHover(sym *symbol) string {
	switch {
	case sym.proc != nil:
		return "```chaos\n" + procSignature(sym.proc) + "\n```"
	case sym.structDecl != nil:
		return "```chaos\n" + structSignature(sym.structDecl) + "\n```"
	case sym.errorDecl != nil:
		return "```chaos\n" + errorSignature(sym.errorDecl) + "\n```"
	case sym.errorMember != nil:
		return "```chaos\n" + sym.errorType + "." + sym.name + " : " + sym.errorType + "\n```"
	case sym.varDecl != nil:
		if sym.varDecl.DeclType != nil {
			return "```chaos\n" + sym.name + " : " + exprText(sym.varDecl.DeclType) + "\n```"
		}
		return "```chaos\n" + sym.name + " : inferred\n```"
	case sym.param != nil:
		if sym.param.Type != nil {
			return "```chaos\n" + sym.name + " : " + exprText(sym.param.Type) + "\n```"
		}
		return "```chaos\n" + sym.name + " : inferred\n```"
	}
	return ""
}

// handleDefinition handles textDocument/definition.
func (s *Server) handleDefinition(msg message) Response {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return errorResponse(msg.ID, -32602, "invalid params")
	}
	s.mu.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		return errorResponse(msg.ID, -32603, "document not open")
	}
	offset := positionToByteOffset(doc.Text, params.Position)
	sym := buildResolver(doc).definitionAt(offset)
	if sym == nil {
		return Response{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage("null")}
	}
	out, err := json.Marshal(Location{URI: doc.URI, Range: toLSPRange(compiler.SpanToRange(sym.span, doc.sf))})
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}

// handleReferences handles textDocument/references.
func (s *Server) handleReferences(msg message) Response {
	var params ReferenceParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return errorResponse(msg.ID, -32602, "invalid params")
	}
	s.mu.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		return errorResponse(msg.ID, -32603, "document not open")
	}
	offset := positionToByteOffset(doc.Text, params.Position)
	occs := buildResolver(doc).referencesAt(offset)
	var locs []Location
	for _, occ := range occs {
		if !params.Context.IncludeDeclaration && occ.sym != nil && occ.span == occ.sym.span {
			continue
		}
		locs = append(locs, Location{URI: doc.URI, Range: toLSPRange(compiler.SpanToRange(occ.span, doc.sf))})
	}
	out, err := json.Marshal(locs)
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}

// handleDocumentHighlight handles textDocument/documentHighlight.
func (s *Server) handleDocumentHighlight(msg message) Response {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return errorResponse(msg.ID, -32602, "invalid params")
	}
	s.mu.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		return errorResponse(msg.ID, -32603, "document not open")
	}
	offset := positionToByteOffset(doc.Text, params.Position)
	occs := buildResolver(doc).referencesAt(offset)
	var highlights []DocumentHighlight
	for _, occ := range occs {
		highlights = append(highlights, DocumentHighlight{Range: toLSPRange(compiler.SpanToRange(occ.span, doc.sf)), Kind: 1})
	}
	out, err := json.Marshal(highlights)
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}

// handleHover handles textDocument/hover.
func (s *Server) handleHover(msg message) Response {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return errorResponse(msg.ID, -32602, "invalid params")
	}
	s.mu.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		return errorResponse(msg.ID, -32603, "document not open")
	}
	offset := positionToByteOffset(doc.Text, params.Position)
	content := buildResolver(doc).hoverAt(offset)
	if content == "" {
		return Response{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage("null")}
	}
	out, err := json.Marshal(Hover{Contents: MarkupContent{Kind: "markdown", Value: content}})
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}

// handleCompletion handles textDocument/completion.
func (s *Server) handleCompletion(msg message) Response {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return errorResponse(msg.ID, -32602, "invalid params")
	}
	s.mu.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.Unlock()
	if doc == nil {
		return errorResponse(msg.ID, -32603, "document not open")
	}
	offset := positionToByteOffset(doc.Text, params.Position)
	items := buildResolver(doc).completionAt(offset)
	out, err := json.Marshal(CompletionList{IsIncomplete: false, Items: items})
	if err != nil {
		return errorResponse(msg.ID, -32603, "internal error")
	}
	return Response{JSONRPC: "2.0", ID: msg.ID, Result: out}
}
