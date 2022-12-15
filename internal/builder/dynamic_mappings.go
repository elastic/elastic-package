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
		addDynamicMappingElements(datastream)
	}

	packageManifest := filepath.Join(destinationDir, packages.PackageManifestFile)

	m, err := packages.ReadPackageManifest(packageManifest)
	if err != nil {
		return err
	}
	if m.Type == "input" {
		logger.Infof("Adding mappings to package manifest %s", packageManifest)
		addDynamicMappingElements(packageManifest)
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

	err = appendListElements(&doc, []string{"elasticsearch", "index_template", "mappings", "dynamic_templates"}, &templates)
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

	err = appendMapElements(&doc, []string{"elasticsearch", "index_template", "mappings", "properties"}, &properties)
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
	return nil, nil

}

func appendListElements(root *yaml.Node, path []string, values *yaml.Node) error {
	if len(path) == 0 {
		root.Content = append(root.Content, values.Content...)
		return nil
	}

	key := path[0]
	rest := path[1:]

	switch root.Kind {
	case yaml.DocumentNode:
		return appendListElements(root.Content[0], path, values)
	case yaml.MappingNode:
		for i := 0; i < len(root.Content); i += 2 {
			child := root.Content[i]
			if child.Value == key {
				return appendListElements(root.Content[i+1], rest, values)
			}
		}
	case yaml.SequenceNode:
		index, err := strconv.Atoi(key)
		if err != nil {
			return err
		}
		return appendListElements(root.Content[index], rest, values)
	}
	return nil
}

func appendMapElements(root *yaml.Node, path []string, values *yaml.Node) error {
	if len(path) == 0 {
		contents, _ := yaml.Marshal(root.Content)
		logger.Infof("appendMapElements> Node kind: %s", root.Kind)
		logger.Infof("appendMapElements> Values kind: %s", values.Kind)
		if len(values.Content) > 0 {
			logger.Infof("appendMapElements> First Values kind: %s", values.Content[0].Kind)
			logger.Infof("appendMapElements> Second Values kind: %s", values.Content[1].Kind)
		}
		logger.Infof("appendMapElements> First root child kind: %s", root.Content[0].Kind)
		logger.Infof("appendMapElements> Second root child kind: %s", root.Content[1].Kind)
		logger.Infof("appendMapElements> Node to update:\n%s", string(contents))
		contents, _ = yaml.Marshal(values.Content)
		logger.Infof("appendMapElements> Values to add :\n%s", string(contents))

		root.Content = append(root.Content, values.Content...)
		return nil
	}

	key := path[0]
	rest := path[1:]

	switch root.Kind {
	case yaml.DocumentNode:
		return appendMapElements(root.Content[0], path, values)
	case yaml.MappingNode:
		for i := 0; i < len(root.Content); i += 2 {
			child := root.Content[i]
			if child.Value == key {
				return appendMapElements(root.Content[i+1], rest, values)
			}
		}
		// TODO not found
		// create
	case yaml.SequenceNode:
		index, err := strconv.Atoi(key)
		if err != nil {
			return err
		}
		return appendMapElements(root.Content[index], rest, values)
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
