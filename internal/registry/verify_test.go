// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyRingFromPair builds a *crypto.KeyRing from a testKeyPairData's public key.
func keyRingFromPair(t *testing.T, kp testKeyPairData) *crypto.KeyRing {
	t.Helper()
	key, err := crypto.NewKeyFromArmored(string(kp.publicArmor))
	require.NoError(t, err)
	ring, err := crypto.NewKeyRing(key)
	require.NoError(t, err)
	return ring
}

func TestVerifyPackageSignatureDisabled(t *testing.T) {
	t.Run("default false", func(t *testing.T) {
		t.Setenv(disableVerifyPackageSignatureEnv, "")
		assert.False(t, verifyPackageSignatureDisabled())
	})

	t.Run("explicit false", func(t *testing.T) {
		t.Setenv(disableVerifyPackageSignatureEnv, "false")
		assert.False(t, verifyPackageSignatureDisabled())
	})

	t.Run("true", func(t *testing.T) {
		t.Setenv(disableVerifyPackageSignatureEnv, "true")
		assert.True(t, verifyPackageSignatureDisabled())
	})

	// Only the exact string "true" disables verification; other truthy-looking
	// values must not (strict equality).
	for _, v := range []string{"1", "yes", "TRUE", "True"} {
		t.Run(v+" is false", func(t *testing.T) {
			t.Setenv(disableVerifyPackageSignatureEnv, v)
			assert.False(t, verifyPackageSignatureDisabled())
		})
	}
}

func TestLoadVerifierKeyring(t *testing.T) {
	t.Run("embedded key by default", func(t *testing.T) {
		t.Setenv(verifierGPGKeyringEnv, "")

		require.NotEmpty(t, elasticPublicKey, "embedded Elastic public key must not be empty")
		ring, err := loadVerifierKeyring()
		require.NoError(t, err)
		require.Equal(t, 1, ring.CountEntities(), "default keyring should contain exactly the embedded Elastic key")

		embeddedKey, err := crypto.NewKeyFromArmored(string(elasticPublicKey))
		require.NoError(t, err)
		require.Equal(t, embeddedKey.GetFingerprint(), ring.GetKeys()[0].GetFingerprint(),
			"default keyring key should match the embedded Elastic key fingerprint")
	})

	t.Run("override single key", func(t *testing.T) {
		kp := testKeyPair(t)
		f := writeTempFile(t, kp.publicArmor)
		t.Setenv(verifierGPGKeyringEnv, f)

		ring, err := loadVerifierKeyring()
		require.NoError(t, err)
		require.Equal(t, 1, ring.CountEntities(), "override with one key should produce a single-key ring")

		overrideKey, err := crypto.NewKeyFromArmored(string(kp.publicArmor))
		require.NoError(t, err)
		require.Equal(t, overrideKey.GetFingerprint(), ring.GetKeys()[0].GetFingerprint())
	})

	t.Run("override multi-key", func(t *testing.T) {
		kp1 := testKeyPair(t)
		kp2 := testKeyPair(t)

		combined := make([]byte, 0, len(kp1.publicArmor)+1+len(kp2.publicArmor))
		combined = append(combined, kp1.publicArmor...)
		combined = append(combined, '\n')
		combined = append(combined, kp2.publicArmor...)
		f := writeTempFile(t, combined)
		t.Setenv(verifierGPGKeyringEnv, f)

		ring, err := loadVerifierKeyring()
		require.NoError(t, err)
		require.Equal(t, 2, ring.CountEntities(), "override with two concatenated keys should produce a two-key ring")

		fingerprints := make(map[string]bool)
		for _, k := range ring.GetKeys() {
			fingerprints[k.GetFingerprint()] = true
		}
		key1, err := crypto.NewKeyFromArmored(string(kp1.publicArmor))
		require.NoError(t, err)
		key2, err := crypto.NewKeyFromArmored(string(kp2.publicArmor))
		require.NoError(t, err)

		assert.True(t, fingerprints[key1.GetFingerprint()], "ring should contain first key's fingerprint")
		assert.True(t, fingerprints[key2.GetFingerprint()], "ring should contain second key's fingerprint")
	})

	t.Run("override replaces embedded key", func(t *testing.T) {
		kp := testKeyPair(t)
		f := writeTempFile(t, kp.publicArmor)
		t.Setenv(verifierGPGKeyringEnv, f)

		ring, err := loadVerifierKeyring()
		require.NoError(t, err)

		embeddedKey, err := crypto.NewKeyFromArmored(string(elasticPublicKey))
		require.NoError(t, err)

		for _, k := range ring.GetKeys() {
			require.NotEqual(t, embeddedKey.GetFingerprint(), k.GetFingerprint(),
				"override env should replace the embedded key, not add to it")
		}
	})

	t.Run("override missing file", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nonexistent.asc")
		t.Setenv(verifierGPGKeyringEnv, missing)

		_, err := loadVerifierKeyring()
		require.Error(t, err)
		require.ErrorContains(t, err, verifierGPGKeyringEnv)
	})

	t.Run("override empty file", func(t *testing.T) {
		f := writeTempFile(t, []byte{})
		t.Setenv(verifierGPGKeyringEnv, f)

		_, err := loadVerifierKeyring()
		require.Error(t, err)
		require.ErrorContains(t, err, "no OpenPGP keys found")
		require.ErrorContains(t, err, verifierGPGKeyringEnv)
	})

	t.Run("override garbage file", func(t *testing.T) {
		f := writeTempFile(t, []byte("this is not a pgp key"))
		t.Setenv(verifierGPGKeyringEnv, f)

		_, err := loadVerifierKeyring()
		require.Error(t, err)
		require.ErrorContains(t, err, "no OpenPGP keys found")
		require.ErrorContains(t, err, verifierGPGKeyringEnv)
	})
}

func TestVerifyDetachedPGP(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		kp := testKeyPair(t)
		data := []byte("package contents")
		sig := signZip(t, kp, data)

		err := verifyDetachedPGP(bytes.NewReader(data), sig, keyRingFromPair(t, kp))
		require.NoError(t, err)
	})

	t.Run("wrong key", func(t *testing.T) {
		signer := testKeyPair(t)
		verifier := testKeyPair(t)
		data := []byte("package contents")
		sig := signZip(t, signer, data)

		err := verifyDetachedPGP(bytes.NewReader(data), sig, keyRingFromPair(t, verifier))
		require.Error(t, err)
		require.ErrorContains(t, err, "signature verification failed")
	})

	t.Run("tampered data", func(t *testing.T) {
		kp := testKeyPair(t)
		data := []byte("package contents")
		sig := signZip(t, kp, data)

		tampered := []byte("package TAMPERED")
		err := verifyDetachedPGP(bytes.NewReader(tampered), sig, keyRingFromPair(t, kp))
		require.Error(t, err)
		require.ErrorContains(t, err, "signature verification failed")
	})

	t.Run("invalid signature armor", func(t *testing.T) {
		kp := testKeyPair(t)
		err := verifyDetachedPGP(bytes.NewReader([]byte("data")), []byte("not-a-pgp-signature"), keyRingFromPair(t, kp))
		require.Error(t, err)
		require.ErrorContains(t, err, "reading signature")
	})

	// A signature from any key in the ring is accepted.
	t.Run("multi-key ring", func(t *testing.T) {
		kp1 := testKeyPair(t)
		kp2 := testKeyPair(t)
		data := []byte("package contents")

		ring := keyRingFromPair(t, kp1)
		key2, err := crypto.NewKeyFromArmored(string(kp2.publicArmor))
		require.NoError(t, err)
		require.NoError(t, ring.AddKey(key2))

		sig := signZip(t, kp2, data)

		require.NoError(t, verifyDetachedPGP(bytes.NewReader(data), sig, ring))
	})
}
