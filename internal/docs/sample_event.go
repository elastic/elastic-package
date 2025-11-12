// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/packages"
)

const sampleEventFile = "sample_event.json"

func renderSampleEvent(packageRoot, dataStreamName string) (string, error) {
	var eventPath string
	if dataStreamName == "" {
		eventPath = filepath.Join(packageRoot, sampleEventFile)
	} else {
		eventPath = filepath.Join(packageRoot, "data_stream", dataStreamName, sampleEventFile)
	}

	body, err := os.ReadFile(eventPath)
	if err != nil {
		return "", fmt.Errorf("reading sample event file failed (path: %s): %w", eventPath, err)
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", fmt.Errorf("reading package manifest failed: %w", err)
	}
	specVersion, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return "", fmt.Errorf("parsing format version %q failed: %w", manifest.SpecVersion, err)
	}

	jsonFormatter := formatter.JSONFormatterBuilder(*specVersion)
	formatted, _, err := jsonFormatter.Format(body)
	if err != nil {
		return "", fmt.Errorf("formatting sample event file failed (path: %s): %w", eventPath, err)
	}

	var builder strings.Builder
	if dataStreamName == "" {
		builder.WriteString("An example event looks as following:\n\n")
	} else {
		builder.WriteString(fmt.Sprintf("An example event for `%s` looks as following:\n\n",
			stripDataStreamFolderSuffix(dataStreamName)))
	}
	builder.WriteString("```json\n")
	builder.Write(bytes.TrimSpace(formatted))
	builder.WriteString("\n```")
	return builder.String(), nil
}

func stripDataStreamFolderSuffix(dataStreamName string) string {
	dataStreamName = strings.ReplaceAll(dataStreamName, "_metrics", "")
	dataStreamName = strings.ReplaceAll(dataStreamName, "_logs", "")
	return dataStreamName
}
