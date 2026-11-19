// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/elastic/go-resource"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/elastic/elastic-package/internal/yamledit"
)

// resourcesWithSettings returns the resources of a profile, with a configuration file that
// has the given settings. The content of the file in configPath is kept, if it exists.
func resourcesWithSettings(configPath string, settings map[string]string) ([]resource.Resource, error) {
	if len(settings) == 0 {
		return profileResources, nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to read profile configuration: %w", err)
	}
	content, err = configWithSettings(content, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to add settings to profile configuration (%s): %w", configPath, err)
	}

	return append(slices.Clone(profileResources), &resource.File{
		Path:    PackageProfileConfigFile,
		Content: resource.FileContentLiteral(string(content)),
	}), nil
}

// configWithSettings returns the content of a profile configuration file with the given
// settings. Settings are written as top-level keys in dotted notation, that take
// precedence over nested ones. The rest of the content is kept.
func configWithSettings(content []byte, settings map[string]string) ([]byte, error) {
	doc, err := yamledit.NewDocumentBytes(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// New settings are added as lines at the end, so the configuration has to be a single
	// block mapping, or have no content.
	var current []*ast.MappingValueNode
	docs := doc.AST().Docs
	if len(docs) > 1 {
		return nil, errors.New("multiple documents are not supported")
	}
	if len(docs) == 1 {
		switch root := docs[0].Body.(type) {
		case nil, *ast.CommentGroupNode:
		case *ast.MappingNode:
			if root.IsFlowStyle {
				return nil, errors.New("mappings in flow style are not supported at the top level")
			}
			current = root.Values
		default:
			return nil, fmt.Errorf("expected a mapping at the top level, found %s", root.Type())
		}
	}

	// Replace the values of the settings that are already in the configuration.
	missing := maps.Clone(settings)
	for _, setting := range current {
		key := setting.Key.GetToken().Value
		value, found := settings[key]
		if !found {
			continue
		}
		setting.Value, err = settingValueNode(value)
		if err != nil {
			return nil, fmt.Errorf("failed to encode value of setting %q: %w", key, err)
		}
		delete(missing, key)
	}

	var result bytes.Buffer
	if doc.Modified() {
		_, err := doc.Write(&result)
		if err != nil {
			return nil, fmt.Errorf("failed to encode configuration: %w", err)
		}
	} else {
		result.Write(content)
	}
	if result.Len() > 0 && !bytes.HasSuffix(result.Bytes(), []byte("\n")) {
		result.WriteString("\n")
	}

	// Add the rest of settings at the end.
	for _, key := range slices.Sorted(maps.Keys(missing)) {
		name, err := yaml.ValueToNode(key)
		if err != nil {
			return nil, fmt.Errorf("failed to encode name of setting %q: %w", key, err)
		}
		value, err := settingValueNode(missing[key])
		if err != nil {
			return nil, fmt.Errorf("failed to encode value of setting %q: %w", key, err)
		}
		fmt.Fprintf(&result, "%s: %s\n", name, value)
	}

	return result.Bytes(), nil
}

// settingValueNode returns the YAML node for the value of a setting. A value that starts
// as a list or a mapping in flow style, like [a, b] or {a: b}, is parsed as YAML. Any
// other value is a string.
func settingValueNode(value string) (ast.Node, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{") {
		return yaml.ValueToNode(value)
	}

	file, err := parser.ParseBytes([]byte(trimmed), 0)
	if err != nil {
		return nil, fmt.Errorf("invalid list or mapping: %w", err)
	}
	if len(file.Docs) != 1 {
		return nil, errors.New("invalid list or mapping: expected a single value")
	}
	switch node := file.Docs[0].Body.(type) {
	case *ast.SequenceNode, *ast.MappingNode:
		return node, nil
	default:
		return nil, errors.New("invalid list or mapping")
	}
}
