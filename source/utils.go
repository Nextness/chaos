package main

import (
	"fmt"
	"os"
)

type DebugLevel int

const (
	DEBUG = iota
	INFO
	WARNING
	ERROR
	CRITICAL
)

func ChaosLogger(debugLevel DebugLevel, loggingMessage string, a ...any) {
	var prefix string
	switch debugLevel {
	case DEBUG:
		if compilationConfig.enableDebugLogger {
			prefix = "DEBUG"
		}
	case INFO:
		prefix = "INFO"
	case WARNING:
		prefix = "WARNING"
	case ERROR:
		prefix = "ERROR"
	case CRITICAL:
		prefix = "CRITICAL"
	default:
		fmt.Printf("LOGGER ERROR: '%v' is not a valid debug level", debugLevel)
		os.Exit(1)
	}
	msg := fmt.Sprintf("%v: %v", prefix, loggingMessage)
	fmt.Printf(msg, a...)
}
