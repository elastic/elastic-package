// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestElasticPublicKey_parses(t *testing.T) {
	require.NotEmpty(t, elasticPublicKey, "embedded Elastic public key must not be empty")

	ring, err := LoadVerifierKeyring()
	require.NoError(t, err, "embedded elastic-gpg-key.asc must parse as a valid OpenPGP keyring")
	require.Equal(t, 1, ring.CountEntities())
}
