// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import "gopkg.in/yaml.v3"

const defaultECSSchemaBaseURL = "https://raw.githubusercontent.com/elastic/ecs"

type SchemaURLs struct {
	ecsBase string `yaml:"ecs_base,omitempty"`
}

func (s *SchemaURLs) UnmarshalYAML(value *yaml.Node) error {
	type tmpSchemaURLs struct {
		ECSBase string `yaml:"ecs_base,omitempty"`
	}

	var tmp tmpSchemaURLs
	if err := value.Decode(&tmp); err != nil {
		return err
	}

	// If not set in the YAML, set to the default value in the struct
	s.ecsBase = tmp.ECSBase
	if tmp.ECSBase == "" {
		s.ecsBase = defaultECSSchemaBaseURL
	}

	return nil
}

func (s SchemaURLs) MarshalYAML() (interface{}, error) {
	type tmpSchemaURLs struct {
		ECSBase string `yaml:"ecs_base,omitempty"`
	}

	// Ensure that empty value is not marshaled, use default instead
	value := s.ecsBase
	if s.ecsBase == "" {
		value = defaultECSSchemaBaseURL
	}
	return tmpSchemaURLs{
		ECSBase: value,
	}, nil
}

type schemaURLOption func(*SchemaURLs)

func WithECSBaseURL(v string) schemaURLOption {
	return func(s *SchemaURLs) {
		s.ecsBase = v
	}
}

func NewSchemaURLs(opts ...schemaURLOption) SchemaURLs {
	s := SchemaURLs{}
	for _, opt := range opts {
		opt(&s)
	}
	// Ensure that ecsBase is set to default if not provided
	if s.ecsBase == "" {
		s.ecsBase = defaultECSSchemaBaseURL
	}
	return s
}

func (s SchemaURLs) ECSBase() string {
	// Safe return default if empty
	if s.ecsBase == "" {
		return defaultECSSchemaBaseURL
	}
	return s.ecsBase
}
