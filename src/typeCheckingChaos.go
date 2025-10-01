package main

import "fmt"

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

func TypeCheckChaosProgram(program *ChaosSlice[Node]) {
	assert[any](program.cursor == 0, "Cursor is not 0")
	for program.cursor < program.count {
		if CSMatch(program, CSNodeNodeTypeComparison, nodeIdentifier) {
			node := CSConsumeAssert(program, CSNodeTypeAssert, nodeIdentifier)

			if node.VarDecl.Assignment.NodeType == nodeIdentifier {
				identifier := node.VarDecl.Name.Symbol

				n1 := node.Reassignable
				n2 := node.VarDecl.Assignment.Reassignable
				if !n1 && n2 {
					assert[any](false, "Invalid operation - cannot assign runtime to a compile time value")
				}

				lhsVarType := node.VarDecl.Type
				rhsVarType := node.VarDecl.Assignment.VarDecl.Type
				if lhsVarType.TokenType == tokInfer {
					node.VarDecl.Type = Token{
						TokenType: tokIdentifier,
						Symbol:    rhsVarType.Symbol,
						Infered:   true,
					}
					continue
				}

				if !TypeCheckLiterals(lhsVarType, rhsVarType) {
					assert[any](
						false,
						fmt.Sprintf("The identifier %s expecetd a type %s but got %s", identifier, lhsVarType.Symbol, rhsVarType.Symbol),
					)
				}
			}

			if node.VarDecl.Assignment.NodeType == nodeIntLiteral {
				value := node.VarDecl.Assignment.Literal.Int.Value
				varType := node.VarDecl.Type
				if varType.TokenType == tokInfer {
					node.VarDecl.Type = Token{
						TokenType: tokIdentifier,
						Symbol:    "S64",
						Infered:   true,
					}
					continue
				}
				if val, ok := cast[int](value); ok {
					allowedTypes := MakeTypeList(
						"S128", "S64", "S32", "S16", "S8", "U128", "U64", "U32", "U16", "U8",
					)
					if !TypeCheckLiterals(varType, allowedTypes...) {
						assert[any](
							false,
							fmt.Sprintf("The value %d expecetd a type for signed or unsigned integers but got %s", val, varType.Symbol),
						)
					}
				}
			}

			if node.VarDecl.Assignment.NodeType == nodeStringLiteral {
				value := node.VarDecl.Assignment.Literal.String.Value
				varType := node.VarDecl.Type
				if varType.TokenType == tokInfer {
					node.VarDecl.Type = Token{
						TokenType: tokIdentifier,
						Symbol:    "String",
						Infered:   true,
					}
					continue
				}
				if val, ok := cast[string](value); ok {
					allowedTypes := MakeTypeList("String")
					if !TypeCheckLiterals(varType, allowedTypes...) {
						assert[any](
							false,
							fmt.Sprintf("The value «%s» must be %s", val, varType.Symbol),
						)
					}
				}
			}

			if node.VarDecl.Assignment.NodeType == nodeBoolLiteral {
				value := node.VarDecl.Assignment.Literal.Boolean.Value
				varType := node.VarDecl.Type
				if varType.TokenType == tokInfer {
					node.VarDecl.Type = Token{
						TokenType: tokIdentifier,
						Symbol:    "Bool",
						Infered:   true,
					}
					continue
				}
				if val, ok := cast[bool](value); ok {
					allowedTypes := MakeTypeList("Bool")
					if !TypeCheckLiterals(varType, allowedTypes...) {
						assert[any](
							false,
							fmt.Sprintf("The value %t must be %s", val, varType.Symbol),
						)
					}
				}
			}

			continue
		}

		assert[any](false, fmt.Sprintf("Unrecheable - unexpected node %s", NodeTypeToString(CSGet(program).NodeType)))
	}
}
