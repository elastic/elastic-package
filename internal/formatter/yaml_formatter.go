// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLFormatter is responsible for formatting the given YAML input.
type YAMLFormatter struct {
	keysWithDotsAction int
}

func NewYAMLFormatter(keysWithDotsAction int) *YAMLFormatter {
	return &YAMLFormatter{
		keysWithDotsAction: keysWithDotsAction,
	}
}

func (f *YAMLFormatter) Format(content []byte) ([]byte, bool, error) {
	// yaml.Unmarshal() requires `yaml.Node` to be passed instead of generic `interface{}`.
	// Otherwise it can't detect any comments and fields are considered as normal map.
	var node yaml.Node
	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, false, fmt.Errorf("unmarshalling YAML file failed: %w", err)
	}

	applyActionOnKeysWithDots(&node, f.keysWithDotsAction)

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

func applyActionOnKeysWithDots(node *yaml.Node, action int) {
	switch action {
	case KeysWithDotActionNested:
		extendNestedObjects(node)
	case KeysWithDotActionNone:
		// Nothing to do.
	}
}

func extendNestedObjects(node *yaml.Node) {
	if node.Kind == yaml.MappingNode {
		extendMapNode(node)
	}
	for _, child := range node.Content {
		extendNestedObjects(child)
	}
}

func extendMapNode(node *yaml.Node) {
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]

		base, rest, found := strings.Cut(key.Value, ".")

		// Insert nested objects only when the key has a dot, and is not quoted.
		if found && key.Style == 0 {
			// Copy key to create the new parent with the first part of the path.
			newKey := *key
			newKey.Value = base
			newKey.FootComment = ""
			newKey.HeadComment = ""
			newKey.LineComment = ""

			// Copy key also to create the key of the child value.
			newChildKey := *key
			newChildKey.Value = rest

			// Copy the parent node to create the nested object, that contains the new
			// child key and the original value.
			newNode := *node
			newNode.Content = []*yaml.Node{
				&newChildKey,
				value,
			}

			// Replace current key and value.
			node.Content[i] = &newKey
			node.Content[i+1] = &newNode
		}

		// Recurse on the current value.
		extendNestedObjects(node.Content[i+1])
	}

	mergeNodes(node)
}

// mergeNodes merges the contents of keys with the same name.
func mergeNodes(node *yaml.Node) {
	keys := make(map[string]*yaml.Node)
	k := 0
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]

		merged, found := keys[key.Value]
		if !found {
			keys[key.Value] = value
			node.Content[k] = key
			node.Content[k+1] = value
			k += 2
			continue
		}

		merged.Content = append(merged.Content, value.Content...)
	}

	node.Content = node.Content[:k]
}
