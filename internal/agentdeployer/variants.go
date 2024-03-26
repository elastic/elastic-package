// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// VariantsFile describes different variants of the service under test.
type VariantsFile struct {
	Default  string `yaml:"default"`
	Variants map[string]Environment
}

// Environment is a key-value map storing environment variables.
type Environment map[string]string

// AgentVariant describes a variant of the service using Environment variables.
type AgentVariant struct {
	Name string
	Env  []string // Environment variables in format of pairs: key=value
}

// String method returns a string representation of the service variant.
func (sv *AgentVariant) String() string {
	return fmt.Sprintf("AgentVariant{Name: %s, Env: %s}", sv.Name, strings.Join(sv.Env, ","))
}

func (sv *AgentVariant) active() bool {
	return sv.Name != ""
}

// ReadVariantsFile function reads available service variants.
func ReadVariantsFile(devDeployPath string) (*VariantsFile, error) {
	variantsYmlPath := filepath.Join(devDeployPath, "variants.yml")
	_, err := os.Stat(variantsYmlPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("can't stat variants file: %w", err)
	}

	content, err := os.ReadFile(variantsYmlPath)
	if err != nil {
		return nil, fmt.Errorf("can't read variants file: %w", err)
	}

	var f VariantsFile
	err = yaml.Unmarshal(content, &f)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal variants file: %w", err)
	}
	return &f, nil
}

func useAgentVariant(devDeployPath, selected string) (AgentVariant, error) {
	f, err := ReadVariantsFile(devDeployPath)
	if errors.Is(err, os.ErrNotExist) {
		return AgentVariant{}, nil // no "variants.yml" present
	} else if err != nil {
		return AgentVariant{}, err
	}

	if selected == "" {
		selected = f.Default
	}

	if f.Default == "" {
		return AgentVariant{}, errors.New("default variant is undefined")
	}

	env, ok := f.Variants[selected]
	if !ok {
		return AgentVariant{}, fmt.Errorf(`variant "%s" is missing`, selected)
	}

	return AgentVariant{
		Name: selected,
		Env:  asEnvVarPairs(env),
	}, nil
}

func asEnvVarPairs(env Environment) []string {
	var pairs []string
	for k, v := range env {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return pairs
}
