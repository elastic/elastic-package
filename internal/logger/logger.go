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
)

var (
	Logger   *slog.Logger
	logLevel *slog.LevelVar

	DefaultLogger   *slog.Logger
	defaultLogLevel *slog.LevelVar

	isDebugMode bool
)

func SetupLogger(debugMode bool) {
	logLevel = new(slog.LevelVar)
	options := slog.HandlerOptions{
		Level:     logLevel,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			if a.Key == slog.TimeKey {
				t := a.Value.Time()
				a.Value = slog.StringValue(t.Format(defaultTimeFormat))
			}
			return a
		},
	}
	Logger = slog.New(newHandler(os.Stdout, &options))

	// Update default logger to start using everywhere slog
	defaultLogLevel = new(slog.LevelVar)
	DefaultLogger = slog.New(newHandler(os.Stdout, &slog.HandlerOptions{
		Level:     defaultLogLevel,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key != slog.TimeKey {
				return a
			}

			t := a.Value.Time()
			a.Value = slog.StringValue(t.Format(defaultTimeFormat))
			return a
		},
	}))

	slog.SetDefault(DefaultLogger)
	if debugMode {
		defaultLogLevel.Set(slog.LevelDebug)
		logLevel.Set(slog.LevelDebug)
		Logger.Debug("Enable verbose logging")
		isDebugMode = true
	}
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
