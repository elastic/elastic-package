// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package kibana provides Fleet API client functionality.
// This file contains the legacy (arrays-based) package policy types and
// conversion logic used for Kibana stacks older than 7.16.

package kibana

// legacyDataStream identifies a data stream in the legacy package policy format.
type legacyDataStream struct {
	Type    string `json:"type"`
	Dataset string `json:"dataset"`
}

// legacyStream is one stream entry in the legacy inputs array format.
type legacyStream struct {
	ID         string           `json:"id,omitempty"`
	Enabled    bool             `json:"enabled"`
	DataStream legacyDataStream `json:"data_stream"`
	Vars       map[string]Var   `json:"vars,omitempty"`
}

// legacyInput is one input entry in the legacy inputs array format.
type legacyInput struct {
	PolicyTemplate string         `json:"policy_template,omitempty"`
	Type           string         `json:"type"`
	Enabled        bool           `json:"enabled"`
	Vars           map[string]Var `json:"vars,omitempty"`
	Streams        []legacyStream `json:"streams"`
}

// legacyPackagePolicy is the legacy (arrays-based) Fleet package policy
// request body, accepted by Kibana < 7.16.
type legacyPackagePolicy struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Namespace   string `json:"namespace"`
	PolicyID    string `json:"policy_id"`
	Enabled     bool   `json:"enabled"`
	Package     struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"package"`
	Inputs []legacyInput  `json:"inputs"`
	Vars   map[string]Var `json:"vars,omitempty"`
	Force  bool           `json:"force"`
}

// toLegacyMapStr converts Vars to the {value, type} map format expected by the
// legacy Fleet API.
func (v Vars) toLegacyMapStr() map[string]Var {
	if len(v) == 0 {
		return nil
	}
	m := make(map[string]Var, len(v))
	for k, val := range v {
		m[k] = val
	}
	return m
}

// toLegacyPackagePolicy converts a PackagePolicy (simplified format) to the
// legacy arrays-based format accepted by Kibana < 7.16.
func toLegacyPackagePolicy(pp PackagePolicy) legacyPackagePolicy {
	legacy := legacyPackagePolicy{
		Name:        pp.Name,
		Description: pp.Description,
		Namespace:   pp.Namespace,
		PolicyID:    pp.PolicyID,
		Enabled:     true,
		Force:       pp.Force,
		Vars:        pp.legacyVars.toLegacyMapStr(),
	}
	legacy.Package.Name = pp.Package.Name
	legacy.Package.Version = pp.Package.Version

	// Convert each input from the simplified map to a legacy input entry.
	for _, inp := range pp.Inputs {
		li := legacyInput{
			PolicyTemplate: inp.policyTemplate,
			Type:           inp.inputType,
			Enabled:        inp.Enabled,
			Vars:           inp.legacyVars.toLegacyMapStr(),
		}

		// Convert each stream from the simplified map to a legacy stream entry.
		for _, s := range inp.Streams {
			ls := legacyStream{
				Enabled: s.Enabled,
				DataStream: legacyDataStream{
					Type:    s.dataStreamType,
					Dataset: s.dataStreamDataset,
				},
				Vars: s.legacyVars.toLegacyMapStr(),
			}
			li.Streams = append(li.Streams, ls)
		}

		legacy.Inputs = append(legacy.Inputs, li)
	}

	return legacy
}
