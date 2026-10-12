// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestMissingAlertRuleTemplateIsNotAnInstalledAsset(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.yml"), []byte("name: example\nversion: 1.0.0\ntype: content\n"), 0o644))
	dir := filepath.Join(root, "kibana", "alerting_rule_template")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rule-v2.json"), []byte(`{"id":"rule-v2","attributes":{"engine":"v2"}}`), 0o644))

	expected, err := packages.LoadPackageAssets(root)
	require.NoError(t, err)
	require.Len(t, expected, 1, "a package containing only a template must not silently have zero asset tests")
	require.False(t, findActualAsset(nil, expected[0]))
	require.False(t, findActualAsset([]packages.Asset{{ID: "rule-v2", Type: "dashboard"}}, expected[0]))
	require.False(t, findActualAsset([]packages.Asset{{ID: "another-rule", Type: "alerting_rule_template"}}, expected[0]))
	require.True(t, findActualAsset([]packages.Asset{{ID: "rule-v2", Type: "alerting_rule_template"}}, expected[0]))
}
