// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
)

// ---- helpers -----------------------------------------------------------------

// varNode builds a minimal YAML mapping node representing a variable with the
// given name and extra key=value pairs (passed as alternating key, value
// strings for simple scalar values).
func varNode(name string, extras ...string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	upsertKey(n, "name", &yaml.Node{Kind: yaml.ScalarNode, Value: name})
	for i := 0; i+1 < len(extras); i += 2 {
		upsertKey(n, extras[i], &yaml.Node{Kind: yaml.ScalarNode, Value: extras[i+1]})
	}
	return n
}

// copyFixturePackage copies the named package from test/manual_packages/required_inputs
// to a fresh temp dir and returns that dir path.
func copyFixturePackage(t *testing.T, fixtureName string) string {
	t.Helper()
	srcPath := filepath.Join("..", "..", "test", "manual_packages", "required_inputs", fixtureName)
	destPath := t.TempDir()
	err := os.CopyFS(destPath, os.DirFS(srcPath))
	require.NoError(t, err, "copying fixture package %q", fixtureName)
	return destPath
}

// ciInputFixturePath returns the path to test/packages/composable/01_ci_input_pkg (repository-relative from this package).
func ciInputFixturePath() string {
	return filepath.Join("..", "..", "test", "packages", "composable", "01_ci_input_pkg")
}

// copyComposableIntegrationFixture copies test/packages/composable/02_ci_composable_integration for integration tests.
func copyComposableIntegrationFixture(t *testing.T) string {
	t.Helper()
	srcPath := filepath.Join("..", "..", "test", "packages", "composable", "02_ci_composable_integration")
	destPath := t.TempDir()
	err := os.CopyFS(destPath, os.DirFS(srcPath))
	require.NoError(t, err, "copying composable CI integration fixture")
	return destPath
}

// Variable merge tests exercise mergeVariables (see variables.go): when an
// integration declares requires.input and references that input package under
// policy_templates[].inputs with optional vars, definitions from the input
// package must be merged into the built integration—composable and data-stream
// overrides on top of the input package as base, with selected vars promoted
// to input-level. Unit tests cover helpers; integration tests run
// Integration tests exercise Bundle on manual fixture packages.

// ---- unit tests --------------------------------------------------------------

// TestCloneNode checks that YAML variable nodes are deep-cloned before merge.
// mergeVariables mutates cloned trees when applying overrides; without
// isolation, the resolver could corrupt cached or shared input-package nodes.
func TestCloneNode(t *testing.T) {
	original := varNode("paths", "type", "text", "multi", "true")
	cloned := cloneNode(original)

	// Mutating the clone must not affect the original.
	upsertKey(cloned, "type", &yaml.Node{Kind: yaml.ScalarNode, Value: "keyword"})
	assert.Equal(t, "text", mappingValue(original, "type").Value)
}

// TestMergeVarNode verifies mergeVarNode: per-variable field merge where the
// input package definition is the base and override keys from the composable
// package or data stream replace or add fields; the variable name always stays
// from the base. This is the primitive used for both promoted input vars and
// stream-level merges.
func TestMergeVarNode(t *testing.T) {
	base := varNode("paths", "type", "text", "title", "Paths", "multi", "true")

	t.Run("full override", func(t *testing.T) {
		override := varNode("paths", "type", "keyword", "title", "Custom Paths", "multi", "false")
		merged := mergeVarNode(base, override)
		assert.Equal(t, "paths", varNodeName(merged))
		assert.Equal(t, "keyword", mappingValue(merged, "type").Value)
		assert.Equal(t, "Custom Paths", mappingValue(merged, "title").Value)
		assert.Equal(t, "false", mappingValue(merged, "multi").Value)
	})

	t.Run("partial override", func(t *testing.T) {
		override := varNode("paths", "title", "My Paths")
		merged := mergeVarNode(base, override)
		assert.Equal(t, "paths", varNodeName(merged))
		assert.Equal(t, "text", mappingValue(merged, "type").Value) // from base
		assert.Equal(t, "My Paths", mappingValue(merged, "title").Value)
		assert.Equal(t, "true", mappingValue(merged, "multi").Value) // from base
	})

	t.Run("empty override", func(t *testing.T) {
		override := varNode("paths")
		merged := mergeVarNode(base, override)
		assert.Equal(t, "paths", varNodeName(merged))
		assert.Equal(t, "text", mappingValue(merged, "type").Value)   // from base
		assert.Equal(t, "Paths", mappingValue(merged, "title").Value) // from base
	})

	t.Run("name not renamed", func(t *testing.T) {
		// Even if the override specifies a different name value, base name wins.
		override := &yaml.Node{Kind: yaml.MappingNode}
		upsertKey(override, "name", &yaml.Node{Kind: yaml.ScalarNode, Value: "should-be-ignored"})
		upsertKey(override, "type", &yaml.Node{Kind: yaml.ScalarNode, Value: "keyword"})
		merged := mergeVarNode(base, override)
		assert.Equal(t, "paths", varNodeName(merged))
	})

	t.Run("adds new field from override", func(t *testing.T) {
		override := varNode("paths", "description", "My description")
		merged := mergeVarNode(base, override)
		assert.Equal(t, "My description", mappingValue(merged, "description").Value)
		assert.Equal(t, "text", mappingValue(merged, "type").Value) // base preserved
	})
}

// TestCheckDuplicateVarNodes ensures duplicate var names in a single vars list
// are rejected before merge. That catches invalid integration manifests early
// instead of producing ambiguous merged output for Fleet.
func TestCheckDuplicateVarNodes(t *testing.T) {
	t.Run("no duplicates", func(t *testing.T) {
		nodes := []*yaml.Node{varNode("paths"), varNode("encoding"), varNode("timeout")}
		assert.NoError(t, checkDuplicateVarNodes(nodes))
	})

	t.Run("one duplicate", func(t *testing.T) {
		nodes := []*yaml.Node{varNode("paths"), varNode("encoding"), varNode("paths")}
		err := checkDuplicateVarNodes(nodes)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "paths")
	})

	t.Run("empty slice", func(t *testing.T) {
		assert.NoError(t, checkDuplicateVarNodes(nil))
	})
}

// TestMergeInputLevelVarNodes covers mergeInputLevelVarNodes: vars that appear
// under policy_templates[].inputs[] next to package: <input_pkg> are promoted
// to merged input-level var definitions, in input-package declaration order,
// with only explicitly listed names included.
func TestMergeInputLevelVarNodes(t *testing.T) {
	pathsBase := varNode("paths", "type", "text", "multi", "true")
	encodingBase := varNode("encoding", "type", "text", "show_user", "false")
	timeoutBase := varNode("timeout", "type", "text", "default", "30s")

	baseOrder := []string{"paths", "encoding", "timeout"}
	baseByName := map[string]*yaml.Node{
		"paths":    pathsBase,
		"encoding": encodingBase,
		"timeout":  timeoutBase,
	}

	t.Run("empty promoted → empty sequence", func(t *testing.T) {
		seq := mergeInputLevelVarNodes(baseOrder, baseByName, map[string]*yaml.Node{})
		assert.Empty(t, seq.Content)
	})

	t.Run("one promoted partial override", func(t *testing.T) {
		promotedOverrides := map[string]*yaml.Node{
			"paths": varNode("paths", "default", "/var/log/custom/*.log"),
		}
		seq := mergeInputLevelVarNodes(baseOrder, baseByName, promotedOverrides)
		require.Len(t, seq.Content, 1)
		assert.Equal(t, "paths", varNodeName(seq.Content[0]))
		assert.Equal(t, "/var/log/custom/*.log", mappingValue(seq.Content[0], "default").Value)
		assert.Equal(t, "text", mappingValue(seq.Content[0], "type").Value) // from base
	})

	t.Run("multiple promoted in base order", func(t *testing.T) {
		promotedOverrides := map[string]*yaml.Node{
			"timeout":  varNode("timeout", "default", "60s"),
			"encoding": varNode("encoding", "show_user", "true"),
		}
		seq := mergeInputLevelVarNodes(baseOrder, baseByName, promotedOverrides)
		require.Len(t, seq.Content, 2)
		// Order must follow baseOrder: encoding before timeout.
		assert.Equal(t, "encoding", varNodeName(seq.Content[0]))
		assert.Equal(t, "timeout", varNodeName(seq.Content[1]))
		assert.Equal(t, "true", mappingValue(seq.Content[0], "show_user").Value)
		assert.Equal(t, "60s", mappingValue(seq.Content[1], "default").Value)
	})
}

// TestMergeStreamLevelVarNodes covers mergeStreamLevelVarNodes: base vars from
// the input package that are not promoted stay on the data stream stream entry;
// they can be field-merged with DS overrides, and DS-only vars are appended.
// Promoted names must not appear on the stream to avoid duplicating Fleet vars.
func TestMergeStreamLevelVarNodes(t *testing.T) {
	pathsBase := varNode("paths", "type", "text", "multi", "true")
	encodingBase := varNode("encoding", "type", "text", "show_user", "false")
	timeoutBase := varNode("timeout", "type", "text", "default", "30s")

	baseOrder := []string{"paths", "encoding", "timeout"}
	baseByName := map[string]*yaml.Node{
		"paths":    pathsBase,
		"encoding": encodingBase,
		"timeout":  timeoutBase,
	}

	t.Run("no promoted, no overrides → all base vars", func(t *testing.T) {
		seq := mergeStreamLevelVarNodes(baseOrder, baseByName, nil, nil)
		require.Len(t, seq.Content, 3)
		assert.Equal(t, "paths", varNodeName(seq.Content[0]))
		assert.Equal(t, "encoding", varNodeName(seq.Content[1]))
		assert.Equal(t, "timeout", varNodeName(seq.Content[2]))
	})

	t.Run("some promoted → promoted excluded", func(t *testing.T) {
		promoted := map[string]bool{"paths": true, "encoding": true}
		seq := mergeStreamLevelVarNodes(baseOrder, baseByName, promoted, nil)
		require.Len(t, seq.Content, 1)
		assert.Equal(t, "timeout", varNodeName(seq.Content[0]))
	})

	t.Run("DS override on existing base var", func(t *testing.T) {
		dsOverrides := []*yaml.Node{varNode("encoding", "show_user", "true")}
		seq := mergeStreamLevelVarNodes(baseOrder, baseByName, nil, dsOverrides)
		require.Len(t, seq.Content, 3)
		// encoding is merged
		encodingMerged := seq.Content[1]
		assert.Equal(t, "encoding", varNodeName(encodingMerged))
		assert.Equal(t, "true", mappingValue(encodingMerged, "show_user").Value)
		assert.Equal(t, "text", mappingValue(encodingMerged, "type").Value) // from base
	})

	t.Run("novel DS var appended", func(t *testing.T) {
		dsOverrides := []*yaml.Node{varNode("custom_tag", "type", "text")}
		seq := mergeStreamLevelVarNodes(baseOrder, baseByName, nil, dsOverrides)
		require.Len(t, seq.Content, 4) // 3 base + 1 novel
		assert.Equal(t, "custom_tag", varNodeName(seq.Content[3]))
	})

	t.Run("mixed: promoted + DS merge + novel", func(t *testing.T) {
		promoted := map[string]bool{"paths": true}
		dsOverrides := []*yaml.Node{
			varNode("encoding", "show_user", "true"),
			varNode("custom_tag", "type", "text"),
		}
		seq := mergeStreamLevelVarNodes(baseOrder, baseByName, promoted, dsOverrides)
		// paths excluded (promoted); encoding merged; timeout base; custom_tag novel
		require.Len(t, seq.Content, 3)
		assert.Equal(t, "encoding", varNodeName(seq.Content[0]))
		assert.Equal(t, "true", mappingValue(seq.Content[0], "show_user").Value)
		assert.Equal(t, "timeout", varNodeName(seq.Content[1]))
		assert.Equal(t, "custom_tag", varNodeName(seq.Content[2]))
	})
}

// TestLoadInputPkgVarNodes checks loadInputPkgVarNodes: variable definitions
// are loaded from the resolved input package manifest so mergeVariables uses
// the input package as the authoritative base (order and fields) for merging.
func TestLoadInputPkgVarNodes(t *testing.T) {
	t.Run("fixture with three vars", func(t *testing.T) {
		pkgPath := ciInputFixturePath()
		order, byName, err := loadInputPkgVarNodes(pkgPath)
		require.NoError(t, err)
		assert.Equal(t, []string{"paths", "encoding", "timeout"}, order)
		assert.Equal(t, "text", mappingValue(byName["paths"], "type").Value)
		assert.Equal(t, "text", mappingValue(byName["encoding"], "type").Value)
		assert.Equal(t, "text", mappingValue(byName["timeout"], "type").Value)
	})

	t.Run("package with no vars", func(t *testing.T) {
		// Use the fake input helper which has no vars in its manifest.
		pkgPath := createFakeInputHelper(t)
		order, byName, err := loadInputPkgVarNodes(pkgPath)
		require.NoError(t, err)
		assert.Empty(t, order)
		assert.Empty(t, byName)
	})
}

// TestPromotedVarNamesForStream_UnionsScopedAndTemplateWide verifies that when
// resolving which base vars are promoted off a data stream, overrides keyed by
// (input package, composable data stream) are unioned with overrides keyed by
// (input package, "") so template-wide promotions still apply to named streams.
func TestPromotedVarNamesForStream_UnionsScopedAndTemplateWide(t *testing.T) {
	const refPkg = "ci_input_pkg"
	dsScoped := varNode("paths", "type", "text")
	templateWide := varNode("encoding", "type", "text")

	byScope := map[promotedVarScopeKey]map[string]*yaml.Node{
		{refInputPackage: refPkg, composableDataStream: "my_logs"}: {
			"paths": dsScoped,
		},
		{refInputPackage: refPkg, composableDataStream: ""}: {
			"encoding": templateWide,
		},
	}

	names := promotedVarNamesForStream(refPkg, "my_logs", byScope)
	assert.True(t, names["paths"])
	assert.True(t, names["encoding"])
	assert.False(t, names["timeout"])
}

// TestUnionPromotedOverridesForInput_MergesOverridesAcrossDataStreams checks
// unionPromotedOverridesForInput: a policy template listing several data streams
// must merge composable-side override nodes from every listed stream so
// input-level promotion sees the full set of vars declared anywhere on that
// template for the referenced input package.
func TestUnionPromotedOverridesForInput_MergesOverridesAcrossDataStreams(t *testing.T) {
	const refPkg = "ci_input_pkg"
	paths := varNode("paths", "title", "P")
	encoding := varNode("encoding", "title", "E")

	byScope := map[promotedVarScopeKey]map[string]*yaml.Node{
		{refInputPackage: refPkg, composableDataStream: "ds_a"}: {"paths": paths},
		{refInputPackage: refPkg, composableDataStream: "ds_b"}: {"encoding": encoding},
	}

	pt := packages.PolicyTemplate{
		Name:        "pt",
		DataStreams: []string{"ds_a", "ds_b"},
	}

	got := unionPromotedOverridesForInput(pt, refPkg, byScope)
	require.Len(t, got, 2)
	assert.Same(t, paths, got["paths"])
	assert.Same(t, encoding, got["encoding"])
}

// TestBuildPromotedVarOverrideMap_PerDataStreamScopes builds the promoted
// override index from aligned manifest + YAML: each composable data stream
// listed under a policy template gets its own scope entry so downstream merge
// can distinguish stream-specific composable vars.
func TestBuildPromotedVarOverrideMap_PerDataStreamScopes(t *testing.T) {
	manifestYAML := []byte(`format_version: 3.0.0
name: scope_test
title: Scope test
version: 0.1.0
type: integration
policy_templates:
  - name: logs
    title: Logs
    data_streams:
      - ds_alpha
      - ds_beta
    inputs:
      - package: ref_pkg
        vars:
          - name: paths
            type: text
            title: Promoted paths
`)

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(manifestYAML, &doc))
	m, err := packages.ReadPackageManifestBytes(manifestYAML)
	require.NoError(t, err)

	idx, err := buildPromotedVarOverrideMap(m, &doc)
	require.NoError(t, err)

	keyAlpha := promotedVarScopeKey{refInputPackage: "ref_pkg", composableDataStream: "ds_alpha"}
	keyBeta := promotedVarScopeKey{refInputPackage: "ref_pkg", composableDataStream: "ds_beta"}
	require.Contains(t, idx, keyAlpha)
	require.Contains(t, idx, keyBeta)
	assert.Contains(t, idx[keyAlpha], "paths")
	assert.Contains(t, idx[keyBeta], "paths")
	assert.Equal(t, "Promoted paths", mappingValue(idx[keyAlpha]["paths"], "title").Value)
}

// TestBuildPromotedVarOverrideMap_NoDataStreamsUsesEmptyScope verifies that a
// policy template without data_streams still records promoted overrides under
// composableDataStream "", matching how streams are matched when the template is
// not scoped to named data streams.
func TestBuildPromotedVarOverrideMap_NoDataStreamsUsesEmptyScope(t *testing.T) {
	manifestYAML := []byte(`format_version: 3.0.0
name: scope_test2
title: Scope test 2
version: 0.1.0
type: integration
policy_templates:
  - name: logs
    title: Logs
    inputs:
      - package: ref_pkg
        vars:
          - name: paths
            type: text
`)

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(manifestYAML, &doc))
	m, err := packages.ReadPackageManifestBytes(manifestYAML)
	require.NoError(t, err)

	idx, err := buildPromotedVarOverrideMap(m, &doc)
	require.NoError(t, err)

	key := promotedVarScopeKey{refInputPackage: "ref_pkg", composableDataStream: ""}
	require.Contains(t, idx, key)
	assert.Contains(t, idx[key], "paths")
}

// ---- integration tests -------------------------------------------------------

// makeFakeEprForVarMerging supplies the ci_input_pkg fixture path as if it were
// downloaded from the registry, so integration tests do not need a running stack.
func makeFakeEprForVarMerging(t *testing.T) *fakeEprClient {
	t.Helper()
	inputPkgPath := ciInputFixturePath()
	return &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgPath, nil
		},
	}
}

// TestMergeVariables_Full runs the full merge pipeline: composable vars under
// the package input promote paths and encoding to manifest input-level defs
// (merged with input package defaults), while timeout stays on the data stream
// merged with a DS override and a novel DS-only var is appended—matching the
// end state Fleet expects for a mixed promotion + DS customization scenario.
func TestMergeVariables_Full(t *testing.T) {
	buildPackageRoot := copyComposableIntegrationFixture(t)
	resolver := NewRequiredInputsResolver(makeFakeEprForVarMerging(t))

	err := resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	// Check package manifest: first input (package ref) should have 2 vars (paths, encoding).
	manifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	inputVars := manifest.PolicyTemplates[0].Inputs[0].Vars
	require.Len(t, inputVars, 2)
	assert.Equal(t, "paths", inputVars[0].Name)
	assert.Equal(t, "encoding", inputVars[1].Name)

	// paths: base fields preserved, default overridden.
	assert.Equal(t, "text", inputVars[0].Type)
	require.NotNil(t, inputVars[0].Default)

	// encoding: show_user overridden to true.
	assert.True(t, inputVars[1].ShowUser)
	assert.Equal(t, "text", inputVars[1].Type)

	// Check DS manifest: streams[0] should have 2 vars (timeout, custom_tag).
	dsManifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "ci_composable_logs", "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)

	streamVars := dsManifest.Streams[0].Vars
	require.Len(t, streamVars, 2)
	assert.Equal(t, "timeout", streamVars[0].Name)
	assert.Equal(t, "custom_tag", streamVars[1].Name)

	// timeout: merged from base + DS override (description).
	assert.Equal(t, "text", streamVars[0].Type)
	assert.Equal(t, "Timeout for log collection.", streamVars[0].Description)
}

// TestMergeVariables_PromotesToInput verifies partial promotion: only vars
// listed under the composable input move to input level; remaining input
// package vars stay on the stream unchanged when the data stream supplies no
// overrides.
func TestMergeVariables_PromotesToInput(t *testing.T) {
	buildPackageRoot := copyFixturePackage(t, "with_merging_promotes_to_input")
	resolver := NewRequiredInputsResolver(makeFakeEprForVarMerging(t))

	err := resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	manifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	// Input should have 1 var: paths (promoted, merged with composable override).
	inputVars := manifest.PolicyTemplates[0].Inputs[0].Vars
	require.Len(t, inputVars, 1)
	assert.Equal(t, "paths", inputVars[0].Name)
	assert.Equal(t, "text", inputVars[0].Type) // from base

	// DS should have 2 vars: encoding and timeout (both from base, no DS overrides).
	dsManifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "var_merging_logs", "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)

	streamVars := dsManifest.Streams[0].Vars
	require.Len(t, streamVars, 2)
	assert.Equal(t, "encoding", streamVars[0].Name)
	assert.Equal(t, "timeout", streamVars[1].Name)
}

// TestMergeVariables_DsMerges covers the case where the composable input
// declares no vars (nothing promoted): all base vars remain on the stream, the
// data stream manifest can merge fields into an existing base var (e.g. title),
// and extra stream-only vars are kept in declaration order after base vars.
func TestMergeVariables_DsMerges(t *testing.T) {
	buildPackageRoot := copyFixturePackage(t, "with_merging_ds_merges")
	resolver := NewRequiredInputsResolver(makeFakeEprForVarMerging(t))

	err := resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	manifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	// No input-level vars (nothing promoted).
	assert.Empty(t, manifest.PolicyTemplates[0].Inputs[0].Vars)

	// DS should have 4 vars: paths, encoding (merged), timeout, custom_tag.
	dsManifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "var_merging_logs", "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)

	streamVars := dsManifest.Streams[0].Vars
	require.Len(t, streamVars, 4)
	assert.Equal(t, "paths", streamVars[0].Name)
	assert.Equal(t, "encoding", streamVars[1].Name)
	assert.Equal(t, "timeout", streamVars[2].Name)
	assert.Equal(t, "custom_tag", streamVars[3].Name)

	// encoding: title overridden.
	assert.Equal(t, "Log Encoding Override", streamVars[1].Title)
	assert.Equal(t, "text", streamVars[1].Type) // from base
}

// TestMergeVariables_NoOverride ensures that when the integration does not
// specify composable or data-stream var overrides, merge still materializes
// input package var definitions onto the stream (cloned base) so behavior stays
// correct for packages that only declare requires.input without local var edits.
func TestMergeVariables_NoOverride(t *testing.T) {
	buildPackageRoot := copyFixturePackage(t, "with_merging_no_override")
	resolver := NewRequiredInputsResolver(makeFakeEprForVarMerging(t))

	err := resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	manifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	// No input-level vars.
	assert.Empty(t, manifest.PolicyTemplates[0].Inputs[0].Vars)

	// DS should have 3 vars: all from base, unmodified.
	dsManifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "var_merging_logs", "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)

	streamVars := dsManifest.Streams[0].Vars
	require.Len(t, streamVars, 3)
	assert.Equal(t, "paths", streamVars[0].Name)
	assert.Equal(t, "encoding", streamVars[1].Name)
	assert.Equal(t, "timeout", streamVars[2].Name)

	// Base fields preserved.
	assert.Equal(t, "text", streamVars[0].Type)
	assert.True(t, streamVars[0].Multi)
	assert.True(t, streamVars[0].Required)
}

// TestMergeVariables_DuplicateError checks that an invalid data stream manifest
// listing the same var name twice fails during mergeVariables, surfacing a
// clear duplicate-variable error instead of silent corruption.
func TestMergeVariables_DuplicateError(t *testing.T) {
	buildPackageRoot := copyFixturePackage(t, "with_merging_duplicate_error")
	resolver := NewRequiredInputsResolver(makeFakeEprForVarMerging(t))

	err := resolver.Bundle(buildPackageRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "paths")
}

// TestMergeVariables_TwoPolicyTemplatesScopedPromotion verifies that promotion
// is scoped per policy template data stream: composable vars under one template
// promote only for that template’s streams; another template referencing the
// same input package without composable vars keeps all base vars on its streams.
// This guards against incorrectly applying one template’s promotions to every
// stream that uses the same input package.
func TestMergeVariables_TwoPolicyTemplatesScopedPromotion(t *testing.T) {
	buildPackageRoot := copyFixturePackage(t, "with_merging_two_policy_templates")
	resolver := NewRequiredInputsResolver(makeFakeEprForVarMerging(t))

	err := resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	manifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)
	require.Len(t, manifest.PolicyTemplates, 2)

	// pt_alpha: composable input has promoted paths (merged title).
	alphaPT := manifest.PolicyTemplates[0]
	require.Equal(t, "pt_alpha", alphaPT.Name)
	require.GreaterOrEqual(t, len(alphaPT.Inputs), 1)
	alphaInputVars := alphaPT.Inputs[0].Vars
	require.Len(t, alphaInputVars, 1)
	assert.Equal(t, "paths", alphaInputVars[0].Name)
	assert.Equal(t, "Alpha-only promoted paths title", alphaInputVars[0].Title)
	assert.Equal(t, "text", alphaInputVars[0].Type)

	// pt_beta: no promotion — no vars on the composable input entry.
	betaPT := manifest.PolicyTemplates[1]
	require.Equal(t, "pt_beta", betaPT.Name)
	assert.Empty(t, betaPT.Inputs[0].Vars)

	// alpha_logs: paths promoted — stream keeps encoding + timeout only.
	alphaDSBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "alpha_logs", "manifest.yml"))
	require.NoError(t, err)
	alphaDS, err := packages.ReadDataStreamManifestBytes(alphaDSBytes)
	require.NoError(t, err)
	alphaStreamVars := alphaDS.Streams[0].Vars
	require.Len(t, alphaStreamVars, 2)
	assert.Equal(t, "encoding", alphaStreamVars[0].Name)
	assert.Equal(t, "timeout", alphaStreamVars[1].Name)

	// beta_logs: no promotion — all three base vars on the stream.
	betaDSBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "beta_logs", "manifest.yml"))
	require.NoError(t, err)
	betaDS, err := packages.ReadDataStreamManifestBytes(betaDSBytes)
	require.NoError(t, err)
	betaStreamVars := betaDS.Streams[0].Vars
	require.Len(t, betaStreamVars, 3)
	assert.Equal(t, "paths", betaStreamVars[0].Name)
	assert.Equal(t, "encoding", betaStreamVars[1].Name)
	assert.Equal(t, "timeout", betaStreamVars[2].Name)
}
