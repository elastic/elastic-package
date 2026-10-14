// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTests_PackageName(t *testing.T) {
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
			r := NewStaticTestRunner(StaticTestRunnerOptions{
				PackageRoot: c.packageRoot,
			})

			testers, err := r.GetTests(context.Background())
			require.NoError(t, err)
			require.NotEmpty(t, testers)

			for _, tt := range testers {
				st, ok := tt.(*tester)
				require.True(t, ok)
				assert.Equal(t, c.packageName, st.testFolder.Package)
			}
		})
	}
}
