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

			// Find the enabled input by its key.
			pt := c.manifest.PolicyTemplates[0]
			inputKey := fmt.Sprintf("%s-%s", pt.Name, pt.Input)
			inputEntry, ok := result.Inputs[inputKey]
			require.True(t, ok, "expected input key %q in inputs map", inputKey)
			require.True(t, inputEntry.Enabled)

			streamKey := fmt.Sprintf("%s.%s", c.manifest.Name, pt.Name)
			streamEntry, ok := inputEntry.Streams[streamKey]
			require.True(t, ok, "expected stream key %q in streams map", streamKey)

			streamVars := streamEntry.Vars
			require.Contains(t, streamVars, "data_stream.dataset", "stream vars must contain data_stream.dataset")

			datasetVar, ok := streamVars["data_stream.dataset"].(kibana.Var)
			require.True(t, ok, "data_stream.dataset must be a kibana.Var")
			val := datasetVar.Value.Value()
			require.NotNil(t, val)
			assert.Equal(t, c.expectedDataset, val, "data_stream.dataset variable value")
		})
	}
}
