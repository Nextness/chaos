package main

import "fmt"

var warnOnce = true

func ChaosInferTypes(program *ChaosSlice[Node], scope Scope) {
	assert[any](program.cursor == 0, "Cursor is not 0")

mainLoop:
	for program.cursor < program.count {
		node := CSGetPointer(program)
		if node.NodeType == nodeIdentifier && node.VarDecl.Type.TokenType != tokInfer {
			CSConsume(program)
			continue mainLoop
		}

		if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					s64 := MakeType("S64")
					node.VarDecl.Type = s64
					scope[idx].VarDecl.Type = s64
					CSConsume(program)
					continue mainLoop
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
					CSConsume(program)
					continue mainLoop
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
					CSConsume(program)
					continue mainLoop
				}
			}
			assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		if node.VarDecl.Assignment.NodeType == nodeIdentifier {
			inferNode, inferScope := false, false
			for idx, n := range scope {
				// TO-DO: I need to make this better - this is shit.
				if node.VarDecl.Assignment.NodeType == nodeIdentifier && n.VarDecl.Name.Symbol == node.VarDecl.Assignment.VarDecl.Name.Symbol {
					node.VarDecl.Type = scope[idx].VarDecl.Type
					inferNode = true
				}
				if node.VarDecl.Name.Symbol == n.VarDecl.Name.Symbol {
					scope[idx].VarDecl.Type = node.VarDecl.Type
					inferScope = true
				}
				if inferNode && inferScope {
					CSConsume(program)
					continue mainLoop
				}
			}
			assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		if node.VarDecl.Assignment.NodeType == nodeCall {
			inferNode, inferScope := false, false
			// TO-DO: I need to make this better. I'm only checking for
			// single return values. This looks like crap.
			for idx, n := range scope {
				if n.NodeType == nodeIdentifier && node.VarDecl.Assignment.Call.Name.Symbol == n.VarDecl.Name.Symbol {
					retLen := len(n.VarDecl.Assignment.Proc.Outputs)
					assert[any](retLen <= 1, "Cannot have multiple returns for now")
					node.VarDecl.Type = MakeType("Void")
					if retLen == 1 {
						node.VarDecl.Type = n.VarDecl.Assignment.Proc.Outputs[0].VarDecl.Type
					}
					inferNode = true
				}
				if inferNode && n.NodeType == nodeIdentifier && node.VarDecl.Name.Symbol == n.VarDecl.Name.Symbol {
					scope[idx].VarDecl.Type = node.VarDecl.Type
					inferScope = true
				}
				if inferNode && inferScope {
					CSConsume(program)
					continue mainLoop
				}
			}
			assert[any](false, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		// TO-DO: Actually handle type inference for functions.
		// For now I don't really know what I want skipping it.
		if node.NodeType == nodeIdentifier && node.VarDecl.Assignment.NodeType == nodeProcDef {
			if warnOnce {
				fmt.Printf("[WARNING] Skipping type inference for proc definition\n")
				warnOnce = false
			}
			CSConsume(program)
			continue mainLoop
		}

		assert[any](false, fmt.Sprintf("Unexpected node type %s", NodeTypeToString(node.NodeType)))
	}
	program.cursor = 0
}
