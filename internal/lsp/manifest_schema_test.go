// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestTopLevelKeysIncludeCurrentPackageSpecFields(t *testing.T) {
	keys := manifestTopLevelKeys(manifestSchemaIntegration)

	assert.Contains(t, keys, "policy_templates_behavior")
	assert.Contains(t, keys, "var_groups")
	assert.Contains(t, keys, "deprecated")
}

func TestManifestDocResolvesNestedRefs(t *testing.T) {
	doc := manifestDoc("policy_templates.inputs.template_paths", manifestSchemaIntegration)

	require.NotEmpty(t, doc)
	assert.Contains(t, doc, "template_paths")
	assert.Contains(t, doc, "array")
}

func TestManifestDocUsesCanonicalDescriptions(t *testing.T) {
	doc := manifestDoc("owner.type", manifestSchemaIntegration)

	require.NotEmpty(t, doc)
	assert.Contains(t, doc, "community")
	assert.Contains(t, doc, "required")
}

func TestManifestDocCoversNestedPolicyTemplateFields(t *testing.T) {
	doc := manifestDoc("policy_templates.deployment_modes", manifestSchemaIntegration)

	require.NotEmpty(t, doc)
	assert.Contains(t, doc, "deployment mode")
}

func TestResolveYAMLPathFromDocumentText(t *testing.T) {
	path := resolveYAMLPath(`policy_templates:
  - name: apache
    inputs:
      - type: logfile
`, 3)

	assert.Equal(t, []string{"policy_templates", "inputs", "type"}, path)
}

func TestManifestSchemaKindForFileUsesPackageTypeAndDataStreamPath(t *testing.T) {
	packageRoot := "/tmp/pkg"

	assert.Equal(t, manifestSchemaInput, manifestSchemaKindForFile(
		filepath.Join(packageRoot, "manifest.yml"),
		packageRoot,
		"type: input\n",
	))
	assert.Equal(t, manifestSchemaContent, manifestSchemaKindForFile(
		filepath.Join(packageRoot, "manifest.yml"),
		packageRoot,
		"type: content\n",
	))
	assert.Equal(t, manifestSchemaIntegration, manifestSchemaKindForFile(
		filepath.Join(packageRoot, "manifest.yml"),
		packageRoot,
		"type: integration\n",
	))
	assert.Equal(t, manifestSchemaDataStream, manifestSchemaKindForFile(
		filepath.Join(packageRoot, "data_stream", "access", "manifest.yml"),
		packageRoot,
		"type: input\n",
	))
}

func TestManifestSchemaHelperFunctions(t *testing.T) {
	assert.Equal(t, "input", packageTypeFromManifest("type: input\n"))
	assert.Equal(t, "", packageTypeFromManifest(":\n"))
	assert.Equal(t, "string | null", schemaType(map[string]any{"type": []any{"string", "null"}}))
	assert.Equal(t, "enum", schemaType(map[string]any{"enum": []any{"a", "b"}}))
	assert.Equal(t, "object", schemaType(map[string]any{"properties": map[string]any{"name": "x"}}))
	assert.Equal(t, "array", schemaType(map[string]any{"items": map[string]any{"type": "string"}}))
	assert.Equal(t, "", schemaType(map[string]any{}))

	value, ok := scalarValue(true)
	require.True(t, ok)
	assert.Equal(t, "true", value)

	value, ok = scalarValue(1.5)
	require.True(t, ok)
	assert.Equal(t, "1.5", value)

	_, ok = scalarValue(map[string]any{"bad": "value"})
	assert.False(t, ok)
}
