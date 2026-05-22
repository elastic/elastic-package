// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/require"
)

func TestElasticPublicKey_parses(t *testing.T) {
	key := ElasticPublicKey()
	require.NotEmpty(t, key, "embedded Elastic public key must not be empty")

	parsed, err := crypto.NewKeyFromArmored(string(key))
	require.NoError(t, err, "embedded elastic-gpg-key.asc must parse as an armored OpenPGP public key")
	require.NotEmpty(t, parsed.GetFingerprint(), "key fingerprint must not be empty")
}
