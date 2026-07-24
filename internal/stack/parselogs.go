// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
)

// ParseLogsOptions configures log parsing.
type ParseLogsOptions struct {
	LogsFilePath string
	StartTime    time.Time
}

// LogLine is a parsed Elastic Agent / stack log entry.
type LogLine struct {
	LogLevel    string    `json:"log.level"`
	Timestamp   time.Time `json:"@timestamp"`
	Logger      string    `json:"log.logger"`
	Message     string    `json:"message"`
	ErrorReason string    `json:"error.reason"`
	ErrorType   string    `json:"error.type"`
}

// LogLineWithType is an alternate Elasticsearch-style log format.
type LogLineWithType struct {
	LogLevel  string    `json:"level"`
	Timestamp time.Time `json:"timestamp"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
}

// FormatError returns a human-readable description of the log line,
// including error.reason and error.type when present.
func (l LogLine) FormatError() string {
	if l.ErrorReason == "" {
		return l.Message
	}
	if l.ErrorType != "" {
		return fmt.Sprintf("%s: %s (%s)", l.Message, l.ErrorReason, l.ErrorType)
	}
	return fmt.Sprintf("%s: %s", l.Message, l.ErrorReason)
}

// ParseLogs returns all the logs for a given docker-compose service log file.
func ParseLogs(options ParseLogsOptions, process func(log LogLine) error) error {
	file, err := os.Open(options.LogsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return ParseLogsFromReader(file, options, process)
}

// ParseLogsFromReader parses docker-compose formatted logs ("service | {json}").
func ParseLogsFromReader(reader io.Reader, options ParseLogsOptions, process func(log LogLine) error) error {
	startProcessing := false

	scanner := bufio.NewScanner(reader)
	// Agent indexing-failure lines can be large; raise the default 64KiB token limit.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		_, messageLog, valid := strings.Cut(line, "|")
		if !valid {
			logger.Debugf("skipped malformed docker-compose log line: %s", line)
			continue
		}

		log := unmarshalLogLine(strings.TrimSpace(messageLog))

		// There could be valid messages with just plain text without timestamp
		// and therefore not processed, cannot be ensured in which timestamp they
		// were generated
		if !startProcessing && log.Timestamp.UTC().Before(options.StartTime.UTC()) {
			continue
		}
		startProcessing = true

		if err := process(log); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// ParseNDJSONLogs parses a raw Elastic Agent NDJSON log file (no docker-compose prefix).
func ParseNDJSONLogs(options ParseLogsOptions, process func(log LogLine) error) error {
	file, err := os.Open(options.LogsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return ParseNDJSONLogsFromReader(file, options, process)
}

// ParseNDJSONLogsFromReader parses raw NDJSON agent log lines.
func ParseNDJSONLogsFromReader(reader io.Reader, options ParseLogsOptions, process func(log LogLine) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		log := unmarshalLogLine(line)
		if !log.Timestamp.IsZero() && log.Timestamp.UTC().Before(options.StartTime.UTC()) {
			continue
		}

		if err := process(log); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// ParseNDJSONLogsDir walks dir for *.ndjson / *.json files and parses each.
func ParseNDJSONLogsDir(dir string, startTime time.Time, process func(log LogLine) error) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".ndjson") && !strings.HasSuffix(name, ".json") {
			return nil
		}
		if err := ParseNDJSONLogs(ParseLogsOptions{
			LogsFilePath: path,
			StartTime:    startTime,
		}, process); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return nil
	})
}

func unmarshalLogLine(raw string) LogLine {
	var log LogLine
	err := json.Unmarshal([]byte(raw), &log)
	if err != nil {
		log.Message = strings.TrimSpace(raw)
		return log
	}
	if log.Timestamp.IsZero() {
		// Try the alternate Elasticsearch server log format.
		var logWithType LogLineWithType
		if err := json.Unmarshal([]byte(raw), &logWithType); err == nil {
			log.Message = logWithType.Message
			log.LogLevel = logWithType.LogLevel
			log.Logger = logWithType.Component
			log.Timestamp = logWithType.Timestamp
		}
	}
	return log
}
