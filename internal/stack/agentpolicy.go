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

const managedAgentPolicyID = "elastic-agent-managed-ep"

// createAgentPolicy creates an agent policy with the initial configuration used for
// agents managed by elastic-package.
func createAgentPolicy(ctx context.Context, kibanaClient *kibana.Client, stackVersion string, outputId string, selfMonitor bool) (*kibana.Policy, error) {
	policy := kibana.Policy{
		ID:                managedAgentPolicyID,
		Name:              "Elastic-Agent (elastic-package)",
		Description:       "Policy created by elastic-package",
		Namespace:         "default",
		MonitoringEnabled: []string{},
		DataOutputID:      outputId,
	}
	if selfMonitor {
		policy.MonitoringEnabled = []string{"logs", "metrics"}
	}

	newPolicy, err := kibanaClient.CreatePolicy(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("error while creating agent policy: %w", err)
	}

	if selfMonitor {
		err := createSystemPackagePolicy(ctx, kibanaClient, stackVersion, newPolicy.ID, newPolicy.Namespace)
		if err != nil {
			return nil, err
		}
	}

	return newPolicy, nil
}

func createSystemPackagePolicy(ctx context.Context, kibanaClient *kibana.Client, stackVersion, agentPolicyID, namespace string) error {
	systemPackages, err := registry.Production.Revisions("system", registry.SearchOptions{
		KibanaVersion: strings.TrimSuffix(stackVersion, kibana.SNAPSHOT_SUFFIX),
	})
	if err != nil {
		return fmt.Errorf("could not get the system package version for Kibana %v: %w", stackVersion, err)
	}
	if len(systemPackages) != 1 {
		return fmt.Errorf("unexpected number of system package versions for Kibana %s - found %d expected 1", stackVersion, len(systemPackages))
	}
	logger.Debugf("Found %s package - version %s", systemPackages[0].Name, systemPackages[0].Version)
	packagePolicy := kibana.PackagePolicy{
		Name:      "system-1",
		PolicyID:  agentPolicyID,
		Namespace: namespace,
	}
	packagePolicy.Package.Name = "system"
	packagePolicy.Package.Version = systemPackages[0].Version

	_, err = kibanaClient.CreatePackagePolicy(ctx, packagePolicy)
	if err != nil {
		return fmt.Errorf("error while creating package policy: %w", err)
	}

	return nil
}

func deleteAgentPolicy(ctx context.Context, kibanaClient *kibana.Client) error {
	err := kibanaClient.DeletePolicy(ctx, managedAgentPolicyID)
	var notFoundError *kibana.ErrPolicyNotFound
	if err != nil && !errors.As(err, &notFoundError) {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	return nil
}

func forceUnenrollAgentsWithPolicy(ctx context.Context, kibanaClient *kibana.Client) error {
	agents, err := kibanaClient.QueryAgents(ctx, fmt.Sprintf("policy_id: %s", managedAgentPolicyID))
	if err != nil {
		return fmt.Errorf("error while querying agents with policy %s: %w", managedAgentPolicyID, err)
	}

	for _, agent := range agents {
		err := kibanaClient.RemoveAgent(ctx, agent)
		if err != nil {
			return fmt.Errorf("failed to remove agent %s: %w", agent.ID, err)
		}
	}

	return nil
}
