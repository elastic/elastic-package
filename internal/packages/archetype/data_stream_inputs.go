// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
)

type Input struct {
	Name         string          `yaml:"name"`
	Title        string          `yaml:"title"`
	Description  string          `yaml:"description"`
	TemplatePath string          `yaml:"template_path"`
	Documentation string `yaml:"documentation"`
	Vars         []InputVariable `yaml:"vars"`
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

// populateInput will populate `dataStreamDescriptor` with the appropriate variables for each input type it contains.
//
// When `dataStreamDescriptor` is created by the create data-stream command, it will be populated with only the input names
// provided by the user. This will further enrich the `dataStreamDescriptor` with the variables for the given inputs.
func populateInput(dataStreamDescriptor *DataStreamDescriptor) error {
	var cfg InputConfig
	err := yaml.Unmarshal([]byte(inputVariables), &cfg)
	if err != nil {
		return fmt.Errorf("error unmarshaling yaml: %w", err)
	}

	for i := range dataStreamDescriptor.Manifest.Streams {
		for _, input := range cfg.Inputs {
			if dataStreamDescriptor.Manifest.Streams[i].Input == input.Name {
				dataStreamDescriptor.Manifest.Streams[i].Title = input.Title
				dataStreamDescriptor.Manifest.Streams[i].Description = input.Description
				dataStreamDescriptor.Manifest.Streams[i].TemplatePath = input.TemplatePath
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
func loadInputDefinitions() ([]Input, error) {
	var inputDefs = []Input{}
	var inputDef Input
	for i := range inputs {
		err := yaml.Unmarshal([]byte(inputs[i]), &inputDef)
		if err != nil {
			return nil, fmt.Errorf("loading input def: %w", err)
		}
		inputDefs = append(inputDefs, inputDef)

	}
	return inputDefs, nil
}

// GetDocumentation returns the documentation for the given input
func GetDocumentation(inputName string) (string, error) {
	inputDefs, err := loadInputDefinitions()
	if err != nil {
		return "", err
	}
	for _, input := range inputDefs {
		if input.Name == inputName {
			return input.Description, nil
		}
	}
	return "", fmt.Errorf("no documentation found for input %s", inputName)
}
