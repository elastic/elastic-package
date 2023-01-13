// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

//go:embed _static/ecs_mappings.json
var staticEcsMappings string

const prefixMapping = "_embedded_ecs"

var semver2_3_0 = semver.MustParse("2.3.0")

type ecsTemplates struct {
	Mappings struct {
		DynamicTemplates []map[string]interface{} `yaml:"dynamic_templates"`
	} `yaml:"mappings"`
}

func addDynamicMappings(packageRoot, destinationDir string) error {
	packageManifest := filepath.Join(destinationDir, packages.PackageManifestFile)

	m, err := packages.ReadPackageManifest(packageManifest)
	if err != nil {
		return err
	}

	shouldImport, err := shouldImportEcsMappings(m.SpecVersion, packageRoot)
	if err != nil {
		return err
	}
	if !shouldImport {
		return nil
	}

	logger.Debug("Import ECS mappings into the built package")

	switch m.Type {
	case "integration":
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
	case "input":
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

func shouldImportEcsMappings(specVersion, packageRoot string) (bool, error) {
	v, err := semver.NewVersion(specVersion)
	if err != nil {
		return false, errors.Wrap(err, "invalid spec version")
	}

	if v.LessThan(semver2_3_0) {
		logger.Debugf("Required spec version >= %s to import ECS mappings", semver2_3_0.String())
		return false, nil
	}

	bm, ok, err := buildmanifest.ReadBuildManifest(packageRoot)
	if err != nil {
		return false, errors.Wrap(err, "can't read build manifest")
	}
	if !ok {
		logger.Debug("Build manifest hasn't been defined for the package")
		return false, nil
	}
	if !bm.ImportMappings() {
		logger.Debug("Package doesn't have to import ECS mappings")
		return false, nil
	}
	return true, nil
}

func addDynamicMappingElements(path string) ([]byte, error) {
	ecsMappings, err := loadEcsMappings()
	if err != nil {
		return nil, errors.New("can't load ecs mappings template")
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("can't read manifest")
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

	contents, err = formatResult(&doc)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

func loadEcsMappings() (ecsTemplates, error) {
	var ecsMappings ecsTemplates
	err := yaml.Unmarshal([]byte(staticEcsMappings), &ecsMappings)
	if err != nil {
		return ecsMappings, err
	}
	return ecsMappings, nil
}

func addEcsMappings(doc *yaml.Node, mappings ecsTemplates) error {
	var templates yaml.Node
	err := templates.Encode(mappings.Mappings.DynamicTemplates)
	if err != nil {
		return errors.Wrap(err, "failed to encode dynamic templates")
	}

	renameMappingsNames(&templates)

	err = appendElements(doc, []string{"elasticsearch", "index_template", "mappings", "dynamic_templates"}, &templates)
	if err != nil {
		return errors.Wrap(err, "failed to append dynamic templates")
	}

	return nil
}

func renameMappingsNames(doc *yaml.Node) {
	switch doc.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(doc.Content); i += 2 {
			doc.Content[i].Value = fmt.Sprintf("%s-%s", prefixMapping, doc.Content[i].Value)
		}
	case yaml.SequenceNode:
		for i := 0; i < len(doc.Content); i++ {
			renameMappingsNames(doc.Content[i])
		}
	case yaml.DocumentNode:
		renameMappingsNames(doc.Content[0])
	}
}

func appendElements(root *yaml.Node, path []string, values *yaml.Node) error {
	if len(path) == 0 {
		contents := values.Content
		if values.Kind == yaml.DocumentNode {
			contents = values.Content[0].Content
		}
		root.Content = append(root.Content, contents...)
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
	case "dynamic_templates":
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
