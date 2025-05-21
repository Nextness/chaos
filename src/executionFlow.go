package main

type ExecutionState struct {
	identifiers    map[Symbol]*NodeIdentifier
	procDefinition map[Symbol]*NodeProcDef
	entryPoint     *NodeProcDef
}

func globalExecutionOrderChaos(ls *LexerState) *ExecutionState {
	assert(ls.cursor == 0, "Cursor is not 0")
	es := ExecutionState{
		identifiers:    map[Symbol]*NodeIdentifier{},
		procDefinition: map[Symbol]*NodeProcDef{},
	}
	for ls.cursor < ls.count {
		if ls.data[ls.cursor].getNodeType() == nodeIdentifier {
			sym := Symbol("")
			if val := ls.data[ls.cursor].(*NodeIdentifier).identifier.symbol; val != Symbol("") {
				sym = val
				es.identifiers[sym] = ls.data[ls.cursor].(*NodeIdentifier)
			}
		}
		if ls.data[ls.cursor].getNodeType() == nodeProc {
			sym := Symbol("")
			if val := ls.data[ls.cursor].(*NodeProcDef).identifier.symbol; val == "main" {
				sym = val
				es.entryPoint = ls.data[ls.cursor].(*NodeProcDef)
				ls.cursor++
				continue
			}
			if val := ls.data[ls.cursor].(*NodeProcDef).identifier.symbol; val != Symbol("") {
				sym = val
				es.procDefinition[sym] = ls.data[ls.cursor].(*NodeProcDef)
			}
		}
		ls.cursor++
	}
	return &es
}
