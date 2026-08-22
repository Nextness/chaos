// HIR to MIR lowering for the Chaos compiler.
//
// LowerToMIR converts the typed HIR into the three-address control-flow MIR.
// Structured if/elif/else chains become basic blocks connected by branch and
// jump terminators; return and exit become terminators. Local variables are
// assigned LocalIDs in declaration order; globals are referenced by symbol.
package compiler

// LowerToMIR lowers the typed HIR into the control-flow MIR.
func LowerToMIR(hir *HIR) (*MIRProgram, DiagnosticList) {
	ml := &MIRLowerer{
		symbols: hir.Symbols,
		types:   hir.Types,
	}
	for _, g := range hir.Globals {
		if g.CompileTime {
			continue
		}
		ml.globals = append(ml.globals, MIRGlobal{
			Symbol:      g.Symbol,
			Name:        g.Name,
			Type:        g.Type,
			Mutable:     g.Mutable,
			CompileTime: g.CompileTime,
			Span:        g.Span,
		})
	}
	for _, p := range hir.Procs {
		ml.lowerFunction(p)
	}
	entry := hir.Entry
	var globalInit *MIRFunction
	for _, g := range hir.Globals {
		if g.Init != nil && !g.CompileTime {
			globalInit = ml.lowerGlobalInit(hir)
			break
		}
	}
	return &MIRProgram{
		Symbols:    hir.Symbols,
		Types:      hir.Types,
		Functions:  ml.functions,
		Globals:    ml.globals,
		Entry:      entry,
		GlobalInit: globalInit,
		Sources:    hir.Sources,
	}, ml.diags
}

// lowerGlobalInit builds a synthetic void function that stores every global
// initializer, so the backend can run it before the entry procedure. The
// function has no parameters and returns void.
func (ml *MIRLowerer) lowerGlobalInit(hir *HIR) *MIRFunction {
	ml.cur = &MIRFunction{
		Symbol: ml.symbols.Declare("__chaos_global_init"),
		Name:   "__chaos_global_init",
		Span:   Span{},
	}
	ml.localIDs = make(map[SymbolID]LocalID)
	ml.localTypes = make(map[SymbolID]TypeID)
	ml.valueCount = 0
	ml.curBlock = ml.newBlock()
	for _, g := range hir.Globals {
		if g.Init == nil || g.CompileTime {
			continue
		}
		v := ml.lowerExpr(g.Init)
		ml.emitInit(MIRStoreGlobal, g.Type, []ValueID{v}, MIRImmediate{Kind: MIRImmSymbol, Symbol: g.Symbol}, g.Span)
	}
	ml.setTerminator(MIRTerminator{Kind: MIRReturn, Value: NoValue, Span: Span{}})
	ml.functions = append(ml.functions, ml.cur)
	return ml.cur
}

// MIRLowerer lowers a HIR into the MIR. Per-function state (current function,
// block, and symbol-to-local maps) is reset for each procedure.
type MIRLowerer struct {
	symbols   *SymbolTable
	types     *TypeTable
	functions []*MIRFunction
	globals   []MIRGlobal
	diags     DiagnosticList

	cur        *MIRFunction
	curBlock   *MIRBlock
	localIDs   map[SymbolID]LocalID
	localTypes map[SymbolID]TypeID
	valueCount ValueID

	breakTargets    []BlockID // stack of loop exit blocks for break
	continueTargets []BlockID // stack of loop after blocks for continue
}

func (ml *MIRLowerer) newBlock() *MIRBlock {
	b := &MIRBlock{ID: BlockID(len(ml.cur.Blocks))}
	ml.cur.Blocks = append(ml.cur.Blocks, b)
	return b
}

func (ml *MIRLowerer) nextValue() ValueID {
	v := ml.valueCount
	ml.valueCount++
	return v
}

// emit appends an instruction to the current block and returns its result
// ValueID. If the current block already has a terminator (for example code
// after a return), a fresh block is started.
func (ml *MIRLowerer) emit(op MIROpcode, t TypeID, args []ValueID, imm MIRImmediate, span Span) ValueID {
	if ml.curBlock.Term.Kind != MIRNoTerm {
		ml.curBlock = ml.newBlock()
	}
	v := ml.nextValue()
	ml.curBlock.Instrs = append(ml.curBlock.Instrs, &MIRInstr{
		Result: v,
		Op:     op,
		Type:   t,
		Args:   args,
		Imm:    imm,
		Span:   span,
	})
	return v
}

// emitVoid appends an instruction that produces no value (for example a
// store) to the current block.
func (ml *MIRLowerer) emitVoid(op MIROpcode, t TypeID, args []ValueID, imm MIRImmediate, span Span) {
	if ml.curBlock.Term.Kind != MIRNoTerm {
		ml.curBlock = ml.newBlock()
	}
	ml.curBlock.Instrs = append(ml.curBlock.Instrs, &MIRInstr{
		Result: NoValue,
		Op:     op,
		Type:   t,
		Args:   args,
		Imm:    imm,
		Span:   span,
	})
}

func (ml *MIRLowerer) emitInit(op MIROpcode, t TypeID, args []ValueID, imm MIRImmediate, span Span) {
	ml.emitVoid(op, t, args, imm, span)
	ml.curBlock.Instrs[len(ml.curBlock.Instrs)-1].Initializing = true
}

// setTerminator sets the terminator of the current block unless it already
// has one (a block that ended in return or exit keeps that terminator).
func (ml *MIRLowerer) setTerminator(t MIRTerminator) {
	if ml.curBlock.Term.Kind == MIRNoTerm {
		ml.curBlock.Term = t
	}
}

func (ml *MIRLowerer) lowerFunction(p *HIRProc) {
	ml.cur = &MIRFunction{
		Symbol:     p.Symbol,
		Name:       p.Name,
		Results:    p.Results,
		ResultType: p.ResultType,
		Span:       p.Span,
	}
	ml.localIDs = make(map[SymbolID]LocalID)
	ml.localTypes = make(map[SymbolID]TypeID)
	ml.valueCount = 0

	for _, param := range p.Params {
		lid := LocalID(len(ml.cur.Locals))
		ml.cur.Locals = append(ml.cur.Locals, lid)
		ml.cur.LocalTypes = append(ml.cur.LocalTypes, param.Type)
		ml.cur.LocalMutable = append(ml.cur.LocalMutable, true)
		ml.cur.Params = append(ml.cur.Params, lid)
		ml.localIDs[param.Symbol] = lid
		ml.localTypes[param.Symbol] = param.Type
	}

	ml.curBlock = ml.newBlock()
	if p.Body != nil {
		ml.lowerBlock(p.Body)
	}

	// Ensure the final block has a terminator. Falling off the end of a
	// procedure that returns a value is a source error; unreachable is the
	// safe lowering. A procedure whose result is Void may fall off the end,
	// which is an implicit bare return.
	if ml.curBlock.Term.Kind == MIRNoTerm {
		if len(p.Results) == 0 || ml.types.Lookup(p.Results[0]).Kind == TypeKindVoid {
			ml.setTerminator(MIRTerminator{Kind: MIRReturn, Value: NoValue, Span: p.Span})
		} else {
			ml.setTerminator(MIRTerminator{Kind: MIRUnreachable, Span: p.Span})
		}
	}

	ml.functions = append(ml.functions, ml.cur)
}

func (ml *MIRLowerer) lowerBlock(b *HIRBlock) {
	for _, stmt := range b.Stmts {
		if ml.curBlock.Term.Kind != MIRNoTerm {
			break
		}
		ml.lowerStmt(stmt)
	}
}

func (ml *MIRLowerer) lowerStmt(s HIRStmt) {
	switch n := s.(type) {
	case *HIRVarDecl:
		ml.lowerVarDecl(n)
	case *HIRAssign:
		ml.lowerAssign(n)
	case *HIRAddrStore:
		ml.lowerAddrStore(n)
	case *HIRReturn:
		ml.lowerReturn(n)
	case *HIRExit:
		ml.lowerExit(n)
	case *HIRIf:
		ml.lowerIfChain(n)
	case *HIRBlock:
		ml.lowerBlock(n)
	case *HIRExprStmt:
		ml.lowerExpr(n.Expr)
	case *HIRIfCatch:
		ml.lowerIfCatch(n)
	case *HIRFor:
		ml.lowerFor(n)
	case *HIRBreak:
		ml.lowerBreak(n)
	case *HIRContinue:
		ml.lowerContinue(n)
	}
}

// lowerIfCatch lowers an error check into a branch on the pair's hasError
// field. The catch block binds the error field when a binding is present and
// must return or exit; the no-error path continues.
func (ml *MIRLowerer) lowerIfCatch(n *HIRIfCatch) {
	st := ml.types.Lookup(n.UnionType)
	base := ml.lowerExpr(n.Cond)
	hasErr := ml.emit(MIRFieldLoad, st.Fields[2].Type, []ValueID{base}, MIRImmediate{Kind: MIRImmField, Int: 2}, n.Span_)
	thenBlock := ml.newBlock()
	elseBlock := ml.newBlock()
	joinBlock := ml.newBlock()
	ml.setTerminator(MIRTerminator{Kind: MIRBranch, Cond: hasErr, Then: thenBlock.ID, Else: elseBlock.ID, Span: n.Span_})

	ml.curBlock = thenBlock
	if n.CatchSym != NoSymbol {
		errVal := ml.emit(MIRFieldLoad, st.Fields[1].Type, []ValueID{base}, MIRImmediate{Kind: MIRImmField, Int: 1}, n.Span_)
		lid := LocalID(len(ml.cur.Locals))
		ml.cur.Locals = append(ml.cur.Locals, lid)
		ml.cur.LocalTypes = append(ml.cur.LocalTypes, st.Fields[1].Type)
		ml.cur.LocalMutable = append(ml.cur.LocalMutable, false)
		ml.localIDs[n.CatchSym] = lid
		ml.localTypes[n.CatchSym] = st.Fields[1].Type
		ml.emitInit(MIRStoreLocal, st.Fields[1].Type, []ValueID{errVal}, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
	}
	ml.lowerBlock(n.CatchBody)
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: joinBlock.ID, Span: n.Span_})

	ml.curBlock = elseBlock
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: joinBlock.ID, Span: n.Span_})
	ml.curBlock = joinBlock
}

func (ml *MIRLowerer) lowerVarDecl(n *HIRVarDecl) {
	lid := LocalID(len(ml.cur.Locals))
	ml.cur.Locals = append(ml.cur.Locals, lid)
	ml.cur.LocalTypes = append(ml.cur.LocalTypes, n.Type)
	ml.cur.LocalMutable = append(ml.cur.LocalMutable, n.Mutable)
	ml.localIDs[n.Symbol] = lid
	ml.localTypes[n.Symbol] = n.Type
	if n.Init != nil {
		v := ml.lowerExpr(n.Init)
		ml.emitInit(MIRStoreLocal, n.Type, []ValueID{v}, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
	}
}

func (ml *MIRLowerer) lowerAssign(n *HIRAssign) {
	v := ml.lowerExpr(n.Value)
	if lid, ok := ml.localIDs[n.Target]; ok {
		ml.emitVoid(MIRStoreLocal, n.Value.hirType(), []ValueID{v}, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
		return
	}
	ml.emitVoid(MIRStoreGlobal, n.Value.hirType(), []ValueID{v}, MIRImmediate{Kind: MIRImmSymbol, Symbol: n.Target}, n.Span_)
}

// lowerAddrStore lowers a store through an address expression.
func (ml *MIRLowerer) lowerAddrStore(n *HIRAddrStore) {
	addr := ml.lowerExpr(n.Addr)
	value := ml.lowerExpr(n.Value)
	ml.emitVoid(MIRDerefStore, n.Type, []ValueID{addr, value}, MIRImmediate{}, n.Span_)
}

func (ml *MIRLowerer) lowerReturn(n *HIRReturn) {
	t := MIRTerminator{Kind: MIRReturn, Value: NoValue, Span: n.Span_}
	if n.Value != nil {
		t.Value = ml.lowerExpr(n.Value)
	}
	ml.setTerminator(t)
}

func (ml *MIRLowerer) lowerExit(n *HIRExit) {
	t := MIRTerminator{Kind: MIRTermExit, Status: NoValue, Message: NoValue, Span: n.Span_}
	if n.Status != nil {
		t.Status = ml.lowerExpr(n.Status)
	}
	if n.Message != nil {
		t.Message = ml.lowerExpr(n.Message)
	}
	ml.setTerminator(t)
}

// lowerIfChain lowers an if/elif/else chain into branch/jump blocks. The elif
// branches are lowered as a nested if inside the else block.
func (ml *MIRLowerer) lowerIfChain(n *HIRIf) {
	cond := ml.lowerExpr(n.Condition)
	thenBlock := ml.newBlock()
	elseBlock := ml.newBlock()
	joinBlock := ml.newBlock()
	ml.setTerminator(MIRTerminator{Kind: MIRBranch, Cond: cond, Then: thenBlock.ID, Else: elseBlock.ID, Span: n.Span_})

	ml.curBlock = thenBlock
	ml.lowerBlock(n.Then)
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: joinBlock.ID, Span: n.Span_})

	ml.curBlock = elseBlock
	if len(n.Elif) > 0 {
		first := n.Elif[0]
		nested := &HIRIf{
			Span_:     first.Span_,
			Condition: first.Condition,
			Then:      first.Then,
			Elif:      n.Elif[1:],
			Else:      n.Else,
		}
		ml.lowerIfChain(nested)
	} else if n.Else != nil {
		ml.lowerBlock(n.Else)
	}
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: joinBlock.ID, Span: n.Span_})

	ml.curBlock = joinBlock
}

// lowerFor lowers a loop into condition, body, after, and exit blocks. Break
// jumps to the exit block; continue jumps to the after block.
func (ml *MIRLowerer) lowerFor(n *HIRFor) {
	condBlock := ml.newBlock()
	bodyBlock := ml.newBlock()
	afterBlock := ml.newBlock()
	exitBlock := ml.newBlock()

	if n.Init != nil {
		ml.lowerStmt(n.Init)
	}
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: condBlock.ID, Span: n.Span_})

	ml.curBlock = condBlock
	cond := ml.lowerExpr(n.Cond)
	ml.setTerminator(MIRTerminator{Kind: MIRBranch, Cond: cond, Then: bodyBlock.ID, Else: exitBlock.ID, Span: n.Span_})

	ml.curBlock = bodyBlock
	ml.breakTargets = append(ml.breakTargets, exitBlock.ID)
	ml.continueTargets = append(ml.continueTargets, afterBlock.ID)
	ml.lowerBlock(n.Body)
	ml.breakTargets = ml.breakTargets[:len(ml.breakTargets)-1]
	ml.continueTargets = ml.continueTargets[:len(ml.continueTargets)-1]
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: afterBlock.ID, Span: n.Span_})

	ml.curBlock = afterBlock
	if n.After != nil {
		ml.lowerStmt(n.After)
	}
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: condBlock.ID, Span: n.Span_})

	ml.curBlock = exitBlock
}

func (ml *MIRLowerer) lowerBreak(n *HIRBreak) {
	if len(ml.breakTargets) == 0 {
		return
	}
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: ml.breakTargets[len(ml.breakTargets)-1], Span: n.Span_})
}

func (ml *MIRLowerer) lowerContinue(n *HIRContinue) {
	if len(ml.continueTargets) == 0 {
		return
	}
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: ml.continueTargets[len(ml.continueTargets)-1], Span: n.Span_})
}

func (ml *MIRLowerer) lowerExpr(e HIRExpr) ValueID {
	switch n := e.(type) {
	case *HIRConst:
		return ml.emit(MIRConst, n.Type, nil, constImmediate(n), n.Span_)
	case *HIRZero:
		return ml.emit(MIRZero, n.Type, nil, MIRImmediate{}, n.Span_)
	case *HIRPoison:
		ml.diags.Error(n.Span_, "poison value reached MIR lowering", "fix the earlier lowering error")
		return NoValue
	case *HIRRef:
		if lid, ok := ml.localIDs[n.Symbol]; ok {
			return ml.emit(MIRLoadLocal, n.Type, nil, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
		}
		return ml.emit(MIRLoadGlobal, n.Type, nil, MIRImmediate{Kind: MIRImmSymbol, Symbol: n.Symbol}, n.Span_)
	case *HIRBinary:
		l := ml.lowerExpr(n.Left)
		r := ml.lowerExpr(n.Right)
		leftType := n.Left.hirType()
		rightType := n.Right.hirType()
		leftIsPtr := ml.types.Lookup(leftType).Kind == TypeKindPointer
		rightIsPtr := ml.types.Lookup(rightType).Kind == TypeKindPointer
		// Pointer arithmetic scales by the pointed-to element size via the
		// ptr.* opcodes; pointer comparisons are plain integer comparisons.
		switch {
		case (n.Op == BinaryOpAdd || n.Op == BinaryOpSub) && leftIsPtr && !rightIsPtr:
			op := MIRPtrAdd
			if n.Op == BinaryOpSub {
				op = MIRPtrSub
			}
			return ml.emit(op, n.Type, []ValueID{l, r}, MIRImmediate{}, n.OpSpan)
		case n.Op == BinaryOpAdd && !leftIsPtr && rightIsPtr:
			// Integer + pointer: the ptr.add opcodes expect the pointer first,
			// so swap the operands for this commutative form.
			return ml.emit(MIRPtrAdd, n.Type, []ValueID{r, l}, MIRImmediate{}, n.OpSpan)
		case n.Op == BinaryOpSub && leftIsPtr && rightIsPtr:
			return ml.emit(MIRPtrDiff, n.Type, []ValueID{l, r}, MIRImmediate{}, n.OpSpan)
		}
		op, ok := binaryOpcode(n.Op)
		if !ok {
			ml.diags.Error(n.Span_, "unsupported binary operator in MIR lowering", "report this compiler bug")
			return NoValue
		}
		span := n.OpSpan
		if span.End <= span.Start {
			span = n.Span_
		}
		return ml.emit(op, n.Type, []ValueID{l, r}, MIRImmediate{}, span)
	case *HIRUnary:
		v := ml.lowerExpr(n.Operand)
		op := MIRNot
		if n.Op == UnaryOpNeg {
			op = MIRNeg
		}
		return ml.emit(op, n.Type, []ValueID{v}, MIRImmediate{}, n.Span_)
	case *HIRCall:
		args := make([]ValueID, len(n.Args))
		for i, a := range n.Args {
			args[i] = ml.lowerExpr(a)
		}
		if n.Type == ml.types.Void() {
			ml.emitVoid(MIRCall, n.Type, args, MIRImmediate{Kind: MIRImmSymbol, Symbol: n.Func}, n.Span_)
			return NoValue
		}
		return ml.emit(MIRCall, n.Type, args, MIRImmediate{Kind: MIRImmSymbol, Symbol: n.Func}, n.Span_)
	case *HIRStructInit:
		return ml.lowerStructInit(n)
	case *HIRFieldLoad:
		base := ml.lowerExpr(n.Base)
		return ml.emit(MIRFieldLoad, n.Type, []ValueID{base}, MIRImmediate{Kind: MIRImmField, Int: int64(n.Field)}, n.Span_)
	case *HIRDeref:
		addr := ml.lowerExpr(n.Operand)
		return ml.emit(MIRDerefLoad, n.Type, []ValueID{addr}, MIRImmediate{}, n.Span_)
	case *HIRAddrOf:
		// '*ident' and '*(p.*)' both produce the address value: of a symbol
		// for an identifier, or the pointer itself for a dereference.
		switch op := n.Operand.(type) {
		case *HIRRef:
			if lid, ok := ml.localIDs[op.Symbol]; ok {
				return ml.emit(MIRAddrOf, n.Type, nil, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
			}
			return ml.emit(MIRAddrOf, n.Type, nil, MIRImmediate{Kind: MIRImmSymbol, Symbol: op.Symbol}, n.Span_)
		default:
			return ml.lowerExpr(n.Operand)
		}
	case *HIRFieldAddr:
		base := ml.lowerExpr(n.Addr)
		return ml.emit(MIRFieldAddr, n.Type, []ValueID{base}, MIRImmediate{Kind: MIRImmField, Int: int64(n.Field)}, n.Span_)
	case *HIRArrayElemAddr:
		arr := ml.lowerExpr(n.Array)
		idx := ml.lowerExpr(n.Index)
		return ml.emit(MIRArrayElemAddr, n.Type, []ValueID{arr, idx}, MIRImmediate{}, n.Span_)
	case *HIRArrayInit:
		args := make([]ValueID, len(n.Items))
		for i, item := range n.Items {
			args[i] = ml.lowerExpr(item)
		}
		return ml.emit(MIRArrayInit, n.Type, args, MIRImmediate{}, n.Span_)
	case *HIRArrayLen:
		arr := ml.lowerExpr(n.Array)
		return ml.emit(MIRArrayLen, n.Type, []ValueID{arr}, MIRImmediate{}, n.Span_)
	case *HIRIndex:
		base := ml.lowerExpr(n.Base)
		idx := ml.lowerExpr(n.Index)
		return ml.emit(MIRArrayIndex, n.Type, []ValueID{base, idx}, MIRImmediate{}, n.Span_)
	case *HIRIfx:
		return ml.lowerIfx(n)
	}
	ml.diags.Error(e.hirSpan(), "unsupported HIR expression reached MIR lowering", "report this compiler bug")
	return NoValue
}

// lowerIfx lowers a ternary expression into a branch over the condition,
// two store blocks, and a join block that loads the selected value from a
// temporary local. The temporary is immutable because exactly one branch
// stores into it before the load.
func (ml *MIRLowerer) lowerIfx(n *HIRIfx) ValueID {
	cond := ml.lowerExpr(n.Cond)
	thenBlock := ml.newBlock()
	elseBlock := ml.newBlock()
	joinBlock := ml.newBlock()
	ml.setTerminator(MIRTerminator{Kind: MIRBranch, Cond: cond, Then: thenBlock.ID, Else: elseBlock.ID, Span: n.Span_})

	lid := LocalID(len(ml.cur.Locals))
	ml.cur.Locals = append(ml.cur.Locals, lid)
	ml.cur.LocalTypes = append(ml.cur.LocalTypes, n.Type)
	ml.cur.LocalMutable = append(ml.cur.LocalMutable, false)

	ml.curBlock = thenBlock
	thenVal := ml.lowerExpr(n.Then)
	ml.emitInit(MIRStoreLocal, n.Type, []ValueID{thenVal}, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: joinBlock.ID, Span: n.Span_})

	ml.curBlock = elseBlock
	elseVal := ml.lowerExpr(n.Else)
	ml.emitInit(MIRStoreLocal, n.Type, []ValueID{elseVal}, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
	ml.setTerminator(MIRTerminator{Kind: MIRJump, Target: joinBlock.ID, Span: n.Span_})

	ml.curBlock = joinBlock
	return ml.emit(MIRLoadLocal, n.Type, nil, MIRImmediate{Kind: MIRImmLocal, Local: lid}, n.Span_)
}

// lowerStructInit lowers a struct literal into a struct.init instruction whose
// arguments are in struct declaration order, so a backend can assign values
// to fields positionally. Fields missing from the literal are zero-filled.
func (ml *MIRLowerer) lowerStructInit(n *HIRStructInit) ValueID {
	st := ml.types.Lookup(n.Type)
	byField := make(map[SymbolID]HIRExpr, len(n.Fields))
	for _, f := range n.Fields {
		byField[f.Field] = f.Value
	}
	args := make([]ValueID, 0, len(st.Fields))
	for _, f := range st.Fields {
		if v, ok := byField[f.Symbol]; ok {
			args = append(args, ml.lowerExpr(v))
		} else {
			args = append(args, ml.emit(MIRZero, f.Type, nil, MIRImmediate{}, n.Span_))
		}
	}
	return ml.emit(MIRStructInit, n.Type, args, MIRImmediate{}, n.Span_)
}

func constImmediate(c *HIRConst) MIRImmediate {
	switch c.Kind {
	case ConstInt:
		return MIRImmediate{Kind: MIRImmInt, Int: c.Int, Str: c.Str}
	case ConstFloat:
		return MIRImmediate{Kind: MIRImmFloat, Float: c.Float}
	case ConstString:
		return MIRImmediate{Kind: MIRImmString, Str: c.Str}
	case ConstBool:
		return MIRImmediate{Kind: MIRImmBool, Bool: c.Bool}
	case ConstError:
		// Error values are nominal but backed by their ordinal as a 16-bit
		// integer, so they lower to an integer immediate.
		return MIRImmediate{Kind: MIRImmInt, Int: c.Int}
	}
	return MIRImmediate{}
}

func binaryOpcode(op BinaryOp) (MIROpcode, bool) {
	switch op {
	case BinaryOpAdd:
		return MIRAdd, true
	case BinaryOpSub:
		return MIRSub, true
	case BinaryOpMul:
		return MIRMul, true
	case BinaryOpDiv:
		return MIRDiv, true
	case BinaryOpMod:
		return MIRMod, true
	case BinaryOpLt:
		return MIRCmpLt, true
	case BinaryOpGt:
		return MIRCmpGt, true
	case BinaryOpLe:
		return MIRCmpLe, true
	case BinaryOpGe:
		return MIRCmpGe, true
	case BinaryOpEq:
		return MIRCmpEq, true
	case BinaryOpNeq:
		return MIRCmpNeq, true
	case BinaryOpAnd:
		return MIRAnd, true
	case BinaryOpOr:
		return MIROr, true
	}
	return 0, false
}
