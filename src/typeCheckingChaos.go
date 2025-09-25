package main

import "fmt"

func TypeCheckChaosProgram(program *Program) {
	for _, node := range program.Nodes {
		if node.NodeType == nodeIdentifier {
			if node.VarDecl.Type.TokenType != tokInfer {
				if node.VarDecl.Assignment.NodeType == nodeIdentifier {
					name := node.VarDecl.Name.Symbol
					varDeclSymbol := node.VarDecl.Type.Symbol
					pos := node.VarDecl.Name.Position
					assert[any](
						varDeclSymbol == node.VarDecl.Assignment.VarDecl.Type.Symbol,
						fmt.Sprintf(
							"[%02d:%02d] For '%s' it is expected '%s' but found '%s'\n",
							pos.line, pos.column, name, varDeclSymbol,
							node.VarDecl.Assignment.VarDecl.Type.Symbol,
						),
					)
					continue
				}
			}
			if node.VarDecl.Type.TokenType == tokInfer {
				if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
					identfier := node.VarDecl.Name.Symbol
					varDeclType := Token{
						TokenType: tokIdentifier,
						Symbol:    "S64",
					}
					node.VarDecl.Type = varDeclType
					program.AllocatedVars[identfier].VarDecl.Type = varDeclType
					continue
				}
				if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
					identfier := node.VarDecl.Name.Symbol
					varDeclType := Token{
						TokenType: tokIdentifier,
						Symbol:    "String",
					}
					node.VarDecl.Type = varDeclType
					program.AllocatedVars[identfier].VarDecl.Type = varDeclType
					continue
				}
				if node.VarDecl.Assignment.NodeType == nodeBoolLiteral {
					identfier := node.VarDecl.Name.Symbol
					varDeclType := Token{
						TokenType: tokIdentifier,
						Symbol:    "Bool",
					}
					node.VarDecl.Type = varDeclType
					program.AllocatedVars[identfier].VarDecl.Type = varDeclType
					continue
				}
				if node.VarDecl.Assignment.NodeType == nodeIdentifier {
					inferedType := node.VarDecl.Assignment.VarDecl.Type.Symbol
					identfier := node.VarDecl.Name.Symbol
					varDeclType := Token{
						TokenType: tokIdentifier,
						Symbol:    inferedType,
					}
					node.VarDecl.Type = varDeclType
					program.AllocatedVars[identfier].VarDecl.Type = varDeclType
					continue
				}
				if node.VarDecl.Assignment.NodeType == nodeCall {
					identifier := node.VarDecl.Name.Symbol
					procSymbol := node.VarDecl.Assignment.Call.Name.Symbol
					procDef := program.AllocatedProcs[procSymbol].VarDecl.Assignment
					if len(procDef.Proc.Outputs) == 1 {
						varDeclType := Token{
							TokenType: tokIdentifier,
							Symbol:    procDef.Proc.Outputs[0].VarDecl.Type.Symbol,
						}
						node.VarDecl.Type = varDeclType
						program.AllocatedVars[identifier].VarDecl.Type = varDeclType
					} else {
						varDeclType := Token{
							TokenType: tokIdentifier,
							Symbol:    "Void",
						}
						node.VarDecl.Type = varDeclType
						program.AllocatedVars[identifier].VarDecl.Type = varDeclType
					}
					continue
				}
				if node.VarDecl.Assignment.NodeType == nodeProcDef {
					// TO-DO: Skipping type checking for procs for now
					continue
				}
			}
		}
		fmt.Printf("Unexpected node: %s\n", node.toString())
	}
}
