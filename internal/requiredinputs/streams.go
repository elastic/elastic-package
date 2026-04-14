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

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
)

func (r *RequiredInputsResolver) bundleDataStreamTemplates(inputPkgPaths map[string]string, buildRoot *os.Root) error {
	// get all data stream manifest paths in the build package
	dsManifestsPaths, err := fs.Glob(buildRoot.FS(), "data_stream/*/manifest.yml")
	if err != nil {
		return fmt.Errorf("failed to glob data stream manifests: %w", err)
	}

	errorList := make([]error, 0)
	for _, manifestPath := range dsManifestsPaths {
		if err := r.processDataStreamManifest(manifestPath, inputPkgPaths, buildRoot); err != nil {
			errorList = append(errorList, err)
		}
	}
	return errors.Join(errorList...)
}

func (r *RequiredInputsResolver) processDataStreamManifest(manifestPath string, inputPkgPaths map[string]string, buildRoot *os.Root) error {
	manifestBytes, err := buildRoot.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read data stream manifest %q: %w", manifestPath, err)
	}
	// parse the manifest YAML document preserving formatting for targeted modifications
	// using manifestBytes allows us to preserve comments and formatting in the manifest when we update it with template paths from input packages
	var doc yaml.Node
	if err := yaml.Unmarshal(manifestBytes, &doc); err != nil {
		return fmt.Errorf("failed to parse data stream manifest YAML: %w", err)
	}

	manifest, err := packages.ReadDataStreamManifestBytes(manifestBytes)
	if err != nil {
		return fmt.Errorf("failed to parse data stream manifest %q: %w", manifestPath, err)
	}

	errorList := make([]error, 0)
	for idx, stream := range manifest.Streams {
		if stream.Package == "" {
			continue
		}
		pkgPath, ok := inputPkgPaths[stream.Package]
		if !ok {
			errorList = append(errorList, fmt.Errorf("failed to resolve input package %q for stream in manifest %q: not listed in requires.input", stream.Package, manifestPath))
			continue
		}
		dsRootDir := path.Dir(manifestPath)
		inputPaths, err := collectAndCopyInputPkgDataStreams(dsRootDir, pkgPath, stream.Package, buildRoot)
		if err != nil {
			return fmt.Errorf("failed to collect and copy input package data stream templates for manifest %q: %w", manifestPath, err)
		}
		if len(inputPaths) == 0 {
			continue
		}

		// current manifest template paths
		paths := make([]string, 0)
		// if composable package has included custom template path or paths, include them
		// if no template paths are included at the manifest, only the imported templates are included
		if stream.TemplatePath != "" {
			paths = append(paths, stream.TemplatePath)
		} else if len(stream.TemplatePaths) > 0 {
			paths = append(paths, stream.TemplatePaths...)
		}
		paths = append(inputPaths, paths...)

		if err := setStreamTemplatePaths(&doc, idx, paths); err != nil {
			return fmt.Errorf("failed to set stream template paths in manifest %q: %w", manifestPath, err)
		}
	}
	if err := errors.Join(errorList...); err != nil {
		return err
	}

	// Serialise the updated YAML document back to disk.
	updated, err := formatYAMLNode(&doc)
	if err != nil {
		return fmt.Errorf("failed to format updated manifest: %w", err)
	}
	if err := buildRoot.WriteFile(manifestPath, updated, 0664); err != nil {
		return fmt.Errorf("failed to write updated manifest: %w", err)
	}

	return nil
}

// collectAndCopyInputPkgDataStreams collects the data streams from the input package and copies them to the agent/stream directory of the build package
// it returns the list of copied data stream names
//
// Design note: input package templates are authored for input-level compilation, where available
// variables are: package vars + input.vars. When these templates are copied to the integration's
// data_stream/<name>/agent/stream/ directory and compiled as stream templates, Fleet compiles them
// with package vars + input.vars + stream.vars. For templates that only reference package-level
// or input-level variables this works correctly. However, stream-level vars defined on the
// integration's data stream will NOT be accessible from input package templates — the template
// content must explicitly reference them. If stream-level vars need to be rendered, add an
// integration-owned stream template and include it after the input package templates in
// template_paths (integration templates are appended last and take precedence).
// See https://github.com/elastic/elastic-package/issues/3279 for the follow-up work on
// merging variable definitions from input packages and composable packages at build time.
func collectAndCopyInputPkgDataStreams(dsRootDir, inputPkgPath, inputPkgName string, buildRoot *os.Root) ([]string, error) {
	agentStreamDir := path.Join(dsRootDir, "agent", "stream")
	return collectAndCopyPolicyTemplateFiles(inputPkgPath, inputPkgName, agentStreamDir, buildRoot)
}

func setStreamTemplatePaths(doc *yaml.Node, streamIdx int, paths []string) error {
	// Navigate: document -> mapping -> "streams" key -> sequence -> item [streamIdx]
	root := doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return fmt.Errorf("failed to set stream template paths: empty YAML document")
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("failed to set stream template paths: expected mapping node at document root")
	}

	streamsNode := mappingValue(root, "streams")
	if streamsNode == nil {
		return fmt.Errorf("failed to set stream template paths: 'streams' key not found in manifest")
	}
	if streamsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("failed to set stream template paths: 'streams' is not a sequence")
	}
	if streamIdx >= len(streamsNode.Content) {
		return fmt.Errorf("failed to set stream template paths: stream index %d out of range (len=%d)", streamIdx, len(streamsNode.Content))
	}

	streamNode := streamsNode.Content[streamIdx]
	if streamNode.Kind != yaml.MappingNode {
		return fmt.Errorf("failed to set stream template paths: stream entry %d is not a mapping", streamIdx)
	}

	// Remove singular template_path if present.
	removeKey(streamNode, "template_path")

	// Build the template_paths sequence node.
	seqNode := &yaml.Node{Kind: yaml.SequenceNode}
	for _, p := range paths {
		seqNode.Content = append(seqNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: p})
	}

	// Upsert template_paths.
	upsertKey(streamNode, "template_paths", seqNode)

	return nil
}
