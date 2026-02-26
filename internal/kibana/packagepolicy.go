// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages"
)

// BuildIntegrationPackagePolicy builds a PackagePolicy for an integration package
// given pre-loaded manifests.
func BuildIntegrationPackagePolicy(
	policyID, namespace, name string,
	manifest packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	dsManifest packages.DataStreamManifest,
	inputName string,
	inputVars, dsVars common.MapStr,
	enabled bool,
	datastreams []packages.DataStreamManifest,
) (PackagePolicy, error) {
	streamIdx, err := packages.GetDataStreamIndex(inputName, dsManifest)
	if err != nil {
		return PackagePolicy{}, fmt.Errorf("could not find stream for input %q: %w", inputName, err)
	}
	stream := dsManifest.Streams[streamIdx]
	streamInput := stream.Input

	// Disable all other inputs; only enable the target one.
	inputs := make(map[string]PackagePolicyInput)
	for _, pt := range manifest.PolicyTemplates {
		for _, inp := range pt.Inputs {
			inputs[fmt.Sprintf("%s-%s", pt.Name, inp.Type)] = PackagePolicyInput{Enabled: false}
		}
	}

	// Build streams map for the enabled input. Explicitly disable all other
	// data streams that share the same input type so Fleet does not auto-enable them.
	streams := map[string]PackagePolicyStream{
		datasetKey(manifest.Name, dsManifest): {
			Enabled: enabled,
			Vars:    SetKibanaVariables(stream.Vars, dsVars).ToMap(),
		},
	}
	for _, ds := range datastreams {
		if ds.Name == dsManifest.Name {
			continue
		}
		streams[datasetKey(manifest.Name, ds)] = PackagePolicyStream{Enabled: false}
	}

	inputEntry := PackagePolicyInput{
		Enabled: enabled,
		Streams: streams,
	}
	if input := policyTemplate.FindInputByType(streamInput); input != nil {
		inputEntry.Vars = SetKibanaVariables(input.Vars, inputVars).ToMap()
	}
	inputs[fmt.Sprintf("%s-%s", policyTemplate.Name, streamInput)] = inputEntry

	pp := PackagePolicy{
		Name:      name,
		Namespace: namespace,
		PolicyID:  policyID,
		Vars:      SetKibanaVariables(manifest.Vars, inputVars).ToMap(),
		Inputs:    inputs,
	}
	pp.Package.Name = manifest.Name
	pp.Package.Version = manifest.Version

	return pp, nil
}

// BuildInputPackagePolicy builds a PackagePolicy for an input package
// given pre-loaded manifests.
func BuildInputPackagePolicy(
	policyID, namespace, name string,
	manifest packages.PackageManifest,
	policyTemplate packages.PolicyTemplate,
	varValues common.MapStr,
	enabled bool,
) PackagePolicy {
	streamInput := policyTemplate.Input

	// Disable all other inputs; only enable the target one.
	inputs := make(map[string]PackagePolicyInput)
	for _, pt := range manifest.PolicyTemplates {
		inputs[fmt.Sprintf("%s-%s", pt.Name, pt.Input)] = PackagePolicyInput{Enabled: false}
	}

	vars := SetKibanaVariables(policyTemplate.Vars, varValues)
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
		vars["data_stream.dataset"] = Var{
			Value: value,
			Type:  "text",
		}
	}

	inputEntry := PackagePolicyInput{
		Enabled: enabled,
		Streams: map[string]PackagePolicyStream{
			// This dataset is the one Fleet uses to identify the stream,
			// it must be <package name>.<policy template name>.
			fmt.Sprintf("%s.%s", manifest.Name, policyTemplate.Name): {
				Enabled: enabled,
				Vars:    vars.ToMap(),
			},
		},
	}
	inputs[fmt.Sprintf("%s-%s", policyTemplate.Name, streamInput)] = inputEntry

	policy := PackagePolicy{
		Name:      name,
		Namespace: namespace,
		PolicyID:  policyID,
		Inputs:    inputs,
	}
	policy.Package.Name = manifest.Name
	policy.Package.Version = manifest.Version

	return policy
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
