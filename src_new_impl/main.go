package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(run(filepath.Base(os.Args[0]), os.Args[1:], os.Stderr))
}

func run(programName string, args []string, logOutput io.Writer) int {
	flags := flag.NewFlagSet(programName, flag.ContinueOnError)
	flags.SetOutput(logOutput)
	logging := registerLogFlags(flags)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	config, err := logging.config()
	if err != nil {
		// This logger is created just for the case where the actual logger is configured incorrectly
		inline_logger := newLogger(logOutput, logConfig{Level: slog.LevelInfo, Format: logFormatText})
		inline_logger.Error("invalid logging configuration", slog.Any("error", err))
		return 2
	}
	logger := newLogger(logOutput, config)

	if flags.NArg() != 1 {
		logger.Error(
			"exactly one Chaos source file is required",
			slog.String("usage", programName+" [logging flags] <file.chaos>"),
			slog.Int("argument_count", flags.NArg()),
		)
		return 2
	}

	path := flags.Arg(0)
	source, err := os.ReadFile(path)
	if err != nil {
		logger.Error(
			"failed to read Chaos source",
			slog.String("path", path),
			slog.Any("error", err),
		)
		return 1
	}

	sm := &SourceManager{}
	fileID := sm.Register(path, source)
	sf := sm.Lookup(fileID)

	tokens, diags := Tokenize(source, fileID)

	if len(diags) > 0 {
		RenderAll(logger, diags, source, sf.LineOffsets)
		if diags.HasErrors() {
			return 1
		}
	}

	ctx := context.Background()
	for _, tok := range tokens {
		line, col := offsetToLineCol(tok.Span.Start, sf.LineOffsets)
		attributes := []slog.Attr{
			slog.Int("file", int(tok.Span.File)),
			slog.Int("line", line),
			slog.Int("column", col),
			slog.String("kind", tok.Kind.String()),
			slog.Int("span_start", tok.Span.Start),
			slog.Int("span_end", tok.Span.End),
		}
		if tok.Kind == TkEOF {
			attributes = append(attributes, slog.Bool("eof", true))
		} else {
			attributes = append(attributes, slog.String("raw", string(tok.Raw)))
		}
		if tok.Value != "" {
			attributes = append(attributes, slog.String("value", tok.Value))
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "token", attributes...)
	}

	// Parse the tokens into an AST.
	result := ParseProgram(tokens)
	if len(result.Diags) > 0 {
		RenderAll(logger, result.Diags, source, sf.LineOffsets)
		if result.Diags.HasErrors() {
			return 1
		}
	}

	return 0
}
