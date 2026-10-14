// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_PackageName(t *testing.T) {
	cases := []struct {
		title       string
		packageRoot string
		packageName string
	}{
		{
			// Folder name (apache_basic_license) differs from the manifest name (apache).
			title:       "integration package",
			packageRoot: "../../../../test/packages/parallel/apache_basic_license",
			packageName: "apache",
		},
		{
			// Folder name (01_ci_input_pkg) differs from the manifest name (ci_input_pkg).
			title:       "input package",
			packageRoot: "../../../../test/packages/composable/01_ci_input_pkg",
			packageName: "ci_input_pkg",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			// Without a Kibana client the tester stops before installing the package,
			// but the result already contains the package name.
			r := NewAssetTestRunner(AssetTestRunnerOptions{
				PackageRoot: c.packageRoot,
			})

			testers, err := r.GetTests(context.Background())
			require.NoError(t, err)
			require.Len(t, testers, 1)

			results, _ := testers[0].Run(context.Background())
			require.NotEmpty(t, results)
			for _, result := range results {
				assert.Equal(t, c.packageName, result.Package)
			}
		})
	}
}

func TestGetTests_MissingManifest(t *testing.T) {
	r := NewAssetTestRunner(AssetTestRunnerOptions{
		PackageRoot: t.TempDir(),
	})

	_, err := r.GetTests(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "reading package manifest failed")
}
