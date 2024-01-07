package main

import (
	"fmt"
)

type NodeExpresionIntegerLiteral struct {
	integerLiteral ChaosToken
}

type NodeExpresionIdentifier struct {
	identifier ChaosToken
}

type NodeExpression struct {
	nodeType                    TokenType
	nodeExpresionIntegerLiteral NodeExpresionIntegerLiteral
	nodeExpresionIdentifier     NodeExpresionIdentifier
}

type NodeStatementExit struct {
	nodeExpression NodeExpression
}

type NodeStatementLet struct {
	identifier     ChaosToken
	nodeExpression NodeExpression
}

type NodeStatement struct {
	nodeType          TokenType
	nodeStatementExit NodeStatementExit
	nodeStatementLet  NodeStatementLet
}

type NodeProgram struct {
	nodeStatement []NodeStatement
}

type ChaosParse struct {
	tokens       []ChaosToken
	bufferSize   int
	currentIndex int
}

func (cp *ChaosParse) ConsumeToken() ChaosToken {
	if cp.currentIndex < cp.bufferSize {
		currentToken := cp.tokens[cp.currentIndex]
		(*cp).currentIndex++
		return currentToken
	}
	return ChaosToken{}
}

func (cp *ChaosParse) Peek(position int) ChaosToken {
	if cp.currentIndex+position < cp.bufferSize {
		return cp.tokens[cp.currentIndex+position]
	}
	return ChaosToken{}
}

func GenerateChaosParse(tokens []ChaosToken) ChaosParse {
	return ChaosParse{
		tokens:       tokens,
		bufferSize:   len(tokens),
		currentIndex: 0,
	}
}

func parseExpression(parse *ChaosParse) NodeExpression {
	if parse.Peek(0).Type == intLiteral {
		token := parse.ConsumeToken()
		return NodeExpression{
			nodeType: token.Type,
			nodeExpresionIntegerLiteral: NodeExpresionIntegerLiteral{
				integerLiteral: token,
			},
		}
	} else if parse.Peek(0).Type == identifier {
		token := parse.ConsumeToken()
		return NodeExpression{
			nodeType: token.Type,
			nodeExpresionIdentifier: NodeExpresionIdentifier{
				identifier: token,
			},
		}
	}
	return NodeExpression{}
}

func parseStatement(parse *ChaosParse) NodeStatement {
	if parse.Peek(0).Type == exit {
		parse.ConsumeToken()
		nodeExpression := parseExpression(parse)
		var nodeStatementExit NodeStatementExit
		if nodeExpression.nodeType == intLiteral {
			nodeStatementExit = NodeStatementExit{
				nodeExpression: nodeExpression,
			}
		}
		if parse.Peek(0).Type != semiColon {
			fmt.Printf(
				"ERROR: %v:%04v:%03v - Invalid token while parsing. Expected ';' but got '%v' (%v)\n",
				parse.Peek(0).FilePath, parse.Peek(0).Line, parse.Peek(0).Column,
				parse.Peek(0).Value, TokenTypeMap[parse.Peek(0).Type])
		}
		parse.ConsumeToken()
		return NodeStatement{
			nodeType:          exit,
			nodeStatementExit: nodeStatementExit,
		}
	} else if parse.Peek(0).Type == let &&
		parse.Peek(1).Type == identifier &&
		parse.Peek(2).Type == equals {
		nodeStatementLet := NodeStatementLet{
			identifier: parse.ConsumeToken(),
		}
		expression := parseExpression(parse)
		if expression.nodeType == identifier {
			nodeStatementLet.nodeExpression.nodeExpresionIdentifier = expression.nodeExpresionIdentifier
			parse.ConsumeToken()
		} else {
			fmt.Printf("Invalid expression: %v\n", expression.nodeExpresionIdentifier.identifier.Value)
		}
		if parse.Peek(1).Type == semiColon {
			parse.ConsumeToken()
			parse.ConsumeToken()
		} else {
			fmt.Printf("Expected ';'\n")
		}
		return NodeStatement{
			nodeType:         let,
			nodeStatementLet: nodeStatementLet,
		}
	}
	return NodeStatement{}
}

func ParseProgram(chaosParse ChaosParse) NodeProgram {
	var nodeProgram NodeProgram
	for chaosParse.currentIndex < chaosParse.bufferSize {
		statement := parseStatement(&chaosParse)
		if statement.nodeType != 0 {
			nodeProgram.nodeStatement = append(nodeProgram.nodeStatement, statement)
		} else {
			fmt.Printf("Invalidi statement aosijdasoijd\n")
		}
	}
	return nodeProgram
}
