package main

import (
	"fmt"
)

func matchVarDeclIdentifierSymbol(a any, b any, c ...bool) bool {
	nodeA := castAssert[Node](a)
	nodeB := castAssert[Node](b)

	result := true
	for _, v := range c {
		result = result && v
	}

	return result && (nodeA.VarDecl.Name.Symbol == nodeB.VarDecl.Assignment.VarDecl.Name.Symbol)
}

func matchVarDeclLiteralSymbol(a any, b any, c ...bool) bool {
	assert[any](len(c) == 0, "This filter doesn't expect more inputs")
	nodeA := castAssert[Node](a)
	nodeB := castAssert[Node](b)
	return nodeA.VarDecl.Name.Symbol == nodeB.VarDecl.Name.Symbol
}

func MatchInScope(scope Scope, compare func(a any, b any, c ...bool) bool, b any, c ...bool) (bool, int) {
	for idx, node := range scope {
		if compare(node, b) {
			return true, idx
		}
	}
	return false, 0
}

func InferVarDeclType(program *ChaosSlice[Node], scope Scope) bool {
	node := CSGetPointer(program)

	if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
		match, idx := MatchInScope(scope, matchVarDeclLiteralSymbol, *node)
		assert[any](match, "Cannot infer variable declaration since it doesn't exist in scope")
		s64 := MakeType("S64")
		node.VarDecl.Type = s64
		scope[idx].VarDecl.Type = s64
		return true
	}

	if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
		match, idx := MatchInScope(scope, matchVarDeclLiteralSymbol, *node)
		assert[any](match, "Cannot infer variable declaration since it doesn't exist in scope")
		str := MakeType("String")
		node.VarDecl.Type = str
		scope[idx].VarDecl.Type = str
		return true
	}

	if node.VarDecl.Assignment.NodeType == nodeBoolLiteral {
		match, idx := MatchInScope(scope, matchVarDeclLiteralSymbol, *node)
		assert[any](match, "Cannot infer variable declaration since it doesn't exist in scope")
		boolean := MakeType("Bool")
		node.VarDecl.Type = boolean
		scope[idx].VarDecl.Type = boolean
		return true
	}

	if node.VarDecl.Assignment.NodeType == nodeIdentifier {
		match, idx := MatchInScope(scope, matchVarDeclIdentifierSymbol, *node, node.VarDecl.Assignment.NodeType == nodeIdentifier)
		assert[any](match, "Cannot infer variable declaration since it doesn't exist in scope")
		node.VarDecl.Type = scope[idx].VarDecl.Type
		return true
	}

	// TO-DO: Handle proc calls
	// if node.VarDecl.Assignment.NodeType == nodeCall {
	// 	rhs := node.VarDecl.Assignment.Call
	// 	for _, n := range scope {
	// 		if n.NodeType == nodeIdentifier && rhs.Name.Symbol == n.VarDecl.Name.Symbol {
	// 			returnsLen := len(n.VarDecl.Assignment.Proc.Outputs)
	// 			assert[any](returnsLen <= 1, "Cannot have multiple returns for now")
	// 			if returnsLen != 0 {
	// 				node.VarDecl.Type = n.VarDecl.Assignment.Proc.Outputs[0].VarDecl.Type
	// 			}
	// 			break
	// 		}
	// 	}
	// 	CSConsume(program)
	// 	continue
	// }
	return false
}

func ChaosInferTypes(program *ChaosSlice[Node], scope Scope) {
	assert[any](program.cursor == 0, "Cursor is not 0")
	for program.cursor < program.count {

		node := CSGetPointer(program)
		if node.NodeType == nodeIdentifier && node.VarDecl.Type.TokenType != tokInfer {
			CSConsume(program)
			continue
		}

		if InferVarDeclType(program, scope) {
			CSConsume(program)
			continue
		}

		// TO-DO: Actually handle type inference for functions.
		// For now I don't really know what I want skipping it.
		if node.NodeType == nodeIdentifier && node.VarDecl.Assignment.NodeType == nodeProcDef {
			fmt.Printf("[WARNING] Skipping type inference for proc definition\n")
			CSConsume(program)
			continue
		}

		assert[any](false, fmt.Sprintf("Unexpected node type %s", NodeTypeToString(node.NodeType)))
	}
	program.cursor = 0
}
