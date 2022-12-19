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
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

//go:embed _static/ecs_mappings.json
var staticEcsMappings string

type ecsTemplates struct {
	Mappings struct {
		Properties       map[string]interface{}   `yaml:"properties"`
		DynamicTemplates []map[string]interface{} `yaml:"dynamic_templates"`
	} `yaml:"mappings"`
}

func addDynamicMappings(packageRoot, destinationDir string) error {
	bm, ok, err := buildmanifest.ReadBuildManifest(packageRoot)
	if err != nil {
		return errors.Wrap(err, "can't read build manifest")
	}
	if !ok {
		logger.Debugf("Build manifest hasn't been defined for the package")
		return nil
	}
	if !bm.ImportCommonDynamicMappings() {
		logger.Debugf("Package doesn't have to import common dynamic mappings")
		return nil
	}

	dataStreamManifests, err := filepath.Glob(filepath.Join(destinationDir, "data_stream", "*", packages.DataStreamManifestFile))
	if err != nil {
		return err
	}

	for _, datastream := range dataStreamManifests {
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

	err = addEcsMappings(&doc, ecsMappings)
	if err != nil {
		return nil, err
	}

	err = addEcsMappingsListMeta(&doc, ecsMappings)
	if err != nil {
		return nil, err
	}

	contents, err = formatResult(&doc)
	if err != nil {
		logger.Errorf("Error formatting %s", err)
		return nil, err
	}

	logger.Debugf("New Contents manifest:\n%s", contents)
	return contents, nil
}

func addEcsMappings(doc *yaml.Node, mappings ecsTemplates) error {
	var templates, properties yaml.Node
	err := templates.Encode(mappings.Mappings.DynamicTemplates)
	if err != nil {
		return err
	}
	err = properties.Encode(mappings.Mappings.Properties)
	if err != nil {
		logger.Errorf("Error encoding properties %s", err)
		return err
	}

	err = appendElements(doc, []string{"elasticsearch", "index_template", "mappings", "dynamic_templates"}, &templates)
	if err != nil {
		logger.Errorf("Error appending elems %s", err)
		return err
	}

	err = appendElements(doc, []string{"elasticsearch", "index_template", "mappings", "properties"}, &properties)
	if err != nil {
		logger.Errorf("Error appending properties %s", err)
		return err
	}

	return nil
}

func addEcsMappingsListMeta(doc *yaml.Node, mappings ecsTemplates) error {
	type ecsMappingsAdded struct {
		Properties       []string `yaml:"properties"`
		DynamicTemplates []string `yaml:"dynamic_templates"`
	}

	var mappingsAdded ecsMappingsAdded
	for property := range mappings.Mappings.Properties {
		mappingsAdded.Properties = append(mappingsAdded.Properties, property)
	}

	for _, dynamicTemplate := range mappings.Mappings.DynamicTemplates {
		if len(dynamicTemplate) > 1 {
			return errors.New("more than one dynamic template present")
		}

		for title := range dynamicTemplate {
			mappingsAdded.DynamicTemplates = append(mappingsAdded.DynamicTemplates, title)
		}
	}

	var properties yaml.Node
	err := properties.Encode(mappingsAdded.Properties)
	if err != nil {
		return err
	}

	err = appendElements(doc, []string{"elasticsearch", "index_template", "mappings", "_meta", "ecs_properties_added"}, &properties)
	if err != nil {
		return err
	}

	var dynamicTemplates yaml.Node
	err = dynamicTemplates.Encode(mappingsAdded.DynamicTemplates)
	if err != nil {
		return err
	}

	err = appendElements(doc, []string{"elasticsearch", "index_template", "mappings", "_meta", "ecs_dynamic_templates_added"}, &dynamicTemplates)
	if err != nil {
		return err
	}

	return nil
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
		newContentNodes := newYamlNode(key)

		root.Content = append(root.Content, newContentNodes...)
		return appendElements(newContentNodes[1], rest, values)
	case yaml.SequenceNode:
		index, err := strconv.Atoi(key)
		if err != nil {
			return err
		}
		if len(root.Content) >= index {
			return errors.Errorf("index out of range in nodes from key %s", key)
		}

		return appendElements(root.Content[index], rest, values)
	}
	return nil
}

func newYamlNode(key string) []*yaml.Node {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	var childNode *yaml.Node
	switch key {
	case "dynamic_templates", "ecs_properties_added", "ecs_dynamic_templates_added":
		childNode = &yaml.Node{Kind: yaml.SequenceNode, Value: key}
	default:
		childNode = &yaml.Node{Kind: yaml.MappingNode, Value: key}
	}
	return []*yaml.Node{keyNode, childNode}
}

func formatResult(result interface{}) ([]byte, error) {
	d, err := yaml.Marshal(result)
	if err != nil {
		return nil, errors.New("failed to encode")
	}
	d, _, err = formatter.YAMLFormatter(d)
	if err != nil {
		return nil, errors.New("failed to format")
	}
	return d, nil
}
