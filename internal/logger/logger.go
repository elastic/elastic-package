// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package logger

import (
	"fmt"
	"log/slog"
	"os"
)

const (
	defaultTimeFormat = "2006/01/02 15:04:05"

	fixAttributeKey   = "logger"
	fixAttributeValue = "FIXME"

	LevelTrace = slog.Level(-8)

	minimumVerbosityCountAddSource = 3

	JSONFormatLabel    = "json"
	TextFormatLabel    = "text"
	DefaultFormatLabel = "default"
)

type LogFormat int

const (
	DefaultFormat LogFormat = iota
	JSONFormat
	TextFormat
)

var (
	Logger *slog.Logger

	DefaultLogger *slog.Logger

	isDebugMode bool

	LevelNames = map[slog.Leveler]string{
		LevelTrace: "TRACE",
	}

	LogFormats = map[string]LogFormat{
		JSONFormatLabel:    JSONFormat,
		TextFormatLabel:    TextFormat,
		DefaultFormatLabel: DefaultFormat,
	}
)

type LoggerOptions struct {
	Verbosity int
	LogFormat string
}

func init() {
	// Avoid being nil points these instances, so they can be used in testing
	DefaultLogger = slog.New(newHandler(os.Stdout, createHandlerOptions(new(slog.LevelVar), false, DefaultFormat)))
	Logger = slog.New(newHandler(os.Stdout, createHandlerOptions(new(slog.LevelVar), false, DefaultFormat)))
}

func SetupLogger(opts LoggerOptions) error {
	addSource := false
	if opts.Verbosity >= minimumVerbosityCountAddSource {
		addSource = true
	}

	if opts.LogFormat == "" {
		opts.LogFormat = DefaultFormatLabel
	}

	format, ok := LogFormats[opts.LogFormat]
	if !ok {
		return fmt.Errorf("unrecognized log format %q", opts.LogFormat)
	}

	logLevel := new(slog.LevelVar)
	switch {
	case opts.Verbosity == 1:
		logLevel.Set(slog.LevelDebug)
	case opts.Verbosity > 1:
		logLevel.Set(LevelTrace)
	}

	var aLogger *slog.Logger
	switch format {
	case JSONFormat:
		aLogger = slog.New(slog.NewJSONHandler(os.Stdout, createHandlerOptions(logLevel, addSource, JSONFormat)))
	case TextFormat:
		aLogger = slog.New(slog.NewTextHandler(os.Stdout, createHandlerOptions(logLevel, addSource, TextFormat)))
	case DefaultFormat:
		aLogger = slog.New(newHandler(os.Stdout, createHandlerOptions(logLevel, addSource, DefaultFormat)))
	default:
		return fmt.Errorf("invalid log format")
	}
	Logger = aLogger

	// Update default logger to start using everywhere slog
	DefaultLogger = slog.New(newHandler(os.Stdout, createHandlerOptions(logLevel, addSource, DefaultFormat)))
	slog.SetDefault(DefaultLogger)

	if opts.Verbosity > 0 {
		Logger.Debug("Enable verbose logging")
		// Required to keep it in case it is used previous logger calls
		isDebugMode = true
	}

	return nil
}

func createHandlerOptions(logLevel *slog.LevelVar, addSource bool, logFormat LogFormat) *slog.HandlerOptions {
	options := slog.HandlerOptions{
		Level:     logLevel,
		AddSource: addSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if logFormat != JSONFormat && a.Key == slog.TimeKey {
				t := a.Value.Time()
				a.Value = slog.StringValue(t.Format(defaultTimeFormat))
				return a
			}
			if logFormat != JSONFormat && a.Key == slog.TimeKey {
				t := a.Value.Time()
				a.Value = slog.StringValue(t.Format(defaultTimeFormat))
				return a
			}

			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				levelLabel, exists := LevelNames[level]
				if !exists {
					levelLabel = level.String()
				}

				a.Value = slog.StringValue(levelLabel)
				return a
			}
			return a
		},
	}
	return &options
}

// IsDebugMode method checks if the debug mode is enabled.
func IsDebugMode() bool {
	return isDebugMode
}

// Debug method logs message with "debug" level.
func Debug(a ...interface{}) {
	DefaultLogger.Debug(fmt.Sprint(a...), slog.String(fixAttributeKey, fixAttributeValue))
}

// Debugf method logs message with "debug" level and formats it.
func Debugf(format string, a ...interface{}) {
	DefaultLogger.Debug(fmt.Sprintf(format, a...), slog.String(fixAttributeKey, fixAttributeValue))
}

// Info method logs message with "info" level.
func Info(a ...interface{}) {
	DefaultLogger.Info(fmt.Sprint(a...), slog.String(fixAttributeKey, fixAttributeValue))
}

// Infof method logs message with "info" level and formats it.
func Infof(format string, a ...interface{}) {
	DefaultLogger.Info(fmt.Sprintf(format, a...), slog.String(fixAttributeKey, fixAttributeValue))
}

// Warn method logs message with "warn" level.
func Warn(a ...interface{}) {
	DefaultLogger.Warn(fmt.Sprint(a...), slog.String(fixAttributeKey, fixAttributeValue))
}

// Warnf method logs message with "warn" level and formats it.
func Warnf(format string, a ...interface{}) {
	DefaultLogger.Warn(fmt.Sprintf(format, a...), slog.String(fixAttributeKey, fixAttributeValue))
}

// Error method logs message with "error" level.
func Error(a ...interface{}) {
	DefaultLogger.Error(fmt.Sprint(a...), slog.String(fixAttributeKey, fixAttributeValue))
}

// Errorf method logs message with "error" level and formats it.
func Errorf(format string, a ...interface{}) {
	DefaultLogger.Error(fmt.Sprintf(format, a...), slog.String(fixAttributeKey, fixAttributeValue))
}
