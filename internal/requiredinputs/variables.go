// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
)

// promotedVarScopeKey is the lookup key for composable-side var overrides: required
// input package name plus composable data stream name ("" if the template has no data_streams).
type promotedVarScopeKey struct {
	refInputPackage      string
	composableDataStream string
}

// mergeVariables merges variable definitions from input packages into the
// composable package's manifests (package-level and data-stream-level) under
// buildRoot (manifest.yml and data_stream/*/manifest.yml).
//
// Merging rule: input package vars are the base; composable package override
// fields win when explicitly specified.
//
// Input-level vars: vars declared in policy_templates[].inputs[].vars are
// "promoted" — they become input-level variables in the merged manifest.
//
// Data-stream-level vars: all remaining (non-promoted) base vars are placed at
// the data-stream level, merged with any stream-level overrides the composable
// package declares.
func (r *RequiredInputsResolver) mergeVariables(
	manifest *packages.PackageManifest,
	inputPkgPaths map[string]string,
	buildRoot *os.Root,
) error {
	doc, err := readYAMLDocFromBuildRoot(buildRoot, "manifest.yml")
	if err != nil {
		return err
	}

	promotedVarOverridesByScope, err := buildPromotedVarOverrideMap(manifest, &doc)
	if err != nil {
		return err
	}

	if err := mergePolicyTemplateInputLevelVars(manifest, &doc, inputPkgPaths, promotedVarOverridesByScope); err != nil {
		return err
	}

	if err := writeFormattedYAMLDoc(buildRoot, "manifest.yml", &doc); err != nil {
		return err
	}

	return mergeDataStreamStreamLevelVars(buildRoot, inputPkgPaths, promotedVarOverridesByScope)
}

// readYAMLDocFromBuildRoot reads relPath from buildRoot and parses it as a YAML document node.
func readYAMLDocFromBuildRoot(buildRoot *os.Root, relPath string) (yaml.Node, error) {
	b, err := buildRoot.ReadFile(relPath)
	if err != nil {
		return yaml.Node{}, fmt.Errorf("reading %q: %w", relPath, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return yaml.Node{}, fmt.Errorf("parsing YAML %q: %w", relPath, err)
	}
	return doc, nil
}

// buildPromotedVarOverrideMap indexes composable policy_templates[].inputs[].vars
// by input package name and data stream scope for use when merging promotions.
func buildPromotedVarOverrideMap(manifest *packages.PackageManifest, doc *yaml.Node) (map[promotedVarScopeKey]map[string]*yaml.Node, error) {
	out := make(map[promotedVarScopeKey]map[string]*yaml.Node)
	for ptIdx, pt := range manifest.PolicyTemplates {
		for inputIdx, input := range pt.Inputs {
			if input.Package == "" || len(input.Vars) == 0 {
				continue
			}

			inputNode, err := getInputMappingNode(doc, ptIdx, inputIdx)
			if err != nil {
				return nil, fmt.Errorf("getting input node at pt[%d].inputs[%d]: %w", ptIdx, inputIdx, err)
			}

			overrideNodes, err := readVarNodes(inputNode)
			if err != nil {
				return nil, fmt.Errorf("reading override var nodes at pt[%d].inputs[%d]: %w", ptIdx, inputIdx, err)
			}

			overrideByName := make(map[string]*yaml.Node, len(overrideNodes))
			for _, n := range overrideNodes {
				overrideByName[varNodeName(n)] = n
			}

			dsNames := pt.DataStreams
			if len(dsNames) == 0 {
				dsNames = []string{""}
			}
			for _, dsName := range dsNames {
				out[promotedVarScopeKey{refInputPackage: input.Package, composableDataStream: dsName}] = overrideByName
			}
		}
	}
	return out, nil
}

// mergePolicyTemplateInputLevelVars writes merged promoted vars onto each
// package-backed input in the composable manifest YAML (in-memory doc).
func mergePolicyTemplateInputLevelVars(
	manifest *packages.PackageManifest,
	doc *yaml.Node,
	inputPkgPaths map[string]string,
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*yaml.Node,
) error {
	for ptIdx, pt := range manifest.PolicyTemplates {
		for inputIdx, input := range pt.Inputs {
			if input.Package == "" {
				continue
			}
			pkgPath, ok := inputPkgPaths[input.Package]
			if !ok {
				continue
			}

			baseVarOrder, baseVarByName, err := loadInputPkgVarNodes(pkgPath)
			if err != nil {
				return fmt.Errorf("loading input pkg var nodes for %q: %w", input.Package, err)
			}
			if len(baseVarOrder) == 0 {
				continue
			}

			promotedOverrides := unionPromotedOverridesForInput(pt, input.Package, promotedVarOverridesByScope)

			inputNode, err := getInputMappingNode(doc, ptIdx, inputIdx)
			if err != nil {
				return fmt.Errorf("getting input node at pt[%d].inputs[%d]: %w", ptIdx, inputIdx, err)
			}

			mergedSeq := mergeInputLevelVarNodes(baseVarOrder, baseVarByName, promotedOverrides)

			if len(mergedSeq.Content) > 0 {
				upsertKey(inputNode, "vars", mergedSeq)
			} else {
				removeKey(inputNode, "vars")
			}
		}
	}
	return nil
}

// unionPromotedOverridesForInput merges override nodes for refInputPackage across
// every data stream listed on the policy template (or "" if none listed).
func unionPromotedOverridesForInput(
	pt packages.PolicyTemplate,
	refInputPackage string,
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*yaml.Node,
) map[string]*yaml.Node {
	promotedOverrides := make(map[string]*yaml.Node)
	dsNames := pt.DataStreams
	if len(dsNames) == 0 {
		dsNames = []string{""}
	}
	for _, dsName := range dsNames {
		maps.Copy(promotedOverrides, promotedVarOverridesByScope[promotedVarScopeKey{
			refInputPackage:      refInputPackage,
			composableDataStream: dsName,
		}])
	}
	return promotedOverrides
}

// writeFormattedYAMLDoc serializes doc with package YAML formatting and writes it to relPath.
func writeFormattedYAMLDoc(buildRoot *os.Root, relPath string, doc *yaml.Node) error {
	updated, err := formatYAMLNode(doc)
	if err != nil {
		return fmt.Errorf("formatting updated %q: %w", relPath, err)
	}
	if err := buildRoot.WriteFile(relPath, updated, 0664); err != nil {
		return fmt.Errorf("writing updated %q: %w", relPath, err)
	}
	return nil
}

// mergeDataStreamStreamLevelVars updates stream vars in every data_stream/*/manifest.yml under buildRoot.
func mergeDataStreamStreamLevelVars(
	buildRoot *os.Root,
	inputPkgPaths map[string]string,
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*yaml.Node,
) error {
	dsManifestPaths, err := fs.Glob(buildRoot.FS(), "data_stream/*/manifest.yml")
	if err != nil {
		return fmt.Errorf("globbing data stream manifests: %w", err)
	}

	for _, manifestPath := range dsManifestPaths {
		dsName := path.Base(path.Dir(manifestPath))

		dsManifestBytes, err := buildRoot.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("reading data stream manifest %q: %w", manifestPath, err)
		}

		var dsDoc yaml.Node
		if err := yaml.Unmarshal(dsManifestBytes, &dsDoc); err != nil {
			return fmt.Errorf("parsing data stream manifest YAML %q: %w", manifestPath, err)
		}

		dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
		if err != nil {
			return fmt.Errorf("parsing data stream manifest %q: %w", manifestPath, err)
		}

		if err := mergeStreamsInDSManifest(&dsDoc, dsManifest, dsName, inputPkgPaths, promotedVarOverridesByScope, manifestPath); err != nil {
			return err
		}

		if err := writeFormattedYAMLDoc(buildRoot, manifestPath, &dsDoc); err != nil {
			return fmt.Errorf("data stream manifest %q: %w", manifestPath, err)
		}
	}

	return nil
}

// mergeStreamsInDSManifest merges non-promoted input vars into package-backed streams in one DS manifest.
func mergeStreamsInDSManifest(
	dsDoc *yaml.Node,
	dsManifest *packages.DataStreamManifest,
	dsName string,
	inputPkgPaths map[string]string,
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*yaml.Node,
	manifestPath string,
) error {
	for streamIdx, stream := range dsManifest.Streams {
		if stream.Package == "" {
			continue
		}
		pkgPath, ok := inputPkgPaths[stream.Package]
		if !ok {
			continue
		}

		baseVarOrder, baseVarByName, err := loadInputPkgVarNodes(pkgPath)
		if err != nil {
			return fmt.Errorf("loading input pkg var nodes for %q: %w", stream.Package, err)
		}
		if len(baseVarOrder) == 0 {
			continue
		}

		promotedNames := promotedVarNamesForStream(stream.Package, dsName, promotedVarOverridesByScope)

		streamNode, err := getStreamMappingNode(dsDoc, streamIdx)
		if err != nil {
			return fmt.Errorf("getting stream node at index %d in %q: %w", streamIdx, manifestPath, err)
		}

		dsOverrideNodes, err := readVarNodes(streamNode)
		if err != nil {
			return fmt.Errorf("reading DS override var nodes in %q: %w", manifestPath, err)
		}

		if err := checkDuplicateVarNodes(dsOverrideNodes); err != nil {
			return fmt.Errorf("duplicate vars in data stream manifest %q: %w", manifestPath, err)
		}

		mergedSeq := mergeStreamLevelVarNodes(baseVarOrder, baseVarByName, promotedNames, dsOverrideNodes)

		if len(mergedSeq.Content) > 0 {
			upsertKey(streamNode, "vars", mergedSeq)
		} else {
			removeKey(streamNode, "vars")
		}
	}
	return nil
}

// promotedVarNamesForStream returns the set of var names promoted for this stream:
// overrides for (refInputPackage, composableDataStream) plus template-wide (refInputPackage, "").
func promotedVarNamesForStream(
	refInputPackage, composableDataStream string,
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*yaml.Node,
) map[string]bool {
	promotedNames := make(map[string]bool)
	for _, key := range []promotedVarScopeKey{
		{refInputPackage: refInputPackage, composableDataStream: composableDataStream},
		{refInputPackage: refInputPackage, composableDataStream: ""},
	} {
		for varName := range promotedVarOverridesByScope[key] {
			promotedNames[varName] = true
		}
	}
	return promotedNames
}

// loadInputPkgVarNodes opens the input package at pkgPath, reads all vars from
// all policy templates (dedup by name, first wins) and returns them as an
// ordered slice and a name→node lookup map.
func loadInputPkgVarNodes(pkgPath string) ([]string, map[string]*yaml.Node, error) {
	pkgFS, closeFn, err := openPackageFS(pkgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening package: %w", err)
	}
	defer func() { _ = closeFn() }()

	manifestBytes, err := fs.ReadFile(pkgFS, packages.PackageManifestFile)
	if err != nil {
		return nil, nil, fmt.Errorf("reading manifest: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(manifestBytes, &doc); err != nil {
		return nil, nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}

	root := &doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, nil, nil
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("expected mapping node at document root")
	}

	policyTemplatesNode := mappingValue(root, "policy_templates")
	if policyTemplatesNode == nil || policyTemplatesNode.Kind != yaml.SequenceNode {
		return nil, nil, nil
	}

	order := make([]string, 0)
	byName := make(map[string]*yaml.Node)

	for _, ptNode := range policyTemplatesNode.Content {
		if ptNode.Kind != yaml.MappingNode {
			continue
		}
		varsNode := mappingValue(ptNode, "vars")
		if varsNode == nil || varsNode.Kind != yaml.SequenceNode {
			continue
		}
		for _, varNode := range varsNode.Content {
			if varNode.Kind != yaml.MappingNode {
				continue
			}
			name := varNodeName(varNode)
			if name == "" || byName[name] != nil {
				continue // skip empty names and duplicates (first wins)
			}
			order = append(order, name)
			byName[name] = varNode
		}
	}

	return order, byName, nil
}

// mergeInputLevelVarNodes returns a sequence node containing only the promoted
// vars (those in promotedOverrides), each merged with the override fields.
// Order follows baseVarOrder (input package declaration order).
func mergeInputLevelVarNodes(
	baseVarOrder []string,
	baseVarByName map[string]*yaml.Node,
	promotedOverrides map[string]*yaml.Node,
) *yaml.Node {
	seqNode := &yaml.Node{Kind: yaml.SequenceNode}
	for _, varName := range baseVarOrder {
		overrideNode, promoted := promotedOverrides[varName]
		if !promoted {
			continue
		}
		merged := mergeVarNode(baseVarByName[varName], overrideNode)
		seqNode.Content = append(seqNode.Content, merged)
	}
	return seqNode
}

// mergeStreamLevelVarNodes returns a sequence node containing:
//  1. Non-promoted base vars (in input package order), merged with any DS
//     override where names match.
//  2. Novel DS vars (names not in baseVarByName) appended in their declaration
//     order.
func mergeStreamLevelVarNodes(
	baseVarOrder []string,
	baseVarByName map[string]*yaml.Node,
	promotedNames map[string]bool,
	dsOverrides []*yaml.Node,
) *yaml.Node {
	dsOverrideByName := make(map[string]*yaml.Node, len(dsOverrides))
	for _, v := range dsOverrides {
		dsOverrideByName[varNodeName(v)] = v
	}

	seqNode := &yaml.Node{Kind: yaml.SequenceNode}

	// Non-promoted base vars first (in input pkg order).
	for _, varName := range baseVarOrder {
		if promotedNames[varName] {
			continue
		}
		baseNode := baseVarByName[varName]
		overrideNode, hasOverride := dsOverrideByName[varName]
		var merged *yaml.Node
		if hasOverride {
			merged = mergeVarNode(baseNode, overrideNode)
		} else {
			merged = cloneNode(baseNode)
		}
		seqNode.Content = append(seqNode.Content, merged)
	}

	// Novel DS vars (not present in base) appended in declaration order.
	for _, v := range dsOverrides {
		if _, inBase := baseVarByName[varNodeName(v)]; !inBase {
			seqNode.Content = append(seqNode.Content, cloneNode(v))
		}
	}

	return seqNode
}

// mergeVarNode merges fields from overrideNode into a clone of baseNode.
// All keys in override win; absent keys in override are inherited from base.
// The "name" key is always preserved from base.
func mergeVarNode(base, override *yaml.Node) *yaml.Node {
	result := cloneNode(base)
	for i := 0; i+1 < len(override.Content); i += 2 {
		keyNode := override.Content[i]
		valNode := override.Content[i+1]
		if keyNode.Value == "name" {
			continue // always preserve name from base
		}
		upsertKey(result, keyNode.Value, cloneNode(valNode))
	}
	return result
}

// checkDuplicateVarNodes returns an error if any var name appears more than
// once in the provided nodes.
func checkDuplicateVarNodes(varNodes []*yaml.Node) error {
	seen := make(map[string]bool, len(varNodes))
	for _, v := range varNodes {
		name := varNodeName(v)
		if seen[name] {
			return fmt.Errorf("duplicate variable %q", name)
		}
		seen[name] = true
	}
	return nil
}

// varNodeName extracts the value of the "name" key from a var mapping node.
func varNodeName(v *yaml.Node) string {
	nameVal := mappingValue(v, "name")
	if nameVal == nil {
		return ""
	}
	return nameVal.Value
}

// readVarNodes extracts the individual var mapping nodes from the "vars"
// sequence of the given mapping node. Returns nil if no "vars" key is present.
func readVarNodes(mappingNode *yaml.Node) ([]*yaml.Node, error) {
	varsNode := mappingValue(mappingNode, "vars")
	if varsNode == nil {
		return nil, nil
	}
	if varsNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("'vars' is not a sequence node")
	}
	result := make([]*yaml.Node, 0, len(varsNode.Content))
	for _, item := range varsNode.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("var entry is not a mapping node")
		}
		result = append(result, item)
	}
	return result, nil
}

// getInputMappingNode navigates to policy_templates[ptIdx].inputs[inputIdx] in
// the given YAML document and returns the input mapping node.
func getInputMappingNode(doc *yaml.Node, ptIdx, inputIdx int) (*yaml.Node, error) {
	root := doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, fmt.Errorf("empty YAML document")
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node at document root")
	}

	ptsNode := mappingValue(root, "policy_templates")
	if ptsNode == nil || ptsNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("'policy_templates' not found or not a sequence")
	}
	if ptIdx < 0 || ptIdx >= len(ptsNode.Content) {
		return nil, fmt.Errorf("policy template index %d out of range (len=%d)", ptIdx, len(ptsNode.Content))
	}

	ptNode := ptsNode.Content[ptIdx]
	if ptNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("policy template %d is not a mapping", ptIdx)
	}

	inputsNode := mappingValue(ptNode, "inputs")
	if inputsNode == nil || inputsNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("'inputs' not found or not a sequence in policy template %d", ptIdx)
	}
	if inputIdx < 0 || inputIdx >= len(inputsNode.Content) {
		return nil, fmt.Errorf("input index %d out of range (len=%d)", inputIdx, len(inputsNode.Content))
	}

	inputNode := inputsNode.Content[inputIdx]
	if inputNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("input %d is not a mapping", inputIdx)
	}

	return inputNode, nil
}

// getStreamMappingNode navigates to streams[streamIdx] in the given YAML
// document and returns the stream mapping node.
func getStreamMappingNode(doc *yaml.Node, streamIdx int) (*yaml.Node, error) {
	root := doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, fmt.Errorf("empty YAML document")
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected mapping node at document root")
	}

	streamsNode := mappingValue(root, "streams")
	if streamsNode == nil || streamsNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("'streams' not found or not a sequence")
	}
	if streamIdx < 0 || streamIdx >= len(streamsNode.Content) {
		return nil, fmt.Errorf("stream index %d out of range (len=%d)", streamIdx, len(streamsNode.Content))
	}

	streamNode := streamsNode.Content[streamIdx]
	if streamNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("stream %d is not a mapping", streamIdx)
	}

	return streamNode, nil
}
