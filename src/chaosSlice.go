// Nextness
package main

import "fmt"

type ChaosSlice[T any] struct {
	data   []T
	count  int
	cursor int
}

/* Operators - comparison */
func CSUint8Uint8Comparison(value any, compare any) bool {
	byte := castAssert[uint8](value)
	comp := castAssert[[]uint8](compare)[0]
	return byte == comp
}

func CSStringUint8Comparison(value any, compare any) bool {
	byte := castAssert[uint8](value)
	comp := castAssert[string](compare)[0]
	return byte == comp
}

func CSTokenTypeComparison(value any, compare any) bool {
	token := castAssert[Token](value)
	tokenType := castAssert[TokenType](compare)
	return token.TokenType == tokenType
}

func CSNodeNodeTypeComparison(value any, compare any) bool {
	node := castAssert[Node](value)
	nodeType := castAssert[NodeType](compare)
	return node.NodeType == nodeType
}

/* Operators - Asserts */
func CSTokenTypeAssert(a any, b any) {
	token := castAssert[Token](a)
	tokenType := castAssert[TokenType](b)
	assert[any](
		token.TokenType == tokenType,
		fmt.Sprintf(
			"Expected %s but got %s",
			TokenTypeToString(tokenType), TokenTypeToString(token.TokenType),
		),
	)
}

func CSNodeTypeAssert(a any, b any) {
	node := castAssert[Node](a)
	nodeType := castAssert[NodeType](b)
	assert[any](
		node.NodeType == nodeType,
		fmt.Sprintf(
			"Expected %s but got %s",
			NodeTypeToString(nodeType), NodeTypeToString(node.NodeType),
		),
	)
}

/* Public Functions */
func CSCheckBounds[T any](chaosSlice *ChaosSlice[T], offset int) {
	assert[any](
		chaosSlice.cursor+offset < chaosSlice.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", chaosSlice.cursor, chaosSlice.count),
	)
}

func CSGet[T any](chaosSlice *ChaosSlice[T], offset ...int) T {
	length := len(offset)
	increment := 0

	if length == 1 {
		increment = offset[0]
	} else if length > 1 {
		assert[any](false, "Only 1 argument or none is allowed")
	}

	CSCheckBounds(chaosSlice, increment)
	var result T = chaosSlice.data[chaosSlice.cursor+increment]
	return result

}

func CSMatch[T any](chaosSlice *ChaosSlice[T], operator func(any, any) bool, expected ...any) bool {
	result := true
	for offset, expect := range expected {
		element := CSGet(chaosSlice, offset)
		result = result && operator(element, expect)
		if !result {
			break
		}
	}
	return result
}

func CSConsume[T any](chaosSlice *ChaosSlice[T], amount ...int) T {
	length := len(amount)
	increment := 1

	if length == 1 {
		increment = amount[0]
	} else if length > 1 {
		assert[any](false, "Only 1 argument or none is allowed")
	}

	element := CSGet(chaosSlice)
	chaosSlice.cursor += increment
	return element
}

func CSConsumeAssert[T any](chaosSlice *ChaosSlice[T], operator func(any, any), expected ...any) T {
	var element T
	for _, exp := range expected {
		element = CSConsume(chaosSlice)
		operator(element, exp)
	}
	return element
}
