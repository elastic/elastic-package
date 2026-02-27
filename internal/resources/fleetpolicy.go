// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"errors"
	"fmt"

	"github.com/elastic/go-resource"

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

	for _, packagePolicy := range f.PackagePolicies {
		pp, err := createPackagePolicy(*f, packagePolicy)
		if err != nil {
			return fmt.Errorf("could not prepare package policy: %w", err)
		}
		_, err = provider.Client.CreatePackagePolicy(ctx, *pp)
		if err != nil {
			return fmt.Errorf("could not add package policy %q to agent policy %q: %w", packagePolicy.Name, f.Name, err)
		}
	}

	return nil
}

func createPackagePolicy(policy FleetAgentPolicy, packagePolicy FleetPackagePolicy) (*kibana.PackagePolicy, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packagePolicy.PackageRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read package manifest at %s: %w", packagePolicy.PackageRoot, err)
	}

	switch manifest.Type {
	case "integration":
		return createIntegrationPackagePolicy(policy, *manifest, packagePolicy)
	case "input":
		return createInputPackagePolicy(policy, *manifest, packagePolicy)
	default:
		return nil, fmt.Errorf("package type %q is not supported", manifest.Type)
	}
}

func createIntegrationPackagePolicy(policy FleetAgentPolicy, manifest packages.PackageManifest, packagePolicy FleetPackagePolicy) (*kibana.PackagePolicy, error) {
	if packagePolicy.DataStreamName == "" {
		return nil, fmt.Errorf("expected data stream for integration package policy %q", packagePolicy.Name)
	}
	dsManifest, err := packages.ReadDataStreamManifestFromPackageRoot(packagePolicy.PackageRoot, packagePolicy.DataStreamName)
	if err != nil {
		return nil, fmt.Errorf("could not read %q data stream manifest for package at %s: %w", packagePolicy.DataStreamName, packagePolicy.PackageRoot, err)
	}
	streamIdx, err := packages.GetDataStreamIndex(packagePolicy.InputName, *dsManifest)
	if err != nil {
		return nil, fmt.Errorf("could not find stream for input %q: %w", packagePolicy.InputName, err)
	}
	policyTemplateName := packagePolicy.TemplateName
	if policyTemplateName == "" {
		name, err := packages.FindPolicyTemplateForInput(&manifest, dsManifest, packagePolicy.InputName)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
		policyTemplateName = name
	}
	policyTemplate, err := packages.SelectPolicyTemplateByName(manifest.PolicyTemplates, policyTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to find the selected policy_template: %w", err)
	}
	datastreams, err := packages.DataStreamsForInput(packagePolicy.PackageRoot, policyTemplate, dsManifest.Streams[streamIdx].Input)
	if err != nil {
		return nil, fmt.Errorf("could not determine data streams for input: %w", err)
	}
	pp, err := kibana.BuildIntegrationPackagePolicy(
		policy.ID, policy.Namespace, packagePolicy.Name,
		manifest, policyTemplate, *dsManifest,
		packagePolicy.InputName,
		common.MapStr(packagePolicy.Vars), common.MapStr(packagePolicy.DataStreamVars),
		!packagePolicy.Disabled, datastreams,
	)
	if err != nil {
		return nil, err
	}
	pp.OutputID = policy.DataOutputID
	return &pp, nil
}

func createInputPackagePolicy(policy FleetAgentPolicy, manifest packages.PackageManifest, packagePolicy FleetPackagePolicy) (*kibana.PackagePolicy, error) {
	if dsName := packagePolicy.DataStreamName; dsName != "" {
		return nil, fmt.Errorf("no data stream expected for input package policy %q, found %q", packagePolicy.Name, dsName)
	}

	policyTemplateName := packagePolicy.TemplateName
	if policyTemplateName == "" {
		name, err := packages.FindPolicyTemplateForInput(&manifest, nil, packagePolicy.InputName)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
		policyTemplateName = name
	}

	policyTemplate, err := packages.SelectPolicyTemplateByName(manifest.PolicyTemplates, policyTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to find the selected policy_template: %w", err)
	}

	pp := kibana.BuildInputPackagePolicy(
		policy.ID, policy.Namespace, packagePolicy.Name,
		manifest, policyTemplate,
		common.MapStr(packagePolicy.Vars),
		!packagePolicy.Disabled,
	)
	pp.OutputID = policy.DataOutputID
	return &pp, nil
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
