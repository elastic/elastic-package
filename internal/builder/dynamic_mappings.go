// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	_ "embed"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

//go:embed _static/ecs_mappings.json
var staticEcsMappings string

func addDynamicMappings(destinationDir string) error {
	// Get raw datastream setFieldmanifest
	dataStreamManifests, err := filepath.Glob(filepath.Join(destinationDir, "data_stream", "*", packages.DataStreamManifestFile))
	if err != nil {
		return err
	}

	for _, datastream := range dataStreamManifests {
		logger.Infof("Adding mappings to datastream %s", datastream)
		contents, err := addDynamicMappingElements(datastream)
		if err != nil {
			return err
		}
		err = os.WriteFile(datastream, contents, 0664)
		if err != nil {
			return err
		}
	}

	packageManifest := filepath.Join(destinationDir, packages.PackageManifestFile)

	m, err := packages.ReadPackageManifest(packageManifest)
	if err != nil {
		return err
	}
	if m.Type == "input" {
		logger.Infof("Adding mappings to package manifest %s", packageManifest)
		contents, err := addDynamicMappingElements(packageManifest)
		if err != nil {
			return err
		}
		os.WriteFile(packageManifest, contents, 0664)
		if err != nil {
			return err
		}
	}

	return nil
}

func addDynamicMappingElements(path string) ([]byte, error) {
	type ecsTemplates struct {
		Mappings struct {
			Properties       map[string]interface{}   `yaml:"properties"`
			DynamicTemplates []map[string]interface{} `yaml:"dynamic_templates"`
		} `yaml:"mappings"`
	}

	var ecsMappings ecsTemplates
	err := yaml.Unmarshal([]byte(staticEcsMappings), &ecsMappings)
	if err != nil {
		return nil, err
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	err = yaml.Unmarshal(contents, &doc)
	if err != nil {
		return nil, err
	}

	logger.Infof("Number of dynamic templates to be added: %d", len(ecsMappings.Mappings.DynamicTemplates))
	var templates yaml.Node
	err = templates.Encode(ecsMappings.Mappings.DynamicTemplates)
	if err != nil {
		return nil, err
	}

	err = appendElements(&doc, []string{"elasticsearch", "index_template", "mappings", "dynamic_templates"}, &templates)
	if err != nil {
		logger.Errorf("Error appending elems %s", err)
		return nil, err
	}

	var properties yaml.Node
	err = properties.Encode(ecsMappings.Mappings.Properties)
	if err != nil {
		logger.Errorf("Error encoding properties %s", err)
		return nil, err
	}
	logger.Infof("Number of properties to be added: %d", len(ecsMappings.Mappings.Properties))

	err = appendElements(&doc, []string{"elasticsearch", "index_template", "mappings", "properties"}, &properties)
	if err != nil {
		logger.Errorf("Error appending properties %s", err)
		return nil, err
	}

	contents, err = formatResult(&doc)
	if err != nil {
		logger.Errorf("Error formatting %s", err)
		return nil, err
	}

	logger.Infof("New Contents manifest:\n%s", contents)
	return contents, nil

}

func appendElements(root *yaml.Node, path []string, values *yaml.Node) error {
	if len(path) == 0 {
		root.Content = append(root.Content, values.Content...)
		return nil
	}

	key := path[0]
	rest := path[1:]

	switch root.Kind {
	case yaml.DocumentNode:
		return appendElements(root.Content[0], path, values)
	case yaml.MappingNode:
		for i := 0; i < len(root.Content); i += 2 {
			child := root.Content[i]
			if child.Value == key {
				return appendElements(root.Content[i+1], rest, values)
			}
		}
		// TODO not found
		// create
	case yaml.SequenceNode:
		index, err := strconv.Atoi(key)
		if err != nil {
			return err
		}
		return appendElements(root.Content[index], rest, values)
	}
	return nil
}

func formatResult(result interface{}) ([]byte, error) {
	d, err := yaml.Marshal(result)
	if err != nil {
		logger.Errorf("formatResult error > %s", err)
		return nil, errors.New("failed to encode")
	}
	d, _, err = formatter.YAMLFormatter(d)
	if err != nil {
		return nil, errors.New("failed to format")
	}
	return d, nil
}
