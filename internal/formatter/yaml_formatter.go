// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"bytes"

	"gopkg.in/yaml.v3"

	"github.com/pkg/errors"
)

// YAMLFormatter function is responsible for formatting the given YAML input.
// The function is exposed, so it can be used by other internal packages.
func YAMLFormatter(content []byte) ([]byte, bool, error) {
	// yaml.Unmarshal() requires `yaml.Node` to be passed instead of generic `interface{}`.
	// Otherwise it can detect any comments and fields are considered as normal map.
	var node yaml.Node
	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, false, errors.Wrap(err, "unmarshalling YAML file failed")
	}

	var b bytes.Buffer
	encoder := yaml.NewEncoder(&b)
	encoder.SetIndent(2)
	err = encoder.Encode(&node)
	if err != nil {
		return nil, false, errors.Wrap(err, "marshalling YAML node failed")
	}
	formatted := b.Bytes()
	return formatted, string(content) == string(formatted), nil
}
