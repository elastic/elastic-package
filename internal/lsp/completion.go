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

type manifestCompletionMode int

const (
	manifestCompletionModeKey manifestCompletionMode = iota
	manifestCompletionModeValue
)

type manifestCompletionContext struct {
	mode           manifestCompletionMode
	path           string
	prefix         string
	currentIndent  int
	parentIndent   int
	listItemPrefix bool
}

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
		items = completeManifestItems(filePath, packageRoot, documentText, params.Position)
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
		detail := formatFieldDetail(info)
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
	prefix, _, _, ok := valueAfterKey(trimmed, "type:")
	if !ok {
		prefix = ""
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
		if prefix != "" && !strings.HasPrefix(t, prefix) {
			continue
		}
		label := t
		items = append(items, protocol.CompletionItem{
			Label: label,
			Kind:  &enumKind,
		})
	}
	return items
}

// completeManifestItems suggests manifest keys or values based on package-spec.
func completeManifestItems(filePath, packageRoot, documentText string, pos protocol.Position) []protocol.CompletionItem {
	kind := manifestSchemaKindForFile(filePath, packageRoot, documentText)
	context, ok := resolveManifestCompletionContext(documentText, pos)
	if !ok {
		return nil
	}

	switch context.mode {
	case manifestCompletionModeKey:
		return completeManifestKeyItems(kind, context)
	case manifestCompletionModeValue:
		return completeManifestValueItems(kind, context)
	default:
		return nil
	}
}

func completeManifestKeyItems(kind manifestSchemaKind, context manifestCompletionContext) []protocol.CompletionItem {
	keys, fromArray := manifestChildKeys(context.path, kind)
	if len(keys) == 0 {
		return nil
	}

	insertPrefix := ""
	if fromArray && !context.listItemPrefix && context.parentIndent >= 0 &&
		context.currentIndent == context.parentIndent+2 {
		insertPrefix = "- "
	}

	propKind := protocol.CompletionItemKindProperty
	var items []protocol.CompletionItem
	for _, k := range keys {
		if context.prefix != "" && !strings.HasPrefix(k, context.prefix) {
			continue
		}
		label := k
		item := protocol.CompletionItem{
			Label:      label,
			Kind:       &propKind,
			InsertText: strPtr(insertPrefix + k + ": "),
		}
		if doc := manifestDoc(joinManifestPath(context.path, k), kind); doc != "" {
			item.Documentation = &protocol.MarkupContent{
				Kind:  protocol.MarkupKindMarkdown,
				Value: doc,
			}
		}
		items = append(items, item)
	}
	return items
}

func completeManifestValueItems(kind manifestSchemaKind, context manifestCompletionContext) []protocol.CompletionItem {
	values := manifestValueCandidates(context.path, kind)
	if len(values) == 0 {
		return nil
	}

	enumKind := protocol.CompletionItemKindEnumMember
	var items []protocol.CompletionItem
	for _, value := range values {
		if context.prefix != "" && !strings.HasPrefix(value, context.prefix) {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label: value,
			Kind:  &enumKind,
		})
	}
	return items
}

// --- helpers ---

func isFieldValueContext(line string) bool {
	for _, key := range []string{"field:", "target_field:", "source:", "copy_to:"} {
		if _, ok := yamlKeyValueStart(line, key); ok {
			return true
		}
	}
	return false
}

func extractFieldPrefix(line string) string {
	for _, key := range []string{"field:", "target_field:", "source:", "copy_to:"} {
		if value, _, _, ok := valueAfterKey(line, key); ok {
			return value
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

func resolveManifestCompletionContext(documentText string, pos protocol.Position) (manifestCompletionContext, bool) {
	lines := splitLines(documentText)
	lineNum := int(pos.Line)
	if lineNum < 0 || lineNum >= len(lines) {
		return manifestCompletionContext{}, false
	}

	linePrefix := linePrefixAtPosition(lines[lineNum], pos)
	trimmed := strings.TrimSpace(linePrefix)
	if strings.HasPrefix(trimmed, "#") {
		return manifestCompletionContext{}, false
	}

	if context, ok := resolveManifestValueContext(lines, lineNum, linePrefix); ok {
		return context, true
	}

	return resolveManifestKeyContext(lines, lineNum, linePrefix), true
}

func resolveManifestValueContext(lines []string, lineNum int, linePrefix string) (manifestCompletionContext, bool) {
	key, _, _ := yamlKeyDetails(linePrefix)
	if key == "" || !strings.Contains(linePrefix, ":") {
		return manifestCompletionContext{}, false
	}

	path, parentIndent := resolveManifestParentPath(lines, lineNum, yamlIndent(linePrefix))
	prefix := ""
	if value, _, _, ok := valueAfterKey(linePrefix, key+":"); ok {
		prefix = value
	}

	return manifestCompletionContext{
		mode:           manifestCompletionModeValue,
		path:           joinManifestPath(strings.Join(path, "."), key),
		prefix:         prefix,
		currentIndent:  yamlIndent(linePrefix),
		parentIndent:   parentIndent,
		listItemPrefix: hasListItemPrefix(linePrefix),
	}, true
}

func resolveManifestKeyContext(lines []string, lineNum int, linePrefix string) manifestCompletionContext {
	path, parentIndent := resolveManifestParentPath(lines, lineNum, yamlIndent(linePrefix))

	return manifestCompletionContext{
		mode:           manifestCompletionModeKey,
		path:           strings.Join(path, "."),
		prefix:         extractManifestKeyPrefix(linePrefix),
		currentIndent:  yamlIndent(linePrefix),
		parentIndent:   parentIndent,
		listItemPrefix: hasListItemPrefix(linePrefix),
	}
}

func resolveManifestParentPath(lines []string, lineNum, currentIndent int) ([]string, int) {
	currentIndent = max(currentIndent, 0)

	var path []string
	parentIndent := -1
	indentLimit := currentIndent
	for i := lineNum - 1; i >= 0; i-- {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := yamlIndent(line)
		if indent >= indentLimit {
			continue
		}

		key, isListItem, hasInlineValue := yamlKeyDetails(line)
		if key == "" {
			continue
		}
		if isListItem && hasInlineValue {
			continue
		}

		if parentIndent < 0 {
			parentIndent = indent
		}
		path = append([]string{key}, path...)
		indentLimit = indent
		if indent == 0 {
			break
		}
	}

	return path, parentIndent
}

func linePrefixAtPosition(line string, pos protocol.Position) string {
	runes := []rune(line)
	offset := utf16ColumnToRuneOffset(line, int(pos.Character))
	offset = min(offset, len(runes))
	return string(runes[:offset])
}

func extractManifestKeyPrefix(linePrefix string) string {
	trimmed := strings.TrimSpace(linePrefix)
	if strings.HasPrefix(trimmed, "-") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
	}
	if colonIdx := strings.Index(trimmed, ":"); colonIdx >= 0 {
		trimmed = trimmed[:colonIdx]
	}
	return strings.TrimSpace(trimmed)
}

func joinManifestPath(base, child string) string {
	if base == "" {
		return child
	}
	return base + "." + child
}

func hasListItemPrefix(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "-")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
