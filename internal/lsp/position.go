// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
	yamlv3 "gopkg.in/yaml.v3"
)

var (
	// Matches "field <path>:" at the start of a message.
	// Examples: "field processors.4:", "field title:", "field (root):"
	fieldPathRe = regexp.MustCompile(`^field ([^:]+):`)

	// Matches "Additional property X is not allowed"
	additionalPropRe = regexp.MustCompile(`Additional property (\S+) is not allowed`)

	// Matches messages like "set processor at line 3 missing required tag"
	lineNumberRe = regexp.MustCompile(`\bat line (\d+)\b`)

	// Matches "dangling reference found: object-id (search)"
	danglingReferenceRe = regexp.MustCompile(`^dangling reference found: ([^ ]+) \(([^)]+)\)`)

	// Matches "reference found in dashboard: object-id (search)"
	dashboardReferenceMessageRe = regexp.MustCompile(`^references? found in dashboard: ([^ ]+) \(([^)]+)\)`)

	// Matches legacy visualization messages on dashboard files.
	dashboardLegacyVisualizationRe = regexp.MustCompile(`^"([^"]+)" contains legacy visualization: "([^"]*)" \(([^,]+), ([^)]+)\)$`)

	// Matches legacy visualization messages on visualization files.
	legacyVisualizationRe = regexp.MustCompile(`^found legacy visualization "([^"]*)" \(([^,]+), ([^)]+)\)$`)
)

// findPosition tries to locate the line in a YAML file corresponding to the
// error message. Returns a range at line 0 if the position cannot be determined.
func findPosition(filePath, message string) protocol.Range {
	return findPositionInFS("", filePath, nil, message)
}

func findPositionInFS(packageRoot, filePath string, fsys fs.FS, message string) protocol.Range {
	zero := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}

	data, err := readDiagnosticFile(packageRoot, filePath, fsys)
	if err != nil {
		return zero
	}
	text := string(data)

	if location, ok := findLineNumberPosition(text, message); ok {
		return location
	}

	if location, ok := findJSONPosition(filePath, text, message); ok {
		return location
	}

	fieldPath, ok := extractFieldPath(message)
	if !ok {
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

	node := resolveDiagnosticNode(root, fieldPath, message)
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

func extractFieldPath(message string) (string, bool) {
	match := fieldPathRe.FindStringSubmatch(message)
	if match == nil {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func findLineNumberPosition(text, message string) (protocol.Range, bool) {
	match := lineNumberRe.FindStringSubmatch(message)
	if match == nil {
		return protocol.Range{}, false
	}

	lineNumber, err := strconv.Atoi(match[1])
	if err != nil || lineNumber <= 0 {
		return protocol.Range{}, false
	}

	return rangeForLine(text, lineNumber-1), true
}

func findJSONPosition(filePath, text, message string) (protocol.Range, bool) {
	if !strings.HasSuffix(filePath, ".json") {
		return protocol.Range{}, false
	}

	if match := danglingReferenceRe.FindStringSubmatch(message); match != nil {
		return findJSONPropertyValueRange(text, "id", match[1])
	}

	if match := dashboardReferenceMessageRe.FindStringSubmatch(message); match != nil {
		return findJSONPropertyValueRange(text, "id", match[1])
	}

	if match := dashboardLegacyVisualizationRe.FindStringSubmatch(message); match != nil {
		if match[2] != "" {
			if location, ok := findJSONPropertyValueRange(text, "title", match[2]); ok {
				return location, true
			}
		}
		if location, ok := findJSONPropertyValueRange(text, "title", match[1]); ok {
			return location, true
		}
		return findJSONPropertyValueRange(text, "type", match[3])
	}

	if match := legacyVisualizationRe.FindStringSubmatch(message); match != nil {
		if match[1] != "" {
			if location, ok := findJSONPropertyValueRange(text, "title", match[1]); ok {
				return location, true
			}
		}
		return findJSONPropertyValueRange(text, "type", match[2])
	}

	return protocol.Range{}, false
}

func resolveDiagnosticNode(root *yamlv3.Node, fieldPath, message string) *yamlv3.Node {
	current := root
	if fieldPath != "" && fieldPath != "(root)" {
		if pm := additionalPropRe.FindStringSubmatch(message); pm != nil {
			current = walkYAMLValuePath(root, strings.Split(fieldPath, "."))
		} else {
			current = walkYAMLPath(root, strings.Split(fieldPath, "."))
		}
		if current == nil {
			return nil
		}
	}

	if pm := additionalPropRe.FindStringSubmatch(message); pm != nil {
		return walkYAMLPath(current, strings.Split(pm[1], "."))
	}

	return current
}

func readDiagnosticFile(packageRoot, filePath string, fsys fs.FS) ([]byte, error) {
	if fsys != nil {
		if relPath, ok := relativeFSPath(packageRoot, filePath); ok {
			data, err := fs.ReadFile(fsys, relPath)
			if err == nil {
				return data, nil
			}
		}
	}

	return os.ReadFile(filePath)
}

func findJSONPropertyValueRange(text, key, value string) (protocol.Range, bool) {
	lines := splitLines(text)
	quotedValue := strconv.Quote(value)

	for lineIndex, line := range lines {
		if !strings.Contains(line, quotedValue) {
			continue
		}
		if key != "" && !strings.Contains(line, `"`+key+`"`) {
			continue
		}

		start := strings.Index(line, quotedValue)
		if start < 0 {
			continue
		}

		return protocol.Range{
			Start: protocol.Position{Line: uint32(lineIndex), Character: uint32(start + 1)},
			End:   protocol.Position{Line: uint32(lineIndex), Character: uint32(start + 1 + len(value))},
		}, true
	}

	return protocol.Range{}, false
}

func rangeForLine(text string, lineNumber int) protocol.Range {
	lines := splitLines(text)
	if lineNumber < 0 || lineNumber >= len(lines) {
		return protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		}
	}

	line := lines[lineNumber]
	start := len(line) - len(strings.TrimLeft(line, " \t"))
	end := len(line)
	if end < start {
		end = start
	}

	return protocol.Range{
		Start: protocol.Position{Line: uint32(lineNumber), Character: uint32(start)},
		End:   protocol.Position{Line: uint32(lineNumber), Character: uint32(end)},
	}
}

// walkYAMLPath navigates a yaml.Node tree following the given path segments.
// Segments can be map keys ("processors") or array indices ("4").
func walkYAMLPath(node *yamlv3.Node, segments []string) *yamlv3.Node {
	return walkYAMLPathMode(node, segments, false)
}

func walkYAMLValuePath(node *yamlv3.Node, segments []string) *yamlv3.Node {
	return walkYAMLPathMode(node, segments, true)
}

func walkYAMLPathMode(node *yamlv3.Node, segments []string, returnValue bool) *yamlv3.Node {
	current := node
	if len(segments) == 0 {
		return current
	}

	for i := 0; i < len(segments); i++ {
		if current == nil {
			return nil
		}
		seg := segments[i]

		switch current.Kind {
		case yamlv3.MappingNode:
			keyNode, valueNode, nextIndex := findMappingPathMatch(current, segments, i)
			if keyNode == nil {
				return nil
			}
			if nextIndex == len(segments) {
				if returnValue {
					return valueNode
				}
				return keyNode
			}
			current = valueNode
			i = nextIndex - 1

		case yamlv3.SequenceNode:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(current.Content) {
				return nil
			}
			current = current.Content[idx]
			if i == len(segments)-1 {
				return current
			}

		default:
			return nil
		}
	}
	return current
}

func findMappingPathMatch(node *yamlv3.Node, segments []string, start int) (*yamlv3.Node, *yamlv3.Node, int) {
	for end := len(segments); end > start; end-- {
		candidate := strings.Join(segments[start:end], ".")
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == candidate {
				return node.Content[i], node.Content[i+1], end
			}
		}
	}
	return nil, nil, 0
}
