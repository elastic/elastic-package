// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

func (r *RequiredInputsResolver) bundlePolicyTemplatesInputPackageTemplates(manifestBytes []byte, manifest *packages.PackageManifest, inputPkgPaths map[string]string, buildRoot *os.Root) error {

	// parse the manifest YAML document preserving formatting for targeted modifications
	// using manifestBytes allows us to preserve comments and formatting in the manifest when we update it with template paths from input packages
	var doc yaml.Node
	if err := yaml.Unmarshal(manifestBytes, &doc); err != nil {
		return fmt.Errorf("parsing manifest YAML: %w", err)
	}

	// for each policy template, with an input package reference:
	// collect the templates from the input package and copy them to the agent/input directory of the build package
	// then update the policy template manifest to include the copied templates as template_paths
	for ptIdx, pt := range manifest.PolicyTemplates {
		for inputIdx, input := range pt.Inputs {
			if input.PackageRef == "" {
				continue
			}
			sourcePath, ok := inputPkgPaths[input.PackageRef]
			if !ok || sourcePath == "" {
				return fmt.Errorf("input package %q referenced by policy template %q not found", input.PackageRef, pt.Name)
			}
			inputPaths, err := r.collectAndCopyInputPkgPolicyTemplates(sourcePath, input.PackageRef, buildRoot)
			if err != nil {
				return fmt.Errorf("collecting and copying input package policy templates: %w", err)
			}
			if len(inputPaths) == 0 {
				continue
			}

			// current manifest template paths
			paths := make([]string, 0)
			if input.TemplatePath != "" {
				// include the existing template path if present, as the input package templates are in addition to any existing template rather than replacing it
				paths = append(paths, input.TemplatePath)
			} else if len(input.TemplatePaths) > 0 {
				paths = append(paths, input.TemplatePaths...)
			} else if input.TemplatePath == "" {
				// default input.yml.hbs
				defaultTemplateFile := "input.yml.hbs"
				if _, err := buildRoot.ReadFile(filepath.Join("agent", "input", defaultTemplateFile)); err == nil {
					paths = append(paths, defaultTemplateFile)
				}
			}
			paths = append(inputPaths, paths...)

			if err := setInputPolicyTemplateTemplatePaths(&doc, ptIdx, inputIdx, paths); err != nil {
				return fmt.Errorf("updating policy template manifest with input package templates: %w", err)
			}
		}
	}

	// Serialise the updated YAML document back to disk.
	updated, err := formatYAMLNode(&doc)
	if err != nil {
		return fmt.Errorf("formatting updated manifest: %w", err)
	}
	if err := buildRoot.WriteFile("manifest.yml", updated, 0664); err != nil {
		return fmt.Errorf("writing updated manifest: %w", err)
	}

	return nil
}

// collectAndCopyInputPkgPolicyTemplates collects the templates from the input package and copies them to the agent/input directory of the build package
// it returns the list of copied template names
func (r *RequiredInputsResolver) collectAndCopyInputPkgPolicyTemplates(inputPkgPath, inputPkgName string, buildRoot *os.Root) ([]string, error) {
	inputPkgFS, closeFn, err := openPackageFS(inputPkgPath)
	if err != nil {
		return nil, fmt.Errorf("opening package %q: %w", inputPkgPath, err)
	}
	defer closeFn()

	manifestBytes, err := fs.ReadFile(inputPkgFS, packages.PackageManifestFile)
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
			// copy the template from "agent/input" directory of the input package to the "agent/input" directory of the build package
			content, err := fs.ReadFile(inputPkgFS, filepath.Join("agent", "input", name))
			if err != nil {
				return nil, fmt.Errorf("template %q declared in manifest not found in agent/input: %w", name, err)
			}
			destName := inputPkgName + "-" + name
			// create the agent/input directory if it doesn't exist
			agentInputDir := filepath.Join("agent", "input")
			if err := buildRoot.MkdirAll(agentInputDir, 0755); err != nil {
				return nil, fmt.Errorf("creating agent/input directory: %w", err)
			}
			destPath := filepath.Join(agentInputDir, destName)
			if err := buildRoot.WriteFile(destPath, content, 0644); err != nil {
				return nil, fmt.Errorf("writing template %s: %w", destName, err)
			}
			logger.Debugf("Copied input package template: %s -> %s", name, destName)
			copiedNames = append(copiedNames, destName)
		}
	}
	return copiedNames, nil
}

// setInputPolicyTemplateTemplatePaths updates the manifest YAML document to set the template_paths for the specified policy template input to the provided paths
func setInputPolicyTemplateTemplatePaths(doc *yaml.Node, policyTemplatesIdx int, inputIdx int, paths []string) error {
	// Navigate: document -> root mapping -> "policy_templates" -> sequence -> item [policyTemplatesIdx] -> mapping -> "inputs" -> sequence -> item [inputIdx] -> input mapping.
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

	// policy_templates:
	// - inputs:
	//   - template_path: foo
	policyTemplatesNode := mappingValue(root, "policy_templates")
	if policyTemplatesNode == nil {
		return fmt.Errorf("'policy_templates' key not found in manifest")
	}
	if policyTemplatesNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("'policy_templates' is not a sequence")
	}
	if policyTemplatesIdx < 0 || policyTemplatesIdx >= len(policyTemplatesNode.Content) {
		return fmt.Errorf("policy template index %d out of range (len=%d)", policyTemplatesIdx, len(policyTemplatesNode.Content))
	}

	policyTemplateNode := policyTemplatesNode.Content[policyTemplatesIdx]
	if policyTemplateNode.Kind != yaml.MappingNode {
		return fmt.Errorf("policy template entry %d is not a mapping", policyTemplatesIdx)
	}

	inputsNode := mappingValue(policyTemplateNode, "inputs")
	if inputsNode == nil {
		return fmt.Errorf("'inputs' key not found in policy template %d", policyTemplatesIdx)
	}
	if inputsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("'inputs' is not a sequence")
	}
	if inputIdx < 0 || inputIdx >= len(inputsNode.Content) {
		return fmt.Errorf("input index %d out of range (len=%d)", inputIdx, len(inputsNode.Content))
	}

	inputNode := inputsNode.Content[inputIdx]
	if inputNode.Kind != yaml.MappingNode {
		return fmt.Errorf("input entry %d is not a mapping", inputIdx)
	}

	// Remove singular template_path if present.
	removeKey(inputNode, "template_path")

	// Build the template_paths sequence node.
	seqNode := &yaml.Node{Kind: yaml.SequenceNode}
	for _, p := range paths {
		seqNode.Content = append(seqNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: p})
	}

	// Upsert template_paths on the input node.
	upsertKey(inputNode, "template_paths", seqNode)

	return nil
}
