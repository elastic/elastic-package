// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Transform struct {
	Path      string
	NestedMap map[string]string
}

func getTransformPolicyMap(path string) (map[string]string, error) {
	fmt.Printf("getting Transform policy map for path: %s", path)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading Transform policy file failed: %w", err)
	}
	var policy map[string]interface{}
	err = yaml.Unmarshal(content, &policy)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling Transform policy failed: %w", err)
	}

	flatMap := make(map[string]string)
	flattenNestedMap("", policy, flatMap)
	return flatMap, nil
}

func renderTransformPaths(packageRoot string) (string, error) {
	// gather the mapping of transforms defined in the package
	// if the directory does not exist, or there are no transforms defined, return an empty string
	transformPaths, err := findTransformPaths(packageRoot)
	if err != nil {
		// if the directory does not exist, return an empty string
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("finding transform paths failed: %w", err)
	}
	if len(transformPaths) == 0 {
		return "", nil
	}

	var renderedDocs strings.Builder
	renderedDocs.WriteString("\n### Transforms used:\n")

	// render the transform map as a markdown table
	renderedDocs.WriteString("| Name | Description | Source | Dest |\n")
	renderedDocs.WriteString("|---|---|---|---|\n")
	for name, transform := range transformPaths {
		// get the description from the nested map
		description, ok := transform.NestedMap["description"]
		if !ok {
			description = ""
		}
		// get the source from the nested map
		source, ok := transform.NestedMap["source.index"]
		if !ok {
			source, ok = transform.NestedMap["source.index.0"]
			if !ok {
				source = ""
			}
		}
		// get the dest from the nested map
		dest, ok := transform.NestedMap["dest.index"]
		if !ok {
			dest, ok = transform.NestedMap["dest.index.0"]
			if !ok {
				dest = ""
			}
		}
		renderedDocs.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", name, description, source, dest))
	}
	return renderedDocs.String(), nil
}

// findTransformPaths scans a given package path for transforms that have been defined
// and returns a mapping of all transform names to their descriptions.
func findTransformPaths(packageRoot string) (map[string]Transform, error) {

	result := make(map[string]Transform)

	transformsRoot := filepath.Join(packageRoot, "elasticsearch", "transform")

	// make sure the directory exists
	stat, err := os.Stat(transformsRoot)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", transformsRoot)
	}

	// walk the transformsRoot directory and collect the transform names and descriptions
	// the transform name is the base name of the directory
	// the transform description is the description field in the transform.yml file
	err = filepath.WalkDir(transformsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// We are only interested in files named "transform.yml".
		// We also check that it's not a directory.
		if d.IsDir() || d.Name() != "transform.yml" {
			return nil
		}

		var transform Transform
		transform.Path = path
		// read the file into a map
		transform.NestedMap, err = getTransformPolicyMap(path)
		if err != nil {
			return fmt.Errorf("getting Transform policy map failed: %w", err)
		}

		// get the transform name from the transformPath
		transformName := filepath.Base(filepath.Dir(path))
		result[transformName] = transform
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
