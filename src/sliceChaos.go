package main

import "fmt"

type ChaosSlice[T any] struct {
	data   []T
	count  int
	cursor int
}

func ChaosSliceCheckBounds[T any](chaosSlice *ChaosSlice[T], offset int) {
	assert[any](
		chaosSlice.cursor+offset < chaosSlice.count,
		fmt.Sprintf("Expected the cursor number '%d' to be lower than found count '%d'", chaosSlice.cursor, chaosSlice.count),
	)
}

func ChaosSliceGetOffset[T any](chaosSlice *ChaosSlice[T], offset int) T {
	ChaosSliceCheckBounds(chaosSlice, offset)
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
	}
	return result
}

func ChaosSliceMatch[T comparable](chaosSlice *ChaosSlice[T], value []T) bool {
	length := len(value)
	result := true
	for i := range length {
		result = result && (ChaosSliceGetOffset(chaosSlice, i) == value[i])
	}
	return result
}
