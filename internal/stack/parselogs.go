// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
)

type ParseLogsOptions struct {
	LogsFilePath string
	StartTime    time.Time
}

type LogLine struct {
	LogLevel  string    `json:"log.lovel"`
	Timestamp time.Time `json:"@timestamp"`
	Message   string    `json:"message"`
}

// ParseLogs returns all the logs for a given service name
func ParseLogs(options ParseLogsOptions, process func(log LogLine) error) error {
	file, err := os.Open(options.LogsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	startProcessing := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		_, messageLog, valid := strings.Cut(line, "|")
		if !valid {
			logger.Debugf("skipped malformed docker-compose log line: %s", line)
			continue
		}

		var log LogLine
		err := json.Unmarshal([]byte(messageLog), &log)
		if err != nil {
			// there are logs that are just plain text in these logs
			log.Message = strings.TrimSpace(messageLog)
		}

		// There could be valid messages with just plain text without timestamp
		// and therefore not processed, cannot be ensured in which timestamp they
		// were generated
		if !startProcessing && log.Timestamp.Before(options.StartTime) {
			continue
		}
		startProcessing = true

		err = process(log)
		if err != nil {
			return err
		}
	}

	return nil
}
