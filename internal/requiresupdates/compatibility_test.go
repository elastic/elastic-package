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
	ok, err := kibanaConstraintsOverlap("^9.4.0", "^9.4.0")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = kibanaConstraintsOverlap(">=9.4.0,<9.6.0", "^9.6.0")
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = kibanaConstraintsOverlap(">=9.4.0,<9.6.0", "^9.4.0")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = kibanaConstraintsOverlap("", "^9.6.0")
	require.NoError(t, err)
	require.True(t, ok)

	// Strict-greater lower bound: the regex floor 9.5.0 fails >9.5.0; 9.5.1 must be
	// tried as a representative so the window (9.5.0, 9.6.0) is not missed.
	ok, err = kibanaConstraintsOverlap(">9.5.0,<9.6.0", ">=9.5.1")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = kibanaConstraintsOverlap(">9.5.0,<9.6.0", "^9.6.0")
	require.NoError(t, err)
	require.False(t, ok)
}
