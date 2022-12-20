// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAddMappingsListMeta(t *testing.T) {
	cases := []struct {
		title        string
		baseDocPath  string
		ecsToAddPath string
		expectedPath string
	}{
		{
			title:        "Add new mappings",
			baseDocPath:  "testdata/empty.yml",
			ecsToAddPath: "testdata/ecs.template.yml",
			expectedPath: "testdata/expected.empty.meta.yml",
		},
		{
			title:        "Append mappings",
			baseDocPath:  "testdata/existing.yml",
			ecsToAddPath: "testdata/ecs.template.yml",
			expectedPath: "testdata/expected.existing.meta.yml",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			contentsBase, _ := os.ReadFile(c.baseDocPath)
			contentsToAdd, _ := os.ReadFile(c.ecsToAddPath)
			contentsExpected, _ := os.ReadFile(c.expectedPath)

			var base yaml.Node
			err := yaml.Unmarshal(contentsBase, &base)
			require.NoError(t, err)

			var template ecsTemplates
			err = yaml.Unmarshal(contentsToAdd, &template)
			require.NoError(t, err)

			err = addEcsMappingsListMeta(&base, template)
			require.NoError(t, err)

			newYaml, _ := formatResult(&base)

			assert.Equal(t, string(contentsExpected), string(newYaml))
		})
	}
}

func TestAddEcsMappings(t *testing.T) {
	cases := []struct {
		title        string
		baseDocPath  string
		ecsToAddPath string
		expectedPath string
	}{
		{
			title:        "Add new mappings from scratch",
			baseDocPath:  "testdata/empty.yml",
			ecsToAddPath: "testdata/ecs.template.yml",
			expectedPath: "testdata/expected.empty.mappings.yml",
		},
		{
			title:        "Append mappings",
			baseDocPath:  "testdata/existing.yml",
			ecsToAddPath: "testdata/ecs.template.yml",
			expectedPath: "testdata/expected.existing.mappings.yml",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			contentsBase, _ := os.ReadFile(c.baseDocPath)
			contentsToAdd, _ := os.ReadFile(c.ecsToAddPath)
			contentsExpected, _ := os.ReadFile(c.expectedPath)

			var base yaml.Node
			err := yaml.Unmarshal(contentsBase, &base)
			require.NoError(t, err)

			var template ecsTemplates
			err = yaml.Unmarshal(contentsToAdd, &template)
			require.NoError(t, err)

			err = addEcsMappings(&base, template)
			require.NoError(t, err)

			newYaml, _ := formatResult(&base)

			assert.Equal(t, string(contentsExpected), string(newYaml))
		})
	}
}
