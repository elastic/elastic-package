// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"fmt"
	"gopkg.in/yaml.v3"
//	"os"
//	"path/filepath"
//
//	"github.com/elastic/elastic-package/internal/formatter"
//	"github.com/elastic/elastic-package/internal/logger"
//	"github.com/elastic/elastic-package/internal/packages"
)


type config struct {
	inputs map[string]input `yaml:"inputs"`
}

type input struct {
	vars []map[string]interface{} `yaml:"vars"`
}

//populateInputVariables will populate `dataStreamDescriptor` with the appropriate variables for each input type it contains.
//
// When `dataStreamDescriptor` is created by the create data-stream command, it will be populated with only the input names
// provided by the user. This will further enrich the `dataStreamDescriptor` with the variables for the inputs.
func populateInputVariables(dataStreamDescriptor *DataStreamDescriptor) error {
	var cfg config
	err := yaml.Unmarshal([]byte(inputVariables), &cfg)
	if err != nil {
		return fmt.Errorf("Error unmarshaling YAML: %w", err)
	}

	// Now you can iterate over the Inputs map to access each input type.
	for inputName, inputConfig := range cfg.inputs {
		fmt.Printf("Found input type: %s", inputName)

		// Access the flexible vars map for each input.
		if len(inputConfig.vars) > 0 {
			firstVar, ok := inputConfig.vars[0]["name"].(string)
			if !ok {
				fmt.Printf("Could not assert 'name' to string for the first variable of %s.", inputName)
			} else {
				fmt.Printf("  First variable name: %s", firstVar)
			}
		}
	}	
	return nil
}

