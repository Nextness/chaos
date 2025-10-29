package main

import "fmt"

func InferVarDeclType(program *ChaosSlice[Node], scope Scope) bool {
	node := CSGetPointer(program)

	if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
		for idx, n := range scope {
			if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
				s64 := MakeType("S64")
				node.VarDecl.Type = s64
				scope[idx].VarDecl.Type = s64
				return true
			}
		}
		assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
	}

	if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
		for idx, n := range scope {
			if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
				str := MakeType("String")
				node.VarDecl.Type = str
				scope[idx].VarDecl.Type = str
				return true
			}
		}
		assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
	}

	if node.VarDecl.Assignment.NodeType == nodeBoolLiteral {
		for idx, n := range scope {
			if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
				boolean := MakeType("Bool")
				node.VarDecl.Type = boolean
				scope[idx].VarDecl.Type = boolean
				return true
			}
		}
		assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
	}

	if node.VarDecl.Assignment.NodeType == nodeIdentifier {
		for idx, n := range scope {
			condition := (node.VarDecl.Assignment.NodeType == nodeIdentifier &&
				n.VarDecl.Name.Symbol == node.VarDecl.Assignment.VarDecl.Name.Symbol)
			if condition {
				node.VarDecl.Type = scope[idx].VarDecl.Type
				return true
			}
		}
		assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
	}

	if node.VarDecl.Assignment.NodeType == nodeCall {
		for _, n := range scope {
			condition := (n.NodeType == nodeIdentifier &&
				node.VarDecl.Assignment.Call.Name.Symbol == n.VarDecl.Name.Symbol)
			if condition {
				retLen := len(n.VarDecl.Assignment.Proc.Outputs)
				assert[any](retLen <= 1, "Cannot have multiple returns for now")
				node.VarDecl.Type = MakeType("Void")
				if retLen == 1 {
					node.VarDecl.Type = n.VarDecl.Assignment.Proc.Outputs[0].VarDecl.Type
				}
				return true
			}
		}
		assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
	}

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
