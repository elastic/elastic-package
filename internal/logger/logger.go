// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package logger

import (
	"fmt"
	"log"
)

var isDebugMode bool
var isTraceMode bool

// EnableDebugMode method enables verbose logging.
func EnableDebugMode() {
	isDebugMode = true

	Debug("Enable verbose logging")
}

// EnableTraceMode method enables trace verbose logging.
func EnableTraceMode() {
	isDebugMode = true
	isTraceMode = true

	Debug("Enable trace verbose logging")
}

// Debug method logs message with "debug" level.
func Debug(a ...interface{}) {
	if !IsDebugMode() {
		return
	}
	logMessage("DEBUG", a...)
}

// Debugf method logs message with "debug" level and formats it.
func Debugf(format string, a ...interface{}) {
	if !IsDebugMode() {
		return
	}
	logMessagef("DEBUG", format, a...)
}

// IsDebugMode method checks if the debug mode is enabled.
func IsDebugMode() bool {
	return isDebugMode || isTraceMode
}

// Trace method logs message with "trace" level.
func Trace(a ...interface{}) {
	if !IsTraceMode() {
		return
	}
	logMessage("TRACE", a...)
}

// Tracef method logs message with "trace" level and formats it.
func Tracef(format string, a ...interface{}) {
	if !IsTraceMode() {
		return
	}
	logMessagef("TRACE", format, a...)
}

// IsTraceMode method checks if the trace mode is enabled.
func IsTraceMode() bool {
	return isTraceMode
}

// Info method logs message with "info" level.
func Info(a ...interface{}) {
	logMessage("INFO", a...)
}

// Infof method logs message with "info" level and formats it.
func Infof(format string, a ...interface{}) {
	logMessagef("INFO", format, a...)
}

// Warn method logs message with "warn" level.
func Warn(a ...interface{}) {
	logMessage("WARN", a...)
}

// Warnf method logs message with "warn" level and formats it.
func Warnf(format string, a ...interface{}) {
	logMessagef("WARN", format, a...)
}

// Error method logs message with "error" level.
func Error(a ...interface{}) {
	logMessage("ERROR", a...)
}

// Errorf method logs message with "error" level and formats it.
func Errorf(format string, a ...interface{}) {
	logMessagef("ERROR", format, a...)
}

func logMessage(level string, a ...interface{}) {
	var all []interface{}
	all = append(all, fmt.Sprintf("%5s ", level))
	all = append(all, a...)
	log.Print(all...)
}

func logMessagef(level string, format string, a ...interface{}) {
	var all []interface{}
	all = append(all, level)
	all = append(all, a...)
	log.Printf("%5s "+format, all...)
}
