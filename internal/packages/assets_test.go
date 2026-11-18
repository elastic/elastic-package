// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPackageAssets_Workflows(t *testing.T) {
	// TODO: move this fixture to test/packages/parallel/workflows once the package-spec
	// release that allows kibana/workflow is bumped in go.mod. Until then, CI would fail
	// running check and build on it, so it lives in testdata, which CI doesn't build.
	packageRoot := "testdata/workflows"

	assets, err := LoadPackageAssets(packageRoot)
	require.NoError(t, err)

	expected := []Asset{
		{
			ID:         "workflows-alert-triage",
			Type:       AssetTypeKibanaWorkflow.typeName,
			SourcePath: filepath.Join(packageRoot, "kibana", "workflow", "workflows-alert-triage.yml"),
		},
		{
			ID:         "workflows-scheduled-report",
			Type:       AssetTypeKibanaWorkflow.typeName,
			SourcePath: filepath.Join(packageRoot, "kibana", "workflow", "workflows-scheduled-report.yml"),
		},
	}
	assert.Equal(t, expected, assets)
}

func TestLoadPackageAssets_SavedObjects(t *testing.T) {
	packageRoot := "../../test/packages/parallel/apache"

	assets, err := LoadPackageAssets(packageRoot)
	require.NoError(t, err)

	countByType := make(map[AssetType]int)
	for _, a := range assets {
		countByType[a.Type]++
	}
	assert.Equal(t, map[AssetType]int{
		AssetTypeKibanaDashboard.typeName:             2,
		AssetTypeKibanaSavedSearch.typeName:           3,
		AssetTypeKibanaVisualization.typeName:         19,
		AssetTypeElasticsearchIndexTemplate.typeName:  3,
		AssetTypeElasticsearchIngestPipeline.typeName: 2,
	}, countByType)

	dashboard := Asset{
		ID:         "apache-Logs-Apache-Dashboard",
		Type:       AssetTypeKibanaDashboard.typeName,
		SourcePath: filepath.Join(packageRoot, "kibana", "dashboard", "apache-Logs-Apache-Dashboard.json"),
	}
	assert.Contains(t, assets, dashboard)
}

func TestWorkflowAssetID(t *testing.T) {
	cases := []struct {
		path     string
		expected string
	}{
		{path: "kibana/workflow/apache-alert-triage.yml", expected: "apache-alert-triage"},
		{path: "kibana/workflow/my_pkg-alert-triage.yml", expected: "my-pkg-alert-triage"},
		{path: "kibana/workflow/My_Pkg-Alert.yml", expected: "my-pkg-alert"},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			id, err := workflowAssetID(c.path)
			require.NoError(t, err)
			assert.Equal(t, c.expected, id)
		})
	}
}
