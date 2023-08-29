// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/formatter"
)

const sampleEventFilePattern = "sample_event*.json"

func renderSampleEvents(packageRoot, dataStreamName string) (string, error) {
	eventPaths, err := filepath.Glob(filepath.Join(packageRoot, "data_stream", dataStreamName, sampleEventFilePattern))
	if err != nil {
		return "", fmt.Errorf("failed to look for sample event files: %w", err)
	}
	if len(eventPaths) == 0 {
		return "", fmt.Errorf("could not find any sample event for data stream %s", dataStreamName)
	}

	var builder strings.Builder
	if len(eventPaths) == 1 {
		fmt.Fprintf(&builder, "An example event for `%s` looks as following:\n\n",
			stripDataStreamFolderSuffix(dataStreamName))
	} else {
		fmt.Fprintf(&builder, "Example events for `%s` look as following:\n\n",
			stripDataStreamFolderSuffix(dataStreamName))
	}

	for i, eventPath := range eventPaths {
		if i > 0 {
			fmt.Fprintln(&builder)
		}
		err := renderSampleEvent(&builder, eventPath)
		if err != nil {
			return "", err
		}
	}

	return builder.String(), nil
}

func renderSampleEvent(w io.Writer, eventPath string) error {
	body, err := os.ReadFile(eventPath)
	if err != nil {
		return fmt.Errorf("reading sample event file failed (path: %s): %w", eventPath, err)
	}

	formatted, _, err := formatter.JSONFormatter(body)
	if err != nil {
		return fmt.Errorf("formatting sample event file failed (path: %s): %w", eventPath, err)
	}

	fmt.Fprintln(w, "```json")
	fmt.Fprintln(w, string(formatted))
	fmt.Fprint(w, "```")
	return nil
}

func stripDataStreamFolderSuffix(dataStreamName string) string {
	dataStreamName = strings.ReplaceAll(dataStreamName, "_metrics", "")
	dataStreamName = strings.ReplaceAll(dataStreamName, "_logs", "")
	return dataStreamName
}
