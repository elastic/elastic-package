// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages"
)

func TestBuildIntegrationPackagePolicy(t *testing.T) {
	tests := []struct {
		name               string
		packageRoot        string
		policyTemplateName string
		dsName             string
		inputName          string
		policyName         string
		inputVars          common.MapStr
		dsVars             common.MapStr
		goldenSimplified   string
		goldenLegacy       string
	}{
		{
			name:               "sophos_xg_tcp",
			packageRoot:        "testdata/packages/sophos_tcp",
			policyTemplateName: "sophos",
			dsName:             "xg",
			inputName:          "tcp",
			policyName:         "sophos-xg-test",
			inputVars:          common.MapStr{},
			dsVars: common.MapStr{
				"syslog_host":   "0.0.0.0",
				"syslog_port":   9549,
				"known_devices": "- hostname: XG230\n  serial_number: \"1234567890123456\"\n- hostname: SG430\n  serial_number: \"S4000806149EE49\"\n",
			},
			goldenSimplified: "testdata/sophos_xg_tcp.json",
			goldenLegacy:     "testdata/sophos_xg_tcp_legacy.json",
		},
		{
			name:               "apache_access_logfile",
			packageRoot:        "testdata/packages/apache",
			policyTemplateName: "apache",
			dsName:             "access",
			inputName:          "logfile",
			policyName:         "apache-access-test",
			inputVars:          common.MapStr{},
			dsVars: common.MapStr{
				"paths": []string{"/tmp/service_logs/access.log*"},
			},
			goldenSimplified: "testdata/apache_access_logfile.json",
			goldenLegacy:     "testdata/apache_access_logfile_legacy.json",
		},
		{
			// Verifies that when building a policy for app_insights/azure/metrics,
			// the sibling disabled input (app_state-azure/metrics) uses azure.app_state
			// as its stream — not azure.app_insights.
			name:               "azure_app_insights_metrics",
			packageRoot:        "testdata/packages/azure_application_insights",
			policyTemplateName: "app_insights",
			dsName:             "app_insights",
			inputName:          "azure/metrics",
			policyName:         "azure-app-insights-test",
			inputVars:          common.MapStr{},
			dsVars: common.MapStr{
				"period":  "300s",
				"metrics": "- id: [\"requests/count\"]\n  aggregation: [\"sum\"]\n  interval: \"P5M\"\n",
			},
			goldenSimplified: "testdata/azure_app_insights_metrics.json",
			goldenLegacy:     "testdata/azure_app_insights_metrics_legacy.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest, err := packages.ReadPackageManifest(filepath.Join(tc.packageRoot, "manifest.yml"))
			require.NoError(t, err)

			dsManifest, err := packages.ReadDataStreamManifestFromPackageRoot(tc.packageRoot, tc.dsName)
			require.NoError(t, err)

			policyTemplate, err := packages.SelectPolicyTemplateByName(manifest.PolicyTemplates, tc.policyTemplateName)
			require.NoError(t, err)

			datastreams, err := packages.ReadAllDataStreamManifests(tc.packageRoot)
			require.NoError(t, err)

			pp, err := BuildIntegrationPackagePolicy(
				"test-policy-id", "test", tc.policyName,
				*manifest, policyTemplate, *dsManifest,
				tc.inputName,
				tc.inputVars, tc.dsVars,
				true, datastreams,
			)
			require.NoError(t, err)

			t.Run("simplified", func(t *testing.T) {
				got, err := json.MarshalIndent(pp, "", "  ")
				require.NoError(t, err)
				assertJSONGolden(t, tc.goldenSimplified, got)
			})

			t.Run("legacy", func(t *testing.T) {
				legacy := pp.toLegacy()
				// Sort inputs by type and streams within each input by dataset
				// for deterministic comparison (p.Inputs is a map).
				slices.SortFunc(legacy.Inputs, func(a, b legacyInput) int {
					return cmp.Compare(a.Type, b.Type)
				})
				for i := range legacy.Inputs {
					slices.SortFunc(legacy.Inputs[i].Streams, func(a, b legacyStream) int {
						return cmp.Compare(a.DataStream.Dataset, b.DataStream.Dataset)
					})
				}
				got, err := json.MarshalIndent(legacy, "", "  ")
				require.NoError(t, err)
				assertJSONGolden(t, tc.goldenLegacy, got)
			})
		})
	}
}

// assertJSONGolden compares got against the golden file at goldenPath using
// semantic JSON equality. If the golden file does not yet exist it is created
// from got so that the next run acts as the regression gate.
func assertJSONGolden(t *testing.T, goldenPath string, got []byte) {
	t.Helper()
	if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
		require.NoError(t, os.WriteFile(goldenPath, got, 0o644))
		t.Logf("created golden file %s", goldenPath)
		return
	}
	golden, err := os.ReadFile(goldenPath)
	require.NoError(t, err)
	assert.JSONEq(t, string(golden), string(got))
}
