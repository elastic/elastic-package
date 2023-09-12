// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/elastic/elastic-package/internal/kibana"
)

const AgentPoliciesDumpDir = "agent_policies"

// AgentPoliciesDumper discovers and dumps agent policies in Fleet
type AgentPoliciesDumper struct {
	client *kibana.Client
}

type AgentPolicy struct {
	name string
	raw  json.RawMessage
}

func (p AgentPolicy) Name() string {
	return p.name
}

func (p AgentPolicy) JSON() []byte {
	return p.raw
}

// NewAgentPoliciesDumper creates an AgentPoliciesDumper
func NewAgentPoliciesDumper(client *kibana.Client) *AgentPoliciesDumper {
	return &AgentPoliciesDumper{
		client: client,
	}
}

func (d *AgentPoliciesDumper) getAgentPolicy(ctx context.Context, name string) (*AgentPolicy, error) {
	policy, err := d.client.GetRawPolicy(name)
	if err != nil {
		return nil, err
	}
	return &AgentPolicy{name: name, raw: policy}, nil
}

func (d *AgentPoliciesDumper) DumpByName(ctx context.Context, dir, name string) error {
	agentPolicy, err := d.getAgentPolicy(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get agent policy: %w", err)
	}

	dir = filepath.Join(dir, AgentPoliciesDumpDir)
	err = dumpJSONResource(dir, agentPolicy)
	if err != nil {
		return fmt.Errorf("failed to dump agent policy %s: %w", agentPolicy.Name(), err)
	}
	return nil
}

func (d *AgentPoliciesDumper) getAllAgentPolicies(ctx context.Context) ([]AgentPolicy, error) {
	return d.getAgentPoliciesFilteredByPackage(ctx, "")
}

type packagePolicy struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Package struct {
		Name    string `json:"name"`
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"package"`
}

func getPackagesUsingAgentPolicy(packagePolicies []packagePolicy) []string {
	var packageNames []string
	for _, packagePolicy := range packagePolicies {
		packageNames = append(packageNames, packagePolicy.Package.Name)
	}
	return packageNames
}

func (d *AgentPoliciesDumper) getAgentPoliciesFilteredByPackage(ctx context.Context, packageName string) ([]AgentPolicy, error) {
	rawPolicies, err := d.client.ListRawPolicies()

	if err != nil {
		return nil, err
	}

	var policyPackages struct {
		ID              string          `json:"id"`
		PackagePolicies []packagePolicy `json:"package_policies"`
	}

	var policies []AgentPolicy

	for _, policy := range rawPolicies {
		err = json.Unmarshal(policy, &policyPackages)
		if err != nil {
			return nil, fmt.Errorf("failed to get Agent Policy ID: %w", err)
		}
		if packageName != "" {
			packageNames := getPackagesUsingAgentPolicy(policyPackages.PackagePolicies)
			if !slices.Contains(packageNames, packageName) {
				continue
			}
		}

		agentPolicy := AgentPolicy{name: policyPackages.ID, raw: policy}
		policies = append(policies, agentPolicy)
	}
	return policies, nil
}

func (d *AgentPoliciesDumper) DumpAll(ctx context.Context, dir string) (count int, err error) {
	agentPolicies, err := d.getAllAgentPolicies(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get agent policy: %w", err)
	}

	dir = filepath.Join(dir, AgentPoliciesDumpDir)
	for _, agentPolicy := range agentPolicies {
		err := dumpJSONResource(dir, agentPolicy)
		if err != nil {
			return 0, fmt.Errorf("failed to dump agent policy %s: %w", agentPolicy.Name(), err)
		}
	}
	return len(agentPolicies), nil
}

func (d *AgentPoliciesDumper) DumpByPackage(ctx context.Context, dir, packageName string) (count int, err error) {
	agentPolicies, err := d.getAgentPoliciesFilteredByPackage(ctx, packageName)
	if err != nil {
		return 0, fmt.Errorf("failed to get agent policy: %w", err)
	}

	dir = filepath.Join(dir, AgentPoliciesDumpDir)
	for _, agentPolicy := range agentPolicies {
		err := dumpJSONResource(dir, agentPolicy)
		if err != nil {
			return 0, fmt.Errorf("failed to dump agent policy %s: %w", agentPolicy.Name(), err)
		}
	}
	return len(agentPolicies), nil
}
