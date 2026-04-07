// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	packagespec "github.com/elastic/package-spec/v3"
	yamlv3 "gopkg.in/yaml.v3"
)

type manifestSchemaKind string

const (
	manifestSchemaIntegration manifestSchemaKind = "integration/manifest.spec.yml"
	manifestSchemaInput       manifestSchemaKind = "input/manifest.spec.yml"
	manifestSchemaContent     manifestSchemaKind = "content/manifest.spec.yml"
	manifestSchemaDataStream  manifestSchemaKind = "integration/data_stream/manifest.spec.yml"
)

type schemaFile struct {
	Spec map[string]any `yaml:"spec"`
}

type manifestSchemaLoader struct {
	fsys fs.FS

	mu    sync.RWMutex
	cache map[string]map[string]any
}

var schemaLoader = newManifestSchemaLoader(packagespec.FS())

func newManifestSchemaLoader(fsys fs.FS) *manifestSchemaLoader {
	return &manifestSchemaLoader{
		fsys:  fsys,
		cache: make(map[string]map[string]any),
	}
}

func manifestSchemaKindForFile(filePath, packageRoot, documentText string) manifestSchemaKind {
	if isDataStreamManifest(filePath, packageRoot) {
		return manifestSchemaDataStream
	}

	switch packageTypeFromManifest(documentText) {
	case "input":
		return manifestSchemaInput
	case "content":
		return manifestSchemaContent
	default:
		return manifestSchemaIntegration
	}
}

func manifestTopLevelKeys(kind manifestSchemaKind) []string {
	keys, err := schemaLoader.topLevelKeys(string(kind))
	if err != nil {
		return nil
	}
	return keys
}

func manifestChildKeys(dottedPath string, kind manifestSchemaKind) ([]string, bool) {
	keys, fromArray, err := schemaLoader.childPropertiesForPath(string(kind), dottedPath)
	if err != nil {
		return nil, false
	}
	return keys, fromArray
}

func manifestValueCandidates(dottedPath string, kind manifestSchemaKind) []string {
	values, err := schemaLoader.valueCandidatesForPath(string(kind), dottedPath)
	if err != nil {
		return nil
	}
	return values
}

func manifestDoc(dottedPath string, kind manifestSchemaKind) string {
	doc, err := schemaLoader.docForPath(string(kind), dottedPath)
	if err != nil {
		return ""
	}
	return doc
}

func packageTypeFromManifest(documentText string) string {
	var manifest struct {
		Type string `yaml:"type"`
	}
	if err := yamlv3.Unmarshal([]byte(documentText), &manifest); err != nil {
		return ""
	}
	return manifest.Type
}

func (l *manifestSchemaLoader) topLevelKeys(schemaPath string) ([]string, error) {
	root, err := l.load(schemaPath)
	if err != nil {
		return nil, err
	}

	props := asMap(root["properties"])
	if len(props) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func (l *manifestSchemaLoader) docForPath(schemaPath, dottedPath string) (string, error) {
	root, err := l.load(schemaPath)
	if err != nil {
		return "", err
	}

	current := any(root)
	currentPath := schemaPath
	var required bool
	segments := strings.Split(dottedPath, ".")

	for _, segment := range segments {
		childPath, child, childRequired, err := l.child(currentPath, current, segment)
		if err != nil {
			return "", err
		}
		currentPath = childPath
		current = child
		required = childRequired
	}

	_, node, err := l.normalize(currentPath, current)
	if err != nil {
		return "", err
	}
	return formatManifestDoc(dottedPath, node, required), nil
}

func (l *manifestSchemaLoader) childPropertiesForPath(schemaPath, dottedPath string) ([]string, bool, error) {
	nodes, err := l.nodesForPath(schemaPath, dottedPath)
	if err != nil {
		return nil, false, err
	}

	keys := make(map[string]struct{})
	var fromArray bool
	for _, node := range nodes {
		childKeys, childFromArray, err := l.childProperties(node.path, node.node)
		if err != nil {
			continue
		}
		fromArray = fromArray || childFromArray
		for _, key := range childKeys {
			keys[key] = struct{}{}
		}
	}

	if len(keys) == 0 {
		return nil, fromArray, nil
	}

	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, fromArray, nil
}

func (l *manifestSchemaLoader) valueCandidatesForPath(schemaPath, dottedPath string) ([]string, error) {
	nodes, err := l.nodesForPath(schemaPath, dottedPath)
	if err != nil {
		return nil, err
	}

	var out []string
	seen := make(map[string]struct{})
	for _, node := range nodes {
		values, err := l.valueCandidates(node.path, node.node)
		if err != nil {
			continue
		}
		for _, value := range values {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out, nil
}

func (l *manifestSchemaLoader) child(schemaPath string, node any, segment string) (string, any, bool, error) {
	schemaPath, current, err := l.normalize(schemaPath, node)
	if err != nil {
		return "", nil, false, err
	}

	if props := asMap(current["properties"]); props != nil {
		if child, ok := props[segment]; ok {
			return schemaPath, child, containsString(asStringSlice(current["required"]), segment), nil
		}
	}

	if items, ok := current["items"]; ok {
		if childPath, child, required, err := l.child(schemaPath, items, segment); err == nil {
			return childPath, child, required, nil
		}
	}

	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		for _, branch := range asSlice(current[key]) {
			if childPath, child, required, err := l.child(schemaPath, branch, segment); err == nil {
				return childPath, child, required, nil
			}
		}
	}

	return "", nil, false, fmt.Errorf("schema path not found: %s", segment)
}

type manifestSchemaNode struct {
	path string
	node map[string]any
}

func (l *manifestSchemaLoader) nodesForPath(schemaPath, dottedPath string) ([]manifestSchemaNode, error) {
	root, err := l.load(schemaPath)
	if err != nil {
		return nil, err
	}

	nodes, err := l.expandNodes(schemaPath, root)
	if err != nil {
		return nil, err
	}

	if dottedPath == "" {
		return nodes, nil
	}

	for _, segment := range strings.Split(dottedPath, ".") {
		if segment == "" {
			continue
		}

		var next []manifestSchemaNode
		for _, node := range nodes {
			children, err := l.childNodes(node.path, node.node, segment)
			if err != nil {
				continue
			}
			next = append(next, children...)
		}

		if len(next) == 0 {
			return nil, fmt.Errorf("schema path not found: %s", dottedPath)
		}
		nodes = next
	}

	return nodes, nil
}

func (l *manifestSchemaLoader) expandNodes(schemaPath string, node any) ([]manifestSchemaNode, error) {
	schemaPath, current, err := l.normalize(schemaPath, node)
	if err != nil {
		return nil, err
	}

	nodes := []manifestSchemaNode{{path: schemaPath, node: current}}
	for _, key := range []string{"allOf", "oneOf", "anyOf", "then", "else"} {
		for _, branch := range schemaBranches(current[key]) {
			branchNodes, err := l.expandNodes(schemaPath, branch)
			if err != nil {
				continue
			}
			nodes = append(nodes, branchNodes...)
		}
	}

	return nodes, nil
}

func (l *manifestSchemaLoader) childNodes(schemaPath string, node map[string]any, segment string) ([]manifestSchemaNode, error) {
	if props := asMap(node["properties"]); props != nil {
		if child, ok := props[segment]; ok {
			return l.expandNodes(schemaPath, child)
		}
	}

	if items, ok := node["items"]; ok {
		itemNodes, err := l.expandNodes(schemaPath, items)
		if err != nil {
			return nil, err
		}

		var children []manifestSchemaNode
		for _, itemNode := range itemNodes {
			itemChildren, err := l.childNodes(itemNode.path, itemNode.node, segment)
			if err != nil {
				continue
			}
			children = append(children, itemChildren...)
		}
		if len(children) > 0 {
			return children, nil
		}
	}

	return nil, fmt.Errorf("schema path not found: %s", segment)
}

func (l *manifestSchemaLoader) childProperties(schemaPath string, node map[string]any) ([]string, bool, error) {
	schemaPath, current, err := l.normalize(schemaPath, node)
	if err != nil {
		return nil, false, err
	}

	keys := make(map[string]struct{})
	for key := range asMap(current["properties"]) {
		keys[key] = struct{}{}
	}

	for _, branchKey := range []string{"allOf", "oneOf", "anyOf", "then", "else"} {
		for _, branch := range schemaBranches(current[branchKey]) {
			branchKeys, _, err := l.childProperties(schemaPath, asMap(branch))
			if err != nil {
				continue
			}
			for _, key := range branchKeys {
				keys[key] = struct{}{}
			}
		}
	}

	if len(keys) > 0 {
		out := make([]string, 0, len(keys))
		for key := range keys {
			out = append(out, key)
		}
		sort.Strings(out)
		return out, false, nil
	}

	if items, ok := current["items"]; ok {
		itemNodes, err := l.expandNodes(schemaPath, items)
		if err != nil {
			return nil, false, err
		}

		for _, itemNode := range itemNodes {
			itemKeys, _, err := l.childProperties(itemNode.path, itemNode.node)
			if err != nil {
				continue
			}
			for _, key := range itemKeys {
				keys[key] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out, len(out) > 0, nil
}

func (l *manifestSchemaLoader) valueCandidates(schemaPath string, node map[string]any) ([]string, error) {
	schemaPath, current, err := l.normalize(schemaPath, node)
	if err != nil {
		return nil, err
	}

	var out []string
	seen := make(map[string]struct{})
	for _, value := range schemaEnum(current) {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	if schemaHasType(current, "boolean") {
		for _, value := range []string{"true", "false"} {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}

	for _, branchKey := range []string{"allOf", "oneOf", "anyOf", "then", "else"} {
		for _, branch := range schemaBranches(current[branchKey]) {
			branchValues, err := l.valueCandidates(schemaPath, asMap(branch))
			if err != nil {
				continue
			}
			for _, value := range branchValues {
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				out = append(out, value)
			}
		}
	}

	// For array types, surface enum values from the items schema so that
	// both inline ("categories: sec") and list-item ("- sec") contexts
	// get value suggestions.
	if items, ok := current["items"]; ok {
		itemValues, err := l.valueCandidates(schemaPath, asMap(items))
		if err == nil {
			for _, value := range itemValues {
				if _, ok := seen[value]; !ok {
					seen[value] = struct{}{}
					out = append(out, value)
				}
			}
		}
	}

	return out, nil
}

func (l *manifestSchemaLoader) normalize(schemaPath string, node any) (string, map[string]any, error) {
	current := cloneMap(asMap(node))
	if current == nil {
		return "", nil, fmt.Errorf("invalid schema node")
	}

	ref := stringValue(current["$ref"])
	if ref == "" {
		return schemaPath, current, nil
	}

	delete(current, "$ref")
	targetPath, targetNode, err := l.resolveRef(schemaPath, ref)
	if err != nil {
		return "", nil, err
	}

	targetPath, normalizedTarget, err := l.normalize(targetPath, targetNode)
	if err != nil {
		return "", nil, err
	}

	return targetPath, mergeMaps(normalizedTarget, current), nil
}

func (l *manifestSchemaLoader) resolveRef(schemaPath, ref string) (string, any, error) {
	filePart, fragment, _ := strings.Cut(ref, "#")
	targetPath := schemaPath
	if filePart != "" {
		targetPath = path.Clean(path.Join(path.Dir(schemaPath), filePart))
	}

	root, err := l.load(targetPath)
	if err != nil {
		return "", nil, err
	}

	var node any = root
	if fragment == "" {
		return targetPath, node, nil
	}

	for _, segment := range strings.Split(strings.TrimPrefix(fragment, "/"), "/") {
		next, ok := navigate(node, segment)
		if !ok {
			return "", nil, fmt.Errorf("invalid schema ref: %s", ref)
		}
		node = next
	}

	return targetPath, node, nil
}

func (l *manifestSchemaLoader) load(schemaPath string) (map[string]any, error) {
	l.mu.RLock()
	if cached, ok := l.cache[schemaPath]; ok {
		l.mu.RUnlock()
		return cached, nil
	}
	l.mu.RUnlock()

	data, err := fs.ReadFile(l.fsys, schemaPath)
	if err != nil {
		return nil, err
	}

	var spec schemaFile
	if err := yamlv3.Unmarshal(data, &spec); err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache[schemaPath] = spec.Spec
	return spec.Spec, nil
}

func formatManifestDoc(dottedPath string, node map[string]any, required bool) string {
	var parts []string

	header := fmt.Sprintf("**%s**", dottedPath)
	if typeName := schemaType(node); typeName != "" {
		header += fmt.Sprintf(" `%s`", typeName)
	}
	if required {
		header += " (required)"
	}
	parts = append(parts, header)

	if desc := strings.TrimSpace(stringValue(node["description"])); desc != "" {
		parts = append(parts, desc)
	}

	if enum := schemaEnum(node); len(enum) > 0 && len(enum) <= 8 {
		parts = append(parts, "Allowed values: `"+strings.Join(enum, "`, `")+"`")
	}

	if defaultValue, ok := scalarValue(node["default"]); ok {
		parts = append(parts, fmt.Sprintf("Default: `%s`", defaultValue))
	}

	return strings.Join(parts, "\n\n")
}

func schemaType(node map[string]any) string {
	typeValue := node["type"]
	switch value := typeValue.(type) {
	case string:
		return value
	case []any:
		var parts []string
		for _, item := range value {
			if str, ok := item.(string); ok {
				parts = append(parts, str)
			}
		}
		return strings.Join(parts, " | ")
	}

	if len(schemaEnum(node)) > 0 {
		return "enum"
	}
	if node["properties"] != nil {
		return "object"
	}
	if node["items"] != nil {
		return "array"
	}
	return ""
}

func schemaEnum(node map[string]any) []string {
	values := asSlice(node["enum"])
	out := make([]string, 0, len(values))
	for _, value := range values {
		if str, ok := scalarValue(value); ok {
			out = append(out, str)
		}
	}
	return out
}

func schemaHasType(node map[string]any, target string) bool {
	switch value := node["type"].(type) {
	case string:
		return value == target
	case []any:
		for _, item := range value {
			if str, ok := item.(string); ok && str == target {
				return true
			}
		}
	}
	return false
}

func navigate(node any, segment string) (any, bool) {
	switch current := node.(type) {
	case map[string]any:
		next, ok := current[segment]
		return next, ok
	case []any:
		return nil, false
	default:
		return nil, false
	}
}

func asMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func asSlice(value any) []any {
	if value == nil {
		return nil
	}
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}

func schemaBranches(value any) []any {
	if list := asSlice(value); list != nil {
		return list
	}
	if value == nil {
		return nil
	}
	return []any{value}
}

func asStringSlice(value any) []string {
	list := asSlice(value)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if str, ok := item.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func mergeMaps(base, override map[string]any) map[string]any {
	merged := cloneMap(base)
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func stringValue(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func scalarValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	case float64:
		return fmt.Sprintf("%v", v), true
	default:
		return "", false
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
