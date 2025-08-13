// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
)

type InputConfig struct {
	Inputs []Input `yaml:"inputs"`
}

type Input struct {
	Name string          `yaml:"name"`
	Vars []InputVariable `yaml:"vars"`
}

type InputVariable struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Title       string `yaml:"title"`
	Multi       bool   `yaml:"multi"`
	Required    bool   `yaml:"required"`
	ShowUser    bool   `yaml:"show_user"`
	Description string `yaml:"description"`
}

// populateInputVariables will populate `dataStreamDescriptor` with the appropriate variables for each input type it contains.
//
// When `dataStreamDescriptor` is created by the create data-stream command, it will be populated with only the input names
// provided by the user. This will further enrich the `dataStreamDescriptor` with the variables for the given inputs.
func populateInputVariables(dataStreamDescriptor *DataStreamDescriptor) error {
	var cfg InputConfig
	err := yaml.Unmarshal([]byte(inputVariables), &cfg)
	if err != nil {
		return fmt.Errorf("error unmarshaling yaml: %w", err)
	}

	for i := range dataStreamDescriptor.Manifest.Streams {
		for _, input := range cfg.Inputs {
			if dataStreamDescriptor.Manifest.Streams[i].Input == input.Name {
				unpackVars(&dataStreamDescriptor.Manifest.Streams[i].Vars, input.Vars)
				break
			}
		}
	}
	return nil
}

func unpackVars(output *[]packages.Variable, input []InputVariable) {
	if output == nil {
		output = new([]packages.Variable)
	}
	for i := range input {
		var newVar packages.Variable
		inputVar := input[i]
		newVar.Name = inputVar.Name
		newVar.Type = inputVar.Type
		newVar.Title = inputVar.Title
		newVar.Multi = inputVar.Multi
		newVar.Required = inputVar.Required
		newVar.ShowUser = inputVar.ShowUser
		newVar.Description = inputVar.Description

		*output = append(*output, newVar)
	}
}
