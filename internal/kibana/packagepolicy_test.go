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
			// Verifies that an empty yaml multi-valued var (e.g. file_selectors: [])
			// is omitted from the simplified API request instead of being sent as the
			// string "[]", which Fleet would reject as invalid YAML when substituted
			// into Handlebars templates. Reproduces the aws_bedrock case.
			name:               "empty_yaml_multi_var",
			packageRoot:        "testdata/packages/test_policy_vars",
			policyTemplateName: "test",
			dsName:             "log",
			inputName:          "cel",
			policyName:         "test-log-test",
			inputVars:          common.MapStr{},
			dsVars: common.MapStr{
				"file_selectors": []interface{}{},
			},
			goldenSimplified: "testdata/test_policy_vars_empty_yaml_multi.json",
			goldenLegacy:     "testdata/test_policy_vars_empty_yaml_multi_legacy.json",
		},
		{
			// Verifies that a select type variable with a non-default value ("false")
			// is correctly serialised in the simplified API request.
			// Reproduces the ti_opencti case where revoked: "false" caused Fleet to
			// reject with "Invalid value for select type".
			name:               "select_var_false_value",
			packageRoot:        "testdata/packages/test_policy_vars",
			policyTemplateName: "test",
			dsName:             "log",
			inputName:          "cel",
			policyName:         "test-log-test",
			inputVars:          common.MapStr{},
			dsVars: common.MapStr{
				"revoked": "false",
			},
			goldenSimplified: "testdata/test_policy_vars_select_false.json",
			goldenLegacy:     "testdata/test_policy_vars_select_false_legacy.json",
		},
		{
			// Verifies that a bool variable with a dotted name (e.g. "active.only")
			// is found when ucfg has stored it as a nested map {"active": {"only": false}}
			// due to PathSep(".") parsing. Reproduces the elasticsearch index_recovery
			// case where active.only: false in the test config was silently ignored,
			// causing the manifest default (true) to be used instead.
			name:               "dotted_bool_var_nested_lookup",
			packageRoot:        "testdata/packages/test_policy_vars",
			policyTemplateName: "test",
			dsName:             "log",
			inputName:          "cel",
			policyName:         "test-log-test",
			inputVars:          common.MapStr{},
			dsVars: common.MapStr{
				"active": common.MapStr{
					"only": false,
				},
			},
			goldenSimplified: "testdata/test_policy_vars_dotted_bool.json",
			goldenLegacy:     "testdata/test_policy_vars_dotted_bool_legacy.json",
		},
		{
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
			// Verifies that package-level vars specified in dsVars (data_stream.vars
			// in the test config) are applied at the package level. This covers the
			// endace case where endace_url is a required package-level var but is
			// written under data_stream.vars in the system test config.
			name:               "endace_netflow_pkg_var_in_dsvars",
			packageRoot:        "testdata/packages/endace_netflow",
			policyTemplateName: "endace",
			dsName:             "log",
			inputName:          "netflow",
			policyName:         "endace-log-test",
			inputVars:          common.MapStr{},
			dsVars: common.MapStr{
				"host":       "0.0.0.0",
				"port":       2055,
				"endace_url": "http://test.elastic.co",
			},
			goldenSimplified: "testdata/endace_netflow_pkg_var_in_dsvars.json",
			goldenLegacy:     "testdata/endace_netflow_pkg_var_in_dsvars_legacy.json",
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

func TestBuildInputPackagePolicy(t *testing.T) {
	tests := []struct {
		name               string
		packageRoot        string
		policyTemplateName string
		policyName         string
		varValues          common.MapStr
		goldenSimplified   string
		goldenLegacy       string
	}{
		{
			name:               "log_custom_logs",
			packageRoot:        "testdata/packages/log_input",
			policyTemplateName: "logs",
			policyName:         "log-logs-test",
			varValues: common.MapStr{
				"paths":               []string{"/tmp/test.log"},
				"data_stream.dataset": "log.custom",
			},
			goldenSimplified: "testdata/log_custom_logs.json",
			goldenLegacy:     "testdata/log_custom_logs_legacy.json",
		},
		{
			name:               "sql_input_custom_dataset",
			packageRoot:        "../../test/packages/parallel/sql_input",
			policyTemplateName: "sql_query",
			policyName:         "sql-query-test",
			varValues: common.MapStr{
				"data_stream.dataset": "custom.sql",
			},
			goldenSimplified: "testdata/sql_input_custom_dataset.json",
			goldenLegacy:     "testdata/sql_input_custom_dataset_legacy.json",
		},
		{
			// Simulates varValues coming from the test runner, which parses config
			// files with ucfg.PathSep("."), causing data_stream.dataset to be stored
			// as a nested map {"data_stream": {"dataset": ...}} rather than a flat key.
			name:               "sql_input_nested_dataset",
			packageRoot:        "../../test/packages/parallel/sql_input",
			policyTemplateName: "sql_query",
			policyName:         "sql-query-test",
			varValues: common.MapStr{
				"data_stream": common.MapStr{
					"dataset": "custom.sql",
				},
			},
			goldenSimplified: "testdata/sql_input_custom_dataset.json",
			goldenLegacy:     "testdata/sql_input_custom_dataset_legacy.json",
		},
		{
			// No data_stream.dataset provided: the default should be
			// "<pkgName>.<policyTemplateName>" so the data lands in the
			// index template installed by the package.
			name:               "sql_input_default_dataset",
			packageRoot:        "../../test/packages/parallel/sql_input",
			policyTemplateName: "sql_query",
			policyName:         "sql-query-test",
			varValues:          common.MapStr{},
			goldenSimplified:   "testdata/sql_input_default_dataset.json",
			goldenLegacy:       "testdata/sql_input_default_dataset_legacy.json",
		},
		{
			// OTel input package with use_apm set by the user. The manifest does not
			// declare use_apm, so ensureUseAPMVar must inject it from varValues.
			name:               "otel_traces_use_apm",
			packageRoot:        "testdata/packages/otel_traces_input",
			policyTemplateName: "receiver",
			policyName:         "otel-traces-test",
			varValues: common.MapStr{
				"endpoint": "0.0.0.0:9411",
				"use_apm":  true,
			},
			goldenSimplified: "testdata/otel_traces_use_apm.json",
			goldenLegacy:     "testdata/otel_traces_use_apm_legacy.json",
		},
		{
			name:               "otel_dynamic_signal_types_default_dataset",
			packageRoot:        "testdata/packages/otel_dynamic_input",
			policyTemplateName: "sqlreceiver",
			policyName:         "otel-dynamic-test",
			varValues:          common.MapStr{},
			goldenSimplified:   "testdata/otel_input_dynamic_signals.json",
			goldenLegacy:       "testdata/otel_input_dynamic_signals_legacy.json",
		},
		{
			// Package-level variable: the user overrides the default package-level
			// var (custom_tag). BuildInputPackagePolicy must forward manifest.Vars
			// into the top-level policy vars just like BuildIntegrationPackagePolicy.
			name:               "input_with_pkg_vars",
			packageRoot:        "testdata/packages/input_with_pkg_vars",
			policyTemplateName: "logs",
			policyName:         "input-pkg-vars-test",
			varValues: common.MapStr{
				"paths":               []string{"/tmp/test.log"},
				"data_stream.dataset": "custom.logs",
				"custom_tag":          "my-tag",
			},
			goldenSimplified: "testdata/input_with_pkg_vars.json",
			goldenLegacy:     "testdata/input_with_pkg_vars_legacy.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest, err := packages.ReadPackageManifest(filepath.Join(tc.packageRoot, "manifest.yml"))
			require.NoError(t, err)

			policyTemplate, err := packages.SelectPolicyTemplateByName(manifest.PolicyTemplates, tc.policyTemplateName)
			require.NoError(t, err)

			pp := BuildInputPackagePolicy(
				"test-policy-id", "test", tc.policyName,
				*manifest, policyTemplate, tc.varValues,
				true,
			)

			t.Run("simplified", func(t *testing.T) {
				got, err := json.MarshalIndent(pp, "", "  ")
				require.NoError(t, err)
				assertJSONGolden(t, tc.goldenSimplified, got)
			})

			t.Run("legacy", func(t *testing.T) {
				legacy := pp.toLegacy()
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

func TestEnsureUseAPMVar(t *testing.T) {
	cases := []struct {
		name              string
		vars              Vars
		varValues         common.MapStr
		wantUseAPMPresent bool
		wantUseAPM        bool
		wantUnchanged     bool // existing keys must be unchanged
	}{
		{
			name:              "use_apm already in vars is left unchanged",
			vars:              Vars{"use_apm": {Value: varValue(false), Type: "boolean", fromUser: true}},
			varValues:         common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
			wantUnchanged:     true,
		},
		{
			name:              "no use_apm in varValues leaves vars unchanged",
			vars:              Vars{},
			varValues:         common.MapStr{},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "use_apm true is added",
			vars:              Vars{},
			varValues:         common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
		},
		{
			name:              "use_apm false is added",
			vars:              Vars{},
			varValues:         common.MapStr{"use_apm": false},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
		},
		{
			name:              "use_apm as string true is added",
			vars:              Vars{},
			varValues:         common.MapStr{"use_apm": "true"},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
		},
		{
			name:              "use_apm as string false is added",
			vars:              Vars{},
			varValues:         common.MapStr{"use_apm": "false"},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
		},
		{
			name:              "use_apm as unexpected string is not added",
			vars:              Vars{},
			varValues:         common.MapStr{"use_apm": "foo"},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "use_apm as int is not added",
			vars:              Vars{},
			varValues:         common.MapStr{"use_apm": 1},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "other vars are preserved when adding use_apm",
			vars:              Vars{"other": {Value: varValue("x"), Type: "text", fromUser: true}},
			varValues:         common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
		},
		{
			name:              "nil varValues does not add use_apm",
			vars:              Vars{},
			varValues:         nil,
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := make(Vars, len(c.vars))
			for k, v := range c.vars {
				got[k] = v
			}

			ensureUseAPMVar(got, c.varValues)

			if c.wantUnchanged {
				assert.Len(t, got, len(c.vars), "vars length should be unchanged")
			}
			if c.wantUseAPMPresent {
				require.Contains(t, got, "use_apm", "vars should contain use_apm")
				assert.Equal(t, "boolean", got["use_apm"].Type)
				assert.Equal(t, c.wantUseAPM, got["use_apm"].Value.Value())
			} else {
				assert.NotContains(t, got, "use_apm", "vars should not contain use_apm")
			}
			// Original vars must always be preserved.
			for k, v := range c.vars {
				require.Contains(t, got, k, "original var %q must be preserved", k)
				assert.Equal(t, v.Value.Value(), got[k].Value.Value(), "value for %q", k)
			}
		})
	}
}

func TestEnsureDatasetVar(t *testing.T) {
	cases := []struct {
		name           string
		vars           Vars
		policyTemplate packages.PolicyTemplate
		varValues      common.MapStr
		wantDataset    string
	}{
		{
			name:           "already set with fromUser=true is left unchanged",
			vars:           Vars{"data_stream.dataset": {Value: varValue("existing"), Type: "text", fromUser: true}},
			policyTemplate: packages.PolicyTemplate{Name: "sql_query"},
			varValues:      common.MapStr{"data_stream.dataset": "override"},
			wantDataset:    "existing",
		},
		{
			name:           "varValues overrides default",
			vars:           Vars{},
			policyTemplate: packages.PolicyTemplate{Name: "sql_query"},
			varValues:      common.MapStr{"data_stream.dataset": "custom.dataset"},
			wantDataset:    "custom.dataset",
		},
		{
			name:           "manifest default in vars is promoted",
			vars:           Vars{"data_stream.dataset": {Value: varValue("manifest.default"), Type: "text"}},
			policyTemplate: packages.PolicyTemplate{Name: "sql_query"},
			varValues:      common.MapStr{},
			wantDataset:    "manifest.default",
		},
		{
			name: "policyTemplate.Vars default is used when no user value",
			vars: Vars{},
			policyTemplate: packages.PolicyTemplate{
				Name: "sql_query",
				Vars: []packages.Variable{
					{Name: "data_stream.dataset", Default: func() *packages.VarValue {
						vv := packages.VarValue{}
						vv.Unpack("template.default")
						return &vv
					}()},
				},
			},
			varValues:   common.MapStr{},
			wantDataset: "template.default",
		},
		{
			name:           "falls back to policy template name",
			vars:           Vars{},
			policyTemplate: packages.PolicyTemplate{Name: "sql_query"},
			varValues:      common.MapStr{},
			wantDataset:    "sql_query",
		},
		{
			name:           "dynamic_signal_types falls back to policy template name",
			vars:           Vars{},
			policyTemplate: packages.PolicyTemplate{Name: "sqlreceiver", DynamicSignalTypes: true},
			varValues:      common.MapStr{},
			wantDataset:    "sqlreceiver",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := make(Vars, len(c.vars))
			for k, v := range c.vars {
				got[k] = v
			}

			ensureDatasetVar(got, c.policyTemplate, c.varValues)

			require.Contains(t, got, "data_stream.dataset")
			assert.Equal(t, "text", got["data_stream.dataset"].Type)
			assert.Equal(t, c.wantDataset, got["data_stream.dataset"].Value.Value())
		})
	}
}

func varValue(v any) packages.VarValue {
	vv := packages.VarValue{}
	vv.Unpack(v)
	return vv
}
