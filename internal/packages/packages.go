// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
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

// Source contains metadata about the source code of the package.
type Source struct {
	License string `config:"license" json:"license" yaml:"license"`
}

// KibanaConditions defines conditions for Kibana (e.g. required version).
type KibanaConditions struct {
	Version string `config:"version" json:"version" yaml:"version"`
}

// ElasticConditions defines conditions related to Elastic subscriptions or partnerships.
type ElasticConditions struct {
	Subscription string `config:"subscription" json:"subscription" yaml:"subscription"`
}

// Conditions define requirements for different parts of the Elastic stack.
type Conditions struct {
	Kibana  KibanaConditions  `config:"kibana" json:"kibana" yaml:"kibana"`
	Elastic ElasticConditions `config:"elastic" json:"elastic" yaml:"elastic"`
}

// PolicyTemplate is a configuration of inputs responsible for collecting log or metric data.
type PolicyTemplate struct {
	Name        string   `config:"name" json:"name" yaml:"name"`                                                       // Name of policy template.
	DataStreams []string `config:"data_streams,omitempty" json:"data_streams,omitempty" yaml:"data_streams,omitempty"` // List of data streams compatible with the policy template.
	Inputs      []Input  `config:"inputs,omitempty" json:"inputs,omitempty" yaml:"inputs,omitempty"`

	// For purposes of "input packages"
	Input        string     `config:"input,omitempty" json:"input,omitempty" yaml:"input,omitempty"`
	Type         string     `config:"type,omitempty" json:"type,omitempty" yaml:"type,omitempty"`
	TemplatePath string     `config:"template_path,omitempty" json:"template_path,omitempty" yaml:"template_path,omitempty"`
	Vars         []Variable `config:"vars,omitempty" json:"vars,omitempty" yaml:"vars,omitempty"`
}

// Owner defines package owners, either a single person or a team.
type Owner struct {
	Github string `config:"github" json:"github" yaml:"github"`
}

// PackageManifest represents the basic structure of a package's manifest
type PackageManifest struct {
	SpecVersion     string           `config:"format_version" json:"format_version" yaml:"format_version"`
	Name            string           `config:"name" json:"name" yaml:"name"`
	Title           string           `config:"title" json:"title" yaml:"title"`
	Type            string           `config:"type" json:"type" yaml:"type"`
	Version         string           `config:"version" json:"version" yaml:"version"`
	Source          Source           `config:"source" json:"source" yaml:"source"`
	Conditions      Conditions       `config:"conditions" json:"conditions" yaml:"conditions"`
	PolicyTemplates []PolicyTemplate `config:"policy_templates" json:"policy_templates" yaml:"policy_templates"`
	Vars            []Variable       `config:"vars" json:"vars" yaml:"vars"`
	Owner           Owner            `config:"owner" json:"owner" yaml:"owner"`
	Description     string           `config:"description" json:"description" yaml:"description"`
	License         string           `config:"license" json:"license" yaml:"license"`
	Categories      []string         `config:"categories" json:"categories" yaml:"categories"`
}

type Elasticsearch struct {
	IndexTemplate *struct {
		IngestPipeline *struct {
			Name string `config:"name" json:"name" yaml:"name"`
		} `config:"ingest_pipeline" json:"ingest_pipeline" yaml:"ingest_pipeline"`
	} `config:"index_template" json:"index_template" yaml:"index_template"`
	SourceMode string `config:"source_mode" json:"source_mode" yaml:"source_mode"`
	IndexMode  string `config:"index_mode" json:"index_mode" yaml:"index_mode"`
}

// DataStreamManifest represents the structure of a data stream's manifest
type DataStreamManifest struct {
	Name          string         `config:"name" json:"name" yaml:"name"`
	Title         string         `config:"title" json:"title" yaml:"title"`
	Type          string         `config:"type" json:"type" yaml:"type"`
	Dataset       string         `config:"dataset" json:"dataset" yaml:"dataset"`
	Hidden        bool           `config:"hidden" json:"hidden" yaml:"hidden"`
	Release       string         `config:"release" json:"release" yaml:"release"`
	Elasticsearch *Elasticsearch `config:"elasticsearch" json:"elasticsearch" yaml:"elasticsearch"`
	Streams       []struct {
		Input string     `config:"input" json:"input" yaml:"input"`
		Vars  []Variable `config:"vars" json:"vars" yaml:"vars"`
	} `config:"streams" json:"streams" yaml:"streams"`
}

// MustFindPackageRoot finds and returns the path to the root folder of a package.
// It fails with an error if the package root can't be found.
func MustFindPackageRoot() (string, error) {
	root, found, err := FindPackageRoot()
	if err != nil {
		return "", fmt.Errorf("locating package root failed: %w", err)
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
		return "", false, fmt.Errorf("locating working directory failed: %w", err)
	}

	// VolumeName() will return something like "C:" in Windows, and "" in other OSs
	// rootDir will be something like "C:\" in Windows, and "/" everywhere else.
	rootDir := filepath.VolumeName(workDir) + string(filepath.Separator)

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, PackageManifestFile)
		fileInfo, err := os.Stat(path)
		if err == nil && !fileInfo.IsDir() {
			ok, err := isPackageManifest(path)
			if err != nil {
				return "", false, fmt.Errorf("verifying manifest file failed (path: %s): %w", path, err)
			}
			if ok {
				return dir, true, nil
			}
		}

		if dir == rootDir {
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
				return "", false, fmt.Errorf("verifying manifest file failed (path: %s): %w", path, err)
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

// ReadPackageManifestFromZipPackage reads and parses the package manifest file for the given zip package.
func ReadPackageManifestFromZipPackage(zipPackage string) (*PackageManifest, error) {
	tempDir, err := os.MkdirTemp("", "elastic-package-")
	if err != nil {
		return nil, fmt.Errorf("can't prepare a temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	contents, err := extractPackageManifestZipPackage(zipPackage, PackageManifestFile)
	if err != nil {
		return nil, fmt.Errorf("extracting manifest from zip file failed (path: %s): %w", zipPackage, err)
	}

	return ReadPackageManifestBytes(contents)
}

func extractPackageManifestZipPackage(zipPath, sourcePath string) ([]byte, error) {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zipReader.Close()

	// elastic-package build command creates a zip that contains all the package files
	// under a folder named "package-version". Example elastic_package_registry-0.0.6/manifest.yml
	matched, err := fs.Glob(zipReader, fmt.Sprintf("*/%s", sourcePath))
	if err != nil {
		return nil, err
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("not found package %s in %s", sourcePath, zipPath)
	}

	contents, err := fs.ReadFile(zipReader, matched[0])
	if err != nil {
		return nil, fmt.Errorf("can't read manifest from zip %s: %w", zipPath, err)
	}

	return contents, nil
}

// ReadPackageManifest reads and parses the given package manifest file.
func ReadPackageManifest(path string) (*PackageManifest, error) {
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("reading file failed (path: %s): %w", path, err)
	}

	var m PackageManifest
	err = cfg.Unpack(&m)
	if err != nil {
		return nil, fmt.Errorf("unpacking package manifest failed (path: %s): %w", path, err)
	}
	return &m, nil
}

func ReadPackageManifestBytes(contents []byte) (*PackageManifest, error) {
	cfg, err := yaml.NewConfig(contents, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("reading manifest file failed: %w", err)
	}

	var m PackageManifest
	err = cfg.Unpack(&m)
	if err != nil {
		return nil, fmt.Errorf("unpacking package manifest failed: %w", err)
	}
	return &m, nil
}

// ReadDataStreamManifest reads and parses the given data stream manifest file.
func ReadDataStreamManifest(path string) (*DataStreamManifest, error) {
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("reading file failed (path: %s): %w", path, err)
	}

	var m DataStreamManifest
	err = cfg.Unpack(&m)
	if err != nil {
		return nil, fmt.Errorf("unpacking data stream manifest failed (path: %s): %w", path, err)
	}

	m.Name = filepath.Base(filepath.Dir(path))
	return &m, nil
}

// ReadDataStreamManifestFromPackageRoot reads and parses the manifest of the given
// data stream from the given package root.
func ReadDataStreamManifestFromPackageRoot(packageRoot string, name string) (*DataStreamManifest, error) {
	return ReadDataStreamManifest(filepath.Join(packageRoot, "data_stream", name, DataStreamManifestFile))
}

// GetPipelineNameOrDefault returns the name of the data stream's pipeline, if one is explicitly defined in the
// data stream manifest. If not, the default pipeline name is returned.
func (dsm *DataStreamManifest) GetPipelineNameOrDefault() string {
	if dsm.Elasticsearch != nil && dsm.Elasticsearch.IndexTemplate != nil && dsm.Elasticsearch.IndexTemplate.IngestPipeline != nil && dsm.Elasticsearch.IndexTemplate.IngestPipeline.Name != "" {
		return dsm.Elasticsearch.IndexTemplate.IngestPipeline.Name
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
		return false, fmt.Errorf("reading package manifest failed (path: %s): %w", path, err)
	}
	return (m.Type == "integration" || m.Type == "input") && m.Version != "", nil
}

func isDataStreamManifest(path string) (bool, error) {
	m, err := ReadDataStreamManifest(path)
	if err != nil {
		return false, fmt.Errorf("reading package manifest failed (path: %s): %w", path, err)
	}
	return m.Title != "" &&
			(m.Type == dataStreamTypeLogs || m.Type == dataStreamTypeMetrics || m.Type == dataStreamTypeSynthetics || m.Type == dataStreamTypeTraces),
		nil
}
