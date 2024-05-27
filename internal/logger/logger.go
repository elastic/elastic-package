// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package logger

import (
	"fmt"
	"log/slog"
	"os"
)

var (
	Logger   *slog.Logger
	logLevel *slog.LevelVar

	defaultLogger   *slog.Logger
	defaultLogLevel *slog.LevelVar

	isDebugMode bool
)

func SetupLogger() {
	logLevel = new(slog.LevelVar)
	options := slog.HandlerOptions{
		Level: logLevel,
	}
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &options))

	// Update default logger to start using everywhere slog
	defaultLogLevel = new(slog.LevelVar)
	defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: defaultLogLevel,
	}))

	slog.SetDefault(defaultLogger)
}

// EnableDebugMode method enables verbose logging.
func EnableDebugMode() {
	logLevel.Set(slog.LevelDebug)
	defaultLogLevel.Set(slog.LevelDebug)

	isDebugMode = true

	Debug("Enable verbose logging")
	Logger.Debug("Enable verbose logging")
}

// IsDebugMode method checks if the debug mode is enabled.
func IsDebugMode() bool {
	return isDebugMode
}

// Debug method logs message with "debug" level.
func Debug(a ...interface{}) {
	defaultLogger.Debug(fmt.Sprint(a...))
}

// Debugf method logs message with "debug" level and formats it.
func Debugf(format string, a ...interface{}) {
	defaultLogger.Debug(fmt.Sprintf(format, a...))
}

// Info method logs message with "info" level.
func Info(a ...interface{}) {
	defaultLogger.Info(fmt.Sprint(a...))
}

// Infof method logs message with "info" level and formats it.
func Infof(format string, a ...interface{}) {
	defaultLogger.Info(fmt.Sprintf(format, a...))
}

// Warn method logs message with "warn" level.
func Warn(a ...interface{}) {
	defaultLogger.Warn(fmt.Sprint(a...))
}

// Warnf method logs message with "warn" level and formats it.
func Warnf(format string, a ...interface{}) {
	defaultLogger.Warn(fmt.Sprintf(format, a...))
}

// Error method logs message with "error" level.
func Error(a ...interface{}) {
	defaultLogger.Error(fmt.Sprint(a...))
}

// Errorf method logs message with "error" level and formats it.
func Errorf(format string, a ...interface{}) {
	defaultLogger.Error(fmt.Sprintf(format, a...))
}
