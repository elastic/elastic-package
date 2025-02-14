// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// Code based on the definition of Runtime Field in package-spec
// https://github.com/elastic/package-spec/blob/964c4a69e024cc464c4808720ba0db9f001a82a7/code/go/internal/validator/semantic/types.go#L26
type runtimeField struct {
	enabled bool
	script  string
}

// Ensure runtime implements these interfaces.
var (
	_ yaml.Unmarshaler = new(runtimeField)
)

func (r *runtimeField) IsEnabled() bool {
	if r.enabled {
		return true
	}
	if r.script != "" {
		return true
	}
	return false
}

func (r runtimeField) String() string {
	if r.script != "" {
		return r.script
	}
	return strconv.FormatBool(r.enabled)
}

func (r *runtimeField) unmarshalString(text string) error {
	value, err := strconv.ParseBool(text)
	if err == nil {
		r.enabled = value
		return nil
	}

	// JSONSchema already checks about the type of this field (e.g. int or float)
	r.enabled = true
	r.script = text
	return nil
}

// UnmarshalYAML implements the yaml.Marshaler interface for runtime.
func (r *runtimeField) UnmarshalYAML(value *yaml.Node) error {
	// For some reason go-yaml doesn't like the UnmarshalJSON function above.
	return r.unmarshalString(value.Value)
}
