// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	// PackageManifestFile is the name of the package's main manifest file.
	PackageManifestFile = "manifest.yml"

	// DataStreamManifestFile is the name of the data stream's manifest file.
	DataStreamManifestFile = "manifest.yml"
)

// VarValue represents a Variable value as defined in a package or data stream
// manifest file.
type VarValue struct {
	scalar string
	list   []string
}

// UnmarshalYAML knows how to parse a Variable value from a package or data stream
// manifest file into a VarValue.
func (vv *VarValue) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		vv.scalar = value.Value
	case yaml.SequenceNode:
		vv.list = make([]string, len(value.Content))
		for idx, content := range value.Content {
			vv.list[idx] = content.Value
		}
	default:
		return errors.New("unknown Variable value")
	}
	return nil
}

// MarshalJSON knows how to serialize a VarValue into the appropriate
// JSON data type and value.
func (vv VarValue) MarshalJSON() ([]byte, error) {
	if vv.scalar != "" {
		return json.Marshal(vv.scalar)
	} else if vv.list != nil {
		return json.Marshal(vv.list)
	}
	return nil, nil
}

// Variable is an instance of configuration variable (named, typed).
type Variable struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Default VarValue `json:"default"`
}

// Input is a single input configuration.
type Input struct {
	Type string     `json:"type"`
	Vars []Variable `json:"vars"`
}

// PolicyTemplate is a configuration of inputs responsible for collecting log or metric data.
type PolicyTemplate struct {
	Inputs []Input `json:"inputs"`
}

// PackageManifest represents the basic structure of a package's manifest
type PackageManifest struct {
	Name            string           `json:"name"`
	Title           string           `json:"title"`
	Type            string           `json:"type"`
	Version         string           `json:"version"`
	PolicyTemplates []PolicyTemplate `json:"policy_templates" yaml:"policy_templates"`
}

// DataStreamManifest represents the structure of a data stream's manifest
type DataStreamManifest struct {
	Name          string `json:"name" yaml:"name"`
	Title         string `json:"title" yaml:"title"`
	Type          string `json:"type" yaml:"type"`
	Elasticsearch *struct {
		IngestPipeline *struct {
			Name string `json:"name"`
		} `json:"ingest_pipeline"`
	} `json:"elasticsearch"`
	Streams []struct {
		Input string     `json:"input"`
		Vars  []Variable `json:"vars"`
	} `json:"streams"`
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

// ReadPackageManifest reads and parses the given package manifest file.
func ReadPackageManifest(path string) (*PackageManifest, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading file body failed (path: %s)", path)
	}

	var m PackageManifest
	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling package manifest failed (path: %s)", path)
	}
	return &m, nil
}

// ReadDataStreamManifest reads and parses the given data stream manifest file.
func ReadDataStreamManifest(path string) (*DataStreamManifest, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading file body failed (path: %s)", path)
	}

	var m DataStreamManifest
	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling data stream manifest failed (path: %s)", path)
	}

	m.Name = filepath.Base(filepath.Dir(path))
	return &m, nil
}

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
	return m.Title != "" && (m.Type == "logs" || m.Type == "metrics"), nil
}
