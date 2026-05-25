// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// setRequiresDependencyVersion updates the version of a package listed under requires.input or requires.content.
// Only the matching version field line is changed; the rest of the file is left unchanged.
func setRequiresDependencyVersion(manifestBytes []byte, section, packageName, newVersion string) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(manifestBytes, &node); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}
	if len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("unexpected manifest content: not a map")
	}
	root := node.Content[0]

	requiresNode := findMapValueNode(root, "requires")
	if requiresNode == nil || requiresNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("manifest has no requires block")
	}

	sectionNode := findMapValueNode(requiresNode, section)
	if sectionNode == nil || sectionNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("manifest has no requires.%s block", section)
	}

	var versionNode *yaml.Node
	for _, item := range sectionNode.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		pkgNode := findMapValueNode(item, "package")
		if pkgNode == nil || pkgNode.Value != packageName {
			continue
		}
		versionNode = findMapValueNode(item, "version")
		if versionNode == nil {
			return nil, fmt.Errorf("requires.%s entry for package %q has no version", section, packageName)
		}
		break
	}
	if versionNode == nil {
		return nil, fmt.Errorf("package %q not found under requires.%s", packageName, section)
	}

	exactPin := section == string(ContentDependency)
	return replaceVersionLine(manifestBytes, versionNode, newVersion, exactPin)
}

func replaceVersionLine(manifestBytes []byte, versionNode *yaml.Node, newVersion string, exactPin bool) ([]byte, error) {
	if versionNode.Line <= 0 {
		return nil, errors.New("version field has no source line information")
	}

	lines := bytes.Split(manifestBytes, []byte("\n"))
	lineIdx := versionNode.Line - 1
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, fmt.Errorf("version field line %d is out of range", versionNode.Line)
	}

	line := string(lines[lineIdx])
	updated, err := replaceVersionOnLine(line, newVersion, exactPin)
	if err != nil {
		return nil, err
	}
	lines[lineIdx] = []byte(updated)

	return bytes.Join(lines, []byte("\n")), nil
}

// replaceVersionOnLine rewrites only the value of a "version:" key on a single line.
// When exactPin is true, the new value is written as a double-quoted exact semver pin.
func replaceVersionOnLine(line, newVersion string, exactPin bool) (string, error) {
	key := "version:"
	idx := strings.Index(line, key)
	if idx < 0 {
		return "", fmt.Errorf("line does not contain %q", key)
	}

	prefix := line[:idx+len(key)]
	remainder := line[idx+len(key):]

	comment := ""
	valuePart := remainder
	if hash := strings.Index(remainder, "#"); hash >= 0 {
		comment = remainder[hash:]
		valuePart = remainder[:hash]
	}

	leadingSpace := ""
	for len(valuePart) > 0 && (valuePart[0] == ' ' || valuePart[0] == '\t') {
		leadingSpace += string(valuePart[0])
		valuePart = valuePart[1:]
	}
	trailingSpace := ""
	for len(valuePart) > 0 {
		c := valuePart[len(valuePart)-1]
		if c != ' ' && c != '\t' {
			break
		}
		trailingSpace = string(c) + trailingSpace
		valuePart = valuePart[:len(valuePart)-1]
	}

	var newLiteral string
	if exactPin {
		newLiteral = formatExactVersionLiteral(newVersion)
	} else {
		newLiteral = formatVersionLiteral(strings.TrimSpace(valuePart), newVersion)
	}
	return prefix + leadingSpace + newLiteral + trailingSpace + comment, nil
}

func formatExactVersionLiteral(version string) string {
	return `"` + version + `"`
}

func formatVersionLiteral(oldLiteral, newVersion string) string {
	if len(oldLiteral) >= 2 {
		quote := oldLiteral[0]
		if (quote == '"' || quote == '\'') && oldLiteral[len(oldLiteral)-1] == quote {
			return string(quote) + newVersion + string(quote)
		}
	}
	return newVersion
}

func findMapValueNode(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Kind == yaml.ScalarNode && mapNode.Content[i].Value == key {
			if i+1 < len(mapNode.Content) {
				return mapNode.Content[i+1]
			}
			return nil
		}
	}
	return nil
}
