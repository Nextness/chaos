package main

import (
	"fmt"
	_ "os"
)

func ChaosInferTypes(program *ChaosSlice[Node], scope Scope) {
	assert[any](program.cursor == 0, "Cursor is not 0")
	for program.cursor < program.count {
		if !CSMatch(program, CSNodeNodeTypeComparison, nodeIdentifier) {
			CSConsume(program)
			continue
		}

		node := CSGetPointer(program)
		if node.VarDecl.Type.TokenType != tokInfer {
			CSConsume(program)
			continue
		}

		if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
			node.VarDecl.Type = MakeType("S64")
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					scope[idx].VarDecl.Type = MakeType("S64")
					break
				}
			}
			CSConsume(program)
			continue
		}

		if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
			node.VarDecl.Type = MakeType("String")
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					scope[idx].VarDecl.Type = MakeType("String")
					break
				}
			}
			CSConsume(program)
			continue
		}

		if node.VarDecl.Assignment.NodeType == nodeBoolLiteral {
			node.VarDecl.Type = MakeType("Bool")
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					scope[idx].VarDecl.Type = MakeType("Bool")
					break
				}
			}
			CSConsume(program)
			continue
		}

		if node.VarDecl.Assignment.NodeType == nodeIdentifier {
			rhs := node.VarDecl
			lhs := node.VarDecl.Assignment
			for _, n := range scope {
				if lhs.NodeType == nodeIdentifier && lhs.VarDecl.Name.Symbol == n.VarDecl.Name.Symbol {
					rhs.Type = n.VarDecl.Type
					break
				}
			}
			CSConsume(program)
			continue
		}

		if node.VarDecl.Assignment.NodeType == nodeCall {
			rhs := node.VarDecl.Assignment.Call
			for _, n := range scope {
				if n.NodeType == nodeIdentifier && rhs.Name.Symbol == n.VarDecl.Name.Symbol {
					returnsLen := len(n.VarDecl.Assignment.Proc.Outputs)
					assert[any](returnsLen <= 1, "Cannot have multiple returns for now")
					if returnsLen != 0 {
						node.VarDecl.Type = n.VarDecl.Assignment.Proc.Outputs[0].VarDecl.Type
					}
					break
				}
			}
			CSConsume(program)
			continue
		}

		// TO-DO: Actually handle type inference for functions.
		// For now I don't really know what I want skipping it.
		if node.VarDecl.Assignment.NodeType == nodeProcDef {
			fmt.Printf("[WARNING] Skipping type inference for proc definition\n")
			CSConsume(program)
			continue
		}

		assert[any](false, fmt.Sprintf("Unexpected node type %s", NodeTypeToString(node.NodeType)))
	}
	program.cursor = 0
}
