package compiler

// Parser is a hand-written recursive-descent parser with a correct Pratt
// expression parser.
//
// Grammar (supported subset):
//
//	program        = decl*
//	decl           = var_decl | proc_decl | struct_decl
//	var_decl       = ident ( "::" ( "proc" ... | "struct" ... | expr ) | ":=" expr | ":" ident ("=" expr)? ) ";"
//	proc_decl      = ident "::" "proc" param_list? result_spec? block
//	struct_decl    = ident "::" "struct" "{" (ident ":" ident ";")* "}"
//	param_list     = "(" (param ("," param)*)? ")"
//	param          = ident ":" ident
//	result_spec    = "->" ident ("," ident)*
//	block          = "{" stmt* "}"
//	stmt           = var_decl | proc_decl | struct_decl | assign_stmt | return_stmt | exit_stmt | if_stmt | block | call_stmt | ";"
//	call_stmt      = ident "(" arg_list? ")" ("(" arg_list? ")")* ";"
//	assign_stmt    = ident "=" expr ";"
//	return_stmt    = "return" expr? ";"
//	exit_stmt      = "exit" expr ("," expr)? ";"
//	if_stmt        = "if" expr block ("elif" expr block)* ("else" block)?
//	expr           = or_expr
//	or_expr        = and_expr ("||" and_expr)*
//	and_expr       = cmp_expr ("&&" cmp_expr)*
//	cmp_expr       = add_expr (cmp_op add_expr)*
//	add_expr       = mul_expr (add_op mul_expr)*
//	mul_expr       = unary_expr (mul_op unary_expr)*
//	unary_expr     = unary_op unary_expr | postfix_expr
//	postfix_expr   = primary_expr ("(" arg_list? ")")*
//	arg_list       = expr ("," expr)*
//	primary_expr   = ident | int | float | string | "true" | "false" | "(" expr ")" | struct_init
//	struct_init    = (ident ".")? "{" struct_init_field ("," struct_init_field)* "}"
//	struct_init_field = (ident "=")? expr
type Parser struct {
	tokens  TokenList
	pos     int
	diags   DiagnosticList
	program *Program

	// tolerant enables editor-friendly parsing: unknown top-level forms
	// (directives, structs, enums) and unknown statement keywords are skipped
	// without diagnostics instead of erroring.
	tolerant bool
}

// ParseResult holds the results of parsing a token list.
type ParseResult struct {
	Program *Program
	Diags   DiagnosticList
}

// ParseProgram parses a complete token list into a Program.
func ParseProgram(tokens TokenList) ParseResult {
	p := &Parser{
		tokens:  tokens,
		program: &Program{},
	}
	for p.pos < len(p.tokens)-1 { // -1 to leave EOF
		if tok := p.peek(); tok.Kind == TkEOF {
			break
		}
		decl, ok := p.parseDecl()
		if ok {
			p.program.Decls = append(p.program.Decls, decl)
		} else {
			// Skip one token on error to guarantee progress
			p.bump()
		}
	}
	return ParseResult{Program: p.program, Diags: p.diags}
}

// ParseProgramTolerant parses a complete token list into a Program, skipping
// language forms the strict parser does not understand (directives, structs,
// enums, unknown statement keywords). It is intended for editor use where
// real-world files may use newer syntax. It uses its own loop so tolerant
// skips that consume their terminator do not lose the next declaration: after
// a failed parseDecl, a token is bumped only if no progress was made.
func ParseProgramTolerant(tokens TokenList) ParseResult {
	p := &Parser{
		tokens:   tokens,
		program:  &Program{},
		tolerant: true,
	}
	for p.pos < len(p.tokens)-1 { // -1 to leave EOF
		if tok := p.peek(); tok.Kind == TkEOF {
			break
		}
		before := p.pos
		decl, ok := p.parseDecl()
		if ok {
			p.program.Decls = append(p.program.Decls, decl)
		} else if p.pos == before {
			// Skip one token on error to guarantee progress, but only if the
			// failed parse made no progress.
			p.bump()
		}
	}
	return ParseResult{Program: p.program, Diags: p.diags}
}

// peek returns the current token without consuming it. Comment tokens are
// skipped transparently so existing parse logic is unaffected.
func (p *Parser) peek() Token {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Kind == TkComment {
		p.pos++
	}
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Kind: TkEOF}
}

// peekN returns the token n positions ahead of the current token, skipping
// comment tokens. It does not consume anything.
func (p *Parser) peekN(n int) Token {
	idx := p.pos
	for idx < len(p.tokens) && p.tokens[idx].Kind == TkComment {
		idx++
	}
	for i := 0; i < n; i++ {
		idx++
		for idx < len(p.tokens) && p.tokens[idx].Kind == TkComment {
			idx++
		}
	}
	if idx < len(p.tokens) {
		return p.tokens[idx]
	}
	return Token{Kind: TkEOF}
}

// bump consumes and returns the current token.
func (p *Parser) bump() Token {
	tok := p.peek()
	p.pos++
	return tok
}

// expect consumes the current token if it matches the given kind,
// otherwise emits an error diagnostic and returns the current token anyway
// (consumed to guarantee progress).
func (p *Parser) expect(kind TokenKind) Token {
	tok := p.peek()
	if tok.Kind == kind {
		return p.bump()
	}
	p.diags.Error(tok.Span, "expected "+kind.String()+", got "+tok.Kind.String(), "add '"+kind.String()+"' here")
	return p.bump()
}

// at returns true if the current token has the given kind.
func (p *Parser) at(kind TokenKind) bool {
	return p.peek().Kind == kind
}

// atAny returns true if the current token has one of the given kinds.
func (p *Parser) atAny(kinds ...TokenKind) bool {
	k := p.peek().Kind
	for _, kk := range kinds {
		if k == kk {
			return true
		}
	}
	return false
}

// atCompTimeAssign reports whether the next two tokens form '::' (two colons).
// The tokenizer emits each ':' as a separate TkColon; the compile-time
// assignment meaning is resolved here at the AST level.
func (p *Parser) atCompTimeAssign() bool {
	return p.at(TkColon) && p.peekN(1).Kind == TkColon
}

// atInfer reports whether the next two tokens form ':=' (colon then assign).
func (p *Parser) atInfer() bool {
	return p.at(TkColon) && p.peekN(1).Kind == TkAssign
}

// match consumes the current token if it matches the given kind and returns
// true. Returns false without consuming otherwise.
func (p *Parser) match(kind TokenKind) bool {
	if p.at(kind) {
		p.bump()
		return true
	}
	return false
}

// syncStmt skips tokens until a statement-starting or block-ending token.
// Used for error recovery after a statement parse failure.
func (p *Parser) syncStmt() {
	for {
		tok := p.peek()
		switch tok.Kind {
		case TkEOF, TkRBrace, TkSemicolon:
			return
		case TkIdent, TkReturn, TkExit, TkIf, TkLBrace:
			return
		}
		p.bump()
	}
}

// skipToMatchedBraces skips tokens from the current position until the
// matching '}', a depth-0 ';', or EOF. The depth-0 ';' stop prevents
// brace-less forms from swallowing the rest of the file. An unmatched '}' at
// depth 0 is left for the enclosing block.
func (p *Parser) skipToMatchedBraces() {
	depth := 0
	for {
		tok := p.peek()
		switch tok.Kind {
		case TkEOF:
			return
		case TkSemicolon:
			if depth == 0 {
				p.bump()
				return
			}
		case TkLBrace:
			depth++
		case TkRBrace:
			if depth == 0 {
				// Unmatched '}' — belongs to an enclosing block.
				return
			}
			depth--
			p.bump()
			if depth == 0 {
				// Matched the opening brace.
				return
			}
			continue
		}
		p.bump()
	}
}

// skipToMatchedBrace skips tokens from the current position until the
// matching '}' or EOF, ignoring semicolons. Used for known-but-unimplemented
// statement keywords (for, while) whose headers may contain semicolons.
func (p *Parser) skipToMatchedBrace() {
	depth := 0
	for {
		tok := p.peek()
		switch tok.Kind {
		case TkEOF:
			return
		case TkLBrace:
			depth++
		case TkRBrace:
			if depth == 0 {
				// Unmatched '}' — belongs to an enclosing block.
				return
			}
			depth--
			p.bump()
			if depth == 0 {
				// Matched the opening brace.
				return
			}
			continue
		}
		p.bump()
	}
}

// parseDecl tries to parse a top-level declaration. Returns (decl, true) on
// success, or (nil, false) on failure (caller must make progress).
func (p *Parser) parseDecl() (Decl, bool) {
	// Skip stray semicolons at top level
	for p.at(TkSemicolon) {
		p.bump()
	}

	// Top-level '#entry name :: proc {...}' is the canonical entry point.
	// Handle it in both strict and tolerant modes so the CLI accepts it.
	if p.at(TkHash) && p.peekN(1).Kind == TkDirec && p.peekN(1).Value == "entry" &&
		p.peekN(2).Kind == TkIdent && p.peekN(3).Kind == TkColon &&
		p.peekN(4).Kind == TkColon && p.peekN(5).Kind == TkProc {
		p.bump() // consume '#'
		p.bump() // consume TkDirec("entry")
		nameTok := p.bump() // consume the ident
		name := nameTok.Text()
		p.bump() // consume first ':' of '::'
		p.bump() // consume second ':' of '::'
		p.bump() // consume "proc"
		return p.parseProcDecl(nameTok, name)
	}

	// Tolerant mode: skip other top-level directives (#import «fmt.chaos»; etc.)
	// without emitting a diagnostic.
	if p.tolerant && p.at(TkHash) {
		p.skipToMatchedBraces()
		return nil, false
	}

	if !p.at(TkIdent) {
		tok := p.peek()
		if tok.Kind != TkEOF {
			p.diags.Error(tok.Span, "expected declaration, got "+tok.Kind.String(), "start the line with an identifier followed by '::', ':', or ':='")
		}
		return nil, false
	}

	nameTok := p.bump()
	name := nameTok.Text()

	// Must be followed by ::, :, or := to be a declaration
	if p.atCompTimeAssign() {
		p.bump() // consume first ':' of '::'
		p.bump() // consume second ':' of '::'
		// Tolerant mode: handle forms the strict parser rejects.
		if p.tolerant {
			// ident :: #entry proc {...} — the canonical entry point. Skip
			// the directive tokens, then parse the procedure normally so
			// `main` stays in the symbol index.
			if p.at(TkHash) && p.peekN(1).Kind == TkDirec && p.peekN(1).Value == "entry" {
				p.bump() // consume '#'
				p.bump() // consume TkDirec("entry")
				if p.at(TkProc) {
					p.bump() // consume "proc"
					return p.parseProcDecl(nameTok, name)
				}
				p.skipToMatchedBraces()
				return nil, false
			}
			// ident :: enum {...} — still tolerant-only (struct is a keyword
			// handled by parseProcOrVarDecl in both modes).
			if p.at(TkIdent) && p.peek().Text() == "enum" {
				p.skipToMatchedBraces()
				return nil, false
			}
		}
		return p.parseProcOrVarDecl(nameTok, name, true)
	}
	if p.atInfer() {
		p.bump() // consume ':' of ':='
		p.bump() // consume '=' of ':='
		return p.parseInferVarDecl(nameTok, name)
	}
	if p.at(TkColon) {
		p.bump()
		return p.parseTypedVarDecl(nameTok, name)
	}

	// Not a declaration — maybe a reassignment or expression statement
	// At top level, these are errors.
	tok := p.peek()
	p.diags.Error(tok.Span, "expected '::', ':', or ':=' after identifier in declaration", "add '::', ':', or ':=' after the identifier")
	return nil, false
}

// parseProcOrVarDecl handles the case where we've consumed ident "::".
// If next token is "proc", it's a procedure declaration. If it is "struct",
// it's a struct type definition. Otherwise it's a compile-time variable
// declaration.
func (p *Parser) parseProcOrVarDecl(nameTok Token, name string, compileTime bool) (Decl, bool) {
	if p.at(TkProc) {
		p.bump() // consume "proc"
		return p.parseProcDecl(nameTok, name)
	}
	if p.at(TkStruct) {
		p.bump() // consume "struct"
		return p.parseStructDecl(nameTok, name)
	}
	// Compile-time variable: ident "::" expr ";"
	init := p.parseExpr(0)
	if init == nil {
		p.diags.Error(p.peek().Span, "expected expression after '::'", "add an expression after '::'")
		return nil, false
	}
	p.expect(TkSemicolon)
	decl := &VarDecl{
		Span_:       spanUnion(nameTok.Span, init.nodeSpan()),
		Name:        name,
		Init:        init,
		Mutable:     false,
		CompileTime: true,
	}
	return decl, true
}

// parseInferVarDecl parses: ident ":=" expr ";"
func (p *Parser) parseInferVarDecl(nameTok Token, name string) (Decl, bool) {
	init := p.parseExpr(0)
	if init == nil {
		// If we can't parse an expression, emit error and try to recover
		p.diags.Error(p.peek().Span, "expected expression after ':='", "add an expression after ':='")
		p.syncStmt()
		if p.at(TkSemicolon) {
			p.bump()
		}
		// Return a partial declaration so we can continue
		decl := &VarDecl{
			Span_:       nameTok.Span,
			Name:        name,
			Mutable:     true,
			CompileTime: false,
		}
		return decl, true
	}
	p.expect(TkSemicolon)
	decl := &VarDecl{
		Span_:       spanUnion(nameTok.Span, init.nodeSpan()),
		Name:        name,
		Init:        init,
		Mutable:     true,
		CompileTime: false,
	}
	return decl, true
}

// parseTypedVarDecl parses: ident ":" type ("=" expr)? ";"
func (p *Parser) parseTypedVarDecl(nameTok Token, name string) (Decl, bool) {
	typeExpr := p.parseTypeExpr()
	if typeExpr == nil {
		p.diags.Error(p.peek().Span, "expected type after ':'", "add a type name after ':'")
		p.syncStmt()
		if p.at(TkSemicolon) {
			p.bump()
		}
		decl := &VarDecl{
			Span_:       nameTok.Span,
			Name:        name,
			Mutable:     true,
			CompileTime: false,
		}
		return decl, true
	}

	var init Expr
	compileTime := false
	if p.match(TkAssign) {
		init = p.parseExpr(0)
		if init == nil {
			p.diags.Error(p.peek().Span, "expected expression after '='", "add an expression after '='")
		}
	} else if p.at(TkColon) {
		// name : Type : value — compile-time constant with explicit type.
		// The second ':' mirrors '::' (compile-time) while keeping the type
		// explicit instead of inferred.
		p.bump() // consume ':'
		compileTime = true
		init = p.parseExpr(0)
		if init == nil {
			p.diags.Error(p.peek().Span, "expected expression after ':'", "add an expression after ':'")
		}
	}

	p.expect(TkSemicolon)

	end := nameTok.Span.End
	if init != nil {
		end = init.nodeSpan().End
	} else if typeExpr != nil {
		end = typeExpr.nodeSpan().End
	}

	decl := &VarDecl{
		Span_:       Span{File: nameTok.Span.File, Start: nameTok.Span.Start, End: end},
		Name:        name,
		DeclType:    typeExpr,
		Init:        init,
		Mutable:     !compileTime,
		CompileTime: compileTime,
	}
	return decl, true
}

// parseTypeExpr parses a type expression (currently just an identifier).
func (p *Parser) parseTypeExpr() Expr {
	if p.at(TkIdent) {
		tok := p.bump()
		return &IdentExpr{Span_: tok.Span, Name: tok.Text()}
	}
	return nil
}

// parseProcDecl parses the rest of a procedure declaration after "proc" has
// been consumed. Parameters, results, and body.
func (p *Parser) parseProcDecl(nameTok Token, name string) (Decl, bool) {
	// Parameters: ( ... )
	var params []Param
	if p.at(TkLParen) {
		p.bump() // consume "("
		for !p.at(TkRParen) && !p.at(TkEOF) {
			param, ok := p.parseParam()
			if !ok {
				break
			}
			params = append(params, param)
			if !p.at(TkComma) {
				break
			}
			commaTok := p.bump()
			if p.at(TkRParen) {
				p.diags.Error(commaTok.Span, "trailing comma after parameter", "remove the trailing comma")
				break
			}
		}
		p.expect(TkRParen)
	} else {
		// No params: empty param list
	}

	// Results: -> type ...
	var results []Expr
	if p.at(TkArrow) {
		p.bump() // consume "->"
		// At least one result type
		first := p.parseTypeExpr()
		if first == nil {
			p.diags.Error(p.peek().Span, "expected return type after '->'", "add a return type after '->'")
		} else {
			results = append(results, first)
			for p.match(TkComma) {
				next := p.parseTypeExpr()
				if next == nil {
					p.diags.Error(p.peek().Span, "expected return type after ','", "add a return type after ','")
					break
				}
				results = append(results, next)
			}
		}
	}

	// Body
	body := p.parseBlock()
	if body == nil {
		p.diags.Error(p.peek().Span, "expected procedure body '{'", "add a '{' block for the procedure body")
		return nil, false
	}

	decl := &ProcDecl{
		Span_:   spanUnion(nameTok.Span, body.Span_),
		Name:    name,
		Params:  params,
		Results: results,
		Body:    body,
	}
	return decl, true
}

// parseStructDecl parses the rest of a struct type definition after "struct"
// has been consumed. Fields are "name: type;" entries inside a braced block.
// The struct declaration itself does not require a trailing semicolon.
func (p *Parser) parseStructDecl(nameTok Token, name string) (Decl, bool) {
	if !p.at(TkLBrace) {
		p.diags.Error(p.peek().Span, "expected '{' after 'struct'", "add a '{' block for the struct fields")
		return nil, false
	}
	p.bump() // consume "{"

	var fields []StructField
	for !p.at(TkRBrace) && !p.at(TkEOF) {
		before := p.pos
		field, ok := p.parseStructField()
		if ok {
			fields = append(fields, field)
		}
		// Guarantee forward motion on malformed fields.
		if p.pos == before && p.peek().Kind != TkRBrace && p.peek().Kind != TkEOF {
			p.bump()
		}
	}
	closeTok := p.expect(TkRBrace)

	decl := &StructDecl{
		Span_:  Span{File: nameTok.Span.File, Start: nameTok.Span.Start, End: closeTok.Span.End},
		Name:   name,
		Fields: fields,
	}
	return decl, true
}

// parseStructField parses a single struct field: "name: type;"
func (p *Parser) parseStructField() (StructField, bool) {
	if !p.at(TkIdent) {
		p.diags.Error(p.peek().Span, "expected field name", "add a field name")
		return StructField{}, false
	}
	nameTok := p.bump()
	name := nameTok.Text()

	if !p.at(TkColon) {
		p.diags.Error(p.peek().Span, "expected ':' after field name", "add ':' after the field name")
		return StructField{Span_: nameTok.Span, Name: name}, true
	}
	p.bump() // consume ":"

	typeExpr := p.parseTypeExpr()
	if typeExpr == nil {
		p.diags.Error(p.peek().Span, "expected field type after ':'", "add a type after ':'")
		return StructField{Span_: nameTok.Span, Name: name}, true
	}

	p.expect(TkSemicolon)

	return StructField{
		Span_: spanUnion(nameTok.Span, typeExpr.nodeSpan()),
		Name:  name,
		Type:  typeExpr,
	}, true
}

// parseParam parses a single parameter: ident ":" type
func (p *Parser) parseParam() (Param, bool) {
	if !p.at(TkIdent) {
		p.diags.Error(p.peek().Span, "expected parameter name", "add a parameter name")
		return Param{}, false
	}
	nameTok := p.bump()
	name := nameTok.Text()

	if !p.at(TkColon) {
		p.diags.Error(p.peek().Span, "expected ':' after parameter name", "add ':' after the parameter name")
		return Param{Span_: nameTok.Span, Name: name}, true
	}
	p.bump() // consume ":"

	typeExpr := p.parseTypeExpr()
	if typeExpr == nil {
		p.diags.Error(p.peek().Span, "expected parameter type after ':'", "add a type after ':'")
		return Param{Span_: nameTok.Span, Name: name}, true
	}

	return Param{
		Span_: spanUnion(nameTok.Span, typeExpr.nodeSpan()),
		Name:  name,
		Type:  typeExpr,
	}, true
}

// parseStmt parses a single statement.
func (p *Parser) parseStmt() Stmt {
	// Skip empty statements (stray semicolons)
	if p.match(TkSemicolon) {
		return nil
	}

	switch p.peek().Kind {
	case TkLBrace:
		return p.parseBlock()

	case TkReturn:
		return p.parseReturnStmt()

	case TkExit:
		return p.parseExitStmt()

	case TkIf:
		return p.parseIfStmt()

	case TkIdent:
		// ident - could be decl, assign, or just an expression statement
		return p.parseIdentStmt()

	default:
		// Tolerant mode: skip directives (#foo ...;) inside bodies.
		if p.tolerant && p.at(TkHash) {
			p.skipToMatchedBraces()
			return nil
		}
		tok := p.peek()
		if tok.Kind != TkEOF && tok.Kind != TkRBrace {
			p.diags.Error(tok.Span, "unexpected token "+tok.Kind.String()+" in statement", "remove the token or start a valid statement")
			p.syncStmt()
			if p.at(TkSemicolon) {
				p.bump()
			}
		}
		return nil
	}
}

// parseIdentStmt handles statements starting with an identifier. This could be
// a declaration, a reassignment, a procedure declaration, or an expression
// statement (call).
func (p *Parser) parseIdentStmt() Stmt {
	nameTok := p.bump()
	name := nameTok.Text()

	switch {
	case p.atCompTimeAssign():
		p.bump() // consume first ':' of '::'
		p.bump() // consume second ':' of '::'
		decl, ok := p.parseProcOrVarDecl(nameTok, name, true)
		if !ok {
			return nil
		}
		return decl

	case p.atInfer():
		p.bump() // consume ':' of ':='
		p.bump() // consume '=' of ':='
		decl, ok := p.parseInferVarDecl(nameTok, name)
		if !ok {
			return nil
		}
		return decl

	case p.at(TkColon):
		p.bump()
		decl, ok := p.parseTypedVarDecl(nameTok, name)
		if !ok {
			return nil
		}
		return decl

	case p.at(TkAssign):
		p.bump()
		return p.parseAssignStmt(nameTok, name)

	case p.at(TkLParen):
		// Call expression used as a statement: f(args);
		p.bump() // consume "("
		expr := p.parseCallArgs(&IdentExpr{Span_: nameTok.Span, Name: name}, nameTok.Span)
		// Handle chained calls: f()()
		for p.at(TkLParen) {
			p.bump() // consume "("
			expr = p.parseCallArgs(expr, expr.nodeSpan())
		}
		p.expect(TkSemicolon)
		// Wrap in an expression statement.
		return &ExprStmt{
			Span_: expr.Span_,
			Expr:  expr,
		}

	default:
		// Tolerant mode: skip known-but-unimplemented statement keywords
		// (for, while) as balanced blocks. Other unknown ident-led
		// statements still error.
		if p.tolerant && (name == "for" || name == "while") {
			p.skipToMatchedBrace()
			return nil
		}
		// Bare identifier without assignment/declaration prefix.
		// Could be an expression statement (e.g., a function call without
		// parens — not currently supported). Emit error.
		tok := p.peek()
		p.diags.Error(tok.Span, "unexpected token after identifier '"+name+"'", "add '=', ':=', '::', or '(' after the identifier")
		p.syncStmt()
		if p.at(TkSemicolon) {
			p.bump()
		}
		return nil
	}
}

// parseAssignStmt parses: name "=" expr ";"
func (p *Parser) parseAssignStmt(nameTok Token, name string) Stmt {
	value := p.parseExpr(0)
	if value == nil {
		p.diags.Error(p.peek().Span, "expected expression after '='", "add an expression after '='")
		p.syncStmt()
		if p.at(TkSemicolon) {
			p.bump()
		}
		return &AssignStmt{
			Span_: nameTok.Span,
			Name:  name,
		}
	}
	p.expect(TkSemicolon)
	return &AssignStmt{
		Span_: spanUnion(nameTok.Span, value.nodeSpan()),
		Name:  name,
		Value: value,
	}
}

// parseReturnStmt parses: "return" expr? ";"
func (p *Parser) parseReturnStmt() Stmt {
	tok := p.bump() // consume "return"

	// If next token is a semicolon, it's a bare return.
	if p.at(TkSemicolon) {
		p.bump()
		return &ReturnStmt{
			Span_: tok.Span,
		}
	}

	value := p.parseExpr(0)
	if value == nil {
		p.diags.Error(p.peek().Span, "expected expression or ';' after 'return'", "add an expression or ';' after 'return'")
		p.syncStmt()
		if p.at(TkSemicolon) {
			p.bump()
		}
		return &ReturnStmt{
			Span_: tok.Span,
		}
	}
	p.expect(TkSemicolon)
	return &ReturnStmt{
		Span_: spanUnion(tok.Span, value.nodeSpan()),
		Value: value,
	}
}

// parseExitStmt parses: "exit" expr ("," expr)? ";"
func (p *Parser) parseExitStmt() Stmt {
	tok := p.bump() // consume "exit"

	status := p.parseExpr(0)
	if status == nil {
		p.diags.Error(p.peek().Span, "expected expression after 'exit'", "add an expression after 'exit'")
		p.syncStmt()
		if p.at(TkSemicolon) {
			p.bump()
		}
		return &ExitStmt{Span_: tok.Span}
	}

	var message Expr
	if p.match(TkComma) {
		message = p.parseExpr(0)
		if message == nil {
			p.diags.Error(p.peek().Span, "expected expression after ',' in exit", "add an expression after ','")
		}
	}

	p.expect(TkSemicolon)

	end := status.nodeSpan().End
	if message != nil {
		end = message.nodeSpan().End
	}
	return &ExitStmt{
		Span_:   Span{File: tok.Span.File, Start: tok.Span.Start, End: end},
		Status:  status,
		Message: message,
	}
}

// parseIfStmt parses: "if" expr block ("elif" expr block)* ("else" block)?
func (p *Parser) parseIfStmt() Stmt {
	tok := p.bump() // consume "if"

	cond := p.parseExpr(0)
	if cond == nil {
		p.diags.Error(p.peek().Span, "expected condition after 'if'", "add a condition after 'if'")
		cond = &ErrorExpr{Span_: p.peek().Span}
	}

	body := p.parseBlock()
	if body == nil {
		p.diags.Error(p.peek().Span, "expected block after 'if' condition", "add a '{' block after the condition")
		body = &BlockStmt{Span_: p.peek().Span}
	}

	var elifs []*IfStmt
	for p.at(TkElif) {
		elifTok := p.bump() // consume "elif"
		elifCond := p.parseExpr(0)
		if elifCond == nil {
			p.diags.Error(p.peek().Span, "expected condition after 'elif'", "add a condition after 'elif'")
			elifCond = &ErrorExpr{Span_: p.peek().Span}
		}
		elifBody := p.parseBlock()
		if elifBody == nil {
			p.diags.Error(p.peek().Span, "expected block after 'elif' condition", "add a '{' block after the condition")
			elifBody = &BlockStmt{Span_: p.peek().Span}
		}
		elifs = append(elifs, &IfStmt{
			Span_:     spanUnion(elifTok.Span, elifBody.Span_),
			Condition: elifCond,
			Body:      elifBody,
		})
	}

	var elseBody *BlockStmt
	if p.at(TkElse) {
		p.bump() // consume "else"
		// Check for elif (else + if)
		if p.at(TkIf) {
			// "else if" is not valid syntax — must use "elif"
			p.diags.Error(p.peek().Span, "use 'elif' instead of 'else if'", "replace 'else if' with 'elif'")
			innerIf := p.parseIfStmt()
			elseBody = &BlockStmt{
				Span_: innerIf.nodeSpan(),
				Stmts: []Stmt{innerIf},
			}
		} else {
			elseBody = p.parseBlock()
			if elseBody == nil {
				p.diags.Error(p.peek().Span, "expected block after 'else'", "add a '{' block after 'else'")
				elseBody = &BlockStmt{Span_: p.peek().Span}
			}
		}
	}

	end := body.Span_.End
	if elseBody != nil {
		end = elseBody.Span_.End
	} else if len(elifs) > 0 {
		end = elifs[len(elifs)-1].Body.Span_.End
	}

	return &IfStmt{
		Span_:     Span{File: tok.Span.File, Start: tok.Span.Start, End: end},
		Condition: cond,
		Body:      body,
		Elif:      elifs,
		ElseBody:  elseBody,
	}
}

// parseBlock parses: "{" stmt* "}"
func (p *Parser) parseBlock() *BlockStmt {
	if !p.at(TkLBrace) {
		return nil
	}

	openTok := p.bump() // consume "{"
	var stmts []Stmt

	for !p.at(TkRBrace) && !p.at(TkEOF) {
		before := p.pos
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		// If no progress was made (parseStmt returned nil without consuming),
		// consume one token to guarantee forward motion.
		if p.pos == before && p.peek().Kind != TkRBrace && p.peek().Kind != TkEOF {
			p.bump()
		}
	}

	closeTok := p.expect(TkRBrace)

	return &BlockStmt{
		Span_: Span{File: openTok.Span.File, Start: openTok.Span.Start, End: closeTok.Span.End},
		Stmts: stmts,
	}
}

// Precedence levels (lower = binds weaker).
const (
	precLowest  = iota // 0
	precOr             // 1  ||
	precAnd            // 2  &&
	precEq             // 3  == !=
	precCmp            // 4  < > <= >=
	precAdd            // 5  + -
	precMul            // 6  * / %
	precUnary          // 7  - !
	precCall           // 8  ()
	precPrimary        // 9  primary
)

// tokenPrecedence returns the left-binding power (lbp) for a token kind.
// Returns 0 for tokens that are not infix/postfix operators.
func tokenPrecedence(kind TokenKind) int {
	switch kind {
	case TkOr:
		return precOr
	case TkAnd:
		return precAnd
	case TkEq, TkNeq:
		return precEq
	case TkLt, TkGt, TkLe, TkGe:
		return precCmp
	case TkPlus, TkMinus:
		return precAdd
	case TkStar, TkSlash, TkPercent:
		return precMul
	case TkLParen:
		return precCall
	}
	return 0
}

// parseExpr is the main expression entry point. It parses an expression with
// minimum binding power minBp.
func (p *Parser) parseExpr(minBp int) Expr {
	left := p.parsePrimaryExpr()
	if left == nil {
		return nil
	}

	for {
		tok := p.peek()
		if tok.Kind == TkEOF || tok.Kind == TkSemicolon || tok.Kind == TkRBrace ||
			tok.Kind == TkRParen || tok.Kind == TkComma {
			break
		}

		bp := tokenPrecedence(tok.Kind)
		if bp == 0 || bp < minBp {
			break
		}

		// Consume the operator
		p.bump()

		// Handle calls (postfix)
		if tok.Kind == TkLParen {
			left = p.parseCallArgs(left, tok.Span)
			continue
		}

		// Binary operator
		op := tokToBinaryOp(tok.Kind)
		if op < 0 {
			p.diags.Error(tok.Span, "unexpected token "+tok.Kind.String()+" in expression", "remove the token or add a valid operator")
			break
		}

		right := p.parseExpr(bp + 1)
		if right == nil {
			p.diags.Error(p.peek().Span, "expected expression after operator", "add an expression after the operator")
			break
		}

		left = &BinaryExpr{
			Span_: spanUnion(left.nodeSpan(), right.nodeSpan()),
			Op:    BinaryOp(op),
			Left:  left,
			Right: right,
		}
	}

	return left
}

// parsePrimaryExpr parses a primary expression (atom or unary prefix).
func (p *Parser) parsePrimaryExpr() Expr {
	tok := p.peek()

	// Unary prefix operators
	if tok.Kind == TkMinus || tok.Kind == TkNot {
		p.bump()
		op := UnaryOpNeg
		if tok.Kind == TkNot {
			op = UnaryOpNot
		}
		operand := p.parseExpr(precUnary)
		if operand == nil {
			p.diags.Error(p.peek().Span, "expected expression after unary operator", "add an expression after the unary operator")
			return &ErrorExpr{Span_: tok.Span}
		}
		return &UnaryExpr{
			Span_:   spanUnion(tok.Span, operand.nodeSpan()),
			Op:      op,
			Operand: operand,
		}
	}

	return p.parseAtom()
}

// parseAtom parses an atomic expression (literal, identifier, parenthesized).
func (p *Parser) parseAtom() Expr {
	tok := p.peek()

	switch tok.Kind {
	case TkInt:
		p.bump()
		return &IntExpr{Span_: tok.Span, Value: tok.Value}

	case TkFloat:
		p.bump()
		return &FloatExpr{Span_: tok.Span, Value: tok.Value}

	case TkString:
		p.bump()
		return &StringExpr{Span_: tok.Span, Value: tok.Value}

	case TkTrue:
		p.bump()
		return &BoolExpr{Span_: tok.Span, Value: true}

	case TkFalse:
		p.bump()
		return &BoolExpr{Span_: tok.Span, Value: false}

	case TkIdent:
		p.bump()
		ident := &IdentExpr{Span_: tok.Span, Name: tok.Text()}
		// Check for call: f(...)
		if p.at(TkLParen) {
			p.bump() // consume "("
			return p.parseCallArgs(ident, tok.Span)
		}
		// Struct literal with explicit type: TypeName.{...}
		if p.at(TkDot) && p.peekN(1).Kind == TkLBrace {
			p.bump() // consume "."
			return p.parseStructInit(ident, tok.Span)
		}
		return ident

	case TkDot:
		// Struct literal with inferred type: .{...}
		if p.peekN(1).Kind == TkLBrace {
			p.bump() // consume "."
			return p.parseStructInit(nil, tok.Span)
		}
		return nil

	case TkLParen:
		p.bump() // consume "("
		inner := p.parseExpr(0)
		if inner == nil {
			p.diags.Error(p.peek().Span, "expected expression after '('", "add an expression after '('")
			p.expect(TkRParen)
			return &ErrorExpr{Span_: tok.Span}
		}
		closeTok := p.expect(TkRParen)
		return &ParenExpr{
			Span_: Span{File: tok.Span.File, Start: tok.Span.Start, End: closeTok.Span.End},
			Inner: inner,
		}

	default:
		return nil
	}
}

// parseCallArgs parses the argument list after "(" has been consumed.
// It returns a CallExpr wrapping the function expression.
func (p *Parser) parseCallArgs(fn Expr, openSpan Span) *CallExpr {
	var args []Expr
	for !p.at(TkRParen) && !p.at(TkEOF) {
		arg := p.parseExpr(0)
		if arg == nil {
			p.diags.Error(p.peek().Span, "expected expression in call argument", "add an expression as the call argument")
			break
		}
		args = append(args, arg)
		if !p.at(TkComma) {
			break
		}
		commaTok := p.bump()
		if p.at(TkRParen) {
			p.diags.Error(commaTok.Span, "trailing comma in call argument", "remove the trailing comma")
			break
		}
	}
	closeTok := p.expect(TkRParen)
	return &CallExpr{
		Span_: spanUnion(fn.nodeSpan(), closeTok.Span),
		Func:  fn,
		Args:  args,
	}
}

// parseStructInit parses the "{ field=value, ... }" part of a struct literal
// after the "." has been consumed. typeName is nil for inferred literals
// (.{...}). Fields may be named (field=value) or positional, in any order;
// ordering and field-existence validation is left to a later type-checking
// pass.
func (p *Parser) parseStructInit(typeName Expr, dotSpan Span) Expr {
	if !p.at(TkLBrace) {
		p.diags.Error(p.peek().Span, "expected '{' after '.' in struct literal", "add a '{' block for the struct fields")
		return &ErrorExpr{Span_: dotSpan}
	}
	openTok := p.bump() // consume "{"

	var fields []StructInitField
	for !p.at(TkRBrace) && !p.at(TkEOF) {
		before := p.pos
		field, ok := p.parseStructInitField()
		if ok {
			fields = append(fields, field)
		}
		// Guarantee forward motion on malformed fields.
		if p.pos == before && p.peek().Kind != TkRBrace && p.peek().Kind != TkEOF {
			p.bump()
		}
		if !p.at(TkComma) {
			break
		}
		commaTok := p.bump()
		if p.at(TkRBrace) {
			p.diags.Error(commaTok.Span, "trailing comma in struct literal", "remove the trailing comma")
			break
		}
	}
	closeTok := p.expect(TkRBrace)

	start := dotSpan.Start
	if typeName != nil {
		start = typeName.nodeSpan().Start
	}
	return &StructInitExpr{
		Span_:  Span{File: openTok.Span.File, Start: start, End: closeTok.Span.End},
		Type:   typeName,
		Fields: fields,
	}
}

// parseStructInitField parses a single struct literal entry: "name=value" or
// a positional value.
func (p *Parser) parseStructInitField() (StructInitField, bool) {
	// Named field: ident "=" expr
	if p.at(TkIdent) && p.peekN(1).Kind == TkAssign {
		nameTok := p.bump()
		p.bump() // consume "="
		value := p.parseExpr(0)
		if value == nil {
			p.diags.Error(p.peek().Span, "expected value after '=' in struct literal", "add a value after '='")
			return StructInitField{Span_: nameTok.Span, Name: nameTok.Text()}, true
		}
		return StructInitField{
			Span_: spanUnion(nameTok.Span, value.nodeSpan()),
			Name:  nameTok.Text(),
			Value: value,
		}, true
	}

	// Positional value
	value := p.parseExpr(0)
	if value == nil {
		p.diags.Error(p.peek().Span, "expected field value in struct literal", "add a field value")
		return StructInitField{}, false
	}
	return StructInitField{
		Span_: value.nodeSpan(),
		Value: value,
	}, true
}

// tokToBinaryOp maps a token kind to a BinaryOp, or -1 if not a binary op.
func tokToBinaryOp(kind TokenKind) int {
	switch kind {
	case TkPlus:
		return int(BinaryOpAdd)
	case TkMinus:
		return int(BinaryOpSub)
	case TkStar:
		return int(BinaryOpMul)
	case TkSlash:
		return int(BinaryOpDiv)
	case TkPercent:
		return int(BinaryOpMod)
	case TkLt:
		return int(BinaryOpLt)
	case TkGt:
		return int(BinaryOpGt)
	case TkLe:
		return int(BinaryOpLe)
	case TkGe:
		return int(BinaryOpGe)
	case TkEq:
		return int(BinaryOpEq)
	case TkNeq:
		return int(BinaryOpNeq)
	case TkAnd:
		return int(BinaryOpAnd)
	case TkOr:
		return int(BinaryOpOr)
	}
	return -1
}

// spanUnion returns a Span that covers both spans.
func spanUnion(a, b Span) Span {
	start := a.Start
	if b.Start < start {
		start = b.Start
	}
	end := a.End
	if b.End > end {
		end = b.End
	}
	return Span{File: a.File, Start: start, End: end}
}
