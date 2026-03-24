// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
	yamlv3 "gopkg.in/yaml.v3"
)

var (
	// Matches "field <path>:" at the start of a message.
	// Examples: "field processors.4:", "field title:", "field vars.1.name:"
	fieldPathRe = regexp.MustCompile(`^field ([a-zA-Z0-9_.]+):`)

	// Matches "Additional property X is not allowed"
	additionalPropRe = regexp.MustCompile(`Additional property (\S+) is not allowed`)
)

// findPosition tries to locate the line in a YAML file corresponding to the
// error message. Returns a range at line 0 if the position cannot be determined.
func findPosition(filePath, message string) protocol.Range {
	zero := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}

	m := fieldPathRe.FindStringSubmatch(message)
	if m == nil {
		return zero
	}
	fieldPath := m[1]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return zero
	}

	var doc yamlv3.Node
	if err := yamlv3.Unmarshal(data, &doc); err != nil {
		return zero
	}

	// The document node wraps the actual content.
	if doc.Kind != yamlv3.DocumentNode || len(doc.Content) == 0 {
		return zero
	}
	root := doc.Content[0]

	// Walk the field path (e.g. "processors.4.grokk" or "processors.4").
	segments := strings.Split(fieldPath, ".")

	// If the message mentions an additional property, append it to the path
	// so we land on the exact key.
	if pm := additionalPropRe.FindStringSubmatch(message); pm != nil {
		segments = append(segments, pm[1])
	}

	node := walkYAMLPath(root, segments)
	if node == nil {
		return zero
	}

	// yaml.Node lines are 1-based, LSP positions are 0-based.
	line := uint32(node.Line - 1)
	col := uint32(node.Column - 1)
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: col},
		End:   protocol.Position{Line: line, Character: col + uint32(len(node.Value))},
	}
}

// walkYAMLPath navigates a yaml.Node tree following the given path segments.
// Segments can be map keys ("processors") or array indices ("4").
func walkYAMLPath(node *yamlv3.Node, segments []string) *yamlv3.Node {
	current := node
	for i, seg := range segments {
		if current == nil {
			return nil
		}
		isLast := i == len(segments)-1

		switch current.Kind {
		case yamlv3.MappingNode:
			// Content alternates: key, value, key, value, ...
			found := false
			for j := 0; j+1 < len(current.Content); j += 2 {
				if current.Content[j].Value == seg {
					if isLast {
						// Return the key node for its position.
						return current.Content[j]
					}
					current = current.Content[j+1]
					found = true
					break
				}
			}
			if !found {
				return nil
			}

		case yamlv3.SequenceNode:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(current.Content) {
				return nil
			}
			current = current.Content[idx]

		default:
			return nil
		}
	}
	return current
}
