/* Nextness */
package main

import "fmt"

type ChaosSlice[T any] struct {
	data   []T
	count  int
	cursor int
}

/* Operators - comparison */
func ChaosSliceDefaultComparison[T comparable](value T, compare T) bool {
	return value == compare
}

func ChaosSliceTokenTypeComparison[T comparable](value T, compare T) bool {
	token := castAssert[Token](value)
	tokenType := castAssert[TokenType](compare)
	return token.TokenType == tokenType
}

/* Private Functions */
func chaosSliceCheckBounds[T any](chaosSlice *ChaosSlice[T], offset int) {
	assert[any](
		chaosSlice.cursor+offset < chaosSlice.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", chaosSlice.cursor, chaosSlice.count),
	)
}

/* Public Functions */
func ChaosSliceSearchValue[T any, R any](value ...T) []R {
	var result []R
	for _, val := range value {
		// Fuck golang...
		if str, ok := cast[string](val); ok {
			instanceR := *new(R)
			if _, ok := cast[byte](instanceR); ok {
				for _, char := range str {
					result = append(result, castAssert[R](byte(char)))
				}
			}
		} else {
			result = append(result, castAssert[R](val))
		}
	}
	return result
}

func ChaosSliceConsume[T any](chaosSlice *ChaosSlice[T], amount ...int) T {
	length := len(amount)
	increment := 1

	if length == 1 {
		increment = amount[0]
	} else if length > 1 {
		assert[any](false, "Only 1 argument or none is allowed")
	}

	chaosSlice.cursor += increment
	return ChaosSliceGet(chaosSlice)
}

func ChaosSliceConsumeAssert[T comparable, R comparable](chaosSlice *ChaosSlice[T], expected R, operator func(any, any)) T {
	element := ChaosSliceConsume(chaosSlice)
	operator(element, expected)
	return element
}

func ChaosSliceGetOffset[T any](chaosSlice *ChaosSlice[T], offset int) T {
	chaosSliceCheckBounds(chaosSlice, offset)
	var result T = chaosSlice.data[chaosSlice.cursor+offset]
	return result
}

func ChaosSliceGet[T any](chaosSlice *ChaosSlice[T]) T {
	return ChaosSliceGetOffset(chaosSlice, 0)
}

func ChaosSliceMatchOffset[T comparable](chaosSlice *ChaosSlice[T], offset int, value []T) bool {
	length := len(value)
	result := true
	for i := range length {
		result = result && (ChaosSliceGetOffset(chaosSlice, offset+i) == value[i])
		if !result {
			break
		}
	}
	return result
}

func ChaosSliceMatch[T comparable](chaosSlice *ChaosSlice[T], value []T) bool {
	length := len(value)
	result := true
	for i := range length {
		result = result && (ChaosSliceGetOffset(chaosSlice, i) == value[i])
		if !result {
			break
		}
	}
	return result
}

func ChaosSliceMatchOp[T comparable, R any](chaosSlice *ChaosSlice[T], operator func(any, any) bool, value []R) bool {
	length := len(value)
	result := true
	for i := range length {
		result = result && operator(ChaosSliceGetOffset(chaosSlice, i), value[i])
		if !result {
			break
		}
	}
	return result
}
