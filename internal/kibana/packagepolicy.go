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
	allDatastreams []packages.DataStreamManifest,
) (PackagePolicy, error) {
	streamIdx, err := packages.GetDataStreamIndex(inputName, dsManifest)
	if err != nil {
		return PackagePolicy{}, fmt.Errorf("could not find stream for input %q: %w", inputName, err)
	}
	stream := dsManifest.Streams[streamIdx]
	streamInput := stream.Input

	// Build streams map for the given input type: the primary data stream is
	// enabled/disabled per the `enabled` flag; all other data streams that share
	// the same input type are explicitly disabled so Fleet does not auto-enable them.
	buildStreamsForInput := func(inputType string, primaryDS packages.DataStreamManifest, primaryEnabled bool, primaryVars common.MapStr) map[string]PackagePolicyStream {
		// Determine whether the primary DS actually has a stream for this input type.
		primarySupportsInput := false
		for _, s := range primaryDS.Streams {
			if s.Input == inputType {
				primarySupportsInput = true
				break
			}
		}

		streams := map[string]PackagePolicyStream{}
		for _, ds := range allDatastreams {
			if ds.Name == primaryDS.Name && primarySupportsInput {
				// Primary data stream: use the caller-provided vars.
				var streamVars Vars
				for _, s := range ds.Streams {
					if s.Input == inputType {
						streamVars = SetKibanaVariables(s.Vars, primaryVars)
						break
					}
				}
				streams[datasetKey(manifest.Name, ds)] = PackagePolicyStream{
					Enabled:           primaryEnabled,
					Vars:              streamVars.ToMapStr(),
					legacyVars:        streamVars,
					dataStreamType:    ds.Type,
					dataStreamDataset: datasetKey(manifest.Name, ds),
				}
				continue
			}
			// Sibling data stream: always disabled; include default vars so Fleet
			// does not fall back to unexpected defaults when compiling templates.
			siblingEntry := PackagePolicyStream{
				Enabled:           false,
				dataStreamType:    ds.Type,
				dataStreamDataset: datasetKey(manifest.Name, ds),
			}
			supportsInput := false
			for _, s := range ds.Streams {
				if s.Input == inputType {
					siblingVars := SetKibanaVariables(s.Vars, common.MapStr{})
					siblingEntry.Vars = siblingVars.ToMapStr()
					siblingEntry.legacyVars = siblingVars
					supportsInput = true
					break
				}
			}
			if supportsInput {
				streams[datasetKey(manifest.Name, ds)] = siblingEntry
			}
		}
		return streams
	}

	// Build all inputs: the enabled one gets proper streams with user vars; all
	// others get streams with manifest defaults and are disabled.
	inputs := make(map[string]PackagePolicyInput)
	for _, pt := range manifest.PolicyTemplates {
		for _, inp := range pt.Inputs {
			inputKey := fmt.Sprintf("%s-%s", pt.Name, inp.Type)
			if inp.Type == streamInput && pt.Name == policyTemplate.Name {
				// The target input: enabled with user-provided vars.
				streams := buildStreamsForInput(streamInput, dsManifest, enabled, dsVars)
				inputEntry := PackagePolicyInput{
					Enabled:        enabled,
					Streams:        streams,
					inputType:      streamInput,
					policyTemplate: pt.Name,
				}
				if input := policyTemplate.FindInputByType(streamInput); input != nil {
					iv := SetKibanaVariables(input.Vars, inputVars)
					inputEntry.Vars = iv.ToMapStr()
					inputEntry.legacyVars = iv
				}
				inputs[inputKey] = inputEntry
			} else {
				// A disabled input: include streams with manifest defaults so Fleet
				// does not need to auto-fill from the package manifest (which can
				// trigger template compilation errors for comment-only yaml defaults).
				streams := buildStreamsForInput(inp.Type, packages.DataStreamManifest{}, false, common.MapStr{})
				entry := PackagePolicyInput{
					Enabled:        false,
					inputType:      inp.Type,
					policyTemplate: pt.Name,
				}
				if len(streams) > 0 {
					entry.Streams = streams
				}
				inputs[inputKey] = entry
			}
		}
	}

	pkgVars := SetKibanaVariables(manifest.Vars, inputVars)
	pp := PackagePolicy{
		Name:               name,
		Namespace:          namespace,
		PolicyID:           policyID,
		Vars:               pkgVars.ToMapStr(),
		Inputs:             inputs,
		legacyPackageTitle: manifest.Title,
		legacyVars:         pkgVars,
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
		inputs[fmt.Sprintf("%s-%s", pt.Name, pt.Input)] = PackagePolicyInput{
			Enabled:        false,
			inputType:      pt.Input,
			policyTemplate: pt.Name,
		}
	}

	vars := SetKibanaVariables(policyTemplate.Vars, varValues)
	ensureDatasetVar(vars, policyTemplate)

	// Dataset key for the stream: <package name>.<policy template name>.
	streamDataset := fmt.Sprintf("%s.%s", manifest.Name, policyTemplate.Name)
	inputEntry := PackagePolicyInput{
		Enabled: enabled,
		Streams: map[string]PackagePolicyStream{
			streamDataset: {
				Enabled:           enabled,
				Vars:              vars.ToMapStr(),
				legacyVars:        vars,
				dataStreamDataset: streamDataset,
				// dataStreamType is intentionally empty: input packages
				// require Kibana >= 7.16 (simplified API), so legacy
				// conversion is not needed.
			},
		},
		inputType:      streamInput,
		policyTemplate: policyTemplate.Name,
		legacyVars:     vars,
	}
	inputs[fmt.Sprintf("%s-%s", policyTemplate.Name, streamInput)] = inputEntry

	policy := PackagePolicy{
		Name:      name,
		Namespace: namespace,
		PolicyID:  policyID,
		Inputs:    inputs,

		legacyPackageTitle: manifest.Title,
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

// ensureDatasetVar guarantees that vars contains a data_stream.dataset entry
// marked fromUser=true, so that it is included in simplified API requests.
// Fleet requires this field for input packages. The value resolution order is:
//  1. user-provided value (already in vars with fromUser=true) — no-op
//  2. manifest default from the policy template variable definitions
//  3. policy template name as a final fallback
func ensureDatasetVar(vars Vars, policyTemplate packages.PolicyTemplate) {
	if v, found := vars["data_stream.dataset"]; found && v.fromUser {
		return
	}
	if v, found := vars["data_stream.dataset"]; found {
		// Exists as a manifest default; promote it so ToMapStr includes it.
		v.fromUser = true
		vars["data_stream.dataset"] = v
		return
	}
	dataset := policyTemplate.Name
	for _, def := range policyTemplate.Vars {
		if def.Name == "data_stream.dataset" && def.Default != nil {
			if s, ok := def.Default.Value().(string); ok && s != "" {
				dataset = s
			}
			break
		}
	}
	var value packages.VarValue
	value.Unpack(dataset)
	vars["data_stream.dataset"] = Var{Value: value, Type: "text", fromUser: true}
}
