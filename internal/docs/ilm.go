// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
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

	ext := filepath.Ext(path)
	if ext == ".yml" || ext == ".yaml" {
		err = yaml.Unmarshal(content, &policy)
	} else {
		err = json.Unmarshal(content, &policy)
	}
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
		fmt.Fprintf(output, "| %s | %s |\n", escaper.Replace(key), escaper.Replace(policyMap[key]))
	}
}

func getILMPolicyFilePath(packageRoot, dataStreamName string) (string, error) {
	// also look for data_stream/<dataStreamName>/lifecycle.yml
	// if lifecycle.yml exists, return that
	lifecyclePath := filepath.Join(packageRoot, "data_stream", dataStreamName, "lifecycle.yml")
	_, err := os.Stat(lifecyclePath)
	if err == nil {
		return lifecyclePath, nil
	}

	// otherwise, look for something in an ilm directory
	paths, err := filepath.Glob(filepath.Join(packageRoot, "data_stream", dataStreamName, "elasticsearch", "ilm", "*.json"))
	if err != nil {
		return "", err
	} else if len(paths) == 0 {
		return "", fmt.Errorf("no ILM policy files found for data stream %s", dataStreamName)
	}
	return paths[0], nil
}

func renderILMPolicySection(out *strings.Builder, title string, policyMap map[string]string) {
	fmt.Fprintf(out, "\n#### %s Policy\n", title)
	out.WriteString("| Key | Value |\n")
	out.WriteString("|---|---|\n")
	renderILMPolicyMap(out, policyMap)
}

// renderDataStreamILM renders ILM policies for named data streams (integration packages).
func renderDataStreamILM(packageRoot string, dataStreamNames []string) (string, error) {
	if len(dataStreamNames) == 0 {
		return "", nil
	}
	var out strings.Builder
	out.WriteString("\n### Data streams using ILM policies\n")
	for _, name := range dataStreamNames {
		ilmPath, err := getILMPolicyFilePath(packageRoot, name)
		if err != nil {
			return "", fmt.Errorf("getting ILM policy file path for data stream %s failed: %w", name, err)
		}
		policyMap, err := getILMPolicyMap(ilmPath)
		if err != nil {
			return "", fmt.Errorf("getting ILM policy map for path %s failed: %w", ilmPath, err)
		}
		renderILMPolicySection(&out, name, policyMap)
	}
	return out.String(), nil
}

// renderInputPackageILM renders the lifecycle policy at the package root (input packages).
// Returns an empty string when no root lifecycle.yml exists.
func renderInputPackageILM(packageRoot string) (string, error) {
	p := filepath.Join(packageRoot, "lifecycle.yml")
	policyMap, err := getILMPolicyMap(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("getting ILM policy map for root lifecycle.yml failed: %w", err)
	}
	sectionName := "lifecycle"
	if manifest, mErr := packages.ReadPackageManifestFromPackageRoot(packageRoot); mErr == nil && manifest.Name != "" {
		sectionName = manifest.Name
	}
	var out strings.Builder
	out.WriteString("\n### Data streams using ILM policies\n")
	renderILMPolicySection(&out, sectionName, policyMap)
	return out.String(), nil
}

// renderILMPaths is the entry point for the {{ ilm }} template function.
// args are data stream names and only apply to integration packages; passing
// them skips the input-package path entirely. Bare invocation checks for a
// root lifecycle.yml (input packages) first, then discovers data streams.
func renderILMPaths(packageRoot string, args []string) (string, error) {
	if len(args) > 0 {
		return renderDataStreamILM(packageRoot, args)
	}

	result, err := renderInputPackageILM(packageRoot)
	if err != nil || result != "" {
		return result, err
	}

	names, err := findILMPaths(packageRoot)
	if err != nil {
		return "", fmt.Errorf("finding ILM paths failed: %w", err)
	}
	return renderDataStreamILM(packageRoot, names)
}

// findILMPaths scans a given package path for data streams that have ILM policies
// or a lifecycle.yml file, and returns a sorted, deduplicated list of data stream names.
func findILMPaths(packageRoot string) ([]string, error) {
	seen := make(map[string]struct{})

	ilmPaths, err := filepath.Glob(filepath.Join(packageRoot, "data_stream", "*", "elasticsearch", "ilm"))
	if err != nil {
		return nil, fmt.Errorf("finding ILM paths failed: %w", err)
	}
	for _, ilmPath := range ilmPaths {
		seen[filepath.Base(filepath.Dir(filepath.Dir(ilmPath)))] = struct{}{}
	}

	lifecyclePaths, err := filepath.Glob(filepath.Join(packageRoot, "data_stream", "*", "lifecycle.yml"))
	if err != nil {
		return nil, fmt.Errorf("finding lifecycle paths failed: %w", err)
	}
	for _, lifecyclePath := range lifecyclePaths {
		seen[filepath.Base(filepath.Dir(lifecyclePath))] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}
