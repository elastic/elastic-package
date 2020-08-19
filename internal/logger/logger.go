package logger

import (
	"fmt"
	"log"
)

var isDebugMode bool

// EnableDebugMode method enables verbose logging.
func EnableDebugMode() {
	isDebugMode = true

	Debug("Enable verbose logging")
}

// Debug method logs message with "debug" level.
func Debug(a ...interface{}) {
	if !isDebugMode {
		return
	}
	logMessage("DEBUG", a...)
}

// Debugf method logs message with "debug" level and formats it.
func Debugf(format string, a ...interface{}) {
	if !isDebugMode {
		return
	}
	logMessagef("DEBUG", format, a...)
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
	log.Print(fmt.Sprintf("%5s "+format, all...))
}
