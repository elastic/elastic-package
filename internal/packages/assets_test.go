// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadAlertRuleTemplateAssets(t *testing.T) {
	root := "../../test/packages/parallel/alert_rule_templates_v2"
	dir := filepath.Join(root, "kibana", "alerting_rule_template")

	assets, err := loadKibanaAssets(root)
	require.NoError(t, err)
	ids := make([]string, 0, len(assets))
	for _, asset := range assets {
		require.Equal(t, AlertRuleTemplateType, asset.Type)
		require.Equal(t, filepath.Join(dir, asset.ID+".json"), asset.SourcePath)
		ids = append(ids, asset.ID)
	}
	require.ElementsMatch(t, []string{
		"alert_rule_templates_v2-errors-v2",
		"alert_rule_templates_v2-host-down-classic",
		"alert_rule_templates_v2-latency-classic",
		"alert_rule_templates_v2-latency-v2",
	}, ids)
}

func TestLoadAlertRuleTemplateAssetsWithoutID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "kibana", "alerting_rule_template")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "invalid.json"), []byte(`{"attributes":{"engine":"v2"}}`), 0o644))

	_, err := loadKibanaAssets(root)
	require.ErrorContains(t, err, "empty asset ID")
}
