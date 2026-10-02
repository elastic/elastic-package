// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
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

// renderInputDocs renders the documentation of the inputs used by the data streams
// of the package. inputTypes maps qualified input names to their input type and is
// used to resolve streams referencing an input by name; it may be nil.
func renderInputDocs(packageRoot string, inputTypes map[string]string) (string, error) {
	streamInputs, err := findDataStreamInputs(packageRoot)
	if err != nil {
		return "", fmt.Errorf("could not find data stream inputs: %w", err)
	}
	inputs := resolveInputTypes(streamInputs, inputTypes)
	if len(inputs) == 0 {
		return "", nil
	}

	inputDefs, err := loadInputDefinitions()
	if err != nil {
		return "", fmt.Errorf("loading input static content: %w", err)
	}

	sort.Strings(inputs)
	var renderedDocs strings.Builder
	renderedDocs.WriteString("These inputs can be used with this integration:\n")
	for _, input := range inputs {
		for _, inputDef := range inputDefs {
			if inputDef.Name == input {
				// Render each input documentation into a collapsible section.
				fmt.Fprintf(&renderedDocs, "<details>\n<summary>%s</summary>\n\n%s\n</details>\n", inputDef.Name, inputDef.Documentation)
				break
			}
		}
	}
	return renderedDocs.String(), nil
}

// inputTypesByName returns the input type of every policy template input that
// carries a name qualifier.
func inputTypesByName(manifest *packages.PackageManifest) map[string]string {
	types := make(map[string]string)
	for _, pt := range manifest.PolicyTemplates {
		for _, input := range pt.Inputs {
			if input.Name != "" && input.Type != "" {
				types[input.Name] = input.Type
			}
		}
	}
	return types
}

// resolveInputTypes maps stream inputs to input types, falling back to the raw
// value when it is not a known qualifier, and removes duplicates.
func resolveInputTypes(streamInputs []string, inputTypes map[string]string) []string {
	unique := make(map[string]struct{}, len(streamInputs))
	for _, input := range streamInputs {
		if inputType, ok := inputTypes[input]; ok {
			input = inputType
		}
		unique[input] = struct{}{}
	}
	resolved := make([]string, 0, len(unique))
	for input := range unique {
		resolved = append(resolved, input)
	}
	return resolved
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
			return nil //nolint:nilerr // walk continues on individual file errors; failures are logged
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

	err := fs.WalkDir(InputDescriptions, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".yml" {
			fileData, readErr := InputDescriptions.ReadFile(path)
			if readErr != nil {
				return readErr
			}

			var inputDef Input
			unmarshalErr := yaml.Unmarshal(fileData, &inputDef)
			if unmarshalErr != nil {
				logger.Errorf("unmarshalling %s: %w", path, unmarshalErr)
				// Continue with other files
				return nil
			}
			inputDefs = append(inputDefs, inputDef)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return inputDefs, nil
}
