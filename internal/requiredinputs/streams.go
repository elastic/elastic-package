// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

func (r *RequiredInputsResolver) bundleDataStreamTemplates(inputPkgPaths map[string]string, buildRoot *os.Root) error {
	// get all data stream manifest paths in the build package
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
		// parse the manifest YAML document preserving formatting for targeted modifications
		// using manifestBytes allows us to preserve comments and formatting in the manifest when we update it with template paths from input packages
		var doc yaml.Node
		if err := yaml.Unmarshal(manifestBytes, &doc); err != nil {
			return fmt.Errorf("parsing manifest YAML: %w", err)
		}

		manifest, err := packages.ReadDataStreamManifestBytes(manifestBytes)
		if err != nil {
			return fmt.Errorf("parsing data stream manifest %q: %w", manifestPath, err)
		}
		for idx, stream := range manifest.Streams {
			if stream.Package == "" {
				continue
			}
			pkgPath, ok := inputPkgPaths[stream.Package]
			if !ok {
				errorList = append(errorList, fmt.Errorf("stream in manifest %q references input package %q which is not listed in requires.input", manifestPath, stream.Package))
				continue
			}
			dsRootDir := filepath.Dir(manifestPath)
			inputPaths, err := r.collectAndCopyInputPkgDataStreams(dsRootDir, pkgPath, stream.Package, buildRoot)
			if err != nil {
				return fmt.Errorf("collecting and copying input package data stream templates for manifest %q: %w", manifestPath, err)
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
				return fmt.Errorf("setting stream template paths in manifest %q: %w", manifestPath, err)
			}

		}

		// Serialise the updated YAML document back to disk.
		updated, err := formatYAMLNode(&doc)
		if err != nil {
			return fmt.Errorf("formatting updated manifest: %w", err)
		}
		if err := buildRoot.WriteFile(manifestPath, updated, 0664); err != nil {
			return fmt.Errorf("writing updated manifest: %w", err)
		}

	}
	return errors.Join(errorList...)
}

// collectAndCopyInputPkgDataStreams collects the data streams from the input package and copies them to the agent/input directory of the build package
// it returns the list of copied data stream names
func (r *RequiredInputsResolver) collectAndCopyInputPkgDataStreams(dsRootDir, inputPkgPath, inputPkgName string, buildRoot *os.Root) ([]string, error) {
	inputPkgFS, closeFn, err := openPackageFS(inputPkgPath)
	if err != nil {
		return nil, fmt.Errorf("opening package %q: %w", inputPkgPath, err)
	}
	defer closeFn()

	manifestBytes, err := fs.ReadFile(inputPkgFS, "manifest.yml")
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	seen := make(map[string]bool)
	copiedNames := make([]string, 0)
	for _, pt := range manifest.PolicyTemplates {
		var names []string
		switch {
		case len(pt.TemplatePaths) > 0:
			names = pt.TemplatePaths
		case pt.TemplatePath != "":
			names = []string{pt.TemplatePath}
		}
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			// copy the template from "agent/input" directory of the input package to the "agent/stream" directory of the build package
			content, err := fs.ReadFile(inputPkgFS, filepath.Join("agent", "input", name))
			if err != nil {
				return nil, fmt.Errorf("template %q declared in manifest not found in agent/input: %w", name, err)
			}
			destName := inputPkgName + "-" + name
			// create the agent/stream directory if it doesn't exist
			agentStreamDir := filepath.Join(dsRootDir, "agent", "stream")
			if err := buildRoot.MkdirAll(agentStreamDir, 0755); err != nil {
				return nil, fmt.Errorf("creating agent/stream directory: %w", err)
			}
			destPath := filepath.Join(agentStreamDir, destName)
			if err := buildRoot.WriteFile(destPath, content, 0644); err != nil {
				return nil, fmt.Errorf("writing template %s: %w", destName, err)
			}
			logger.Debugf("Copied input package template: %s -> %s", name, destName)
			copiedNames = append(copiedNames, destName)
		}
	}
	return copiedNames, nil
}

func setStreamTemplatePaths(doc *yaml.Node, streamIdx int, paths []string) error {
	// Navigate: document -> mapping -> "streams" key -> sequence -> item [streamIdx]
	root := doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return fmt.Errorf("empty YAML document")
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node at document root")
	}

	streamsNode := mappingValue(root, "streams")
	if streamsNode == nil {
		return fmt.Errorf("'streams' key not found in manifest")
	}
	if streamsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("'streams' is not a sequence")
	}
	if streamIdx >= len(streamsNode.Content) {
		return fmt.Errorf("stream index %d out of range (len=%d)", streamIdx, len(streamsNode.Content))
	}

	streamNode := streamsNode.Content[streamIdx]
	if streamNode.Kind != yaml.MappingNode {
		return fmt.Errorf("stream entry %d is not a mapping", streamIdx)
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
