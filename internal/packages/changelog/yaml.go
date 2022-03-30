// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package changelog

import (
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
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

		var newNode yaml.Node
		if patchVersion.Equal(foundVersion) {
			// Add the change to current entry.
			fmt.Println("Adding changelog entry in version", foundVersion)
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
			setYamlMapValueVersionStyle(&newNode, "version", yaml.DoubleQuotedStyle)
			result = append(result, newNode)
			patched = true
			continue
		}

		// Add the change before first entry
		fmt.Println("Adding changelog entry before version", foundVersion)
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
		setYamlMapValueVersionStyle(&newNode, "version", yaml.DoubleQuotedStyle)
		result = append(result, newNode, node)
		patched = true
	}

	if !patched {
		return nil, errors.New("changelog entry was not added, this is probably a bug")
	}

	d, err = yaml.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode resulting changelog")
	}

	return d, nil
}

// setYamlMapValueVersionStyle changes the style of one value in a YAML map. If the key
// is not found, it does nothing.
func setYamlMapValueVersionStyle(node *yaml.Node, key string, style yaml.Style) {
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
