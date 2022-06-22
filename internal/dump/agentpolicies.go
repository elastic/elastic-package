// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
)

const AgentPoliciesDumpDir = "agent_policies"

type AgentPolicyDumper struct {
	name   string
	client *kibana.Client

	policy *AgentPolicy
}

type AgentPoliciesDumper struct {
	client *kibana.Client

	policies []AgentPolicy
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

func NewAgentPolicyDumper(client *kibana.Client, agentPolicy string) *AgentPolicyDumper {
	return &AgentPolicyDumper{
		name:   agentPolicy,
		client: client,
	}
}

func NewAgentPoliciesDumper(client *kibana.Client) *AgentPoliciesDumper {
	return &AgentPoliciesDumper{
		client: client,
	}
}

func (d *AgentPolicyDumper) getAgentPolicy(ctx context.Context) (*AgentPolicy, error) {
	if d.policy == nil {
		policy, err := d.client.GetRawPolicy(d.name)

		if err != nil {
			return nil, err
		}
		agentPolicy := AgentPolicy{name: d.name, raw: policy}
		d.policy = &agentPolicy
	}
	return d.policy, nil
}

func (d *AgentPolicyDumper) DumpAgentPolicy(ctx context.Context, dir string) error {
	agentPolicy, err := d.getAgentPolicy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agent policy: %w", err)
	}

	dir = filepath.Join(dir, AgentPoliciesDumpDir)
	err = dumpInstalledObject(dir, agentPolicy)
	if err != nil {
		return fmt.Errorf("failed to dump agent policy %s: %w", agentPolicy.Name(), err)
	}
	return nil
}

func (d *AgentPoliciesDumper) getAllAgentPolicies(ctx context.Context) ([]AgentPolicy, error) {
	if len(d.policies) == 0 {
		policies, err := d.client.ListRawPolicy()

		if err != nil {
			return nil, err
		}

		var policyName struct {
			ID string `json:"id"`
		}

		for _, policy := range policies {
			err = json.Unmarshal(policy, &policyName)
			if err != nil {
				return nil, fmt.Errorf("failed to get Agent Policy ID: %w", err)
			}
			agentPolicy := AgentPolicy{name: policyName.ID, raw: policy}
			d.policies = append(d.policies, agentPolicy)
		}
	}
	return d.policies, nil
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
	if len(d.policies) == 0 {
		policies, err := d.client.ListRawPolicy()

		if err != nil {
			return nil, err
		}

		var policyPackages struct {
			ID              string          `json:"id"`
			PackagePolicies []packagePolicy `json:"package_policies"`
		}

		for _, policy := range policies {
			err = json.Unmarshal(policy, &policyPackages)
			if err != nil {
				return nil, fmt.Errorf("failed to get Agent Policy ID: %w", err)
			}
			packageNames := getPackagesUsingAgentPolicy(policyPackages.PackagePolicies)
			fmt.Printf("package Name %s\n", packageName)
			fmt.Printf("packageNames %s\n", packageNames)
			fmt.Printf("Struct package policies %+v\n", policyPackages.PackagePolicies)
			if !common.StringSliceContains(packageNames, packageName) {
				continue
			}

			agentPolicy := AgentPolicy{name: policyPackages.ID, raw: policy}
			d.policies = append(d.policies, agentPolicy)
		}
	}
	return d.policies, nil
}

func (d *AgentPoliciesDumper) DumpAll(ctx context.Context, dir string) (count int, err error) {
	agentPolicies, err := d.getAllAgentPolicies(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get agent policy: %w", err)
	}

	dir = filepath.Join(dir, AgentPoliciesDumpDir)
	for _, agentPolicy := range agentPolicies {
		err := dumpInstalledObject(dir, agentPolicy)
		if err != nil {
			return 0, fmt.Errorf("failed to dump agent policy %s: %w", agentPolicy.Name(), err)
		}
	}
	return len(agentPolicies), nil
}

func (d *AgentPoliciesDumper) DumpAgentPoliciesFileteredByPackage(ctx context.Context, packageName, dir string) (count int, err error) {
	agentPolicies, err := d.getAgentPoliciesFilteredByPackage(ctx, packageName)
	if err != nil {
		return 0, fmt.Errorf("failed to get agent policy: %w", err)
	}

	dir = filepath.Join(dir, AgentPoliciesDumpDir)
	for _, agentPolicy := range agentPolicies {
		err := dumpInstalledObject(dir, agentPolicy)
		if err != nil {
			return 0, fmt.Errorf("failed to dump agent policy %s: %w", agentPolicy.Name(), err)
		}
	}
	return len(agentPolicies), nil
}
