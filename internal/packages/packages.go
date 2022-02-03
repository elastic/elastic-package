// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
)

const (
	// PackageManifestFile is the name of the package's main manifest file.
	PackageManifestFile = "manifest.yml"

	// DataStreamManifestFile is the name of the data stream's manifest file.
	DataStreamManifestFile = "manifest.yml"

	defaultPipelineName = "default"

	dataStreamTypeLogs       = "logs"
	dataStreamTypeMetrics    = "metrics"
	dataStreamTypeSynthetics = "synthetics"
	dataStreamTypeTraces     = "traces"
)

// VarValue represents a variable value as defined in a package or data stream
// manifest file.
type VarValue struct {
	scalar interface{}
	list   []interface{}
}

// Unpack knows how to parse a variable value from a package or data stream
// manifest file into a VarValue.
func (vv *VarValue) Unpack(value interface{}) error {
	switch u := value.(type) {
	case []interface{}:
		vv.list = u
	default:
		vv.scalar = u
	}
	return nil
}

// MarshalJSON knows how to serialize a VarValue into the appropriate
// JSON data type and value.
func (vv VarValue) MarshalJSON() ([]byte, error) {
	if vv.scalar != nil {
		return json.Marshal(vv.scalar)
	} else if vv.list != nil {
		return json.Marshal(vv.list)
	}
	return []byte("null"), nil
}

// Variable is an instance of configuration variable (named, typed).
type Variable struct {
	Name    string   `config:"name" json:"name" yaml:"name"`
	Type    string   `config:"type" json:"type" yaml:"type"`
	Default VarValue `config:"default" json:"default" yaml:"default"`
}

// Input is a single input configuration.
type Input struct {
	Type string     `config:"type" json:"type" yaml:"type"`
	Vars []Variable `config:"vars" json:"vars" yaml:"vars"`
}

// KibanaConditions defines conditions for Kibana (e.g. required version).
type KibanaConditions struct {
	Version string `config:"version" json:"version" yaml:"version"`
}

// Conditions define requirements for different parts of the Elastic stack.
type Conditions struct {
	Kibana KibanaConditions `config:"kibana" json:"kibana" yaml:"kibana"`
}

// PolicyTemplate is a configuration of inputs responsible for collecting log or metric data.
type PolicyTemplate struct {
	Inputs []Input `config:"inputs" json:"inputs" yaml:"inputs"`
}

// Owner defines package owners, either a single person or a team.
type Owner struct {
	Github string `config:"github" json:"github" yaml:"github"`
}

// PackageManifest represents the basic structure of a package's manifest
type PackageManifest struct {
	Name            string           `config:"name" json:"name" yaml:"name"`
	Title           string           `config:"title" json:"title" yaml:"title"`
	Type            string           `config:"type" json:"type" yaml:"type"`
	Version         string           `config:"version" json:"version" yaml:"version"`
	Conditions      Conditions       `config:"conditions" json:"conditions" yaml:"conditions"`
	PolicyTemplates []PolicyTemplate `config:"policy_templates" json:"policy_templates" yaml:"policy_templates"`
	Vars            []Variable       `config:"vars" json:"vars" yaml:"vars"`
	Owner           Owner            `config:"owner" json:"owner" yaml:"owner"`
	Description     string           `config:"description" json:"description" yaml:"description"`
	License         string           `config:"license" json:"license" yaml:"license"`
	Categories      []string         `config:"categories" json:"categories" yaml:"categories"`
}

// DataStreamManifest represents the structure of a data stream's manifest
type DataStreamManifest struct {
	Name          string `config:"name" json:"name" yaml:"name"`
	Title         string `config:"title" json:"title" yaml:"title"`
	Type          string `config:"type" json:"type" yaml:"type"`
	Dataset       string `config:"dataset" json:"dataset" yaml:"dataset"`
	Hidden        bool   `config:"hidden" json:"hidden" yaml:"hidden"`
	Release       string `config:"release" json:"release" yaml:"release"`
	Elasticsearch *struct {
		IngestPipeline *struct {
			Name string `config:"name" json:"name" yaml:"name"`
		} `config:"ingest_pipeline" json:"ingest_pipeline" yaml:"ingest_pipeline"`
	} `config:"elasticsearch" json:"elasticsearch" yaml:"elasticsearch"`
	Streams []struct {
		Input string     `config:"input" json:"input" yaml:"input"`
		Vars  []Variable `config:"vars" json:"vars" yaml:"vars"`
	} `config:"streams" json:"streams" yaml:"streams"`
}

// MustFindPackageRoot finds and returns the path to the root folder of a package.
// It fails with an error if the package root can't be found.
func MustFindPackageRoot() (string, error) {
	root, found, err := FindPackageRoot()
	if err != nil {
		return "", errors.Wrap(err, "locating package root failed")
	}
	if !found {
		return "", errors.New("package root not found")
	}
	return root, nil
}

// FindPackageRoot finds and returns the path to the root folder of a package.
func FindPackageRoot() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, PackageManifestFile)
		fileInfo, err := os.Stat(path)
		if err == nil && !fileInfo.IsDir() {
			ok, err := isPackageManifest(path)
			if err != nil {
				return "", false, errors.Wrapf(err, "verifying manifest file failed (path: %s)", path)
			}
			if ok {
				return dir, true, nil
			}
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", false, nil
}

// FindDataStreamRootForPath finds and returns the path to the root folder of a data stream.
func FindDataStreamRootForPath(workDir string) (string, bool, error) {
	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, DataStreamManifestFile)
		fileInfo, err := os.Stat(path)
		if err == nil && !fileInfo.IsDir() {
			ok, err := isDataStreamManifest(path)
			if err != nil {
				return "", false, errors.Wrapf(err, "verifying manifest file failed (path: %s)", path)
			}
			if ok {
				return dir, true, nil
			}
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", false, nil
}

// ReadPackageManifestFromPackageRoot reads and parses the package manifest file for the given package.
func ReadPackageManifestFromPackageRoot(packageRoot string) (*PackageManifest, error) {
	return ReadPackageManifest(filepath.Join(packageRoot, PackageManifestFile))
}

// ReadPackageManifest reads and parses the given package manifest file.
func ReadPackageManifest(path string) (*PackageManifest, error) {
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "reading file failed (path: %s)", path)
	}

	var m PackageManifest
	err = cfg.Unpack(&m)
	if err != nil {
		return nil, errors.Wrapf(err, "unpacking package manifest failed (path: %s)", path)
	}
	return &m, nil
}

// ReadDataStreamManifest reads and parses the given data stream manifest file.
func ReadDataStreamManifest(path string) (*DataStreamManifest, error) {
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "reading file failed (path: %s)", path)
	}

	var m DataStreamManifest
	err = cfg.Unpack(&m)
	if err != nil {
		return nil, errors.Wrapf(err, "unpacking data stream manifest failed (path: %s)", path)
	}

	m.Name = filepath.Base(filepath.Dir(path))
	return &m, nil
}

// GetPipelineNameOrDefault returns the name of the data stream's pipeline, if one is explicitly defined in the
// data stream manifest. If not, the default pipeline name is returned.
func (dsm *DataStreamManifest) GetPipelineNameOrDefault() string {
	if dsm.Elasticsearch != nil && dsm.Elasticsearch.IngestPipeline != nil && dsm.Elasticsearch.IngestPipeline.Name != "" {
		return dsm.Elasticsearch.IngestPipeline.Name
	}
	return defaultPipelineName
}

// IndexTemplateName returns the name of the Elasticsearch index template that would be installed
// for this data stream.
// The template name starts with dot "." if the datastream is hidden which is consistent with kibana implementation
// https://github.com/elastic/kibana/blob/3955d0dc819fec03f68cd1d931f64da8472e34b2/x-pack/plugins/fleet/server/services/epm/elasticsearch/index.ts#L14
func (dsm *DataStreamManifest) IndexTemplateName(pkgName string) string {
	if dsm.Dataset == "" {
		return fmt.Sprintf("%s%s-%s.%s", dsm.indexTemplateNamePrefix(), dsm.Type, pkgName, dsm.Name)
	}

	return fmt.Sprintf("%s%s-%s", dsm.indexTemplateNamePrefix(), dsm.Type, dsm.Dataset)
}

func (dsm *DataStreamManifest) indexTemplateNamePrefix() string {
	if dsm.Hidden {
		return "."
	}
	return ""
}

// FindInputByType returns the input for the provided type.
func (pt *PolicyTemplate) FindInputByType(inputType string) *Input {
	for _, input := range pt.Inputs {
		if input.Type == inputType {
			return &input
		}
	}
	return nil
}

func isPackageManifest(path string) (bool, error) {
	m, err := ReadPackageManifest(path)
	if err != nil {
		return false, errors.Wrapf(err, "reading package manifest failed (path: %s)", path)
	}
	return m.Type == "integration" && m.Version != "", nil // TODO add support for other package types
}

func isDataStreamManifest(path string) (bool, error) {
	m, err := ReadDataStreamManifest(path)
	if err != nil {
		return false, errors.Wrapf(err, "reading package manifest failed (path: %s)", path)
	}
	return m.Title != "" &&
			(m.Type == dataStreamTypeLogs || m.Type == dataStreamTypeMetrics || m.Type == dataStreamTypeSynthetics || m.Type == dataStreamTypeTraces),
		nil
}
