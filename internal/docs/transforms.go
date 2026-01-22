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

	"github.com/elastic/elastic-package/internal/logger"
)

type Transform struct {
	Description string `yaml:"description"`
}

func renderTransformPaths(packageRoot string) (string, error) {
	// look for transform/ from the packageRoot/elasticsearch/transform/<transform_name>/transform.yml
	// add the transform_name to the list
	// if the list is empty, return ""
	// if the list is not empty, format the list as a markdown list
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
	for name, transform := range transformPaths {
		// Render each input documentation into a collapsible section.
		fmt.Fprintf(&renderedDocs, "<details>\n<summary>%s</summary>\n\n%s\n</details>\n", name, transform.Description)

	}
	return renderedDocs.String(), nil
}

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

	err = filepath.WalkDir(transformsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// We are only interested in files named "transform.yml".
		// We also check that it's not a directory.
		if d.IsDir() || d.Name() != "transform.yml" {
			return nil
		}

		yamlFile, err := os.ReadFile(path)
		if err != nil {
			logger.Warnf("could not read %s", path)
			return nil // Continue walking even if one file fails.
		}

		var transform Transform
		if err := yaml.Unmarshal(yamlFile, &transform); err != nil {
			logger.Errorf("Error unmarshalling YAML from %s: %v", path, err)
			return nil
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
