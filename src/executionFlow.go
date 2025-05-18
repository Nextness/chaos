package main

type ExecutionState struct {
	identifiers    map[Symbol]*NodeIdentifier
	procDefinition map[Symbol]*NodeProcDef
	entryPoint     *NodeProcDef
}

func executionOrderChaos(ls *LexerState) {
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
				chaosDebug("Entry:\n%+v", es.entryPoint)
				ls.cursor++
				continue
			}
			if val := ls.data[ls.cursor].(*NodeProcDef).identifier.symbol; val != Symbol("") {
				sym = val
				es.procDefinition[sym] = ls.data[ls.cursor].(*NodeProcDef)
				chaosDebug("%s:\n%+v", sym, es.procDefinition[sym])
			}
		}
		ls.cursor++
	}
}
