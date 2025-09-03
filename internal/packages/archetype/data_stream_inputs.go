// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/docs"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

type Input struct {
	Name         string          `yaml:"name"`
	Title        string          `yaml:"title"`
	Description  string          `yaml:"description"`
	TemplatePath string          `yaml:"template_path"`
	Vars         []InputVariable `yaml:"vars"`
}

type InputVariable struct {
	Name                  string      `yaml:"name"`
	Type                  string      `yaml:"type"`
	Title                 string      `yaml:"title"`
	Description           string      `yaml:"description"`
	Multi                 bool        `yaml:"multi"`
	Required              bool        `yaml:"required"`
	Secret                bool        `yaml:"secret"`
	ShowUser              bool        `yaml:"show_user"`
	HideInDeploymentModes []string    `yaml:"hide_in_deployment_modes"`
	UrlAllowedSchemes     []string    `yaml:"url_allowed_schemes"`
	MinDuration           string      `yaml:"min_duration"`
	MaxDuration           string      `yaml:"max_duration"`
	Default               interface{} `yaml:"default"`
}

// populateInputs will populate `dataStreamDescriptor` with the appropriate variables for each input type it contains.
//
// When `dataStreamDescriptor` is created by the create data-stream command, it will be populated with only the input names
// provided by the user. This will further enrich the `dataStreamDescriptor` with the variables for the given inputs.
func populateInputs(dataStreamDescriptor *DataStreamDescriptor) error {
	inputDefs, err := loadInputDefinitions()
	if err != nil {
		return fmt.Errorf("populating inputs: %w", err)
	}
	for i := range dataStreamDescriptor.Manifest.Streams {
		for _, input := range inputDefs {
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
		newVar.Secret = inputVar.Secret
		newVar.ShowUser = inputVar.ShowUser
		newVar.HideInDeploymentModes = inputVar.HideInDeploymentModes
		newVar.UrlAllowedSchemes = inputVar.UrlAllowedSchemes
		newVar.MinDuration = inputVar.MinDuration
		newVar.MaxDuration = inputVar.MaxDuration
		newVar.Description = inputVar.Description
		if inputVar.Default != nil {
			newVar.Default.Unpack(inputVar.Default)
		}
		*output = append(*output, newVar)
	}
}

// loadInputDefinitions loads from the embedded _static/inputs yml files.
func loadInputDefinitions() ([]Input, error) {
	var inputDefs = []Input{}
	agentTemplates := getAgentTemplateFileNames()

	err := fs.WalkDir(docs.InputDescriptions, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".yml" {
			fileData, readErr := docs.InputDescriptions.ReadFile(path)
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
			template, ok := agentTemplates[inputDef.Name]
			if ok {
				inputDef.TemplatePath = template
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

// getAgentTemplateFileNames returns a maps of agent template file names.
func getAgentTemplateFileNames() map[string]string {
	templates := make(map[string]string)
	fs.WalkDir(docs.AgentTemplates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			templates[strings.ReplaceAll(strings.TrimSuffix(filepath.Base(path), ".yml.hbs"), "_", "-")] = filepath.Base(path)
		}
		return nil
	})
	return templates
}

// loadRawAgentTemplate returns the raw agent template for a specific input.
func loadRawAgentTemplate(inputName string) (string, error) {
	var agentTemplate string
	templateFileName := strings.ReplaceAll(inputName, "-", "_")

	err := fs.WalkDir(docs.AgentTemplates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Ext(path) == ".hbs" {
			if strings.HasPrefix(filepath.Base(path), templateFileName) {
				// Found the agent file for the input
				fileData, readErr := docs.AgentTemplates.ReadFile(path)
				if readErr != nil {
					return readErr
				}

				agentTemplate = strings.TrimSpace(string(fileData))
				return nil
			}
		}
		return nil
	})

	return agentTemplate, err
}
