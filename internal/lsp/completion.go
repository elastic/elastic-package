// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentCompletion(ctx *glsp.Context, params *protocol.CompletionParams) (any, error) {
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

	var items []protocol.CompletionItem

	switch {
	case isFieldValueContext(line):
		// Completing a "field: " value in a pipeline — suggest field names.
		items = s.completeFieldNames(packageRoot, filePath, line)
	case isManifestFile(filePath, packageRoot):
		items = completeManifestKeys(filePath, packageRoot, line, documentText)
	case isFieldsDefinitionFile(filePath):
		items = completeFieldTypeValues(line)
	}

	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

// completeFieldNames suggests dotted field names from the package's field definitions.
func (s *Server) completeFieldNames(packageRoot, filePath, line string) []protocol.CompletionItem {
	// Determine which data stream we're in, if any.
	ds := dataStreamFromPath(filePath, packageRoot)
	var idx FieldIndex
	if ds != "" {
		idx = BuildFieldIndexForDataStream(packageRoot, ds)
	} else {
		idx = BuildFieldIndex(packageRoot)
	}

	// Extract partial text after "field:" on the line.
	prefix := extractFieldPrefix(line)

	var items []protocol.CompletionItem
	names := make([]string, 0, len(idx))
	for name := range idx {
		names = append(names, name)
	}
	sort.Strings(names)

	fieldKind := protocol.CompletionItemKindField
	for _, name := range names {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		info := idx[name]
		detail := info.Type
		if info.Unit != "" {
			detail += " (" + info.Unit + ")"
		}
		doc := info.Description
		item := protocol.CompletionItem{
			Label:  name,
			Kind:   &fieldKind,
			Detail: &detail,
		}
		if doc != "" {
			item.Documentation = &protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: doc,
			}
		}
		items = append(items, item)
	}
	return items
}

// completeFieldTypeValues suggests valid type values when editing fields/*.yml.
func completeFieldTypeValues(line string) []protocol.CompletionItem {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "type:") {
		return nil
	}

	types := []string{
		"keyword", "text", "match_only_text", "wildcard", "constant_keyword",
		"long", "integer", "short", "byte", "double", "float", "half_float",
		"scaled_float", "unsigned_long",
		"date", "date_nanos", "boolean", "binary", "ip",
		"geo_point", "object", "nested", "flattened", "group",
		"alias", "histogram", "aggregate_metric_double",
		"integer_range", "float_range", "long_range", "double_range",
		"date_range", "ip_range",
		"version", "counted_keyword", "semantic_text",
	}

	enumKind := protocol.CompletionItemKindEnumMember
	var items []protocol.CompletionItem
	for _, t := range types {
		label := t
		items = append(items, protocol.CompletionItem{
			Label: label,
			Kind:  &enumKind,
		})
	}
	return items
}

// completeManifestKeys suggests top-level manifest keys based on package-spec.
func completeManifestKeys(filePath, packageRoot, line, documentText string) []protocol.CompletionItem {
	trimmed := strings.TrimSpace(line)
	// Only suggest at start of a line (not inside a value).
	if strings.Contains(trimmed, ":") {
		return nil
	}

	keys := manifestTopLevelKeys(manifestSchemaKindForFile(filePath, packageRoot, documentText))

	propKind := protocol.CompletionItemKindProperty
	var items []protocol.CompletionItem
	for _, k := range keys {
		label := k
		items = append(items, protocol.CompletionItem{
			Label:      label,
			Kind:       &propKind,
			InsertText: strPtr(k + ": "),
		})
	}
	return items
}

// --- helpers ---

func isFieldValueContext(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "field:") ||
		strings.HasPrefix(trimmed, "target_field:") ||
		strings.HasPrefix(trimmed, "source:") ||
		strings.HasPrefix(trimmed, "copy_to:")
}

func extractFieldPrefix(line string) string {
	for _, key := range []string{"field:", "target_field:", "source:", "copy_to:"} {
		if idx := strings.Index(line, key); idx >= 0 {
			return strings.TrimSpace(line[idx+len(key):])
		}
	}
	return ""
}

func isManifestFile(filePath, packageRoot string) bool {
	return filepath.Base(filePath) == "manifest.yml" &&
		strings.HasPrefix(filePath, packageRoot)
}

func isDataStreamManifest(filePath, packageRoot string) bool {
	rel, err := filepath.Rel(packageRoot, filePath)
	if err != nil {
		return false
	}
	// data_stream/<name>/manifest.yml
	parts := strings.Split(rel, string(filepath.Separator))
	return len(parts) == 3 && parts[0] == "data_stream" && parts[2] == "manifest.yml"
}

func isFieldsDefinitionFile(filePath string) bool {
	return strings.Contains(filePath, string(filepath.Separator)+"fields"+string(filepath.Separator)) &&
		(strings.HasSuffix(filePath, ".yml") || strings.HasSuffix(filePath, ".yaml"))
}

func dataStreamFromPath(filePath, packageRoot string) string {
	rel, err := filepath.Rel(packageRoot, filePath)
	if err != nil {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) >= 2 && parts[0] == "data_stream" {
		return parts[1]
	}
	return ""
}

// formatFieldDetail returns a short detail string for a field.
func formatFieldDetail(f FieldInfo) string {
	parts := []string{f.Type}
	if f.Unit != "" {
		parts = append(parts, fmt.Sprintf("unit: %s", f.Unit))
	}
	if f.MetricType != "" {
		parts = append(parts, fmt.Sprintf("metric: %s", f.MetricType))
	}
	return strings.Join(parts, ", ")
}
