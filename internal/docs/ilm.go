// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// flattenNestedMap flattens a nested JSON-like structure (maps and slices) into
// a flat map with dot-separated keys.
func flattenNestedMap(prefix string, nested map[string]interface{}, flatMap map[string]string) {
	for k, v := range nested {
		key := k
		if prefix != "" {
			key = fmt.Sprintf("%s.%s", prefix, k)
		}

		switch child := v.(type) {
		case map[string]interface{}:
			flattenNestedMap(key, child, flatMap)
		case []interface{}:
			for i, val := range child {
				// handle slices with index
				newKey := fmt.Sprintf("%s.%d", key, i)
				if nextMap, ok := val.(map[string]interface{}); ok {
					flattenNestedMap(newKey, nextMap, flatMap)
				} else {
					flatMap[newKey] = fmt.Sprintf("%v", val)
				}
			}
		default:
			flatMap[key] = fmt.Sprintf("%v", v)
		}
	}
}

func getILMPolicyMap(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading ILM policy file failed: %w", err)
	}
	var policy map[string]interface{}
	err = json.Unmarshal(content, &policy)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling ILM policy failed: %w", err)
	}

	flatMap := make(map[string]string)
	flattenNestedMap("", policy, flatMap)
	return flatMap, nil
}

func renderILMPolicyMap(output *strings.Builder, policyMap map[string]string) {
	keys := make([]string, 0, len(policyMap))
	for key := range policyMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		output.WriteString(fmt.Sprintf("| %s | %s |\n", key, policyMap[key]))
	}
}

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
		ilmPolicyPath := filepath.Join(packageRoot, "data_stream", ilmPath, "elasticsearch", "ilm", "default_policy.json")
		// get the policy map
		policyMap, err := getILMPolicyMap(ilmPolicyPath)
		if err != nil {
			return "", fmt.Errorf("getting ILM policy map for path %s failed: %w", ilmPolicyPath, err)
		}
		renderedDocs.WriteString(fmt.Sprintf("#### %s\n", ilmPath))

		// render the policy map as a markdown table
		renderedDocs.WriteString("| Key | Value |\n")
		renderedDocs.WriteString("|---|---|\n")
		renderILMPolicyMap(&renderedDocs, policyMap)
	}
	return renderedDocs.String(), nil
}

// findILMPaths scans a given package path for data streams that have ILM policies
// and returns a list of all data stream names that have ILM policies defined.
func findILMPaths(packageRoot string) ([]string, error) {
	// look for ilm/ from the packageRoot/data_stream/<data_stream_name>/elasticsearch/ilm/
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
