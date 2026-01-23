// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func renderILMPaths(packageRoot string) (string, error) {
	// gather the list of data streams that have ILM policies defined
	// if the list is empty, return ""
	// if the list is not empty, format the list as a markdown list
	ilmPaths, err := findILMPaths(packageRoot)
	if err != nil {
		return "", fmt.Errorf("finding ILM paths failed: %w", err)
	}
	if len(ilmPaths) == 0 {
		return "", nil
	}

	sort.Strings(ilmPaths)
	var renderedDocs strings.Builder
	renderedDocs.WriteString("\n### Data streams using ILM policies:\n")
	for _, ilmPath := range ilmPaths {
		renderedDocs.WriteString(fmt.Sprintf("- [%s](./%s.md)\n", ilmPath, ilmPath))
	}
	return renderedDocs.String(), nil
}

// findILMPaths scans a given package path for data streams that have ILM policies
// and returns a list of all data stream names that have ILM policies defined.
func findILMPaths(packageRoot string) ([]string, error) {
	// look for ilm/ from the packageRoot/data_stream/<data_stream_name>/elastsicsearch/ilm/
	// add the data_stream_name to the list
	ilmPaths, err := filepath.Glob(filepath.Join(packageRoot, "data_stream", "*", "elasticsearch", "ilm"))
	if err != nil {
		return nil, fmt.Errorf("finding ILM paths failed: %w", err)
	}

	result := make([]string, 0, len(ilmPaths))

	// return the list of globbed paths
	for _, ilmPath := range ilmPaths {
		// get the data_stream_name from the ilmPath
		dataStreamName := filepath.Base(filepath.Dir(filepath.Dir(ilmPath)))
		result = append(result, dataStreamName)
	}
	return result, nil
}
