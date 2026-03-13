// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/formatter"
)

// mappingValue returns the value node for the given key in a YAML mapping node,
// or nil if the key is not present.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	idx := slices.IndexFunc(node.Content, func(n *yaml.Node) bool {
		return n.Value == key
	})
	if idx < 0 || idx+1 >= len(node.Content) {
		return nil
	}
	return node.Content[idx+1]
}

// removeKey removes a key-value pair from a YAML mapping node.
func removeKey(node *yaml.Node, key string) {
	idx := slices.IndexFunc(node.Content, func(n *yaml.Node) bool {
		return n.Value == key
	})
	if idx >= 0 && idx+1 < len(node.Content) {
		node.Content = slices.Delete(node.Content, idx, idx+2)
	}
}

// upsertKey sets key to value in a YAML mapping node, adding it if absent.
func upsertKey(node *yaml.Node, key string, value *yaml.Node) {
	idx := slices.IndexFunc(node.Content, func(n *yaml.Node) bool {
		return n.Value == key
	})
	if idx >= 0 && idx+1 < len(node.Content) {
		node.Content[idx+1] = value
		return
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	node.Content = append(node.Content, keyNode, value)
}

func formatYAMLNode(doc *yaml.Node) ([]byte, error) {
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshalling YAML: %w", err)
	}
	yamlFormatter := formatter.NewYAMLFormatter(formatter.KeysWithDotActionNone)
	formatted, _, err := yamlFormatter.Format(raw)
	if err != nil {
		return nil, fmt.Errorf("formatting YAML: %w", err)
	}
	return formatted, nil
}
