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
	var dir string
	if dataStreamName == "" {
		dir = packageRoot
	} else {
		dir = filepath.Join(packageRoot, "data_stream", dataStreamName)
	}

	// Glob for all sample event files in the directory (e.g. sample_event.json,
	// sample_event.logs.json, sample_event.metrics.json).
	matches, err := filepath.Glob(filepath.Join(dir, "sample_event*.json"))
	if err != nil {
		return "", fmt.Errorf("globbing for sample event files failed: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("reading sample event file failed (path: %s): %w",
			filepath.Join(dir, sampleEventFile), os.ErrNotExist)
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

	// Determine whether we have only the plain sample_event.json or type-qualified files.
	hasBaseOnly := len(matches) == 1 && filepath.Base(matches[0]) == sampleEventFile

	var builder strings.Builder
	for i, eventPath := range matches {
		if i > 0 {
			builder.WriteString("\n\n")
		}

		body, err := os.ReadFile(eventPath)
		if err != nil {
			return "", fmt.Errorf("reading sample event file failed (path: %s): %w", eventPath, err)
		}
		formatted, _, err := jsonFormatter.Format(body)
		if err != nil {
			return "", fmt.Errorf("formatting sample event file failed (path: %s): %w", eventPath, err)
		}

		switch {
		case hasBaseOnly && dataStreamName == "":
			builder.WriteString("An example event looks as following:\n\n")
		case hasBaseOnly:
			builder.WriteString(fmt.Sprintf("An example event for `%s` looks as following:\n\n",
				stripDataStreamFolderSuffix(dataStreamName)))
		default:
			sigType := sampleEventSignalType(filepath.Base(eventPath))
			if sigType == "" {
				return "", fmt.Errorf("cannot extract signal type from sample event filename %q", filepath.Base(eventPath))
			}
			builder.WriteString(fmt.Sprintf("An example **%s** event looks as following:\n\n", sigType))
		}
		builder.WriteString("```json\n")
		builder.Write(bytes.TrimSpace(formatted))
		builder.WriteString("\n```")
	}
	return builder.String(), nil
}

// sampleEventSignalType extracts the signal type from a type-qualified sample
// event filename such as "sample_event.logs.json" → "logs". Returns an empty
// string when the filename does not match the expected pattern.
func sampleEventSignalType(filename string) string {
	// "sample_event.logs.json" → suffix "logs.json" → sigType "logs"
	suffix, found := strings.CutPrefix(filename, "sample_event.")
	if !found {
		return ""
	}
	sigType, _, _ := strings.Cut(suffix, ".")
	return sigType
}

func stripDataStreamFolderSuffix(dataStreamName string) string {
	dataStreamName = strings.ReplaceAll(dataStreamName, "_metrics", "")
	dataStreamName = strings.ReplaceAll(dataStreamName, "_logs", "")
	return dataStreamName
}
