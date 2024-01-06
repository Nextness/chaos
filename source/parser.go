package main

import (
	"fmt"
)

type NodeExpression struct {
	token ChaosToken
}

type NodeExit struct {
	nodeExpression NodeExpression
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

func parseExpression(parse ChaosParse) NodeExpression {
	if parse.Peek(0).Type == intLiteral {
		return NodeExpression{
			token: parse.ConsumeToken(),
		}
	}
	return NodeExpression{}
}

func Parser(chaosParse ChaosParse) NodeExit {
	var nodeExit NodeExit
	for chaosParse.bufferSize > chaosParse.currentIndex {
		if chaosParse.Peek(0).Type == exit {
			chaosParse.ConsumeToken()
			nodeExpression := parseExpression(chaosParse)
			if nodeExpression.token.Type == intLiteral {
				nodeExit = NodeExit{nodeExpression: nodeExpression}
				chaosParse.ConsumeToken()
			} else {
				return NodeExit{}
			}
			if chaosParse.Peek(0).Type != semiColon {
				fmt.Printf(
					"ERROR: %v:%04v:%03v - Invalid token while parsing. Expected ';' but got '%v' (%v)\n",
					chaosParse.Peek(0).FilePath, chaosParse.Peek(0).Line, chaosParse.Peek(0).Column,
					chaosParse.Peek(0).Value, TokenTypeMap[chaosParse.Peek(0).Type])
			}
			chaosParse.ConsumeToken()
			continue
		} else {
			fmt.Printf(
				"ERROR: %v:%04v:%03v - Invalid token while parsing '%v' (%v)\n",
				chaosParse.Peek(0).FilePath, chaosParse.Peek(0).Line, chaosParse.Peek(0).Column,
				chaosParse.Peek(0).Value, TokenTypeMap[chaosParse.Peek(0).Type])
			chaosParse.ConsumeToken()
			continue
		}
	}
	return nodeExit
}
