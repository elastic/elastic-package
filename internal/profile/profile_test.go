// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateProfile(t *testing.T) {
	options := Options{
		ProfilesDirPath: t.TempDir(),
	}
	err := CreateProfile(options)
	require.NoError(t, err)
}
