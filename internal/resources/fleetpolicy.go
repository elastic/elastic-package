// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package resources

import (
	"errors"
	"fmt"
	"slices"
	"strings"

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

	policyTemplateName := packagePolicy.TemplateName
	if policyTemplateName == "" {
		name, err := findPolicyTemplateForDataStream(manifest, *dsManifest, packagePolicy.InputName)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
		policyTemplateName = name
	}

	policyTemplate, err := selectPolicyTemplateByName(manifest.PolicyTemplates, policyTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to find the selected policy_template: %w", err)
	}

	stream := dsManifest.Streams[getDataStreamIndex(packagePolicy.InputName, *dsManifest)]
	streamInput := stream.Input
	enabled := !packagePolicy.Disabled

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
		fmt.Sprintf("%s.%s", manifest.Name, dsManifest.Name): {
			Enabled: enabled,
			Vars:    setKibanaVariables(stream.Vars, common.MapStr(packagePolicy.DataStreamVars)).ToMap(),
		},
	}
	allDS, err := packages.ReadAllDataStreamManifests(packagePolicy.PackageRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read data stream manifests: %w", err)
	}
	for _, other := range allDS {
		if other.Name == dsManifest.Name {
			continue
		}
		for _, s := range other.Streams {
			if s.Input == streamInput {
				otherDataset := fmt.Sprintf("%s.%s", manifest.Name, other.Name)
				if len(other.Dataset) > 0 {
					otherDataset = other.Dataset
				}
				streams[otherDataset] = kibana.PackagePolicyStream{Enabled: false}
				break
			}
		}
	}

	inputEntry := kibana.PackagePolicyInput{
		Enabled: enabled,
		Streams: streams,
	}
	if input := policyTemplate.FindInputByType(streamInput); input != nil {
		inputEntry.Vars = setKibanaVariables(input.Vars, common.MapStr(packagePolicy.Vars)).ToMap()
	}
	inputs[fmt.Sprintf("%s-%s", policyTemplate.Name, streamInput)] = inputEntry

	pp := kibana.PackagePolicy{
		Name:      packagePolicy.Name,
		Namespace: policy.Namespace,
		PolicyID:  policy.ID,
		Vars:      setKibanaVariables(manifest.Vars, common.MapStr(packagePolicy.Vars)).ToMap(),
		Inputs:    inputs,
	}
	pp.Package.Name = manifest.Name
	pp.Package.Version = manifest.Version

	return &pp, nil
}

func createInputPackagePolicy(policy FleetAgentPolicy, manifest packages.PackageManifest, packagePolicy FleetPackagePolicy) (*kibana.PackagePolicy, error) {
	if dsName := packagePolicy.DataStreamName; dsName != "" {
		return nil, fmt.Errorf("no data stream expected for input package policy %q, found %q", packagePolicy.Name, dsName)
	}

	policyTemplateName := packagePolicy.TemplateName
	if policyTemplateName == "" {
		name, err := findPolicyTemplateForInputPackage(manifest, packagePolicy.InputName)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the associated policy_template: %w", err)
		}
		policyTemplateName = name
	}

	policyTemplate, err := selectPolicyTemplateByName(manifest.PolicyTemplates, policyTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to find the selected policy_template: %w", err)
	}

	streamInput := policyTemplate.Input
	enabled := !packagePolicy.Disabled

	// Disable all other inputs; only enable the target one.
	inputs := make(map[string]kibana.PackagePolicyInput)
	for _, pt := range manifest.PolicyTemplates {
		inputs[fmt.Sprintf("%s-%s", pt.Name, pt.Input)] = kibana.PackagePolicyInput{Enabled: false}
	}

	vars := setKibanaVariables(policyTemplate.Vars, common.MapStr(packagePolicy.Vars))
	if _, found := vars["data_stream.dataset"]; !found {
		var value packages.VarValue
		value.Unpack(policyTemplate.Name)
		vars["data_stream.dataset"] = kibana.Var{
			Value: value,
			Type:  "text",
		}
	}

	inputEntry := kibana.PackagePolicyInput{
		Enabled: enabled,
		Streams: map[string]kibana.PackagePolicyStream{
			fmt.Sprintf("%s.%s", manifest.Name, policyTemplate.Name): {
				Enabled: enabled,
				Vars:    vars.ToMap(),
			},
		},
	}
	inputs[fmt.Sprintf("%s-%s", policyTemplate.Name, streamInput)] = inputEntry

	pp := kibana.PackagePolicy{
		Name:      packagePolicy.Name,
		Namespace: policy.Namespace,
		PolicyID:  policy.ID,
		Inputs:    inputs,
	}
	pp.Package.Name = manifest.Name
	pp.Package.Version = manifest.Version

	return &pp, nil
}

func setKibanaVariables(definitions []packages.Variable, values common.MapStr) kibana.Vars {
	vars := kibana.Vars{}
	for _, definition := range definitions {
		val := definition.Default

		value, err := values.GetValue(definition.Name)
		if err == nil {
			val = &packages.VarValue{}
			val.Unpack(value)
		} else if errors.Is(err, common.ErrKeyNotFound) && definition.Default == nil {
			// Do not include nulls for unset variables.
			continue
		}

		vars[definition.Name] = kibana.Var{
			Type:  definition.Type,
			Value: *val,
		}
	}
	return vars
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

// getDataStreamIndex returns the index of the data stream whose input name
// matches. Otherwise it returns the 0.
func getDataStreamIndex(inputName string, ds packages.DataStreamManifest) int {
	for i, s := range ds.Streams {
		if s.Input == inputName {
			return i
		}
	}
	return 0
}

func findPolicyTemplateForDataStream(pkg packages.PackageManifest, ds packages.DataStreamManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(ds.Streams) == 0 {
			return "", errors.New("no streams declared in data stream manifest")
		}
		inputName = ds.Streams[getDataStreamIndex(inputName, ds)].Input
	}

	var matchedPolicyTemplates []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		// Does this policy_template include this input type?
		if policyTemplate.FindInputByType(inputName) == nil {
			continue
		}

		// Does the policy_template apply to this data stream (when data streams are specified)?
		if len(policyTemplate.DataStreams) > 0 && !slices.Contains(policyTemplate.DataStreams, ds.Name) {
			continue
		}

		matchedPolicyTemplates = append(matchedPolicyTemplates, policyTemplate.Name)
	}

	switch len(matchedPolicyTemplates) {
	case 1:
		return matchedPolicyTemplates[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found for data stream %q "+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", ds.Name, inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"were found that apply to data stream %q with input type %q: please "+
			"specify the 'policy_template' in the system test config",
			strings.Join(matchedPolicyTemplates, ", "), ds.Name, inputName)
	}
}

func findPolicyTemplateForInputPackage(pkg packages.PackageManifest, inputName string) (string, error) {
	if inputName == "" {
		if len(pkg.PolicyTemplates) == 0 {
			return "", errors.New("no policy templates specified for input package")
		}
		inputName = pkg.PolicyTemplates[0].Input
	}

	var matched []string
	for _, policyTemplate := range pkg.PolicyTemplates {
		if policyTemplate.Input != inputName {
			continue
		}

		matched = append(matched, policyTemplate.Name)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", fmt.Errorf("no policy template was found"+
			"with input type %q: verify that you have included the data stream "+
			"and input in the package's policy_template list", inputName)
	default:
		return "", fmt.Errorf("ambiguous result: multiple policy templates ([%s]) "+
			"with input type %q: please "+
			"specify the 'policy_template' in the system test config",
			strings.Join(matched, ", "), inputName)
	}
}

func selectPolicyTemplateByName(policies []packages.PolicyTemplate, name string) (packages.PolicyTemplate, error) {
	for _, policy := range policies {
		if policy.Name == name {
			return policy, nil
		}
	}
	return packages.PolicyTemplate{}, fmt.Errorf("policy template %q not found", name)
}
