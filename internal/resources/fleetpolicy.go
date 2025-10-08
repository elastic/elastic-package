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

	// RootPath is the root of the source of the package to configure, from it we should
	// be able to read the manifest, the data stream manifests and the policy template to use.
	RootPath string

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
		policy, err := createPackagePolicy(*f, packagePolicy)
		if err != nil {
			return fmt.Errorf("could not prepare package policy: %w", err)
		}
		err = provider.Client.AddPackageDataStreamToPolicy(ctx, *policy)
		if err != nil {
			return fmt.Errorf("could not add package policy %q to agent policy %q: %w", packagePolicy.Name, f.Name, err)
		}
	}

	return nil
}

func createPackagePolicy(policy FleetAgentPolicy, packagePolicy FleetPackagePolicy) (*kibana.PackageDataStream, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packagePolicy.RootPath)
	if err != nil {
		return nil, fmt.Errorf("could not read package manifest at %s: %w", packagePolicy.RootPath, err)
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

func createIntegrationPackagePolicy(policy FleetAgentPolicy, manifest packages.PackageManifest, packagePolicy FleetPackagePolicy) (*kibana.PackageDataStream, error) {
	if packagePolicy.DataStreamName == "" {
		return nil, fmt.Errorf("expected data stream for integration package policy %q", packagePolicy.Name)
	}

	dsManifest, err := packages.ReadDataStreamManifestFromPackageRoot(packagePolicy.RootPath, packagePolicy.DataStreamName)
	if err != nil {
		return nil, fmt.Errorf("could not read %q data stream manifest for package at %s: %w", packagePolicy.DataStreamName, packagePolicy.RootPath, err)
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

	ds := kibana.PackageDataStream{
		Name:      packagePolicy.Name,
		Namespace: policy.Namespace,
		PolicyID:  policy.ID,
		Enabled:   !packagePolicy.Disabled,
		Inputs: []kibana.Input{
			{
				PolicyTemplate: policyTemplate.Name,
				Enabled:        !packagePolicy.Disabled,
			},
		},
	}
	ds.Package.Name = manifest.Name
	ds.Package.Title = manifest.Title
	ds.Package.Version = manifest.Version

	stream := dsManifest.Streams[getDataStreamIndex(packagePolicy.InputName, *dsManifest)]
	streamInput := stream.Input
	ds.Inputs[0].Type = streamInput

	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, manifest.Name, dsManifest.Name),
			Enabled: !packagePolicy.Disabled,
			DataStream: kibana.DataStream{
				Type:    dsManifest.Type,
				Dataset: getDataStreamDataset(manifest, *dsManifest),
			},
		},
	}

	// Add dataStream-level vars
	streams[0].Vars = setKibanaVariables(stream.Vars, common.MapStr(packagePolicy.DataStreamVars))
	ds.Inputs[0].Streams = streams

	// Add input-level vars
	input := policyTemplate.FindInputByType(streamInput)
	if input != nil {
		ds.Inputs[0].Vars = setKibanaVariables(input.Vars, common.MapStr(packagePolicy.Vars))
	}

	// Add package-level vars
	ds.Vars = setKibanaVariables(manifest.Vars, common.MapStr(packagePolicy.Vars))

	return &ds, nil
}

func createInputPackagePolicy(policy FleetAgentPolicy, manifest packages.PackageManifest, packagePolicy FleetPackagePolicy) (*kibana.PackageDataStream, error) {
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

	ds := kibana.PackageDataStream{
		Name:      packagePolicy.Name,
		Namespace: policy.Namespace,
		PolicyID:  policy.ID,
		Enabled:   !packagePolicy.Disabled,
		Inputs: []kibana.Input{
			{
				PolicyTemplate: policyTemplate.Name,
				Enabled:        !packagePolicy.Disabled,
			},
		},
	}
	ds.Package.Name = manifest.Name
	ds.Package.Title = manifest.Title
	ds.Package.Version = manifest.Version

	streamInput := policyTemplate.Input
	ds.Inputs[0].Type = streamInput

	dataset := fmt.Sprintf("%s.%s", manifest.Name, policyTemplate.Name)
	streams := []kibana.Stream{
		{
			ID:      fmt.Sprintf("%s-%s.%s", streamInput, manifest.Name, policyTemplate.Name),
			Enabled: !packagePolicy.Disabled,
			DataStream: kibana.DataStream{
				Type:    policyTemplate.Type,
				Dataset: dataset,
			},
		},
	}

	// Add policyTemplate-level vars.
	vars := setKibanaVariables(policyTemplate.Vars, common.MapStr(packagePolicy.Vars))
	if _, found := vars["data_stream.dataset"]; !found {
		var value packages.VarValue
		value.Unpack(dataset)
		vars["data_stream.dataset"] = kibana.Var{
			Value: value,
			Type:  "text",
		}
	}

	streams[0].Vars = vars
	ds.Inputs[0].Streams = streams

	return &ds, nil
}

func setKibanaVariables(definitions []packages.Variable, values common.MapStr) kibana.Vars {
	vars := kibana.Vars{}
	for _, definition := range definitions {
		// Elastic Package uses the deprecated 'inputs' array in its /api/fleet/package_policies request.
		// When using this API parameter, default values are not automatically incorporated into
		// the policy, whereas with the 'inputs' object, defaults are incorporated by the API service.
		// This means that our client must include the default values in its request to ensure correct behavior.
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

func getDataStreamDataset(pkg packages.PackageManifest, ds packages.DataStreamManifest) string {
	if len(ds.Dataset) > 0 {
		return ds.Dataset
	}
	return fmt.Sprintf("%s.%s", pkg.Name, ds.Name)
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
