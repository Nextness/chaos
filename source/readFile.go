package main

import (
	"bytes"
	"os"
)

type ChaosFileData struct {
	runeBuffer   []rune
	tokenBuffer  bytes.Buffer
	currentIndex int
	bufferSize   int
	lineCount    int
}

func ReadChaosFile(filePath string) (ChaosFileData, error) {
	buffer, err := os.ReadFile(filePath)
	if err != nil {
		return ChaosFileData{}, err
	}
	newBuffer := bytes.NewBuffer(buffer)
	data := ChaosFileData{
		runeBuffer:   bytes.Runes(newBuffer.Bytes()),
		currentIndex: 0,
		bufferSize:   newBuffer.Len(),
		lineCount:    0,
	}
	return data, nil
}
