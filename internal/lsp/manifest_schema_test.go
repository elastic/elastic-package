// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
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
