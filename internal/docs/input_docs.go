// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/logger"
)

type Input struct {
	Name          string `yaml:"name"`
	Documentation string `yaml:"documentation"`
}

type DataStreamManifest struct {
	Streams []struct {
		Input string `yaml:"input"`
	} `yaml:"streams"`
}

func RenderInputDocs(packageRoot string) (string, error) {
	inputs, err := findDataStreamInputs(packageRoot)
	if err != nil {
		return "", fmt.Errorf("could not find data stream inputs: %w", err)
	}

	inputDefs, err := loadInputDefinitions()
	if err != nil {
		return "", fmt.Errorf("loading input static content: %w", err)
	}

	sort.Strings(inputs)
	renderedDocs := ""

	re := regexp.MustCompile(`(?m)^(#+)(\s+)`)
	for _, input := range inputs {
		for _, inputDef := range inputDefs {
			if inputDef.Name == input {
				// Increase header level to match the level it will be placed into when rendered.
				// In the rendered readme. input documentation is three levels lower than in the input definitions.
				docs := re.ReplaceAllString(inputDef.Documentation, "${1}###${2}")
				renderedDocs = fmt.Sprintf("%s#### %s\n\n%s\n\n", renderedDocs, inputDef.Name, docs)
				break
			}
		}
	}
	return renderedDocs, nil
}

// FindDataStreamInputs scans a given package path for data stream manifests
// and returns a list of all inputs used in the package.
func findDataStreamInputs(packagePath string) ([]string, error) {
	// Use a map to collect unique inputs
	uniqueInputs := make(map[string]struct{})

	dataStreamsRoot := filepath.Join(packagePath, "data_stream")
	if _, err := os.Stat(dataStreamsRoot); os.IsNotExist(err) {
		// It's not an error if a package has no data streams, just return empty.
		return []string{}, nil
	}

	err := filepath.WalkDir(dataStreamsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// We are only interested in files named "manifest.yml".
		// We also check that it's not a directory.
		if d.IsDir() || d.Name() != "manifest.yml" {
			return nil
		}

		yamlFile, err := os.ReadFile(path)
		if err != nil {
			logger.Warnf("could not read %s", path)
			return nil // Continue walking even if one file fails.
		}

		var manifest DataStreamManifest
		if err := yaml.Unmarshal(yamlFile, &manifest); err != nil {
			logger.Errorf("Error unmarshalling YAML from %s: %v", path, err)
			return nil
		}

		for _, stream := range manifest.Streams {
			if stream.Input != "" {
				uniqueInputs[stream.Input] = struct{}{}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking directory %s: %w", dataStreamsRoot, err)
	}

	var inputs = []string{}
	for input := range uniqueInputs {
		inputs = append(inputs, input)
	}
	return inputs, nil
}

// loadInputDefinitions loads from the embedded _static/inputs yml files.
func loadInputDefinitions() ([]Input, error) {
	var inputDefs = []Input{}
	var inputDef Input
	for i := range inputResources {
		err := yaml.Unmarshal([]byte(inputResources[i]), &inputDef)
		if err != nil {
			return nil, fmt.Errorf("loading input def: %w", err)
		}
		inputDefs = append(inputDefs, inputDef)
	}
	return inputDefs, nil
}
