// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package changelog

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/formatter"
)

// PatchYAML looks for the proper place to add the new revision in the changelog,
// trying to conserve original format and comments.
func PatchYAML(d []byte, patch Revision) ([]byte, error) {
	var nodes []yaml.Node
	err := yaml.Unmarshal(d, &nodes)
	if err != nil {
		return nil, err
	}

	patchVersion, err := semver.NewVersion(patch.Version)
	if err != nil {
		return nil, err
	}

	patched := false
	var result []yaml.Node
	for _, node := range nodes {
		if patched {
			result = append(result, node)
			continue
		}

		var entry Revision
		err := node.Decode(&entry)
		if err != nil {
			result = append(result, node)
			continue
		}

		foundVersion, err := semver.NewVersion(entry.Version)
		if err != nil {
			return nil, err
		}

		if foundVersion.GreaterThan(patchVersion) {
			return nil, errors.New("cannot add change to old version")
		}

		var newNode yaml.Node
		if patchVersion.Equal(foundVersion) {
			// Add the change to current entry.
			entry.Changes = append(patch.Changes, entry.Changes...)
			err := newNode.Encode(entry)
			if err != nil {
				return nil, err
			}
			// Keep comments of the original node.
			newNode.HeadComment = node.HeadComment
			newNode.LineComment = node.LineComment
			newNode.FootComment = node.FootComment
			// Quote version to keep common style.
			setYamlMapValueStyle(&newNode, "version", yaml.DoubleQuotedStyle)
			result = append(result, newNode)
			patched = true
			continue
		}

		// Add the change before first entry
		err = newNode.Encode(patch)
		if err != nil {
			return nil, err
		}
		// If there is a comment on top, leave it there.
		if node.HeadComment != "" {
			newNode.HeadComment = node.HeadComment
			node.HeadComment = ""
		}
		// Quote version to keep common style.
		setYamlMapValueStyle(&newNode, "version", yaml.DoubleQuotedStyle)
		result = append(result, newNode, node)
		patched = true
	}

	if !patched {
		return nil, errors.New("changelog entry was not added, this is probably a bug")
	}

	d, err = formatResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to format manifest: %w", err)
	}
	return d, nil
}

func SetManifestVersion(d []byte, version string) ([]byte, error) {
	var node yaml.Node
	err := yaml.Unmarshal(d, &node)
	if err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	// Manifest is a document, with a single element, that should be a map.
	if len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("unexpected manifest content: not a map")
	}

	setYamlMapValue(node.Content[0], "version", version)

	d, err = formatResult(&node)
	if err != nil {
		return nil, fmt.Errorf("failed to format manifest: %w", err)
	}
	return d, nil
}

func formatResult(result interface{}) ([]byte, error) {
	d, err := yaml.Marshal(result)
	if err != nil {
		return nil, errors.New("failed to encode")
	}
	yamlFormatter := &formatter.YAMLFormatter{}
	d, _, err = yamlFormatter.Format(d)
	if err != nil {
		return nil, errors.New("failed to format")
	}
	return d, nil
}

// setYamlMapValueStyle changes the style of one value in a YAML map. If the key
// is not found, it does nothing.
func setYamlMapValueStyle(node *yaml.Node, key string, style yaml.Style) {
	// Check first if this is a map.
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	// Look for the key, the value will be the next one.
	var keyIdx int
	for keyIdx = range node.Content {
		child := node.Content[keyIdx]
		if child.Kind == yaml.ScalarNode && child.Value == key {
			break
		}
	}
	valueIdx := keyIdx + 1
	if valueIdx < len(node.Content) {
		node.Content[valueIdx].Style = style
	}
}

// setYamlMapValue sets a value in a map.
func setYamlMapValue(node *yaml.Node, key string, value string) {
	// Check first if this is a map.
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	// Look for the key, the value will be the next one.
	var keyIdx int
	for keyIdx = range node.Content {
		child := node.Content[keyIdx]
		if child.Kind == yaml.ScalarNode && child.Value == key {
			break
		}
	}
	valueIdx := keyIdx + 1
	if valueIdx < len(node.Content) {
		node.Content[valueIdx].Value = value
	}
}
