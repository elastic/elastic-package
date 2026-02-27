// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package kibana provides Fleet API client functionality.
// This file contains the legacy (arrays-based) package policy types and
// conversion logic used for old Kibana versions.

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

// legacyPackagePolicy is the legacy (arrays-based) Fleet package policy.
type legacyPackagePolicy struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Namespace   string `json:"namespace"`
	PolicyID    string `json:"policy_id"`
	Enabled     bool   `json:"enabled"`
	Package     struct {
		Name    string `json:"name"`
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"package"`
	Inputs   []legacyInput  `json:"inputs"`
	OutputID string         `json:"output_id"`
	Vars     map[string]Var `json:"vars,omitempty"`
	Force    bool           `json:"force"`
}

// toLegacyMapVar converts Vars to the {value, type} map format expected by the
// legacy Fleet API.
func (v Vars) toLegacyMapVar() map[string]Var {
	if len(v) == 0 {
		return nil
	}
	m := make(map[string]Var, len(v))
	for k, val := range v {
		m[k] = val
	}
	return m
}

// toLegacy converts a PackagePolicy (simplified format) to the
// legacy arrays-based format.
func (p PackagePolicy) toLegacy() legacyPackagePolicy {
	legacy := legacyPackagePolicy{
		Name:        p.Name,
		Description: p.Description,
		Namespace:   p.Namespace,
		PolicyID:    p.PolicyID,
		Enabled:     true,
		Force:       p.Force,
		Vars:        p.legacyVars.toLegacyMapVar(),
	}
	legacy.Package.Name = p.Package.Name
	legacy.Package.Title = p.Package.Title
	legacy.Package.Version = p.Package.Version
	legacy.OutputID = p.OutputID

	// Convert each input from the simplified map to a legacy input entry.
	for _, i := range p.Inputs {
		input := legacyInput{
			PolicyTemplate: i.policyTemplate,
			Type:           i.inputType,
			Enabled:        i.Enabled,
			Vars:           i.legacyVars.toLegacyMapVar(),
			Streams:        []legacyStream{},
		}

		// Convert each stream from the simplified map to a legacy stream entry.
		for _, s := range i.Streams {
			stream := legacyStream{
				Enabled: s.Enabled,
				DataStream: legacyDataStream{
					Type:    s.dataStreamType,
					Dataset: s.dataStreamDataset,
				},
				Vars: s.legacyVars.toLegacyMapVar(),
			}
			input.Streams = append(input.Streams, stream)
		}

		legacy.Inputs = append(legacy.Inputs, input)
	}

	return legacy
}
