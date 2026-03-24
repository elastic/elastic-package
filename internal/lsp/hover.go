// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentHover(ctx *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	filePath, err := uriToPath(params.TextDocument.URI)
	if err != nil {
		return nil, nil
	}

	packageRoot, err := findPackageRoot(filePath)
	if err != nil {
		return nil, nil
	}

	documentText := s.documentText(filePath)
	line := getLineAtText(documentText, int(params.Position.Line))
	if line == "" {
		return nil, nil
	}

	// Try different hover strategies.
	if md := hoverFieldReference(line, params.Position, packageRoot, filePath); md != "" {
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: md},
		}, nil
	}

	if md := hoverManifestKey(line, params.Position, filePath, packageRoot, documentText); md != "" {
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: md},
		}, nil
	}

	if md := hoverFieldDefinition(line, params.Position, filePath); md != "" {
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: md},
		}, nil
	}

	return nil, nil
}

// hoverFieldReference shows info when hovering over a field name in a pipeline.
func hoverFieldReference(line string, pos protocol.Position, packageRoot, filePath string) string {
	// Check if cursor is on a field value (e.g. "field: message").
	fieldName := extractFieldValueAtCursor(line, pos)
	if fieldName == "" {
		return ""
	}

	ds := dataStreamFromPath(filePath, packageRoot)
	var idx FieldIndex
	if ds != "" {
		idx = BuildFieldIndexForDataStream(packageRoot, ds)
	} else {
		idx = BuildFieldIndex(packageRoot)
	}

	info, ok := idx[fieldName]
	if !ok {
		return ""
	}

	return formatFieldHover(fieldName, info)
}

// hoverManifestKey shows documentation when hovering over a manifest key.
func hoverManifestKey(line string, pos protocol.Position, filePath, packageRoot, documentText string) string {
	if !isManifestFile(filePath, packageRoot) {
		return ""
	}

	key := extractYAMLKey(line)
	if key == "" {
		return ""
	}

	// Check cursor is on the key part (before the colon).
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 || int(pos.Character) > uint32ToInt(uint32(colonIdx)) {
		return ""
	}

	// Resolve full dotted path by walking up the YAML indentation.
	fullPath := resolveYAMLPath(documentText, int(pos.Line))
	kind := manifestSchemaKindForFile(filePath, packageRoot, documentText)

	// Try the full path first, then progressively shorter suffixes.
	for i := 0; i < len(fullPath); i++ {
		candidate := strings.Join(fullPath[i:], ".")
		md := manifestDoc(candidate, kind)
		if md != "" {
			return md
		}
	}

	return ""
}

// hoverFieldDefinition shows info when hovering in a fields/*.yml file.
func hoverFieldDefinition(line string, pos protocol.Position, filePath string) string {
	if !isFieldsDefinitionFile(filePath) {
		return ""
	}

	// Hovering over a type value.
	if typeName := valueAfterKeyAtCursor(line, pos, "type:"); typeName != "" {
		return fieldTypeDocs(typeName)
	}

	// Hovering over a unit value.
	if unit := valueAfterKeyAtCursor(line, pos, "unit:"); unit != "" {
		return unitDocs(unit)
	}

	// Hovering over a metric_type value.
	if mt := valueAfterKeyAtCursor(line, pos, "metric_type:"); mt != "" {
		return metricTypeDocs(mt)
	}

	return ""
}

// --- formatters ---

func formatFieldHover(name string, f FieldInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "**%s** `%s`\n\n", name, f.Type)
	if f.Description != "" {
		sb.WriteString(f.Description + "\n\n")
	}
	if f.Unit != "" {
		fmt.Fprintf(&sb, "Unit: `%s`\n\n", f.Unit)
	}
	if f.MetricType != "" {
		fmt.Fprintf(&sb, "Metric type: `%s`\n\n", f.MetricType)
	}
	if f.External != "" {
		fmt.Fprintf(&sb, "Source: %s\n", f.External)
	}
	return sb.String()
}

// --- extractors ---

// resolveYAMLPath walks up lines from the given position to build the full
// dotted YAML key path using indentation. For example, if the cursor is on
// "input:" indented under "- " inside "streams:", it returns ["streams", "input"].
func resolveYAMLPath(documentText string, lineNum int) []string {
	lines := splitLines(documentText)
	if lineNum < 0 || lineNum >= len(lines) {
		return nil
	}

	// Get the key and indentation of the target line.
	targetKey, _, _ := yamlKeyDetails(lines[lineNum])
	if targetKey == "" {
		return nil
	}

	targetIndent := yamlIndent(lines[lineNum])
	path := []string{targetKey}

	// Walk upward to find parent keys at decreasing indentation.
	currentIndent := targetIndent
	for i := lineNum - 1; i >= 0; i-- {
		line := lines[i]
		indent := yamlIndent(line)

		// Skip blank lines, comments, and lines at same/deeper indent.
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent >= currentIndent {
			continue
		}

		key, isListItem, hasInlineValue := yamlKeyDetails(line)
		if key != "" {
			if isListItem && hasInlineValue {
				continue
			}
			path = append([]string{key}, path...)
			currentIndent = indent
			if indent == 0 {
				break
			}
		}
	}

	return path
}

// yamlIndent returns the number of leading spaces (ignoring "- " list markers).
func yamlIndent(line string) int {
	n := 0
	for _, ch := range line {
		if ch == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

func extractFieldValueAtCursor(line string, pos protocol.Position) string {
	for _, key := range []string{"field:", "target_field:", "source:", "copy_to:"} {
		if value := valueAfterKeyAtCursor(line, pos, key); value != "" {
			return value
		}
	}
	return ""
}

func valueAfterKeyAtCursor(line string, pos protocol.Position, key string) string {
	value, start, end, ok := valueAfterKey(line, key)
	if !ok {
		return ""
	}

	cursor := utf16ColumnToRuneOffset(line, int(pos.Character))
	if cursor < start || cursor >= end {
		return ""
	}

	return value
}

func valueAfterKey(line string, key string) (string, int, int, bool) {
	start, ok := yamlKeyValueStart(line, key)
	if !ok {
		return "", 0, 0, false
	}

	runes := []rune(line)
	for start < len(runes) && unicode.IsSpace(runes[start]) {
		start++
	}

	if start >= len(runes) {
		return "", 0, 0, false
	}

	quote := rune(0)
	if runes[start] == '"' || runes[start] == '\'' {
		quote = runes[start]
		start++
	}

	end := start
	if quote != 0 {
		for end < len(runes) && runes[end] != quote {
			end++
		}
	} else {
		for end < len(runes) && !unicode.IsSpace(runes[end]) && runes[end] != '#' {
			end++
		}
	}

	if end <= start {
		return "", 0, 0, false
	}

	return string(runes[start:end]), start, end, true
}

func yamlKeyValueStart(line string, key string) (int, bool) {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	leadingRunes := len([]rune(line)) - len([]rune(trimmed))

	switch {
	case strings.HasPrefix(trimmed, key):
		return leadingRunes + len([]rune(key)), true
	case strings.HasPrefix(trimmed, "- "+key):
		return leadingRunes + len([]rune("- "+key)), true
	default:
		return 0, false
	}
}

func extractYAMLKey(line string) string {
	key, _, _ := yamlKeyDetails(line)
	return key
}

func yamlKeyDetails(line string) (key string, isListItem, hasInlineValue bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return "", false, false
	}
	if strings.HasPrefix(trimmed, "-") {
		isListItem = true
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
	}
	colonIdx := strings.Index(trimmed, ":")
	if colonIdx <= 0 {
		return "", isListItem, false
	}
	key = trimmed[:colonIdx]
	hasInlineValue = strings.TrimSpace(trimmed[colonIdx+1:]) != ""
	return key, isListItem, hasInlineValue
}

func fieldTypeDocs(typeName string) string {
	docs := map[string]string{
		"keyword":          "**keyword**\n\nExact-value string. Used for filtering, sorting, and aggregations.\n\nNot analyzed for full-text search.",
		"text":             "**text**\n\nFull-text searchable string. Analyzed into tokens.\n\nNot efficient for sorting or aggregations.",
		"match_only_text":  "**match_only_text**\n\nLike `text` but optimized for storage. Only supports match queries.\n\nNo scoring, no positions stored.",
		"long":             "**long**\n\n64-bit signed integer. Range: -2^63 to 2^63-1.",
		"integer":          "**integer**\n\n32-bit signed integer. Range: -2^31 to 2^31-1.",
		"double":           "**double**\n\n64-bit IEEE 754 floating point.",
		"float":            "**float**\n\n32-bit IEEE 754 floating point.",
		"scaled_float":     "**scaled_float**\n\nFloat stored as a long with a `scaling_factor`.\n\nMore storage efficient for fixed-precision decimals.",
		"boolean":          "**boolean**\n\nTrue/false value.",
		"date":             "**date**\n\nDate/time value. Supports multiple formats via `date_format`.\n\nDefault: strict_date_optional_time || epoch_millis.",
		"ip":               "**ip**\n\nIPv4 or IPv6 address.",
		"geo_point":        "**geo_point**\n\nLatitude/longitude point.",
		"object":           "**object**\n\nJSON object. Fields inside are flattened by default.",
		"nested":           "**nested**\n\nLike `object` but maintains field relationships for queries.\n\nMore expensive than `object`.",
		"group":            "**group**\n\nLogical grouping of sub-fields. Not an Elasticsearch type.\n\nUse `fields:` to define children.",
		"flattened":        "**flattened**\n\nEntire JSON object as a single field. All values treated as keywords.\n\nUseful for dynamic or unknown structures.",
		"wildcard":         "**wildcard**\n\nLike `keyword` but optimized for wildcard/regex queries.",
		"constant_keyword": "**constant_keyword**\n\nKeyword that has the same value across all documents in the index.\n\nVery storage efficient.",
		"alias":            "**alias**\n\nAlternate name for an existing field. Requires `path` property.",
		"histogram":        "**histogram**\n\nPre-aggregated histogram values.",
		"version":          "**version**\n\nSemantic version string. Supports version-aware sorting.",
		"unsigned_long":    "**unsigned_long**\n\n64-bit unsigned integer. Range: 0 to 2^64-1.",
	}
	if d, ok := docs[typeName]; ok {
		return d
	}
	return ""
}

func unitDocs(unit string) string {
	docs := map[string]string{
		"byte":    "**byte** — Data size in bytes",
		"percent": "**percent** — Percentage value (0-100)",
		"d":       "**d** — Duration in days",
		"h":       "**h** — Duration in hours",
		"m":       "**m** — Duration in minutes",
		"s":       "**s** — Duration in seconds",
		"ms":      "**ms** — Duration in milliseconds",
		"micros":  "**micros** — Duration in microseconds",
		"nanos":   "**nanos** — Duration in nanoseconds",
	}
	if d, ok := docs[unit]; ok {
		return d
	}
	return ""
}

func metricTypeDocs(mt string) string {
	docs := map[string]string{
		"counter": "**counter**\n\nA cumulative metric that only increases (or resets to zero).\n\nExamples: total requests, bytes sent.",
		"gauge":   "**gauge**\n\nA metric that can arbitrarily go up and down.\n\nExamples: CPU usage, memory used, temperature.",
	}
	if d, ok := docs[mt]; ok {
		return d
	}
	return ""
}

func uint32ToInt(v uint32) int {
	return int(v)
}
