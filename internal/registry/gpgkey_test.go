// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"strings"
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/require"
)

// expectedEmbeddedKeyFingerprint is the known fingerprint of the Elastic public
// GPG key embedded in this binary. When the upstream key rotates, update this
// constant together with elastic-gpg-key.asc by running:
//
//	go run ./internal/registry/gpgkey/fetch
const expectedEmbeddedKeyFingerprint = "46095ACC8548582C1A2699A9D27D666CD88E42B4"

// TestEmbeddedKeyFingerprint checks that the embedded public key has the
// expected fingerprint. If the key has been rotated without updating
// expectedEmbeddedKeyFingerprint, this test will fail.
func TestEmbeddedKeyFingerprint(t *testing.T) {
	key, err := crypto.NewKeyFromArmored(string(elasticPublicKey))
	require.NoError(t, err)
	got := strings.ToUpper(key.GetFingerprint())
	require.Equal(t, expectedEmbeddedKeyFingerprint, got,
		"embedded Elastic GPG key fingerprint changed — update expectedEmbeddedKeyFingerprint and elastic-gpg-key.asc together")
}
