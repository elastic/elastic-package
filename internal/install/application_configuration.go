// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

type ApplicationConfiguration struct{}

// StackImageRefs function selects the appropriate set of Docker image references for the default stack version.
func (ac *ApplicationConfiguration) DefaultStackImageRefs() (*ImageRefs, error) {
	return ac.StackImageRefs(DefaultStackVersion)
}

// StackImageRefs function selects the appropriate set of Docker image references for the given stack version.
func (ac *ApplicationConfiguration) StackImageRefs(version string) (*ImageRefs, error) {
	panic("TODO")
}

type configFile struct {
	Stack stack `yaml:"stack"`
}

type stack struct {
	ImageRefOverrides map[string]ImageRefs `yaml:"imageRefOverrides"`
}

// ImageRefs stores Docker image references used to create the Elastic stack containers.
type ImageRefs struct {
	ElasticAgent  string `yaml:"elastic-agent"`
	Elasticsearch string `yaml:"elasticsearch"`
	Kibana        string `yaml:"kibana"`
}

// AsEnv method returns key=value representation of image refs.
func (ir *ImageRefs) AsEnv() []string {
	var vars []string
	vars = append(vars, "ELASTIC_AGENT_IMAGE_REF="+ir.ElasticAgent)
	vars = append(vars, "ELASTICSEARCH_IMAGE_REF="+ir.Elasticsearch)
	vars = append(vars, "KIBANA_IMAGE_REF="+ir.Kibana)
	return vars
}

// Configuration function returns the elastic-package configuration.
func Configuration() (*ApplicationConfiguration, error) {
	return &ApplicationConfiguration{}, nil
}
