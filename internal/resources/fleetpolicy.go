// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"errors"
	"fmt"
	"slices"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
)

type FleetAgentPolicy struct {
	// Provider is the name of the provider to use, defaults to "kibana".
	Provider string

	// Name of the policy.
	Name string

	// ID of the policy.
	ID string

	// Description of the policy.
	Description string

	// Namespace to use for the policy.
	Namespace string

	// DataOutputID is the identifier of the output to use.
	DataOutputID string

	// Absent is set to true to indicate that the policy should not be present.
	Absent bool

	// PackagePolicies
	PackagePolicies []FleetPackagePolicy
}

type FleetPackagePolicy struct {
	// Provider is the name of the provider to use, defaults to "kibana".
	Provider string

	// Name of the policy.
	Name string

	// Disabled indicates whether this policy is disabled.
	Disabled bool

	// TemplateName is the policy template to use from the package manifest.
	TemplateName string

	// PackageRoot is the root of the source of the package to configure, from it we should
	// be able to read the manifest, the data stream manifests and the policy template to use.
	PackageRoot string

	// DataStreamName is the name of the data stream to configure, for integration packages.
	DataStreamName string

	// InputName is the name of the input to select.
	InputName string

	// Vars contains the values for the variables.
	Vars map[string]any

	// DataStreamVars contains the values for the variables at the data stream level.
	DataStreamVars map[string]any

	// PolicyAPIFormat overrides the Fleet API format used to create the package policy.
	// Valid values: "simplified", "legacy", "" (auto-detect, default).
	PolicyAPIFormat string

	// Absent is set to true to indicate that the policy should not be present.
	Absent bool
}

func (f *FleetPackagePolicy) String() string {
	return fmt.Sprintf("[FleetPackagePolicy:%s:%s]", f.Provider, f.Name)
}

func (f *FleetAgentPolicy) String() string {
	return fmt.Sprintf("[FleetAgentPolicy:%s:%s]", f.Provider, f.Name)
}

func (f *FleetAgentPolicy) provider(ctx resource.Context) (*KibanaProvider, error) {
	name := f.Provider
	if name == "" {
		name = DefaultKibanaProviderName
	}
	var provider *KibanaProvider
	ok := ctx.Provider(name, &provider)
	if !ok {
		return nil, fmt.Errorf("provider %q must be explicitly defined", name)
	}
	return provider, nil
}

func (f *FleetAgentPolicy) Get(ctx resource.Context) (current resource.ResourceState, err error) {
	state := FleetAgentPolicyState{
		expected: !f.Absent,
	}
	if f.ID == "" {
		return &state, nil
	}

	provider, err := f.provider(ctx)
	if err != nil {
		return nil, err
	}

	policy, err := provider.Client.GetPolicy(ctx, f.ID)
	var notFoundError *kibana.ErrPolicyNotFound
	if errors.As(err, &notFoundError) {
		return &state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get policy %q with id %q: %w", f.Name, f.ID, err)
	}
	state.current = policy

	return &state, nil
}

func (f *FleetAgentPolicy) Create(ctx resource.Context) error {
	provider, err := f.provider(ctx)
	if err != nil {
		return err
	}

	policy, err := provider.Client.CreatePolicy(ctx, kibana.Policy{
		ID:           f.ID,
		Name:         f.Name,
		Namespace:    f.Namespace,
		Description:  f.Description,
		DataOutputID: f.DataOutputID,
	})
	if err != nil {
		return fmt.Errorf("could not create policy %q: %w", f.Name, err)
	}
	f.ID = policy.ID

	kibanaPolicy := &kibana.Policy{ID: f.ID, Namespace: f.Namespace}
	for _, packagePolicy := range f.PackagePolicies {
		pp, err := buildFleetPackagePolicy(kibanaPolicy, f.DataOutputID, packagePolicy)
		if err != nil {
			return fmt.Errorf("could not prepare package policy: %w", err)
		}
		_, err = provider.Client.CreatePackagePolicy(ctx, pp, packagePolicy.PolicyAPIFormat)
		if err != nil {
			return fmt.Errorf("could not add package policy %q to agent policy %q: %w", packagePolicy.Name, f.Name, err)
		}
	}

	return nil
}

// buildFleetPackagePolicy loads manifests from the built package tree and delegates
// to kibana.BuildPackagePolicy, then applies fleet-specific overrides (name, output).
// Composable integrations require the built tree so that RequiredInputsResolver has
// already materialized package: references into concrete input types.
func buildFleetPackagePolicy(kibanaPolicy *kibana.Policy, outputID string, fp FleetPackagePolicy) (kibana.PackagePolicy, error) {
	builtRoot, manifest, err := builder.ReadBuiltPackageManifest(fp.PackageRoot)
	if err != nil {
		return kibana.PackagePolicy{}, fmt.Errorf("reading built package manifest: %w", err)
	}

	if manifest.Type == "input" && fp.DataStreamName != "" {
		return kibana.PackagePolicy{}, fmt.Errorf("no data stream expected for input package policy %q, found %q", fp.Name, fp.DataStreamName)
	}
	if manifest.Type == "integration" && fp.DataStreamName == "" {
		return kibana.PackagePolicy{}, fmt.Errorf("expected data stream for integration package policy %q", fp.Name)
	}

	var dsManifest *packages.DataStreamManifest
	var allDatastreams []packages.DataStreamManifest
	if manifest.Type == "integration" {
		allDatastreams, err = packages.ReadAllDataStreamManifests(builtRoot)
		if err != nil {
			return kibana.PackagePolicy{}, fmt.Errorf("could not read data stream manifests: %w", err)
		}
		i := slices.IndexFunc(allDatastreams, func(ds packages.DataStreamManifest) bool {
			return ds.Name == fp.DataStreamName
		})
		if i < 0 {
			return kibana.PackagePolicy{}, fmt.Errorf("data stream %q not found in built package at %s", fp.DataStreamName, builtRoot)
		}
		dsManifest = &allDatastreams[i]
	}

	policyTemplateName := fp.TemplateName
	if policyTemplateName == "" {
		policyTemplateName, err = packages.FindPolicyTemplateForInput(manifest, dsManifest, fp.InputName)
		if err != nil {
			return kibana.PackagePolicy{}, fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
	}

	pp, _, _, err := kibana.BuildPackagePolicy(
		kibanaPolicy, policyTemplateName, fp.DataStreamName, fp.InputName,
		common.MapStr(fp.Vars), common.MapStr(fp.DataStreamVars),
		"", *manifest, dsManifest, allDatastreams,
	)
	if err != nil {
		return kibana.PackagePolicy{}, err
	}

	pp.Name = fp.Name
	pp.OutputID = outputID
	return pp, nil
}

func (f *FleetAgentPolicy) Update(ctx resource.Context) error {
	if f.Absent {
		provider, err := f.provider(ctx)
		if err != nil {
			return err
		}

		err = provider.Client.DeletePolicy(ctx, f.ID)
		if err != nil {
			return fmt.Errorf("could not delete policy %q: %w", f.Name, err)
		}

		return nil
	}

	return errors.New("update not implemented")
}

type FleetAgentPolicyState struct {
	current  *kibana.Policy
	expected bool
}

func (s *FleetAgentPolicyState) Found() bool {
	return !s.expected || s.current != nil
}

func (s *FleetAgentPolicyState) NeedsUpdate(resource resource.Resource) (bool, error) {
	policy := resource.(*FleetAgentPolicy)
	return policy.Absent == (s.current != nil), nil
}
