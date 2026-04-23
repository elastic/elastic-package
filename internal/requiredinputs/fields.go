// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// bundleDataStreamFields bundles field definitions from required input packages
// into the composable integration package's data stream fields directories.
// For each data stream that references an input package, fields defined in the
// input package but not already present in the integration's data stream are
// copied into a new file named <inputPkgName>-fields.yml.
func (r *RequiredInputsResolver) bundleDataStreamFields(inputPkgPaths map[string]string, buildRoot *os.Root) error {
	dsManifestsPaths, err := fs.Glob(buildRoot.FS(), "data_stream/*/manifest.yml")
	if err != nil {
		return fmt.Errorf("globbing data stream manifests: %w", err)
	}

	errorList := make([]error, 0)
	for _, manifestPath := range dsManifestsPaths {
		manifestBytes, err := buildRoot.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("reading data stream manifest %q: %w", manifestPath, err)
		}
		manifest, err := packages.ReadDataStreamManifestBytes(manifestBytes)
		if err != nil {
			return fmt.Errorf("parsing data stream manifest %q: %w", manifestPath, err)
		}
		for _, stream := range manifest.Streams {
			if stream.Package == "" {
				continue
			}
			pkgPath, ok := inputPkgPaths[stream.Package]
			if !ok {
				errorList = append(errorList, fmt.Errorf("stream in manifest %q references input package %q which is not listed in requires.input", manifestPath, stream.Package))
				continue
			}
			dsRootDir := path.Dir(manifestPath)
			if err := r.mergeInputPkgFields(dsRootDir, pkgPath, stream.Package, buildRoot); err != nil {
				return fmt.Errorf("merging input package fields for manifest %q: %w", manifestPath, err)
			}
		}
	}
	return errors.Join(errorList...)
}

// mergeInputPkgFields copies field definitions from the input package into the
// integration's data stream fields directory. Fields already defined in the
// integration take precedence; only fields absent from the integration are
// written to <dsRootDir>/fields/<inputPkgName>-fields.yml.
func (r *RequiredInputsResolver) mergeInputPkgFields(dsRootDir, inputPkgPath, inputPkgName string, buildRoot *os.Root) error {
	existingNames, err := collectExistingFieldNames(dsRootDir, buildRoot)
	if err != nil {
		return fmt.Errorf("collecting existing field names: %w", err)
	}

	inputPkgFS, closeFn, err := openPackageFS(inputPkgPath)
	if err != nil {
		return fmt.Errorf("opening package %q: %w", inputPkgPath, err)
	}
	defer func() { _ = closeFn() }()

	inputFieldFiles, err := fs.Glob(inputPkgFS, "fields/*.yml")
	if err != nil {
		return fmt.Errorf("globbing input package fields: %w", err)
	}
	if len(inputFieldFiles) == 0 {
		logger.Debugf("Input package %q has no fields files, skipping field bundling", inputPkgName)
		return nil
	}

	// Collect field nodes from input package that are not already defined in the integration.
	seenNames := make(map[string]bool)
	newNodes := make([]ast.Node, 0)
	for _, filePath := range inputFieldFiles {
		nodes, err := loadFieldNodesFromFile(inputPkgFS, filePath)
		if err != nil {
			return fmt.Errorf("loading field nodes from %q: %w", filePath, err)
		}
		for _, node := range nodes {
			name := fieldNodeName(node)
			if name == "" || existingNames[name] || seenNames[name] {
				continue
			}
			seenNames[name] = true
			newNodes = append(newNodes, cloneNode(node))
		}
	}

	if len(newNodes) == 0 {
		logger.Debugf("No new fields from input package %q to bundle into %q", inputPkgName, dsRootDir)
		return nil
	}

	// Build a YAML sequence containing the new field nodes.
	seqNode := newSeqNode(newNodes...)

	output, err := formatYAMLNode(seqNode)
	if err != nil {
		return fmt.Errorf("formatting bundled fields YAML: %w", err)
	}

	fieldsDir := path.Join(dsRootDir, "fields")
	if err := buildRoot.MkdirAll(fieldsDir, 0755); err != nil {
		return fmt.Errorf("creating fields directory %q: %w", fieldsDir, err)
	}

	destPath := path.Join(fieldsDir, inputPkgName+"-fields.yml")
	if err := buildRoot.WriteFile(destPath, output, 0644); err != nil {
		return fmt.Errorf("writing bundled fields to %q: %w", destPath, err)
	}
	logger.Debugf("Bundled %d field(s) from input package %q into %s", len(newNodes), inputPkgName, destPath)
	return nil
}

// collectExistingFieldNames returns the set of top-level field names already
// defined in the integration's data stream fields directory.
func collectExistingFieldNames(dsRootDir string, buildRoot *os.Root) (map[string]bool, error) {
	pattern := path.Join(dsRootDir, "fields", "*.yml")
	paths, err := fs.Glob(buildRoot.FS(), pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing fields in %q: %w", dsRootDir, err)
	}

	names := make(map[string]bool)
	for _, p := range paths {
		data, err := buildRoot.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading fields file %q: %w", p, err)
		}
		nodes, err := loadFieldNodesFromBytes(data)
		if err != nil {
			return nil, fmt.Errorf("parsing fields file %q: %w", p, err)
		}
		for _, node := range nodes {
			if name := fieldNodeName(node); name != "" {
				names[name] = true
			}
		}
	}
	return names, nil
}

// loadFieldNodesFromFile reads a fields YAML file from an fs.FS and returns
// its top-level sequence items as individual ast.Node values.
func loadFieldNodesFromFile(fsys fs.FS, filePath string) ([]ast.Node, error) {
	data, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %q: %w", filePath, err)
	}
	return loadFieldNodesFromBytes(data)
}

// loadFieldNodesFromBytes parses a fields YAML document (expected to be a
// sequence at the document root) and returns the individual item nodes.
func loadFieldNodesFromBytes(data []byte) ([]ast.Node, error) {
	f, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing fields YAML: %w", err)
	}
	if len(f.Docs) == 0 || f.Docs[0] == nil {
		return nil, nil
	}
	body := f.Docs[0].Body
	if body == nil {
		return nil, nil
	}
	seqNode, ok := body.(*ast.SequenceNode)
	if !ok {
		return nil, fmt.Errorf("expected sequence at fields document root, got %T", body)
	}
	return seqNode.Values, nil
}

// fieldNodeName returns the value of the "name" key in a field mapping node,
// or an empty string if the key is absent or the node is not a mapping.
func fieldNodeName(n ast.Node) string {
	mn, ok := n.(*ast.MappingNode)
	if !ok || mn == nil {
		return ""
	}
	return nodeStringValue(mappingValue(mn, "name"))
}
