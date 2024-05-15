// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/elastic/elastic-package/internal/kibana"
	kibanatest "github.com/elastic/elastic-package/internal/kibana/test"
	"github.com/elastic/go-resource"
	"github.com/stretchr/testify/assert"
)

func TestRequiredProviderFleetPolicy(t *testing.T) {
	manager := resource.NewManager()
	_, err := manager.Apply(resource.Resources{
		&FleetAgentPolicy{
			Name: "test-policy",
		},
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), fmt.Sprintf("provider %q must be explicitly defined", DefaultKibanaProviderName))
	}
}

func TestPolicyLifecycle(t *testing.T) {
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
					RootPath:       "../../test/packages/parallel/nginx",
					DataStreamName: "stubstatus",
				},
			},
		},
		{
			title: "multiple-packages",
			packagePolicies: []FleetPackagePolicy{
				{
					Name:           "nginx-1",
					RootPath:       "../../test/packages/parallel/nginx",
					DataStreamName: "stubstatus",
				},
				{
					Name:           "system-1",
					RootPath:       "../../test/packages/parallel/system",
					DataStreamName: "process",
				},
			},
		},
		{
			title: "input-package",
			packagePolicies: []FleetPackagePolicy{
				{
					Name:     "input-1",
					RootPath: "../../test/packages/parallel/sql_input",
				},
			},
		},
	}

	for i, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			recordPath := filepath.Join("testdata", "kibana-8-mock-package-lifecycle-"+c.title)
			kibanaClient := kibanatest.NewClient(t, recordPath)

			id := fmt.Sprintf("test-policy-%d", i)
			t.Cleanup(func() { deletePolicy(t, kibanaClient, id) })

			agentPolicy := FleetAgentPolicy{
				Name:            id,
				ID:              id,
				Description:     fmt.Sprintf("Test policy for %s", c.title),
				Namespace:       "eptest",
				PackagePolicies: c.packagePolicies,
			}
			resources := preInstallPackages(&agentPolicy)

			manager := resource.NewManager()
			manager.RegisterProvider(DefaultKibanaProviderName, &KibanaProvider{Client: kibanaClient})
			_, err := manager.Apply(resources)
			assert.NoError(t, err)
			assertPolicyPresent(t, kibanaClient, true, agentPolicy.ID)

			agentPolicy.Absent = true
			_, err = manager.Apply(resources)
			assert.NoError(t, err)
			assertPolicyPresent(t, kibanaClient, false, agentPolicy.ID)
		})
	}
}

// preInstallPackages prepares a list of resources that ensures that all required packages are installed
// before creating the policy.
func preInstallPackages(agentPolicy *FleetAgentPolicy) resource.Resources {
	var resources resource.Resources
	for _, policy := range agentPolicy.PackagePolicies {
		resources = append(resources, &FleetPackage{
			RootPath: policy.RootPath,
		})
	}
	return append(resources, agentPolicy)
}

func assertPolicyPresent(t *testing.T, client *kibana.Client, expected bool, policyID string) bool {
	t.Helper()

	_, err := client.GetPolicy(context.Background(), policyID)
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

func deletePolicy(t *testing.T, client *kibana.Client, policyID string) {
	err := client.DeletePolicy(context.Background(), policyID)
	var notFoundError *kibana.ErrPolicyNotFound
	if errors.As(err, &notFoundError) {
		return
	}
	assert.NoError(t, err)
}
