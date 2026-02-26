// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/elastic/go-resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/kibana"
	kibanatest "github.com/elastic/elastic-package/internal/kibana/test"
	"github.com/elastic/elastic-package/internal/packages"
)

func TestRequiredProviderFleetPolicy(t *testing.T) {
	repositoryRoot, err := files.FindRepositoryRoot()
	require.NoError(t, err)
	t.Cleanup(func() { _ = repositoryRoot.Close() })

	manager := resource.NewManager()
	_, err = manager.Apply(resource.Resources{
		&FleetAgentPolicy{
			Name: "test-policy",
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), fmt.Sprintf("provider %q must be explicitly defined", DefaultKibanaProviderName))
	}
}

func TestPolicyLifecycle(t *testing.T) {
	repositoryRoot, err := files.FindRepositoryRoot()
	require.NoError(t, err)
	t.Cleanup(func() { _ = repositoryRoot.Close() })

	cases := []struct {
		title           string
		packagePolicies []FleetPackagePolicy
	}{
		{
			title: "empty-policy",
		},
		{
			title: "one-package",
			packagePolicies: []FleetPackagePolicy{
				{
					Name:           "nginx-1",
					PackageRoot:    filepath.Join(repositoryRoot.Name(), "test", "packages", "parallel", "nginx"),
					DataStreamName: "stubstatus",
				},
			},
		},
		{
			title: "multiple-packages",
			packagePolicies: []FleetPackagePolicy{
				{
					Name:           "nginx-1",
					PackageRoot:    filepath.Join(repositoryRoot.Name(), "test", "packages", "parallel", "nginx"),
					DataStreamName: "stubstatus",
				},
				{
					Name:           "system-1",
					PackageRoot:    filepath.Join(repositoryRoot.Name(), "test", "packages", "parallel", "system"),
					DataStreamName: "process",
				},
			},
		},
		{
			title: "input-package",
			packagePolicies: []FleetPackagePolicy{
				{
					Name:        "input-1",
					PackageRoot: filepath.Join(repositoryRoot.Name(), "test", "packages", "parallel", "sql_input"),
				},
			},
		},
	}

	for i, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			recordPath := filepath.Join("testdata", "kibana-8-mock-policy-lifecycle-"+c.title)
			kibanaClient := kibanatest.NewClient(t, recordPath)

			manager := resource.NewManager()
			manager.RegisterProvider(DefaultKibanaProviderName, &KibanaProvider{Client: kibanaClient})

			id := fmt.Sprintf("test-policy-%d", i)
			agentPolicy := FleetAgentPolicy{
				Name:            id,
				ID:              id,
				Description:     fmt.Sprintf("Test policy for %s", c.title),
				Namespace:       "eptest",
				PackagePolicies: c.packagePolicies,
			}
			t.Cleanup(func() { deletePolicy(t, manager, agentPolicy, repositoryRoot) })

			_, err := manager.Apply(withPackageResources(&agentPolicy, repositoryRoot))
			assert.NoError(t, err)
			assertPolicyPresent(t, kibanaClient, true, agentPolicy.ID)

			agentPolicy.Absent = true
			_, err = manager.Apply(withPackageResources(&agentPolicy, repositoryRoot))
			assert.NoError(t, err)
			assertPolicyPresent(t, kibanaClient, false, agentPolicy.ID)
		})
	}
}

// withPackageResources prepares a list of resources that ensures that all required packages are installed
// before creating the policy.
func withPackageResources(agentPolicy *FleetAgentPolicy, repostoryRoot *os.Root) resource.Resources {
	var resources resource.Resources
	for _, policy := range agentPolicy.PackagePolicies {
		resources = append(resources, &FleetPackage{
			PackageRoot:    policy.PackageRoot,
			Absent:         agentPolicy.Absent,
			RepositoryRoot: repostoryRoot,
			SchemaURLs:     fields.NewSchemaURLs(),
		})
	}
	return append(resources, agentPolicy)
}

func assertPolicyPresent(t *testing.T, client *kibana.Client, expected bool, policyID string) bool {
	t.Helper()

	_, err := client.GetPolicy(t.Context(), policyID)
	if expected {
		return assert.NoError(t, err)
	}
	var notFoundError *kibana.ErrPolicyNotFound
	if errors.As(err, &notFoundError) {
		return true
	}
	assert.NoError(t, err)
	return false
}

func deletePolicy(t *testing.T, manager *resource.Manager, agentPolicy FleetAgentPolicy, repositoryRoot *os.Root) {
	t.Helper()

	agentPolicy.Absent = true
	_, err := manager.Apply(withPackageResources(&agentPolicy, repositoryRoot))
	assert.NoError(t, err, "cleanup execution")
}

func TestCreateInputPackagePolicy_DatasetVariable(t *testing.T) {
	defaultValue := func(v any) *packages.VarValue {
		vv := &packages.VarValue{}
		vv.Unpack(v)
		return vv
	}

	policy := FleetAgentPolicy{
		ID:        "test-policy-id",
		Namespace: "eptest",
	}

	cases := []struct {
		name            string
		manifest        packages.PackageManifest
		packagePolicy   FleetPackagePolicy
		wantErr         bool
		expectedDataset string
	}{
		{
			name: "dataset var added when missing",
			manifest: packages.PackageManifest{
				Type:    "input",
				Name:    "sql_input",
				Title:   "SQL Input",
				Version: "0.2.0",
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:  "sql_query",
						Input: "sql/metrics",
						Type:  "metrics",
					},
				},
			},
			packagePolicy: FleetPackagePolicy{
				Name:         "input-1",
				TemplateName: "sql_query",
			},
			expectedDataset: "sql_query",
		},
		{
			name: "dataset var from template default",
			manifest: packages.PackageManifest{
				Type:    "input",
				Name:    "custom_input",
				Title:   "Custom Input",
				Version: "1.0.0",
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:  "custom_template",
						Input: "custom/metrics",
						Type:  "metrics",
						Vars: []packages.Variable{
							{
								Name:    "data_stream.dataset",
								Type:    "text",
								Default: defaultValue("custom.default"),
							},
						},
					},
				},
			},
			packagePolicy: FleetPackagePolicy{
				Name:         "input-2",
				TemplateName: "custom_template",
			},
			expectedDataset: "custom.default",
		},
		{
			name: "dataset var from user (packagePolicy.Vars)",
			manifest: packages.PackageManifest{
				Type:    "input",
				Name:    "sql_input",
				Title:   "SQL Input",
				Version: "0.2.0",
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:  "sql_query",
						Input: "sql/metrics",
						Type:  "metrics",
						Vars: []packages.Variable{
							{
								Name:    "data_stream.dataset",
								Type:    "text",
								Default: defaultValue("sql_query"),
							},
						},
					},
				},
			},
			packagePolicy: FleetPackagePolicy{
				Name:         "input-3",
				TemplateName: "sql_query",
				Vars: map[string]any{
					"data_stream.dataset": "user.custom",
				},
			},
			expectedDataset: "user.custom",
		},
		{
			name: "error when DataStreamName is set for input package",
			manifest: packages.PackageManifest{
				Type:    "input",
				Name:    "sql_input",
				Title:   "SQL Input",
				Version: "0.2.0",
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:  "sql_query",
						Input: "sql/metrics",
						Type:  "metrics",
					},
				},
			},
			packagePolicy: FleetPackagePolicy{
				Name:           "input-4",
				TemplateName:   "sql_query",
				DataStreamName: "some_stream",
			},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result, err := createInputPackagePolicy(policy, c.manifest, c.packagePolicy)
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.Inputs, 1)
			require.Len(t, result.Inputs[0].Streams, 1)

			streamVars := result.Inputs[0].Streams[0].Vars
			require.Contains(t, streamVars, "data_stream.dataset", "stream vars must contain data_stream.dataset")

			datasetVar := streamVars["data_stream.dataset"]
			val := datasetVar.Value.Value()
			require.NotNil(t, val)
			assert.Equal(t, c.expectedDataset, val, "data_stream.dataset variable value")
		})
	}
}

func TestSetUseAPMVariable(t *testing.T) {
	defaultVarValue := func(v any) packages.VarValue {
		vv := packages.VarValue{}
		vv.Unpack(v)
		return vv
	}

	cases := []struct {
		name              string
		vars              kibana.Vars
		variablesToAssign common.MapStr
		wantUseAPM        bool // true = key present and true, false = key present and false, <not set> = key absent
		wantUseAPMPresent bool
		wantUnchanged     bool // when true, vars must be the same map (no new keys, existing keys unchanged)
	}{
		{
			// use_apm is defined in the package manifest, so it is not added by setUseAPMVariable
			name:              "use_apm already in vars is left unchanged",
			vars:              kibana.Vars{"use_apm": {Value: defaultVarValue(false), Type: "boolean"}},
			variablesToAssign: common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
			wantUnchanged:     true,
		},
		{
			name:              "no use_apm in variablesToAssign leaves vars unchanged",
			vars:              kibana.Vars{},
			variablesToAssign: common.MapStr{},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "use_apm true is added",
			vars:              kibana.Vars{},
			variablesToAssign: common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
		},
		{
			name:              "use_apm false is added",
			vars:              kibana.Vars{},
			variablesToAssign: common.MapStr{"use_apm": false},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
		},
		{
			name:              "use_apm as string true is added",
			vars:              kibana.Vars{},
			variablesToAssign: common.MapStr{"use_apm": "true"},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
			wantUnchanged:     false,
		},
		{
			name:              "use_apm as string false is added",
			vars:              kibana.Vars{},
			variablesToAssign: common.MapStr{"use_apm": "false"},
			wantUseAPMPresent: true,
			wantUseAPM:        false,
			wantUnchanged:     false,
		},
		{
			name:              "use_apm as unexpected string is not added",
			vars:              kibana.Vars{},
			variablesToAssign: common.MapStr{"use_apm": "foo"},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "use_apm as int is not added",
			vars:              kibana.Vars{},
			variablesToAssign: common.MapStr{"use_apm": 1},
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
		{
			name:              "other vars are preserved when adding use_apm",
			vars:              kibana.Vars{"other": {Value: defaultVarValue("x"), Type: "text"}},
			variablesToAssign: common.MapStr{"use_apm": true},
			wantUseAPMPresent: true,
			wantUseAPM:        true,
		},
		{
			name:              "nil variablesToAssign does not add use_apm",
			vars:              kibana.Vars{},
			variablesToAssign: nil,
			wantUseAPMPresent: false,
			wantUnchanged:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Copy vars so we can compare for unchanged when needed
			inputVars := make(kibana.Vars, len(c.vars))
			for k, v := range c.vars {
				inputVars[k] = v
			}

			got := setUseAPMVariable(inputVars, c.variablesToAssign)

			if c.wantUnchanged && len(c.vars) == len(got) {
				for k, v := range c.vars {
					g, ok := got[k]
					require.True(t, ok, "key %q should remain", k)
					assert.Equal(t, v.Value.Value(), g.Value.Value(), "value for %q", k)
				}
			}

			if c.wantUseAPMPresent {
				require.Contains(t, got, "use_apm", "vars should contain use_apm")
				assert.Equal(t, "boolean", got["use_apm"].Type)
				assert.Equal(t, c.wantUseAPM, got["use_apm"].Value.Value())
			} else {
				assert.NotContains(t, got, "use_apm", "vars should not contain use_apm")
			}

			// Original vars must always be preserved
			for k, v := range c.vars {
				require.Contains(t, got, k, "original var %q must be preserved", k)
				assert.Equal(t, v.Value.Value(), got[k].Value.Value(), "value for %q", k)
			}
		})
	}
}
