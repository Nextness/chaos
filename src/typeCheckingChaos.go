package main

import "fmt"

func TypeCheckChaosProgram(program *Program) {
	for _, node := range program.Nodes {
		if node.NodeType == nodeIdentifier {
			if node.VarDecl.Type.Symbol != "" {
				name := node.VarDecl.Name.Symbol
				varDeclSymbol := node.VarDecl.Type.Symbol
				if node.VarDecl.Assignment.NodeType == nodeIdentifier {
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
			// Infer type
			if node.VarDecl.Type.Symbol == "" {
				if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
					identfier := node.VarDecl.Name.Symbol
					varDeclType := Token{
						TokenType: tokIdentifier,
						Symbol:    "S64",
						Value:     "",
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
						Value:     "",
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
						Value:     "",
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
						Value:     "",
					}
					node.VarDecl.Type = varDeclType
					program.AllocatedVars[identfier].VarDecl.Type = varDeclType
					continue
				}
			}
		}
		fmt.Printf("Unexpected node: %v\n", node)
	}
}
