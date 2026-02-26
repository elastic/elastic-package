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
			name, err := packages.FindPolicyTemplateForInput(manifest, dsManifest, packagePolicy.InputName)
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
		return createIntegrationPackagePolicy(policy, *manifest, *dsManifest, policyTemplate, datastreams, packagePolicy)
	case "input":
		return createInputPackagePolicy(policy, *manifest, packagePolicy)
	default:
		return nil, fmt.Errorf("package type %q is not supported", manifest.Type)
	}
}

func createIntegrationPackagePolicy(policy FleetAgentPolicy, manifest packages.PackageManifest, dsManifest packages.DataStreamManifest, policyTemplate packages.PolicyTemplate, datastreams []packages.DataStreamManifest, packagePolicy FleetPackagePolicy) (*kibana.PackagePolicy, error) {
	pp, err := BuildIntegrationPackagePolicy(
		policy.ID, policy.Namespace, packagePolicy.Name,
		manifest, policyTemplate, dsManifest,
		packagePolicy.InputName,
		common.MapStr(packagePolicy.Vars), common.MapStr(packagePolicy.DataStreamVars),
		!packagePolicy.Disabled, datastreams,
	)
	if err != nil {
		return nil, err
	}
	return &pp, nil
}

// BuildIntegrationPackagePolicy builds a PackagePolicy for an integration package
// given pre-loaded manifests. It does not perform any disk I/O.
func BuildIntegrationPackagePolicy(
	policyID, namespace, name string,
	manifest packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	ds packages.DataStreamManifest,
	inputName string,
	inputVars, dsVars common.MapStr,
	enabled bool,
	datasetsForInput []packages.DataStreamManifest,
) (kibana.PackagePolicy, error) {
	streamIdx, err := packages.GetDataStreamIndex(inputName, ds)
	if err != nil {
		return kibana.PackagePolicy{}, fmt.Errorf("could not find stream for input %q: %w", inputName, err)
	}
	stream := ds.Streams[streamIdx]
	streamInput := stream.Input

	// Disable all other inputs; only enable the target one.
	inputs := make(map[string]kibana.PackagePolicyInput)
	for _, pt := range manifest.PolicyTemplates {
		for _, inp := range pt.Inputs {
			inputs[fmt.Sprintf("%s-%s", pt.Name, inp.Type)] = kibana.PackagePolicyInput{Enabled: false}
		}
	}

	// Build streams map for the enabled input. Explicitly disable all other
	// data streams that share the same input type so Fleet does not auto-enable them.
	streams := map[string]kibana.PackagePolicyStream{
		datasetKey(manifest.Name, ds): {
			Enabled: enabled,
			Vars:    kibana.SetKibanaVariables(stream.Vars, dsVars).ToMap(),
		},
	}
	for _, sibling := range datasetsForInput {
		if sibling.Name == ds.Name {
			continue
		}
		streams[datasetKey(manifest.Name, sibling)] = kibana.PackagePolicyStream{Enabled: false}
	}

	inputEntry := kibana.PackagePolicyInput{
		Enabled: enabled,
		Streams: streams,
	}
	if input := policyTemplate.FindInputByType(streamInput); input != nil {
		inputEntry.Vars = kibana.SetKibanaVariables(input.Vars, inputVars).ToMap()
	}
	inputs[fmt.Sprintf("%s-%s", policyTemplate.Name, streamInput)] = inputEntry

	pp := kibana.PackagePolicy{
		Name:      name,
		Namespace: namespace,
		PolicyID:  policyID,
		Vars:      kibana.SetKibanaVariables(manifest.Vars, inputVars).ToMap(),
		Inputs:    inputs,
	}
	pp.Package.Name = manifest.Name
	pp.Package.Version = manifest.Version

	return pp, nil
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

	pp := BuildInputPackagePolicy(
		policy.ID, policy.Namespace, packagePolicy.Name,
		manifest, policyTemplate,
		common.MapStr(packagePolicy.Vars),
		!packagePolicy.Disabled,
	)
	return &pp, nil
}

// BuildInputPackagePolicy builds a PackagePolicy for an input package
// given pre-loaded manifests. It does not perform any disk I/O.
func BuildInputPackagePolicy(
	policyID, namespace, name string,
	manifest packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	varValues common.MapStr,
	enabled bool,
) kibana.PackagePolicy {
	streamInput := policyTemplate.Input

	// Disable all other inputs; only enable the target one.
	inputs := make(map[string]kibana.PackagePolicyInput)
	for _, pt := range manifest.PolicyTemplates {
		inputs[fmt.Sprintf("%s-%s", pt.Name, pt.Input)] = kibana.PackagePolicyInput{Enabled: false}
	}

	vars := kibana.SetKibanaVariables(policyTemplate.Vars, varValues)
	if _, found := vars["data_stream.dataset"]; !found {
		// Use overriding value from varValues if provided (when not declared in policyTemplate.Vars).
		dataset := policyTemplate.Name
		if v, err := varValues.GetValue("data_stream.dataset"); err == nil {
			if dsVal, ok := v.(string); ok && dsVal != "" {
				dataset = dsVal
			}
		}
		var value packages.VarValue
		value.Unpack(dataset)
		vars["data_stream.dataset"] = kibana.Var{
			Value: value,
			Type:  "text",
		}
	}

	inputEntry := kibana.PackagePolicyInput{
		Enabled: enabled,
		Streams: map[string]kibana.PackagePolicyStream{
			// This dataset is the one Fleet uses to identify the stream,
			// it must be <package name>.<policy template name>.
			fmt.Sprintf("%s.%s", manifest.Name, policyTemplate.Name): {
				Enabled: enabled,
				Vars:    vars.ToMap(),
			},
		},
	}
	inputs[fmt.Sprintf("%s-%s", policyTemplate.Name, streamInput)] = inputEntry

	pp := kibana.PackagePolicy{
		Name:      name,
		Namespace: namespace,
		PolicyID:  policyID,
		Inputs:    inputs,
	}
	pp.Package.Name = manifest.Name
	pp.Package.Version = manifest.Version

	return pp
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

// datasetKey returns the Fleet stream key for a data stream. When the data
// stream manifest declares an explicit dataset, that value is used directly;
// otherwise the key is "<pkgName>.<dsName>".
func datasetKey(pkgName string, ds packages.DataStreamManifest) string {
	if ds.Dataset != "" {
		return ds.Dataset
	}
	return fmt.Sprintf("%s.%s", pkgName, ds.Name)
}
