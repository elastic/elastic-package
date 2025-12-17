// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
)

func TestRemoveFleetTagsDashboard(t *testing.T) {
	b, err := os.ReadFile("./testdata/elastic_package_registry.dashboard.json")
	require.NoError(t, err)

	var given common.MapStr
	err = json.Unmarshal(b, &given)
	require.NoError(t, err)

	ctx := &transformationContext{
		packageName: "elastic_package_registry",
	}

	result, err := removeFleetTags(ctx, given)
	require.NoError(t, err)

	resultJson, err := json.MarshalIndent(&result, "", "    ")
	require.NoError(t, err)

	expected, err := os.ReadFile("./test/elastic_package_registry.dashboard.json-expected.json")
	require.NoError(t, err)

	require.Equal(t, string(expected), string(resultJson))
}

func TestRemoveFleetTagsObjects(t *testing.T) {
	cases := []struct {
		title           string
		objectFile      string
		expectedRemoved bool
	}{
		{
			title:           "Tag managed by fleet",
			objectFile:      "./testdata/elastic_package_registry.tag_managed_by_fleet.json",
			expectedRemoved: true,
		},
		{
			title:           "Random tag",
			objectFile:      "./testdata/elastic_package_registry.random_tag.json",
			expectedRemoved: false,
		},
		{
			title:           "Shared tag - default",
			objectFile:      "./testdata/elastic_package_registry.shared_tag_default.json",
			expectedRemoved: true,
		},
		{
			title:           "Shared tag - security solution",
			objectFile:      "./testdata/elastic_package_registry.shared_tag_security_solution.json",
			expectedRemoved: true,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			b, err := os.ReadFile(c.objectFile)
			require.NoError(t, err)

			var given common.MapStr
			err = json.Unmarshal(b, &given)
			require.NoError(t, err)

			ctx := &transformationContext{
				packageName: "elastic_package_registry",
			}

			result, err := removeFleetTags(ctx, given)
			require.NoError(t, err)

			if c.expectedRemoved {
				assert.Nil(t, result)
				return
			}

			resultJson, err := json.MarshalIndent(&result, "", "    ")
			require.NoError(t, err)

			assert.Equal(t, string(b), string(resultJson))
		})
	}
}
