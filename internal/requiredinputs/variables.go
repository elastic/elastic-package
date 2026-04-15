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

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

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
	root, err := readYAMLDocFromBuildRoot(buildRoot, "manifest.yml")
	if err != nil {
		return err
	}

	promotedVarOverridesByScope, err := buildPromotedVarOverrideMap(manifest, root)
	if err != nil {
		return err
	}

	if err := mergePolicyTemplateInputLevelVars(manifest, root, inputPkgPaths, promotedVarOverridesByScope); err != nil {
		return err
	}

	if err := writeFormattedYAMLDoc(buildRoot, "manifest.yml", root); err != nil {
		return err
	}

	return mergeDataStreamStreamLevelVars(buildRoot, inputPkgPaths, promotedVarOverridesByScope)
}

// readYAMLDocFromBuildRoot reads relPath from buildRoot, parses it via yamledit,
// and returns the document root as a *ast.MappingNode.
func readYAMLDocFromBuildRoot(buildRoot *os.Root, relPath string) (*ast.MappingNode, error) {
	b, err := buildRoot.ReadFile(relPath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", relPath, err)
	}
	root, err := parseDocumentRootMapping(b)
	if err != nil {
		return nil, fmt.Errorf("parsing YAML %q: %w", relPath, err)
	}
	return root, nil
}

// buildPromotedVarOverrideMap indexes composable policy_templates[].inputs[].vars
// by input package name and data stream scope for use when merging promotions.
func buildPromotedVarOverrideMap(manifest *packages.PackageManifest, root *ast.MappingNode) (map[promotedVarScopeKey]map[string]*ast.MappingNode, error) {
	out := make(map[promotedVarScopeKey]map[string]*ast.MappingNode)
	for ptIdx, pt := range manifest.PolicyTemplates {
		for inputIdx, input := range pt.Inputs {
			if input.Package == "" || len(input.Vars) == 0 {
				continue
			}

			inputNode, err := getInputMappingNode(root, ptIdx, inputIdx)
			if err != nil {
				return nil, fmt.Errorf("getting input node at pt[%d].inputs[%d]: %w", ptIdx, inputIdx, err)
			}

			overrideNodes, err := readVarNodes(inputNode)
			if err != nil {
				return nil, fmt.Errorf("reading override var nodes at pt[%d].inputs[%d]: %w", ptIdx, inputIdx, err)
			}

			overrideByName := make(map[string]*ast.MappingNode, len(overrideNodes))
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
// package-backed input in the composable manifest YAML (in-memory root mapping).
func mergePolicyTemplateInputLevelVars(
	manifest *packages.PackageManifest,
	root *ast.MappingNode,
	inputPkgPaths map[string]string,
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*ast.MappingNode,
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

			inputNode, err := getInputMappingNode(root, ptIdx, inputIdx)
			if err != nil {
				return fmt.Errorf("getting input node at pt[%d].inputs[%d]: %w", ptIdx, inputIdx, err)
			}

			mergedSeq := mergeInputLevelVarNodes(baseVarOrder, baseVarByName, promotedOverrides)

			if len(mergedSeq.Values) > 0 {
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
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*ast.MappingNode,
) map[string]*ast.MappingNode {
	promotedOverrides := make(map[string]*ast.MappingNode)
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

// writeFormattedYAMLDoc serializes root with package YAML formatting and writes it to relPath.
func writeFormattedYAMLDoc(buildRoot *os.Root, relPath string, root *ast.MappingNode) error {
	updated, err := formatYAMLNode(root)
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
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*ast.MappingNode,
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

		dsRoot, err := parseDocumentRootMapping(dsManifestBytes)
		if err != nil {
			return fmt.Errorf("parsing data stream manifest YAML %q: %w", manifestPath, err)
		}

		dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
		if err != nil {
			return fmt.Errorf("parsing data stream manifest %q: %w", manifestPath, err)
		}

		if err := mergeStreamsInDSManifest(dsRoot, dsManifest, dsName, inputPkgPaths, promotedVarOverridesByScope, manifestPath); err != nil {
			return err
		}

		if err := writeFormattedYAMLDoc(buildRoot, manifestPath, dsRoot); err != nil {
			return fmt.Errorf("data stream manifest %q: %w", manifestPath, err)
		}
	}

	return nil
}

// mergeStreamsInDSManifest merges non-promoted input vars into package-backed streams in one DS manifest.
func mergeStreamsInDSManifest(
	dsRoot *ast.MappingNode,
	dsManifest *packages.DataStreamManifest,
	dsName string,
	inputPkgPaths map[string]string,
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*ast.MappingNode,
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

		streamNode, err := getStreamMappingNode(dsRoot, streamIdx)
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

		if len(mergedSeq.Values) > 0 {
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
	promotedVarOverridesByScope map[promotedVarScopeKey]map[string]*ast.MappingNode,
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
func loadInputPkgVarNodes(pkgPath string) ([]string, map[string]*ast.MappingNode, error) {
	pkgFS, closeFn, err := openPackageFS(pkgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening package: %w", err)
	}
	defer func() { _ = closeFn() }()

	manifestBytes, err := fs.ReadFile(pkgFS, packages.PackageManifestFile)
	if err != nil {
		return nil, nil, fmt.Errorf("reading manifest: %w", err)
	}

	f, err := parser.ParseBytes(manifestBytes, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}
	if len(f.Docs) == 0 || f.Docs[0] == nil {
		return nil, nil, nil
	}
	root, ok := f.Docs[0].Body.(*ast.MappingNode)
	if !ok {
		return nil, nil, fmt.Errorf("expected mapping node at document root")
	}

	policyTemplatesNode, ok := mappingValue(root, "policy_templates").(*ast.SequenceNode)
	if !ok {
		return nil, nil, nil
	}

	order := make([]string, 0)
	byName := make(map[string]*ast.MappingNode)

	for _, ptNode := range policyTemplatesNode.Values {
		ptMapping, ok := ptNode.(*ast.MappingNode)
		if !ok {
			continue
		}
		varsNode, ok := mappingValue(ptMapping, "vars").(*ast.SequenceNode)
		if !ok {
			continue
		}
		for _, varNode := range varsNode.Values {
			varMapping, ok := varNode.(*ast.MappingNode)
			if !ok {
				continue
			}
			name := varNodeName(varMapping)
			if name == "" || byName[name] != nil {
				continue // skip empty names and duplicates (first wins)
			}
			order = append(order, name)
			byName[name] = varMapping
		}
	}

	return order, byName, nil
}

// mergeInputLevelVarNodes returns a sequence node containing only the promoted
// vars (those in promotedOverrides), each merged with the override fields.
// Order follows baseVarOrder (input package declaration order).
func mergeInputLevelVarNodes(
	baseVarOrder []string,
	baseVarByName map[string]*ast.MappingNode,
	promotedOverrides map[string]*ast.MappingNode,
) *ast.SequenceNode {
	seqNode := newSeqNode()
	for _, varName := range baseVarOrder {
		overrideNode, promoted := promotedOverrides[varName]
		if !promoted {
			continue
		}
		merged := mergeVarNode(baseVarByName[varName], overrideNode)
		seqNode.Values = append(seqNode.Values, merged)
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
	baseVarByName map[string]*ast.MappingNode,
	promotedNames map[string]bool,
	dsOverrides []*ast.MappingNode,
) *ast.SequenceNode {
	dsOverrideByName := make(map[string]*ast.MappingNode, len(dsOverrides))
	for _, v := range dsOverrides {
		dsOverrideByName[varNodeName(v)] = v
	}

	seqNode := newSeqNode()

	// Non-promoted base vars first (in input pkg order).
	for _, varName := range baseVarOrder {
		if promotedNames[varName] {
			continue
		}
		baseNode := baseVarByName[varName]
		overrideNode, hasOverride := dsOverrideByName[varName]
		var merged *ast.MappingNode
		if hasOverride {
			merged = mergeVarNode(baseNode, overrideNode)
		} else {
			merged = cloneNode(baseNode).(*ast.MappingNode)
		}
		seqNode.Values = append(seqNode.Values, merged)
	}

	// Novel DS vars (not present in base) appended in declaration order.
	for _, v := range dsOverrides {
		if _, inBase := baseVarByName[varNodeName(v)]; !inBase {
			seqNode.Values = append(seqNode.Values, cloneNode(v).(*ast.MappingNode))
		}
	}

	return seqNode
}

// mergeVarNode merges fields from overrideNode into a clone of baseNode.
// All keys in override win; absent keys in override are inherited from base.
// The "name" key is always preserved from base.
func mergeVarNode(base, override *ast.MappingNode) *ast.MappingNode {
	result := cloneNode(base).(*ast.MappingNode)
	for _, kv := range override.Values {
		if kv.Key.String() == "name" {
			continue // always preserve name from base
		}
		upsertKey(result, kv.Key.String(), cloneNode(kv.Value))
	}
	return result
}

// checkDuplicateVarNodes returns an error if any var name appears more than
// once in the provided nodes.
func checkDuplicateVarNodes(varNodes []*ast.MappingNode) error {
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
func varNodeName(v *ast.MappingNode) string {
	return nodeStringValue(mappingValue(v, "name"))
}

// readVarNodes extracts the individual var mapping nodes from the "vars"
// sequence of the given mapping node. Returns nil if no "vars" key is present.
func readVarNodes(mappingNode *ast.MappingNode) ([]*ast.MappingNode, error) {
	varsSeq, ok := mappingValue(mappingNode, "vars").(*ast.SequenceNode)
	if !ok {
		v := mappingValue(mappingNode, "vars")
		if v == nil {
			return nil, nil
		}
		return nil, fmt.Errorf("'vars' is not a sequence node")
	}
	result := make([]*ast.MappingNode, 0, len(varsSeq.Values))
	for _, item := range varsSeq.Values {
		mn, ok := item.(*ast.MappingNode)
		if !ok {
			return nil, fmt.Errorf("var entry is not a mapping node")
		}
		result = append(result, mn)
	}
	return result, nil
}

// getInputMappingNode navigates to policy_templates[ptIdx].inputs[inputIdx] in
// the given YAML root mapping and returns the input mapping node.
func getInputMappingNode(root *ast.MappingNode, ptIdx, inputIdx int) (*ast.MappingNode, error) {
	ptsNode, ok := mappingValue(root, "policy_templates").(*ast.SequenceNode)
	if !ok {
		return nil, fmt.Errorf("'policy_templates' not found or not a sequence")
	}
	if ptIdx < 0 || ptIdx >= len(ptsNode.Values) {
		return nil, fmt.Errorf("policy template index %d out of range (len=%d)", ptIdx, len(ptsNode.Values))
	}

	ptNode, ok := ptsNode.Values[ptIdx].(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("policy template %d is not a mapping", ptIdx)
	}

	inputsNode, ok := mappingValue(ptNode, "inputs").(*ast.SequenceNode)
	if !ok {
		return nil, fmt.Errorf("'inputs' not found or not a sequence in policy template %d", ptIdx)
	}
	if inputIdx < 0 || inputIdx >= len(inputsNode.Values) {
		return nil, fmt.Errorf("input index %d out of range (len=%d)", inputIdx, len(inputsNode.Values))
	}

	inputNode, ok := inputsNode.Values[inputIdx].(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("input %d is not a mapping", inputIdx)
	}

	return inputNode, nil
}

// getStreamMappingNode navigates to streams[streamIdx] in the given YAML
// root mapping and returns the stream mapping node.
func getStreamMappingNode(root *ast.MappingNode, streamIdx int) (*ast.MappingNode, error) {
	streamsNode, ok := mappingValue(root, "streams").(*ast.SequenceNode)
	if !ok {
		return nil, fmt.Errorf("'streams' not found or not a sequence")
	}
	if streamIdx < 0 || streamIdx >= len(streamsNode.Values) {
		return nil, fmt.Errorf("stream index %d out of range (len=%d)", streamIdx, len(streamsNode.Values))
	}

	streamNode, ok := streamsNode.Values[streamIdx].(*ast.MappingNode)
	if !ok {
		return nil, fmt.Errorf("stream %d is not a mapping", streamIdx)
	}

	return streamNode, nil
}
