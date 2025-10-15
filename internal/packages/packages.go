// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	yamlv3 "gopkg.in/yaml.v3"

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

// VarValueYamlString will return a YAML style string representation of vv,
// in the given YAML field, and with numSpaces indentation if it's a list.
func VarValueYamlString(vv VarValue, field string, numSpaces ...int) string {
	// Default indentation is 4 spaces
	n := 4
	if len(numSpaces) == 1 {
		n = numSpaces[0]
	}

	var valueToMarshal interface{}
	if vv.scalar != nil {
		valueToMarshal = vv.scalar
	} else if vv.list != nil {
		valueToMarshal = vv.list
	} else {
		return ""
	}

	// Use yaml.v3 encoder to ensure correct yaml string formatting
	data := map[string]interface{}{
		field: valueToMarshal,
	}

	var b strings.Builder
	encoder := yamlv3.NewEncoder(&b)
	encoder.SetIndent(n) // Apply the custom indentation.

	if err := encoder.Encode(&data); err != nil {
		return ""
	}

	return strings.TrimSpace(b.String())
}

// Variable is an instance of configuration variable (named, typed).
type Variable struct {
	Name                  string   `config:"name" json:"name" yaml:"name"`
	Type                  string   `config:"type" json:"type" yaml:"type"`
	Title                 string   `config:"title" json:"title" yaml:"title"`
	Description           string   `config:"description" json:"description" yaml:"description"`
	Multi                 bool     `config:"multi" json:"multi" yaml:"multi"`
	Required              bool     `config:"required" json:"required" yaml:"required"`
	Secret                bool     `config:"secret" json:"secret" yaml:"secret"`
	ShowUser              bool     `config:"show_user" json:"show_user" yaml:"show_user"`
	HideInDeploymentModes []string `config:"hide_in_deployment_modes" json:"hide_in_deployment_modes" yaml:"hide_in_deployment_modes"`
	UrlAllowedSchemes     []string `config:"url_allowed_schemes" json:"url_allowed_schemes" yaml:"url_allowed_schemes"`
	MinDuration           string   `config:"min_duration" json:"min_duration" yaml:"min_duration"`
	MaxDuration           string   `config:"max_duration" json:"max_duration" yaml:"max_duration"`
	Default               VarValue `config:"default" json:"default" yaml:"default"`
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

// Discovery define indications for the data this package can be useful with.
type Discovery struct {
	Fields []DiscoveryField `config:"fields" json:"fields" yaml:"fields"`
}

// DiscoveryField defines a field used for discovery.
type DiscoveryField struct {
	Name string `config:"name" json:"name" yaml:"name"`
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
	Type   string `config:"type" json:"type" yaml:"type"`
}

type Agent struct {
	Privileges struct {
		Root bool `config:"root" json:"root" yaml:"root"`
	} `config:"privileges" json:"privileges" yaml:"privileges"`
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
	Discovery       Discovery        `config:"discovery" json:"discovery" yaml:"discovery"`
	PolicyTemplates []PolicyTemplate `config:"policy_templates" json:"policy_templates" yaml:"policy_templates"`
	Vars            []Variable       `config:"vars" json:"vars" yaml:"vars"`
	Owner           Owner            `config:"owner" json:"owner" yaml:"owner"`
	Description     string           `config:"description" json:"description" yaml:"description"`
	License         string           `config:"license" json:"license" yaml:"license"`
	Categories      []string         `config:"categories" json:"categories" yaml:"categories"`
	Agent           Agent            `config:"agent" json:"agent" yaml:"agent"`
	Elasticsearch   *Elasticsearch   `config:"elasticsearch" json:"elasticsearch" yaml:"elasticsearch"`
}

type ManifestIndexTemplate struct {
	IngestPipeline *ManifestIngestPipeline `config:"ingest_pipeline" json:"ingest_pipeline" yaml:"ingest_pipeline"`
	Mappings       *ManifestMappings       `config:"mappings" json:"mappings" yaml:"mappings"`
}

type ManifestIngestPipeline struct {
	Name string `config:"name" json:"name" yaml:"name"`
}

type ManifestMappings struct {
	Subobjects bool `config:"subobjects" json:"subobjects" yaml:"subobjects"`
}

type Elasticsearch struct {
	IndexTemplate *ManifestIndexTemplate `config:"index_template" json:"index_template" yaml:"index_template"`
	SourceMode    string                 `config:"source_mode" json:"source_mode" yaml:"source_mode"`
	IndexMode     string                 `config:"index_mode" json:"index_mode" yaml:"index_mode"`
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
	Streams       []Stream       `config:"streams" json:"streams" yaml:"streams"`
	Agent         Agent          `config:"agent" json:"agent" yaml:"agent"`
}

// Transform contains information about a transform included in a package.
type Transform struct {
	Name       string
	Path       string
	Definition TransformDefinition
}

// TransformDefinition is the definition of an Elasticsearch transform
type TransformDefinition struct {
	Source struct {
		Index []string `config:"index" yaml:"index"`
	} `config:"source" yaml:"source"`
	Dest struct {
		Pipeline string `config:"pipeline" yaml:"pipeline"`
	} `config:"dest" yaml:"dest"`
	Meta struct {
		FleetTransformVersion string `config:"fleet_transform_version" yaml:"fleet_transform_version"`
	} `config:"_meta" yaml:"_meta"`
}

// Stream contains information about an input stream.
type Stream struct {
	Input        string     `config:"input" json:"input" yaml:"input"`
	Title        string     `config:"title" json:"title" yaml:"title"`
	Description  string     `config:"description" json:"description" yaml:"description"`
	TemplatePath string     `config:"template_path" json:"template_path" yaml:"template_path"`
	Vars         []Variable `config:"vars" json:"vars" yaml:"vars"`
}

// HasSource checks if a given index or data stream name maches the transform sources
func (t *Transform) HasSource(name string) (bool, error) {
	for _, indexPattern := range t.Definition.Source.Index {
		// Split the pattern by commas in case the source indexes are provided with a
		// comma-separated index strings
		patterns := strings.Split(indexPattern, ",")
		for _, pattern := range patterns {
			// Using filepath.Match to match index patterns because the syntax
			// is basically the same.
			found, err := filepath.Match(pattern, name)
			if err != nil {
				return false, fmt.Errorf("maching pattern %q with %q: %w", pattern, name, err)
			}
			if found {
				return true, nil
			}
		}
	}
	return false, nil
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

// FindPackageRoot finds and returns the path to the root folder of a package from the working directory.
func FindPackageRoot() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("locating working directory failed: %w", err)
	}
	return FindPackageRootFrom(workDir)
}

// FindPackageRootFrom finds and returns the path to the root folder of a package from a given directory.
func FindPackageRootFrom(fromDir string) (string, bool, error) {
	// VolumeName() will return something like "C:" in Windows, and "" in other OSs
	// rootDir will be something like "C:\" in Windows, and "/" everywhere else.
	rootDir := filepath.VolumeName(fromDir) + string(filepath.Separator)

	dir := fromDir
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

// ReadTransformDefinitionFile reads and parses the transform definition (elasticsearch/transform/<name>/transform.yml)
// file for the given transform. It also applies templating to the file, allowing to set the final ingest pipeline name
// by adding the package version defined in the package manifest.
// It fails if the referenced destination pipeline doesn't exist.
func ReadTransformDefinitionFile(transformPath, packageRootPath string) ([]byte, TransformDefinition, error) {
	manifest, err := ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, TransformDefinition{}, fmt.Errorf("could not read package manifest: %w", err)
	}

	if manifest.Version == "" {
		return nil, TransformDefinition{}, fmt.Errorf("package version is not defined in the package manifest")
	}

	t, err := template.New(filepath.Base(transformPath)).Funcs(template.FuncMap{
		"ingestPipelineName": func(pipelineName string) (string, error) {
			if pipelineName == "" {
				return "", fmt.Errorf("ingest pipeline name is empty")
			}
			return fmt.Sprintf("%s-%s", manifest.Version, pipelineName), nil
		},
	}).ParseFiles(transformPath)
	if err != nil {
		return nil, TransformDefinition{}, fmt.Errorf("parsing transform template failed (path: %s): %w", transformPath, err)
	}

	var rendered bytes.Buffer
	err = t.Execute(&rendered, nil)
	if err != nil {
		return nil, TransformDefinition{}, fmt.Errorf("executing template failed: %w", err)
	}
	cfg, err := yaml.NewConfig(rendered.Bytes(), ucfg.PathSep("."))
	if err != nil {
		return nil, TransformDefinition{}, fmt.Errorf("reading file failed (path: %s): %w", transformPath, err)
	}

	var definition TransformDefinition
	err = cfg.Unpack(&definition)
	if err != nil {
		return nil, TransformDefinition{}, fmt.Errorf("failed to parse transform file \"%s\": %w", transformPath, err)
	}

	if definition.Dest.Pipeline == "" {
		return rendered.Bytes(), definition, nil
	}

	// Is it using the Ingest pipeline defined in the package (elasticsearch/ingest_pipeline/<version>-<pipeline>.yml)?
	// <version>-<pipeline>.yml
	// example: 0.1.0-pipeline_extract_metadata

	pipelineFileName := fmt.Sprintf("%s.yml", strings.TrimPrefix(definition.Dest.Pipeline, manifest.Version+"-"))
	_, err = os.Stat(filepath.Join(packageRootPath, "elasticsearch", "ingest_pipeline", pipelineFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, TransformDefinition{}, fmt.Errorf("checking for destination ingest pipeline file %s: %w", pipelineFileName, err)
	}
	if err == nil {
		return rendered.Bytes(), definition, nil
	}

	// Is it using the Ingest pipeline from any data stream (data_stream/*/elasticsearch/pipeline/*.yml)?
	// <data_stream>-<version>-<data_stream_pipeline>.yml
	// example: metrics-aws_billing.cur-0.1.0-pipeline_extract_metadata
	dataStreamPaths, err := filepath.Glob(filepath.Join(packageRootPath, "data_stream", "*"))
	if err != nil {
		return nil, TransformDefinition{}, fmt.Errorf("error finding data streams: %w", err)
	}

	for _, dataStreamPath := range dataStreamPaths {
		matched, err := filepath.Glob(filepath.Join(dataStreamPath, "elasticsearch", "ingest_pipeline", "*.yml"))
		if err != nil {
			return nil, TransformDefinition{}, fmt.Errorf("error finding ingest pipelines in data stream %s: %w", dataStreamPath, err)
		}
		dataStreamName := filepath.Base(dataStreamPath)
		for _, pipelinePath := range matched {
			dataStreamPipelineName := strings.TrimSuffix(filepath.Base(pipelinePath), filepath.Ext(pipelinePath))
			expectedSuffix := fmt.Sprintf("-%s.%s-%s-%s.yml", manifest.Name, dataStreamName, manifest.Version, dataStreamPipelineName)
			if strings.HasSuffix(pipelineFileName, expectedSuffix) {
				return rendered.Bytes(), definition, nil
			}
		}
	}
	pipelinePaths, err := filepath.Glob(filepath.Join(packageRootPath, "data_stream", "*", "elasticsearch", "ingest_pipeline", "*.yml"))
	if err != nil {
		return nil, TransformDefinition{}, fmt.Errorf("error finding ingest pipelines in data streams: %w", err)
	}
	for _, pipelinePath := range pipelinePaths {
		dataStreamPipelineName := strings.TrimSuffix(filepath.Base(pipelinePath), filepath.Ext(pipelinePath))
		if strings.HasSuffix(pipelineFileName, fmt.Sprintf("-%s-%s.yml", manifest.Version, dataStreamPipelineName)) {
			return rendered.Bytes(), definition, nil
		}
	}

	return nil, TransformDefinition{}, fmt.Errorf("destination ingest pipeline file %s not found: incorrect version used in pipeline or unknown pipeline", pipelineFileName)
}

// ReadTransformsFromPackageRoot looks for transforms in the given package root.
func ReadTransformsFromPackageRoot(packageRoot string) ([]Transform, error) {
	files, err := filepath.Glob(filepath.Join(packageRoot, "elasticsearch", "transform", "*", "transform.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed matching files with transform definitions: %w", err)
	}

	var transforms []Transform
	for _, file := range files {
		_, definition, err := ReadTransformDefinitionFile(file, packageRoot)
		if err != nil {
			return nil, fmt.Errorf("failed reading transform definition file %q: %w", file, err)
		}

		transforms = append(transforms, Transform{
			Name:       filepath.Base(filepath.Dir(file)),
			Path:       file,
			Definition: definition,
		})
	}

	return transforms, nil
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
	supportedTypes := []string{
		"content",
		"input",
		"integration",
	}
	return slices.Contains(supportedTypes, m.Type) && m.Version != "", nil
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
