// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"
	"os"
	"path"

	"github.com/goccy/go-yaml/ast"

	"github.com/elastic/elastic-package/internal/packages"
)

func (r *RequiredInputsResolver) bundlePolicyTemplatesInputPackageTemplates(manifestBytes []byte, manifest *packages.PackageManifest, inputPkgPaths map[string]string, buildRoot *os.Root) error {

	// parse the manifest YAML document preserving formatting for targeted modifications
	// using manifestBytes allows us to preserve comments and formatting in the manifest when we update it with template paths from input packages
	root, err := parseDocumentRootMapping(manifestBytes)
	if err != nil {
		return fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	// for each policy template, with an input package reference:
	// collect the templates from the input package and copy them to the agent/input directory of the build package
	// then update the policy template manifest to include the copied templates as template_paths
	for ptIdx, pt := range manifest.PolicyTemplates {
		for inputIdx, input := range pt.Inputs {
			if input.Package == "" {
				continue
			}
			sourcePath, ok := inputPkgPaths[input.Package]
			if !ok || sourcePath == "" {
				return fmt.Errorf("failed to find input package %q referenced by policy template %q", input.Package, pt.Name)
			}
			inputPaths, err := collectAndCopyInputPkgPolicyTemplates(sourcePath, input.Package, buildRoot)
			if err != nil {
				return fmt.Errorf("failed to collect and copy input package policy templates: %w", err)
			}
			if len(inputPaths) == 0 {
				continue
			}

			// current manifest template paths
			paths := make([]string, 0)
			// if composable package has included custom template path or paths, include them
			// if no template paths are included at the manifest, only the imported templates are included
			if input.TemplatePath != "" {
				paths = append(paths, input.TemplatePath)
			} else if len(input.TemplatePaths) > 0 {
				paths = append(paths, input.TemplatePaths...)
			}
			paths = append(inputPaths, paths...)

			if err := setInputPolicyTemplateTemplatePaths(root, ptIdx, inputIdx, paths); err != nil {
				return fmt.Errorf("failed to update policy template manifest with input package templates: %w", err)
			}
		}
	}

	// Serialise the updated YAML document back to disk.
	updated, err := formatYAMLNode(root)
	if err != nil {
		return fmt.Errorf("failed to format updated manifest: %w", err)
	}
	if err := buildRoot.WriteFile("manifest.yml", updated, 0664); err != nil {
		return fmt.Errorf("failed to write updated manifest: %w", err)
	}

	return nil
}

// collectAndCopyInputPkgPolicyTemplates collects the templates from the input package and copies them to the agent/input directory of the build package
// it returns the list of copied template names
func collectAndCopyInputPkgPolicyTemplates(inputPkgPath, inputPkgName string, buildRoot *os.Root) ([]string, error) {
	return collectAndCopyPolicyTemplateFiles(inputPkgPath, inputPkgName, path.Join("agent", "input"), buildRoot)
}

// setInputPolicyTemplateTemplatePaths updates the manifest YAML root mapping to
// set template_paths for the specified policy template input.
func setInputPolicyTemplateTemplatePaths(root *ast.MappingNode, policyTemplatesIdx int, inputIdx int, paths []string) error {
	// Navigate: root mapping -> "policy_templates" -> sequence -> item [policyTemplatesIdx] -> mapping -> "inputs" -> sequence -> item [inputIdx] -> input mapping.
	policyTemplatesNode, ok := mappingValue(root, "policy_templates").(*ast.SequenceNode)
	if !ok {
		return fmt.Errorf("failed to set policy template input paths: 'policy_templates' key not found in manifest")
	}
	if policyTemplatesIdx < 0 || policyTemplatesIdx >= len(policyTemplatesNode.Values) {
		return fmt.Errorf("failed to set policy template input paths: policy template index %d out of range (len=%d)", policyTemplatesIdx, len(policyTemplatesNode.Values))
	}

	policyTemplateNode, ok := policyTemplatesNode.Values[policyTemplatesIdx].(*ast.MappingNode)
	if !ok {
		return fmt.Errorf("failed to set policy template input paths: policy template entry %d is not a mapping", policyTemplatesIdx)
	}

	inputsNode, ok := mappingValue(policyTemplateNode, "inputs").(*ast.SequenceNode)
	if !ok {
		return fmt.Errorf("failed to set policy template input paths: 'inputs' key not found in policy template %d", policyTemplatesIdx)
	}
	if inputIdx < 0 || inputIdx >= len(inputsNode.Values) {
		return fmt.Errorf("failed to set policy template input paths: input index %d out of range (len=%d)", inputIdx, len(inputsNode.Values))
	}

	inputNode, ok := inputsNode.Values[inputIdx].(*ast.MappingNode)
	if !ok {
		return fmt.Errorf("failed to set policy template input paths: input entry %d is not a mapping", inputIdx)
	}

	// Remove singular template_path if present.
	removeKey(inputNode, "template_path")

	// Build the template_paths sequence node.
	seqNode := newSeqNode()
	for _, p := range paths {
		seqNode.Values = append(seqNode.Values, strVal(p))
	}

	// Upsert template_paths on the input node.
	upsertKey(inputNode, "template_paths", seqNode)

	return nil
}
