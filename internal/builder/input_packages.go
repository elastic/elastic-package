// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/registry"
)

// bundleInputPackageTemplates copies templates from required input packages into
// the built integration package and updates data stream manifests with
// template_paths so Fleet can merge them in the correct order.
//
// Template ordering rule: input package templates are listed first so that
// the integration's own template (rendered last) takes precedence.
//
// Input packages are downloaded from the Elastic Package Registry by default.
// A local directory can be used instead by passing pre-merged overrides
// (typically from _dev/test/config.yml, resolved by the test runner).
func bundleInputPackageTemplates(packageRoot string, buildPackageRoot string, registryURL string, overrides map[string]packages.RequiresOverride) error {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(buildPackageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest: %w", err)
	}

	if manifest.Type != "integration" {
		return nil
	}

	if manifest.Requires == nil || len(manifest.Requires.Input) == 0 {
		logger.Debug("Package has no required input packages, skipping template bundling")
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "elastic-package-input-pkgs-*")
	if err != nil {
		return fmt.Errorf("creating temp directory for input packages: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if registryURL == "" {
		registryURL = registry.ProductionURL
	}
	eprClient := registry.NewClient(registryURL)

	inputPkgPaths := make(map[string]string, len(manifest.Requires.Input))
	for _, dep := range manifest.Requires.Input {
		pkgPath, err := resolveInputPackage(dep, overrides, packageRoot, eprClient, tmpDir)
		if err != nil {
			return fmt.Errorf("resolving required input package %q: %w", dep.Package, err)
		}
		inputPkgPaths[dep.Package] = pkgPath
		logger.Debugf("Resolved input package %q at %s", dep.Package, pkgPath)
	}

	// Walk every data stream in the built package.
	dsManifestPaths, err := filepath.Glob(filepath.Join(buildPackageRoot, "data_stream", "*", packages.DataStreamManifestFile))
	if err != nil {
		return fmt.Errorf("listing data stream manifests: %w", err)
	}

	for _, dsManifestPath := range dsManifestPaths {
		if err := processDSManifest(dsManifestPath, inputPkgPaths); err != nil {
			return fmt.Errorf("processing data stream manifest %s: %w", dsManifestPath, err)
		}
	}

	return nil
}

// resolveInputPackage resolves a single input package dependency to its on-disk
// location. When a source override is present the local directory is used;
// otherwise the package is downloaded from the registry.
func resolveInputPackage(
	dep packages.PackageDependency,
	overrides map[string]packages.RequiresOverride,
	packageRoot string,
	eprClient *registry.Client,
	tmpDir string,
) (string, error) {
	if ovr, ok := overrides[dep.Package]; ok && ovr.Source != "" {
		src := ovr.Source
		if !filepath.IsAbs(src) {
			src = filepath.Join(packageRoot, src)
		}
		info, err := os.Stat(src)
		if err != nil {
			return "", fmt.Errorf("source override directory %q: %w", src, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("source override %q is not a directory", src)
		}
		logger.Debugf("Using local source override for input package %q: %s", dep.Package, src)
		return src, nil
	}

	version := dep.Version
	if ovr, ok := overrides[dep.Package]; ok && ovr.Version != "" {
		version = ovr.Version
	}

	pkgPath, err := eprClient.DownloadPackage(dep.Package, version, tmpDir)
	if err != nil {
		return "", fmt.Errorf("downloading from registry: %w", err)
	}
	return pkgPath, nil
}

// processDSManifest handles one data stream manifest file: for each stream
// entry that references an input package, it copies the templates and rewrites
// the manifest to use template_paths.
func processDSManifest(dsManifestPath string, inputPkgPaths map[string]string) error {
	dsManifest, err := packages.ReadDataStreamManifest(dsManifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	// Check whether any stream references an input package at all.
	var hasRef bool
	for _, s := range dsManifest.Streams {
		if s.PackageRef != "" {
			hasRef = true
			break
		}
	}
	if !hasRef {
		return nil
	}

	dsRoot := filepath.Dir(dsManifestPath)
	agentStreamDir := filepath.Join(dsRoot, "agent", "stream")

	// Parse the YAML document preserving formatting for targeted modifications.
	raw, err := os.ReadFile(dsManifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest file: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing manifest YAML: %w", err)
	}

	for i, stream := range dsManifest.Streams {
		if stream.PackageRef == "" {
			continue
		}

		pkgPath, ok := inputPkgPaths[stream.PackageRef]
		if !ok {
			return fmt.Errorf("stream references input package %q which is not listed in requires.input", stream.PackageRef)
		}

		// Collect template files from the input package.
		inputPkgTemplates, err := collectInputPkgTemplates(pkgPath, stream.PackageRef)
		if err != nil {
			return fmt.Errorf("collecting templates from input package %q: %w", stream.PackageRef, err)
		}

		// Ensure the target directory exists.
		if err := os.MkdirAll(agentStreamDir, 0755); err != nil {
			return fmt.Errorf("creating agent/stream directory: %w", err)
		}

		// Copy each template file into data_stream/<ds>/agent/stream/.
		var copiedNames []string
		for _, tmpl := range inputPkgTemplates {
			destPath := filepath.Join(agentStreamDir, tmpl.destName)
			if err := sh.Copy(destPath, tmpl.srcPath); err != nil {
				return fmt.Errorf("copying template %s to %s: %w", tmpl.srcPath, destPath, err)
			}
			copiedNames = append(copiedNames, tmpl.destName)
			logger.Debugf("Copied input package template: %s -> %s", tmpl.srcPath, destPath)
		}

		// Build the ordered template_paths list:
		//   1. Input package templates (copied, in original order)
		//   2. Integration's own template(s)
		var integrationTemplates []string
		switch {
		case len(stream.TemplatePaths) > 0:
			integrationTemplates = stream.TemplatePaths
		case stream.TemplatePath != "":
			integrationTemplates = []string{stream.TemplatePath}
		}

		allPaths := append(copiedNames, integrationTemplates...)

		// Rewrite the YAML node for this stream entry.
		if err := setStreamTemplatePaths(&doc, i, allPaths); err != nil {
			return fmt.Errorf("updating template_paths in manifest YAML (stream %d): %w", i, err)
		}
	}

	// Serialise the updated YAML document back to disk.
	updated, err := formatYAMLNode(&doc)
	if err != nil {
		return fmt.Errorf("formatting updated manifest: %w", err)
	}
	if err := os.WriteFile(dsManifestPath, updated, 0664); err != nil {
		return fmt.Errorf("writing updated manifest: %w", err)
	}

	return nil
}

// inputPkgTemplate pairs the source path of a template file inside an input
// package with the prefixed destination filename it will be given inside the
// integration's data_stream/<ds>/agent/stream/ directory.
type inputPkgTemplate struct {
	srcPath  string
	destName string
}

// collectInputPkgTemplates returns the ordered list of template files declared
// by the input package in its policy_templates entries (template_path /
// template_paths). Only files explicitly listed in the manifest are included,
// preserving their declared order. Destination filenames are prefixed with
// "<packageName>-" to prevent collisions with integration-owned templates.
func collectInputPkgTemplates(pkgPath, pkgName string) ([]inputPkgTemplate, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	inputDir := filepath.Join(pkgPath, "agent", "input")

	seen := make(map[string]bool)
	var result []inputPkgTemplate
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
			src := filepath.Join(inputDir, name)
			if _, err := os.Stat(src); err != nil {
				return nil, fmt.Errorf("template %q declared in manifest not found in agent/input: %w", name, err)
			}
			result = append(result, inputPkgTemplate{
				srcPath:  src,
				destName: pkgName + "-" + name,
			})
		}
	}
	return result, nil
}

// setStreamTemplatePaths rewrites the YAML node for stream at index streamIdx
// so that it has template_paths set to the given paths, and template_path
// (singular) is removed.
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

// mappingValue returns the value node for the given key in a YAML mapping node,
// or nil if the key is not present.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// removeKey removes a key-value pair from a YAML mapping node.
func removeKey(node *yaml.Node, key string) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

// upsertKey sets key to value in a YAML mapping node, adding it if absent.
func upsertKey(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	node.Content = append(node.Content, keyNode, value)
}

func formatYAMLNode(doc *yaml.Node) ([]byte, error) {
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshalling YAML: %w", err)
	}
	yamlFormatter := formatter.NewYAMLFormatter(formatter.KeysWithDotActionNone)
	formatted, _, err := yamlFormatter.Format(raw)
	if err != nil {
		return nil, fmt.Errorf("formatting YAML: %w", err)
	}
	return formatted, nil
}
