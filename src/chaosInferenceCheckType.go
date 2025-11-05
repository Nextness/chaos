package main

import "fmt"

var warnOnce = true

func MakeType(typeSymbol string) Token {
	return Token{
		TokenType: tokIdentifier,
		Symbol:    typeSymbol,
	}
}

func MakeTypeList(typeSymbols ...string) []Token {
	tokenList := []Token{}
	for _, str := range typeSymbols {
		tokenList = append(tokenList, Token{
			TokenType: tokIdentifier,
			Symbol:    str,
		})
	}
	return tokenList
}

func TypeCheckLiterals(varType Token, allowedTypes ...Token) bool {
	result := false
	for _, allowedType := range allowedTypes {
		if allowedType.Symbol == varType.Symbol {
			result = true
			break
		}
	}
	return result
}

func InferAndTypeCheckIdentifier(program *ChaosSlice[Node], scope Scope) {
	node := CSGetPointer(program)
	assert[any](
		node.NodeType == nodeIdentifier,
		fmt.Sprintf("We were expecting a NodeIdentifier but we got %s", NodeTypeToString(node.NodeType)),
	)

	if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
		infered := false
		if node.VarDecl.Type.TokenType == tokInfer {
			infered = true
			foundInScope := false
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					s64 := MakeType("S64")
					node.VarDecl.Type = s64
					scope[idx].VarDecl.Type = s64
					foundInScope = true
					break
				}
			}
			assert[any](foundInScope, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		if !infered {
			varType := node.VarDecl.Type
			allowedTypes := MakeTypeList(
				"S128", "S64", "S32", "S16", "S8", "U128", "U64", "U32", "U16", "U8",
			)
			if !TypeCheckLiterals(varType, allowedTypes...) {
				assert[any](
					false,
					fmt.Sprintf("The identifier '%s' expected a Signed or Unsigned integers but got %s", node.VarDecl.Name.Symbol, varType.Symbol),
				)
			}
		}

		return
	}

	if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
		infered := false
		if node.VarDecl.Type.TokenType == tokInfer {
			infered = true
			foundInScope := false
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					string := MakeType("String")
					node.VarDecl.Type = string
					scope[idx].VarDecl.Type = string
					foundInScope = true
					break
				}
			}
			assert[any](foundInScope, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		if !infered {
			varType := node.VarDecl.Type
			allowedTypes := MakeTypeList("String")
			if !TypeCheckLiterals(varType, allowedTypes...) {
				assert[any](
					false,
					fmt.Sprintf("The identifier '%s' expected a String but got %s", node.VarDecl.Name.Symbol, varType.Symbol),
				)
			}
		}

		return
	}

	if node.VarDecl.Assignment.NodeType == nodeBoolLiteral {
		infered := false
		if node.VarDecl.Type.TokenType == tokInfer {
			infered = true
			foundInScope := false
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					boolean := MakeType("Bool")
					node.VarDecl.Type = boolean
					scope[idx].VarDecl.Type = boolean
					foundInScope = true
					break
				}
			}
			assert[any](foundInScope, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		if !infered {
			varType := node.VarDecl.Type
			allowedTypes := MakeTypeList("Bool")
			if !TypeCheckLiterals(varType, allowedTypes...) {
				assert[any](
					false,
					fmt.Sprintf("The identifier '%s' expected a Bool but got %s", node.VarDecl.Name.Symbol, varType.Symbol),
				)
			}
		}

		return
	}

	if node.VarDecl.Assignment.NodeType == nodeIdentifier {
		infered := false
		if node.VarDecl.Type.TokenType == tokInfer {
			infered = true
			foundInScope := false
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Assignment.VarDecl.Name.Symbol {
					node.VarDecl.Type = scope[idx].VarDecl.Type
					foundInScope = true
					continue
				}
				if n.VarDecl.Name.Symbol == node.VarDecl.Name.Symbol {
					scope[idx].VarDecl.Type = node.VarDecl.Type
					break
				}
			}
			assert[any](foundInScope, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		if !infered {
			varType := node.VarDecl.Type
			for _, n := range scope {
				if node.VarDecl.Assignment.VarDecl.Name.Symbol == n.VarDecl.Name.Symbol {
					identType := n.VarDecl.Type
					if !TypeCheckLiterals(varType, identType) {
						assert[any](
							false,
							fmt.Sprintf("The identifier '%s' expected a %s but got %s", node.VarDecl.Name.Symbol, varType.Symbol, identType.Symbol),
						)
					}
					break
				}
			}
		}

		return
	}

	if node.VarDecl.Assignment.NodeType == nodeCall {
		infered := false
		if node.VarDecl.Type.TokenType == tokInfer {
			infered = true
			foundInScope := false
			for idx, n := range scope {
				if n.VarDecl.Name.Symbol == node.VarDecl.Assignment.Call.Name.Symbol {
					retLen := len(n.VarDecl.Assignment.Proc.Outputs)
					assert[any](retLen <= 1, "Cannot have multiple returns for now")
					node.VarDecl.Type = MakeType("Void")
					if retLen == 1 {
						node.VarDecl.Type = n.VarDecl.Assignment.Proc.Outputs[0].VarDecl.Type
					}
					foundInScope = true
					continue
				}
				if n.NodeType == nodeIdentifier && node.VarDecl.Name.Symbol == n.VarDecl.Name.Symbol {
					scope[idx].VarDecl.Type = node.VarDecl.Type
					break
				}
			}
			assert[any](foundInScope, "Cannot infer variable declaration since it doesn't exist in scope")
		}

		if !infered {
			varType := node.VarDecl.Type
			for _, n := range scope {
				if node.VarDecl.Assignment.Call.Name.Symbol == n.VarDecl.Name.Symbol {
					retLen := len(n.VarDecl.Assignment.Proc.Outputs)
					assert[any](retLen <= 1, "Cannot have multiple returns for now")
					identType := n.VarDecl.Assignment.Proc.Outputs[0].VarDecl.Type
					if !TypeCheckLiterals(varType, identType) {
						assert[any](
							false,
							fmt.Sprintf("The identifier '%s' expected a %s but got %s", node.VarDecl.Name.Symbol, varType.Symbol, identType.Symbol),
						)
					}
					break
				}
			}
		}
		return
	}

	// TO-DO: Actually handle type inference for functions.
	// For now I don't really know what I want skipping it.
	if node.NodeType == nodeIdentifier && node.VarDecl.Assignment.NodeType == nodeProcDef {
		if warnOnce {
			fmt.Printf("[WARNING] Skipping type inference for proc definition\n")
			warnOnce = false
		}
		return
	}

	assert[any](false, fmt.Sprintf("Unexpected node - %s", NodeTypeToString(node.NodeType)))
}

func TypeCheckExit(program *ChaosSlice[Node], scope Scope) {
	node := CSGet(program)

	// TO-DO: Later we should not allow all the types in here.
	// Probably just U8 and S8. I'm not sure about this, since we can just
	// mod by 256 - I should think about it later.
	allowedTypes := MakeTypeList(
		"S128", "S64", "S32", "S16", "S8", "U128", "U64", "U32", "U16", "U8",
	)

	condition1 := node.Exit.Status.NodeType == nodeIntLiteral
	condition2 := node.Exit.Status.NodeType == nodeIdentifier
	condition3 := false
	if condition2 {
		for _, n := range scope {
			if node.Exit.Status.VarDecl.Name.Symbol == n.VarDecl.Name.Symbol {
				condition3 = (TypeCheckLiterals(n.VarDecl.Type, allowedTypes...))
				break
			}
		}
	}

	// TO-DO: improve this fucking shit, because
	// the error message sucks and it doesn't tell anything
	// to the user when there is an actual error...
	if !condition1 && (!condition2 || !condition3) {
		assert[any](
			false,
			fmt.Sprintf("Exit expects Integer, but got %s", NodeTypeToString(node.Exit.Status.NodeType)),
		)
	}

	if node.Exit.Message.NodeType != nodeNull {
		allowedTypes := MakeTypeList("String")

		condition1 := node.Exit.Message.NodeType == nodeStringLiteral
		condition2 := node.Exit.Message.NodeType == nodeIdentifier
		condition3 := false
		if condition2 {
			for _, n := range scope {
				if node.Exit.Message.VarDecl.Name.Symbol == n.VarDecl.Name.Symbol {
					condition3 = (TypeCheckLiterals(n.VarDecl.Type, allowedTypes...))
					break
				}
			}
		}

		// TO-DO: improve this fucking shit, because
		// the error message sucks and it doesn't tell anything
		// to the user when there is an actual error...
		if !condition1 && (!condition2 || !condition3) {
			assert[any](
				false,
				fmt.Sprintf("Exit expects String, but got %s", NodeTypeToString(node.Exit.Message.NodeType)),
			)
		}
	}

	return
}

func ChaosInferAndCheckType(program *ChaosSlice[Node], scope Scope) {
	assert[any](program.cursor == 0, "Cursor is not 0")

	for program.cursor < program.count {
		if CSMatch(program, CSNodeNodeTypeComparison, nodeIdentifier) {
			InferAndTypeCheckIdentifier(program, scope)
			CSConsume(program)
			continue
		}

		if CSMatch(program, CSNodeNodeTypeComparison, nodeExit) {
			TypeCheckExit(program, scope)
			CSConsume(program)
			continue
		}

		node := CSGet(program)
		assert[any](false, fmt.Sprintf("Unrecheable - unexpected node %s", NodeTypeToString(node.NodeType)))
	}
}
