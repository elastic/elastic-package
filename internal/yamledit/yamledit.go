// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package yamledit

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/printer"
)

const (
	// IndexPrepend is a shorthand for an index that prepends to a list.
	IndexPrepend = 0
	// IndexAppend is a shorthand for an index that appends to a list.
	IndexAppend = -1
)

// ErrInvalidNodeType indicates a node was not of an expected type.
var ErrInvalidNodeType = errors.New("invalid node type")

// Document defines a YAML document.
type Document struct {
	f *ast.File
	h uint64
}

// AST returns the underlying AST of the document.
func (d *Document) AST() *ast.File {
	return d.f
}

// Modified returns true if the document has been modified.
func (d *Document) Modified() bool {
	return d.Hash() != d.h
}

// Hash returns a FNV1a 64-bit hash of the document.
func (d *Document) Hash() uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(d.f.String()))

	return h.Sum64()
}

// Filename returns the filename of the document (if applicable).
func (d *Document) Filename() string {
	return d.f.Name
}

// WriteFile writes the document to the original file.
func (d *Document) WriteFile() error {
	if d.f.Name == "" {
		return errors.New("failed to write document: empty filename")
	}

	return d.WriteFileAs(d.Filename())
}

// WriteFileAs writes the document to the given file.
func (d *Document) WriteFileAs(filename string) error {
	p := printer.Printer{}
	data := p.PrintNode(d.f.Docs[0])

	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("failed to write document to file %q: %w", filename, err)
	}

	return nil
}

// Write writes the document to writer.
func (d *Document) Write(w io.Writer) (int, error) {
	p := printer.Printer{}
	data := p.PrintNode(d.f.Docs[0])

	return w.Write(data)
}

// Parse will attempt to parse the document into v.
func (d *Document) Parse(v any) error {
	if err := yaml.NodeToValue(d.f.Docs[0].Body, v); err != nil {
		return err
	}

	// Set the Document field on v, if v is a pointer to a struct and the field
	// on the struct is exported.
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Ptr {
		if structValue := rv.Elem(); structValue.Kind() == reflect.Struct {
			for i := 0; i < structValue.NumField(); i++ {
				if field := structValue.Field(i); field.CanAddr() && field.CanSet() {
					if _, ok := field.Interface().(*Document); ok {
						field.Set(reflect.ValueOf(d))
						break
					}
				}
			}
		}
	}

	return nil
}

// GetNode gets the node the given path.
func (d *Document) GetNode(path string) (ast.Node, error) {
	p, err := yaml.PathString(path)
	if err != nil {
		return nil, err
	}

	return p.FilterFile(d.f)
}

// GetMappingNode gets the mapping node at the given path.
func (d *Document) GetMappingNode(path string) (*ast.MappingNode, error) {
	n, err := d.GetNode(path)
	if err != nil {
		return nil, err
	}

	mn, ok := n.(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("%w: expected a MappingNode, got %T", ErrInvalidNodeType, n)
	}

	return mn, nil
}

// GetSequenceNode gets the sequence node at the given path.
func (d *Document) GetSequenceNode(path string) (*ast.SequenceNode, error) {
	n, err := d.GetNode(path)
	if err != nil {
		return nil, err
	}

	mn, ok := n.(*ast.SequenceNode)
	if !ok {
		return nil, fmt.Errorf("%w: expected a SequenceNode, got %T", ErrInvalidNodeType, n)
	}

	return mn, nil
}

// GetParentNode gets the parent node for the node at the given path. If the
// node is the root node of the document, nil is returned. If the leaf node of
// the path does not exist in the document, but the parent node exists, the
// parent node will still be returned.
func (d *Document) GetParentNode(path string) (ast.Node, error) {
	if path == "$" {
		return nil, nil
	}

	lastDot := strings.LastIndex(path, ".")
	lastSeq := strings.LastIndex(path, "[")

	parentStr := path[:max(lastDot, lastSeq)]
	parentPath, _ := yaml.PathString(parentStr)

	return parentPath.FilterFile(d.f)
}

// DeleteNode deletes the node from the document at the given path. Only supported
// in cases where the parent node is mapping or sequence node.
func (d *Document) DeleteNode(path string) (bool, error) {
	p, err := yaml.PathString(path)
	if err != nil {
		return false, err
	}
	n, err := p.FilterFile(d.f)
	if err != nil {
		return false, err
	}

	parentNode, err := d.GetParentNode(path)
	if err != nil {
		return false, err
	}
	if parentNode == nil {
		return false, errors.New("cannot delete root node")
	}

	switch parentNode.Type() {
	case ast.MappingType:
		mn := parentNode.(*ast.MappingNode)
		for i, kv := range mn.Values {
			if kv.Value == n {
				mn.Values = append(mn.Values[:i], mn.Values[i+1:]...)

				return true, nil
			}
		}
	case ast.SequenceType:
		sn := parentNode.(*ast.SequenceNode)
		index := getPathIndex(path)

		sn.Values = append(sn.Values[:index], sn.Values[index+1:]...)
		sn.ValueHeadComments = append(sn.ValueHeadComments[:index], sn.ValueHeadComments[index+1:]...)

		return true, nil
	default:
		return false, fmt.Errorf("unable to delete node with parent type %s at %q: %w", parentNode.Type(), p, err)
	}

	return false, nil
}

// AddValue is like AddNode, but takes a raw value rather than a YAML node.
func (d *Document) AddValue(path string, v any, index int, replace bool) (bool, error) {
	n, err := yaml.ValueToNode(v)
	if err != nil {
		return false, err
	}

	return d.AddNode(path, n, index, replace)
}

// PrependValue prepends the value to the sequence at the given path.
func (d *Document) PrependValue(path string, v any) (bool, error) {
	return d.AddValue(path, v, IndexPrepend, false)
}

// AppendValue appends the value to the sequence at the given path.
func (d *Document) AppendValue(path string, v any) (bool, error) {
	return d.AddValue(path, v, IndexAppend, false)
}

// AddNode adds a node to the sequence node at the given path. The insertion
// point and behavior at insertion can be controlled by the index and replace
// arguments, respectively.
func (d *Document) AddNode(path string, n ast.Node, index int, replace bool) (bool, error) {
	sn, err := d.GetSequenceNode(path)
	if err != nil {
		return false, err
	}

	doReplace := replace && index > 0 && index < len(sn.Values)
	if doReplace {
		if !nodeEqual(sn.Values[index], n) {
			sn.Values[index] = n
			sn.ValueHeadComments[index] = n.GetComment()

			return true, nil
		}
		return false, nil
	}

	if index < 0 || index >= len(sn.Values) {
		sn.Values = append(sn.Values, n)
		sn.ValueHeadComments = append(sn.ValueHeadComments, n.GetComment())
	} else {
		sn.Values = slices.Insert(sn.Values, index, n)
		sn.ValueHeadComments = slices.Insert(sn.ValueHeadComments, index, n.GetComment())
	}

	return true, nil
}

// SetKeyValue is like SetKeyNode, but takes a raw value rather than a YAML node.
func (d *Document) SetKeyValue(path, key string, v any, index int) (bool, error) {
	n, err := yaml.ValueToNode(v)
	if err != nil {
		return false, err
	}

	return d.SetKeyNode(path, key, n, index)
}

// SetKeyNode sets the node for a key in a mapping node at the given path.
func (d *Document) SetKeyNode(path, key string, n ast.Node, index int) (bool, error) {
	mn, err := d.GetMappingNode(path)
	if err != nil {
		return false, err
	}

	for _, kv := range mn.Values {
		if kv.Key.String() != key {
			continue
		}

		p, err := yaml.PathString(path + "." + key)
		if err != nil {
			return false, err
		}

		err = p.ReplaceWithNode(d.f, n)
		return err == nil, err
	}

	newNode, err := yaml.ValueToNode(map[string]any{
		key: n,
	})
	if err != nil {
		return false, err
	}

	newValue := newNode.(*ast.MappingNode).Values[0]
	newValue.AddColumn(mn.GetToken().Position.IndentNum)

	if index >= 0 && index < len(mn.Values) {
		mn.Values = slices.Insert(mn.Values, index, newValue)
	} else {
		mn.Values = append(mn.Values, newValue)
	}

	return true, nil
}

// ParseDocumentFile is like NewDocumentFile, but will unmarshal the
// document into value given by v.
func ParseDocumentFile(filename string, v any) (*Document, error) {
	d, err := NewDocumentFile(filename)
	if err != nil {
		return nil, err
	}

	if err = d.Parse(v); err != nil {
		return d, err
	}

	return d, nil
}

// ParseDocumentBytes is like NewDocumentBytes, but will unmarshal the
// document into value given by v.
func ParseDocumentBytes(data []byte, v any) (*Document, error) {
	d, err := NewDocumentBytes(data)
	if err != nil {
		return nil, err
	}

	if err = d.Parse(v); err != nil {
		return nil, err
	}

	return d, nil
}

// NewDocumentFile creates a new document from the given yaml file.
func NewDocumentFile(filename string) (*Document, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %q: %w", filename, err)
	}

	d, err := NewDocumentBytes(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse file %q: %w", filename, err)
	}
	d.f.Name = filename

	return d, nil
}

// NewDocumentBytes creates a new document from the given yaml bytes.
func NewDocumentBytes(data []byte) (*Document, error) {
	var d Document
	var err error

	d.f, err = parser.ParseBytes(data, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	d.h = d.Hash()

	return &d, nil
}

// getPathIndex gets the index referenced at the end of the path. The last
// element of the path must be a sequence, otherwise -1 will be returned.
func getPathIndex(path string) int {
	lastDot := strings.LastIndex(path, ".")
	lastSeq := strings.LastIndex(path, "[")

	if lastSeq == -1 {
		return -1
	}
	if lastDot > lastSeq {
		return -1
	}

	closeSeq := strings.LastIndex(path[lastSeq:], "]")
	if closeSeq == -1 {
		return -1
	}

	i, err := strconv.Atoi(path[lastSeq+1 : lastSeq+closeSeq])
	if err != nil {
		return -1
	}

	return i
}

// cutPath splits the last element of the path, returning the parent path and
// the last element of the path, or an error if the path is not valid.
func cutPath(path string) (string, string, error) {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return "", "", fmt.Errorf("unable to get parent path of %q", path)
	}

	before := path[:idx]
	after := path[idx+1:]

	return before, after, nil
}

// nodeEqual returns true if the two nodes are equal.
func nodeEqual(a, b ast.Node) bool {
	var x, y any
	_ = yaml.NodeToValue(a, &x)
	_ = yaml.NodeToValue(b, &y)

	return reflect.DeepEqual(x, y)
}
