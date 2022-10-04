// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestFindPolicyTemplateForInput(t *testing.T) {
	const policyTemplateName = "my_policy_template"
	const dataStreamName = "my_data_stream"
	const inputName = "logfile"

	var testCases = []struct {
		testName string
		err      string
		pkg      packages.PackageManifest
		input    string
	}{
		{
			testName: "single policy_template",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
		{
			testName: "unspecified input name",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
		},
		{
			testName: "input matching",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
					{
						Name:        policyTemplateName + "1",
						DataStreams: nil,
						Inputs: []packages.Input{
							{
								Type: "not_" + inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
		{
			testName: "data stream not specified",
			err:      "no policy template was found",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: []string{"not_" + dataStreamName},
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
		{
			testName: "multiple matches",
			err:      "ambiguous result",
			pkg: packages.PackageManifest{
				PolicyTemplates: []packages.PolicyTemplate{
					{
						Name:        policyTemplateName,
						DataStreams: []string{dataStreamName},
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
					{
						Name:        policyTemplateName + "1",
						DataStreams: []string{dataStreamName},
						Inputs: []packages.Input{
							{
								Type: inputName,
							},
						},
					},
				},
			},
			input: inputName,
		},
	}

	ds := packages.DataStreamManifest{
		Name: dataStreamName,
		Streams: []struct {
			Input string              `config:"input" json:"input" yaml:"input"`
			Vars  []packages.Variable `config:"vars" json:"vars" yaml:"vars"`
		}{
			{Input: inputName},
		},
	}

	t.Parallel()
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			name, err := findPolicyTemplateForInput(tc.pkg, ds, inputName)

			if tc.err != "" {
				require.Errorf(t, err, "expected err containing %q", tc.err)
				assert.Contains(t, err.Error(), tc.err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, policyTemplateName, name)
		})
	}
}
