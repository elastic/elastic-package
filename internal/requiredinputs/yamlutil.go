// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/printer"
	"github.com/goccy/go-yaml/token"

	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/yamledit"
)

// mappingValue returns the value ast.Node for the given key in a YAML mapping
// node, or nil if the key is not present.
func mappingValue(node *ast.MappingNode, key string) ast.Node {
	for _, kv := range node.Values {
		if kv.Key.String() == key {
			return kv.Value
		}
	}
	return nil
}

// removeKey removes a key-value pair from a YAML mapping node.
func removeKey(node *ast.MappingNode, key string) {
	for i, kv := range node.Values {
		if kv.Key.String() == key {
			node.Values = append(node.Values[:i], node.Values[i+1:]...)
			return
		}
	}
}

// upsertKey sets key to value in a YAML mapping node, adding it if absent.
// When inserting a new key, the column position is derived from the existing
// entries so the new node serialises with the same indentation as its siblings.
// For block-style SequenceNode values, the sequence Start column is set to
// match the key column so blockStyleString generates correct indentation.
func upsertKey(node *ast.MappingNode, key string, value ast.Node) {
	// Derive column from existing entries so new nodes indent like their
	// siblings. Fall back to 1 when the mapping has no entries yet (e.g.
	// freshly constructed nodes in tests).
	col := 1
	if len(node.Values) > 0 {
		col = node.Values[0].Key.GetToken().Position.Column
	}
	// For block-style sequences, match the sequence Start column to the key
	// column so SequenceNode.blockStyleString produces the correct indentation
	// regardless of whether the key is new or already exists.
	if sn, ok := value.(*ast.SequenceNode); ok && !sn.IsFlowStyle {
		sn.Start.Position.Column = col
	}

	for _, kv := range node.Values {
		if kv.Key.String() == key {
			kv.Value = value
			return
		}
	}
	// Key not present — build a new MappingValueNode directly to avoid
	// yaml.ValueToNode's MarshalYAML path which requires non-nil Start tokens
	// on sequence/mapping nodes.
	pos := &token.Position{Column: col, Line: 1}
	keyTk := token.New(key, key, pos)
	colonTk := token.New(":", ":", pos)
	mv := ast.MappingValue(colonTk, ast.String(keyTk), value)
	node.Values = append(node.Values, mv)
}

// newSeqNode creates a *ast.SequenceNode with a valid Start token so that the
// goccy printer can serialise it without a nil-pointer panic.
// Values can be any ast.Node; for string scalars prefer strVal().
func newSeqNode(values ...ast.Node) *ast.SequenceNode {
	pos := &token.Position{Column: 1, Line: 1}
	sn := ast.Sequence(token.New("-", "-", pos), false)
	sn.Values = values
	return sn
}

// cloneNode returns a deep copy of the YAML node tree via round-trip
// serialization so base nodes from the input package can be reused for multiple
// independent merges without aliasing.
// Panics if serialization or parsing of an already-valid node fails (impossible).
func cloneNode(n ast.Node) ast.Node {
	if n == nil {
		return nil
	}
	p := printer.Printer{}
	b := p.PrintNode(n)
	f, err := parser.ParseBytes(b, 0)
	if err != nil {
		panic(fmt.Sprintf("cloneNode: failed to re-parse: %v", err))
	}
	if len(f.Docs) == 0 || f.Docs[0] == nil {
		return nil
	}
	return f.Docs[0].Body
}

// formatYAMLNode marshals an ast.Node to bytes and applies the package's YAML
// formatter with KeysWithDotActionNone.
func formatYAMLNode(node ast.Node) ([]byte, error) {
	p := printer.Printer{}
	raw := p.PrintNode(node)
	yamlFormatter := formatter.NewYAMLFormatter(formatter.KeysWithDotActionNone)
	formatted, _, err := yamlFormatter.Format(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to format YAML: %w", err)
	}
	return formatted, nil
}

// nodeStringValue extracts the string value from a scalar ast.Node. For
// StringNode, the raw Value field is returned. For other scalars, String() is
// used. Returns "" for nil nodes.
func nodeStringValue(n ast.Node) string {
	if n == nil {
		return ""
	}
	if sn, ok := n.(*ast.StringNode); ok {
		return sn.Value
	}
	return n.String()
}

// strVal converts a plain string to a YAML scalar ast.Node.
// Panics if construction fails (impossible for string inputs).
func strVal(s string) ast.Node {
	n, err := yaml.ValueToNode(s)
	if err != nil {
		panic(fmt.Sprintf("strVal: unexpected error for %q: %v", s, err))
	}
	return n
}

// parseDocumentRootMapping parses YAML bytes via yamledit and returns the
// document root as a *ast.MappingNode. Reuses internal/yamledit for parsing.
func parseDocumentRootMapping(data []byte) (*ast.MappingNode, error) {
	doc, err := yamledit.NewDocumentBytes(data)
	if err != nil {
		return nil, err
	}
	if len(doc.AST().Docs) == 0 || doc.AST().Docs[0] == nil {
		return nil, fmt.Errorf("empty YAML document")
	}
	root, ok := doc.AST().Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("expected mapping node at document root, got %T", doc.AST().Docs[0].Body)
	}
	return root, nil
}
