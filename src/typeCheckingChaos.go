package main

import (
	_ "fmt"
)

func typeCheckingChaos(es *ExecutionState) {
	// TODO: This probably should be something that already exists from the exection order
	checked := map[Symbol]Node{}

	for _, node := range es.entryPoint.statements {
		var typeMatches bool = true
		if node, ok := cast[*NodeIdentifier](node); ok {
			// RHS is a token literal
			if token, ok := cast[*TokenLiteral](node.value); ok {
				typeMatches = typeMatches && node.varType.tokKind.matchAt(0, kindU64, kindU32)
				typeMatches = typeMatches && token.tokType == tokNumber
				checked[node.identifier.symbol] = node
				if !typeMatches {
					panic("Handle type checking error")
				}
				continue
			}

			// RHS is a node identifier
			if n, ok := cast[*NodeIdentifier](node); ok {

				// TODO: Improve how casting is handled in here
				token := castAssert[*TokenIdentifier](n.value)
				a := castAssert[*NodeIdentifier](checked[token.symbol])
				b := castAssert[*TokenLiteral](a.value)

				typeMatches = typeMatches && b.tokType == tokNumber
				typeMatches = typeMatches && n.varType.tokKind == a.varType.tokKind
				typeMatches = typeMatches && n.varType.tokKind.matchAt(0, kindU64, kindU32)
				checked[node.identifier.symbol] = a
				if !typeMatches {
					panic("Handle type checking error")
				}
				continue
			}
		}
	}
}

