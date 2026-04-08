// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"fmt"
	"strconv"

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
	streamIdx := packages.GetDataStreamIndex(inputName, dsManifest)
	stream := dsManifest.Streams[streamIdx]
	streamInput := stream.Input

	// Data streams for the given policy template.
	datastreams := packages.FilterDatastreamsForPolicyTemplate(allDatastreams, policyTemplate)

	// Merge dsVars into inputVars for package/input-level resolution so that
	// variables specified under data_stream.vars in the test config are also
	// applied at the package or input level when they match those definitions.
	// inputVars takes precedence over dsVars.
	allInputVars := make(common.MapStr, len(inputVars)+len(dsVars))
	for k, v := range dsVars {
		allInputVars[k] = v
	}
	for k, v := range inputVars {
		allInputVars[k] = v
	}

	// Build all inputs: the enabled one gets proper streams with user vars; all
	// others get streams with manifest defaults and are disabled.
	inputs := make(map[string]PackagePolicyInput)
	for _, pt := range manifest.PolicyTemplates {
		for _, input := range pt.Inputs {
			inputKey := fmt.Sprintf("%s-%s", pt.Name, input.Type)
			if input.Type == streamInput && pt.Name == policyTemplate.Name {
				// The target input: enabled with user-provided vars.
				streams := buildStreamsForInput(streamInput, manifest, dsManifest, enabled, dsVars, datastreams)
				inputEntry := PackagePolicyInput{
					Enabled:        enabled,
					Streams:        streams,
					inputType:      streamInput,
					policyTemplate: pt.Name,
				}
				if foundInput := policyTemplate.FindInputByType(streamInput); foundInput != nil {
					iv := SetKibanaVariables(foundInput.Vars, allInputVars)
					inputEntry.Vars = iv.ToMapStr()
					inputEntry.legacyVars = iv
				}
				inputs[inputKey] = inputEntry
			} else {
				// A disabled input: use data streams scoped to this policy template
				// so that sibling stream keys are correct even when multiple policy
				// templates declare different data_streams lists.
				ptDatastreams := packages.FilterDatastreamsForPolicyTemplate(allDatastreams, pt)
				streams := buildStreamsForInput(input.Type, manifest, packages.DataStreamManifest{}, false, common.MapStr{}, ptDatastreams)
				entry := PackagePolicyInput{
					Enabled:        false,
					inputType:      input.Type,
					policyTemplate: pt.Name,
				}
				if len(streams) > 0 {
					entry.Streams = streams
				}
				inputs[inputKey] = entry
			}
		}
	}

	pkgVars := SetKibanaVariables(manifest.Vars, allInputVars)
	policy := PackagePolicy{
		Name:               name,
		Namespace:          namespace,
		PolicyID:           policyID,
		Vars:               pkgVars.ToMapStr(),
		Inputs:             inputs,
		legacyPackageTitle: manifest.Title,
		legacyVars:         pkgVars,
	}
	policy.Package.Name = manifest.Name
	policy.Package.Version = manifest.Version

	return policy, nil
}

// buildStreamsForInput builds a streams map for the inputType.
// All siblings that support the input type are
// explicitly disabled so Fleet does not auto-enable them.
func buildStreamsForInput(inputType string, manifest packages.PackageManifest, dsManifest packages.DataStreamManifest, enabled bool, vars common.MapStr, datastreams []packages.DataStreamManifest) map[string]PackagePolicyStream {
	streams := map[string]PackagePolicyStream{}
	for _, ds := range datastreams {
		s, ok := streamForInput(ds, inputType)
		if !ok {
			continue
		}

		streamVars := SetKibanaVariables(s.Vars, common.MapStr{})
		streamEnabled := false
		if ds.Name == dsManifest.Name {
			streamEnabled = enabled
			streamVars = SetKibanaVariables(s.Vars, vars)
		}
		streams[datasetKey(manifest.Name, ds)] = PackagePolicyStream{
			Enabled:           streamEnabled,
			Vars:              streamVars.ToMapStr(),
			legacyVars:        streamVars,
			dataStreamType:    ds.Type,
			dataStreamDataset: datasetKey(manifest.Name, ds),
		}
	}
	return streams
}

// buildStreamsForInput builds a streams map for the inputType.
// All siblings that support the input type are
// explicitly disabled so Fleet does not auto-enable them.
func streamForInput(ds packages.DataStreamManifest, inputType string) (packages.Stream, bool) {
	for _, s := range ds.Streams {
		if s.Input == inputType {
			return s, true
		}
	}
	return packages.Stream{}, false
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

	// Dataset key for the stream: <package name>.<policy template name>.
	streamDataset := fmt.Sprintf("%s.%s", manifest.Name, policyTemplate.Name)

	vars := SetKibanaVariables(policyTemplate.Vars, varValues)
	ensureDatasetVar(vars, policyTemplate, varValues)
	if policyTemplate.Input == "otelcol" {
		ensureUseAPMVar(vars, varValues)
	}
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
	pkgVars := SetKibanaVariables(manifest.Vars, varValues)
	policy.Vars = pkgVars.ToMapStr()
	policy.legacyVars = pkgVars
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
//  1. user-provided value supplied via varValues (e.g. when the manifest does not declare the variable)
//  2. manifest default already parsed into vars — promoted to fromUser=true
//  3. explicit default in the policy-template var definitions
//  4. policy template name as a final fallback
func ensureDatasetVar(vars Vars, policyTemplate packages.PolicyTemplate, varValues common.MapStr) {
	if raw, err := varValues.GetValue("data_stream.dataset"); err == nil {
		var val packages.VarValue
		if err := val.Unpack(raw); err == nil {
			setVarFromUser(vars, "data_stream.dataset", "text", val)
			return
		}
	}
	if v, found := vars["data_stream.dataset"]; found {
		// Exists as a manifest default; promote it so ToMapStr includes it.
		setVarFromUser(vars, "data_stream.dataset", "text", v.Value)
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
	setVarFromUser(vars, "data_stream.dataset", "text", value)
}

// ensureUseAPMVar injects use_apm into vars from varValues if not already present.
// It is only meaningful for otelcol inputs. The value must be a bool or
// "true"/"false" string in varValues; if absent or unparseable, vars is unchanged.
func ensureUseAPMVar(vars Vars, varValues common.MapStr) {
	raw, err := varValues.GetValue("use_apm")
	if err != nil {
		return
	}
	var val packages.VarValue
	switch v := raw.(type) {
	case bool:
		val.Unpack(v)
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return
		}
		val.Unpack(b)
	default:
		return
	}
	if val.Value() != nil {
		setVarFromUser(vars, "use_apm", "boolean", val)
	}
}

// setVarFromUser sets vars[name] with fromUser=true so that the variable is included
// in simplified API requests (ToMapStr). It is a no-op when the var is already
// user-set, preserving any value previously established by SetKibanaVariables.
func setVarFromUser(vars Vars, name, varType string, val packages.VarValue) {
	if v, found := vars[name]; found && v.fromUser {
		return
	}
	vars[name] = Var{Type: varType, Value: val, fromUser: true}
}
