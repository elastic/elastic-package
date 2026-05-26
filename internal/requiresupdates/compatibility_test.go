// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveEPRKibanaVersions(t *testing.T) {
	t.Run("empty integration constraint", func(t *testing.T) {
		require.Nil(t, deriveEPRKibanaVersions(""))
	})

	t.Run("single branch", func(t *testing.T) {
		got := deriveEPRKibanaVersions("^9.4.0")
		require.Equal(t, []string{"9.4.0"}, got)
	})

	t.Run("OR branches", func(t *testing.T) {
		got := deriveEPRKibanaVersions("^8.0.0 || ^9.0.0")
		require.ElementsMatch(t, []string{"8.0.0", "9.0.0"}, got)
	})
}

func TestKibanaConstraintsOverlap(t *testing.T) {
	tests := []struct {
		name        string
		integration string
		dependency  string
		want        bool
	}{
		{
			name:        "identical constraints overlap",
			integration: "^9.4.0",
			dependency:  "^9.4.0",
			want:        true,
		},
		{
			name:        "non-overlapping ranges",
			integration: ">=9.4.0,<9.6.0",
			dependency:  "^9.6.0",
			want:        false,
		},
		{
			name:        "dependency range contained in integration range",
			integration: ">=9.4.0,<9.6.0",
			dependency:  "^9.4.0",
			want:        true,
		},
		{
			name:        "empty integration constraint always overlaps",
			integration: "",
			dependency:  "^9.6.0",
			want:        true,
		},
		{
			name:        "empty dependency constraint always overlaps",
			integration: "^9.4.0",
			dependency:  "",
			want:        true,
		},
		{
			// Strict-greater lower bound: the regex floor 9.5.0 fails >9.5.0; 9.5.1 must be
			// tried as a representative so the window (9.5.0, 9.6.0) is not missed.
			name:        "strict-greater lower bound covered by patch+1 representative",
			integration: ">9.5.0,<9.6.0",
			dependency:  ">=9.5.1",
			want:        true,
		},
		{
			name:        "strict-greater range does not overlap higher constraint",
			integration: ">9.5.0,<9.6.0",
			dependency:  "^9.6.0",
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := kibanaConstraintsOverlap(tt.integration, tt.dependency)
			require.NoError(t, err)
			require.Equal(t, tt.want, ok)
		})
	}
}
