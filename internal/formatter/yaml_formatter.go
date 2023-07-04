// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// YAMLFormatter function is responsible for formatting the given YAML input.
// The function is exposed, so it can be used by other internal packages.
func YAMLFormatter(content []byte) ([]byte, bool, error) {
	// yaml.Unmarshal() requires `yaml.Node` to be passed instead of generic `interface{}`.
	// Otherwise it can't detect any comments and fields are considered as normal map.
	var node yaml.Node
	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, false, fmt.Errorf("unmarshalling YAML file failed: %w", err)
	}

	var b bytes.Buffer
	encoder := yaml.NewEncoder(&b)
	encoder.SetIndent(2)
	err = encoder.Encode(&node)
	if err != nil {
		return nil, false, fmt.Errorf("marshalling YAML node failed: %w", err)
	}
	formatted := b.Bytes()

	prefix := []byte("---\n")
	// required to preserve yaml files starting with "---" as yaml.Encoding strips them
	if bytes.HasPrefix(content, prefix) && !bytes.HasPrefix(formatted, prefix) {
		formatted = append(prefix, formatted...)
	}

	return formatted, string(content) == string(formatted), nil
}
