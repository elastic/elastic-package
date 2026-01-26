// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/registry"
)

const (
	managedFleetServerPolicyID = "fleet-server-managed-ep"
)

// createFleetServerPolicy creates an agent policy with the initial configuration used for
// agents managed by elastic-package.
func createFleetServerPolicy(ctx context.Context, kibanaClient *kibana.Client, registryClient *registry.Client, stackVersion string, namespace string) (*kibana.Policy, error) {
	policy := kibana.Policy{
		Name:                 "Fleet Server (elastic-package)",
		ID:                   managedFleetServerPolicyID,
		IsDefaultFleetServer: true,
		Description:          "Policy created by elastic-package",
		Namespace:            "default",
		MonitoringEnabled:    []string{},
	}

	newPolicy, err := kibanaClient.CreatePolicy(ctx, policy)
	if errors.Is(err, kibana.ErrConflict) {
		newPolicy, err = kibanaClient.GetPolicy(ctx, policy.ID)
		if err != nil {
			return nil, fmt.Errorf("error while getting existing policy: %w", err)
		}
		return newPolicy, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error while creating agent policy: %w", err)
	}

	err = createFleetServerPackagePolicy(ctx, kibanaClient, registryClient, stackVersion, newPolicy.ID, newPolicy.Namespace)
	if err != nil {
		return nil, err
	}

	return newPolicy, nil
}

func createFleetServerPackagePolicy(ctx context.Context, kibanaClient *kibana.Client, registryClient *registry.Client, stackVersion, agentPolicyID, namespace string) error {
	packages, err := registryClient.Revisions("fleet_server", registry.SearchOptions{
		KibanaVersion: strings.TrimSuffix(stackVersion, kibana.SNAPSHOT_SUFFIX),
	})
	if err != nil {
		return fmt.Errorf("could not get the fleet_server package version for Kibana %v: %w", stackVersion, err)
	}
	if len(packages) != 1 {
		return fmt.Errorf("unexpected number of fleet_server package versions for Kibana %s - found %d expected 1", stackVersion, len(packages))
	}
	logger.Debugf("Found %s package - version %s", packages[0].Name, packages[0].Version)
	packagePolicy := kibana.PackagePolicy{
		Name:      "fleet-server-ep",
		PolicyID:  agentPolicyID,
		Namespace: namespace,
	}
	packagePolicy.Package.Name = "fleet_server"
	packagePolicy.Package.Version = packages[0].Version

	_, err = kibanaClient.CreatePackagePolicy(ctx, packagePolicy)
	if err != nil {
		return fmt.Errorf("error while creating package policy: %w", err)
	}

	return nil
}

func deleteFleetServerPolicy(ctx context.Context, kibanaClient *kibana.Client) error {
	err := kibanaClient.DeletePolicy(ctx, managedFleetServerPolicyID)
	var notFoundError *kibana.ErrPolicyNotFound
	if err != nil && !errors.As(err, &notFoundError) {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	return nil
}

func forceUnenrollFleetServerWithPolicy(ctx context.Context, kibanaClient *kibana.Client) error {
	agents, err := kibanaClient.QueryAgents(ctx, fmt.Sprintf("policy_id: %s", managedFleetServerPolicyID))
	if err != nil {
		return fmt.Errorf("error while querying agents with policy %s: %w", managedFleetServerPolicyID, err)
	}

	for _, agent := range agents {
		err := kibanaClient.RemoveAgent(ctx, agent)
		if err != nil {
			return fmt.Errorf("failed to remove agent %s: %w", agent.ID, err)
		}
	}

	return nil
}
