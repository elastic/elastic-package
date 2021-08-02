// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/formatter"

	"github.com/pkg/errors"
)

const sampleEventFile = "sample_event.json"

func renderSampleEvent(packageRoot, dataStreamName string) (string, error) {
	eventPath := filepath.Join(packageRoot, "data_stream", dataStreamName, sampleEventFile)

	body, err := os.ReadFile(eventPath)
	if err != nil {
		return "", errors.Wrapf(err, "reading sample event file failed (path: %s)", eventPath)
	}

	formatted, _, err := formatter.JSONFormatter(body)
	if err != nil {
		return "", errors.Wrapf(err, "formatting sample event file failed (path: %s)", eventPath)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("An example event for `%s` looks as following:\n\n",
		stripDataStreamFolderSuffix(dataStreamName)))
	builder.WriteString("```json\n")
	builder.Write(formatted)
	builder.WriteString("\n```")
	return builder.String(), nil
}

func stripDataStreamFolderSuffix(dataStreamName string) string {
	dataStreamName = strings.ReplaceAll(dataStreamName, "_metrics", "")
	dataStreamName = strings.ReplaceAll(dataStreamName, "_logs", "")
	return dataStreamName
}
