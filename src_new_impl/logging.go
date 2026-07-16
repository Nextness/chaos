package main

import (
	"errors"
	"flag"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

const (
	logFormatText = "text"
	logFormatJSON = "json"
)

// logConfig contains the application-wide slog handler configuration.
type logConfig struct {
	Level     slog.Level
	Format    string
	AddSource bool
}

type logFlags struct {
	level     string
	format    string
	addSource bool
}

// registerLogFlags defines all logging-related command-line options in one
// place so output behavior is configured consistently throughout the driver.
func registerLogFlags(flags *flag.FlagSet) *logFlags {
	options := &logFlags{}
	flags.StringVar(&options.level, "log-level", slog.LevelInfo.String(), "minimum log level: debug, info, warn, or error")
	flags.StringVar(&options.format, "log-format", logFormatText, "log output format: text or json")
	flags.BoolVar(&options.addSource, "log-source", false, "include the Go source location in log records")
	return options
}

func (f logFlags) config() (logConfig, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(f.level)); err != nil {
		return logConfig{}, err
	}

	format := strings.ToLower(f.format)
	if format != logFormatText && format != logFormatJSON {
		return logConfig{}, errors.New("unsupported log format " + strconv.Quote(f.format))
	}

	return logConfig{
		Level:     level,
		Format:    format,
		AddSource: f.addSource,
	}, nil
}

func newLogger(output io.Writer, config logConfig) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{
		AddSource: config.AddSource,
		Level:     config.Level,
	}

	if config.Format == logFormatJSON {
		return slog.New(slog.NewJSONHandler(output, handlerOptions))
	}
	return slog.New(slog.NewTextHandler(output, handlerOptions))
}
