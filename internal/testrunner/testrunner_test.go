// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestPackageHasDataStreams(t *testing.T) {
	cases := []struct {
		packageType string
		expected    bool
	}{
		{"integration", true},
		{"input", false},
		{"content", false},
		{"blueprint", false},
	}
	for _, c := range cases {
		t.Run(c.packageType, func(t *testing.T) {
			manifest := &packages.PackageManifest{Type: c.packageType}
			hasDataStreams, err := PackageHasDataStreams(manifest)
			require.NoError(t, err)
			assert.Equal(t, c.expected, hasDataStreams)
		})
	}

	t.Run("unknown type", func(t *testing.T) {
		manifest := &packages.PackageManifest{Type: "unknown"}
		_, err := PackageHasDataStreams(manifest)
		require.Error(t, err)
	})
}
