// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/profile"
)

type ParseLogsOptions struct {
	ServiceName string
	LogsPath    string
	Profile     *profile.Profile
}

type DockerComposeLogs []LogLine
type LogLine struct {
	LogLevel  string    `json:"log.lovel"`
	Timestamp time.Time `json:"@timestamp"`
	Message   string    `json:"message"`
}

func ParseLogs(options ParseLogsOptions) (DockerComposeLogs, error) {
	// create dump
	outputPath, err := Dump(DumpOptions{Output: options.LogsPath, Profile: options.Profile})
	if err != nil {
		return nil, err
	}

	// check logs for a service
	serviceLogs := filepath.Join(outputPath, "logs", fmt.Sprintf("%s.log", options.ServiceName))

	file, err := os.Open(serviceLogs)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var logs DockerComposeLogs
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		messageSlice := strings.SplitN(line, "|", 2)

		if len(messageSlice) != 2 {
			return nil, fmt.Errorf("malformed docker-compose log line")
		}

		// service := messageSlice[0]
		messageLog := messageSlice[1]

		var log LogLine
		err := json.Unmarshal([]byte(messageLog), &log)
		if err != nil {
			log.Message = strings.TrimSpace(messageLog)
		}

		logs = append(logs, log)
	}

	return logs, nil
}
