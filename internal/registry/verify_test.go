// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"bytes"
	"path/filepath"
	"strings"
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

// --- VerifyPackageSignatureDisabled ---

func TestVerifyPackageSignatureDisabled_defaultFalse(t *testing.T) {
	t.Setenv(disableVerifyPackageSignatureEnv, "")
	assert.False(t, VerifyPackageSignatureDisabled())
}

func TestVerifyPackageSignatureDisabled_explicitFalse(t *testing.T) {
	t.Setenv(disableVerifyPackageSignatureEnv, "false")
	assert.False(t, VerifyPackageSignatureDisabled())
}

func TestVerifyPackageSignatureDisabled_true(t *testing.T) {
	t.Setenv(disableVerifyPackageSignatureEnv, "true")
	assert.True(t, VerifyPackageSignatureDisabled())
}

// Other truthy-looking strings must NOT disable verification (strict equality).
func TestVerifyPackageSignatureDisabled_otherValuesAreFalse(t *testing.T) {
	for _, v := range []string{"1", "yes", "TRUE", "True"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(disableVerifyPackageSignatureEnv, v)
			assert.False(t, VerifyPackageSignatureDisabled())
		})
	}
}

// --- LoadVerifierKeyring ---

func TestLoadVerifierKeyring_embedded(t *testing.T) {
	t.Setenv(verifierGPGKeyringEnv, "")

	ring, err := LoadVerifierKeyring()
	require.NoError(t, err)
	require.Equal(t, 1, ring.CountEntities(), "default keyring should contain exactly the embedded Elastic key")

	embeddedKey, err := crypto.NewKeyFromArmored(string(elasticPublicKey))
	require.NoError(t, err)
	require.Equal(t, embeddedKey.GetFingerprint(), ring.GetKeys()[0].GetFingerprint(),
		"default keyring key should match the embedded Elastic key fingerprint")
}

func TestLoadVerifierKeyring_override_singleKey(t *testing.T) {
	kp := testKeyPair(t)
	f := writeTempFile(t, kp.publicArmor)
	t.Setenv(verifierGPGKeyringEnv, f)

	ring, err := LoadVerifierKeyring()
	require.NoError(t, err)
	require.Equal(t, 1, ring.CountEntities(), "override with one key should produce a single-key ring")

	overrideKey, err := crypto.NewKeyFromArmored(string(kp.publicArmor))
	require.NoError(t, err)
	require.Equal(t, overrideKey.GetFingerprint(), ring.GetKeys()[0].GetFingerprint())
}

func TestLoadVerifierKeyring_override_multiKey(t *testing.T) {
	kp1 := testKeyPair(t)
	kp2 := testKeyPair(t)

	// Concatenate both armored public keys into one file.
	combined := make([]byte, 0, len(kp1.publicArmor)+1+len(kp2.publicArmor))
	combined = append(combined, kp1.publicArmor...)
	combined = append(combined, '\n')
	combined = append(combined, kp2.publicArmor...)
	f := writeTempFile(t, combined)
	t.Setenv(verifierGPGKeyringEnv, f)

	ring, err := LoadVerifierKeyring()
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
}

func TestLoadVerifierKeyring_override_missingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nonexistent.asc")
	t.Setenv(verifierGPGKeyringEnv, missing)

	_, err := LoadVerifierKeyring()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), verifierGPGKeyringEnv),
		"error should mention the env var name, got: %v", err)
}

func TestLoadVerifierKeyring_override_emptyFile(t *testing.T) {
	f := writeTempFile(t, []byte{})
	t.Setenv(verifierGPGKeyringEnv, f)

	_, err := LoadVerifierKeyring()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), verifierGPGKeyringEnv),
		"error should mention the env var name, got: %v", err)
}

func TestLoadVerifierKeyring_override_garbageFile(t *testing.T) {
	f := writeTempFile(t, []byte("this is not a pgp key"))
	t.Setenv(verifierGPGKeyringEnv, f)

	_, err := LoadVerifierKeyring()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), verifierGPGKeyringEnv),
		"error should mention the env var name, got: %v", err)
}

// --- VerifyDetachedPGP ---

func TestVerifyDetachedPGP_success(t *testing.T) {
	kp := testKeyPair(t)
	data := []byte("package contents")
	sig := signZip(t, kp, data)

	err := VerifyDetachedPGP(bytes.NewReader(data), sig, keyRingFromPair(t, kp))
	require.NoError(t, err)
}

func TestVerifyDetachedPGP_wrongKey(t *testing.T) {
	signer := testKeyPair(t)
	verifier := testKeyPair(t)

	data := []byte("package contents")
	sig := signZip(t, signer, data)

	err := VerifyDetachedPGP(bytes.NewReader(data), sig, keyRingFromPair(t, verifier))
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

func TestVerifyDetachedPGP_tamperedData(t *testing.T) {
	kp := testKeyPair(t)
	data := []byte("package contents")
	sig := signZip(t, kp, data)

	tampered := []byte("package TAMPERED")
	err := VerifyDetachedPGP(bytes.NewReader(tampered), sig, keyRingFromPair(t, kp))
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

func TestVerifyDetachedPGP_invalidSignatureArmor(t *testing.T) {
	kp := testKeyPair(t)
	err := VerifyDetachedPGP(bytes.NewReader([]byte("data")), []byte("not-a-pgp-signature"), keyRingFromPair(t, kp))
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading signature")
}

// TestVerifyDetachedPGP_multiKeyRing verifies that a signature from any key in
// the ring is accepted.
func TestVerifyDetachedPGP_multiKeyRing(t *testing.T) {
	kp1 := testKeyPair(t)
	kp2 := testKeyPair(t)
	data := []byte("package contents")

	// Build a ring with both keys.
	ring := keyRingFromPair(t, kp1)
	key2, err := crypto.NewKeyFromArmored(string(kp2.publicArmor))
	require.NoError(t, err)
	require.NoError(t, ring.AddKey(key2))

	// Sign with kp2 only.
	sig := signZip(t, kp2, data)

	// Ring contains kp1 and kp2; kp2 signed it — should verify.
	require.NoError(t, VerifyDetachedPGP(bytes.NewReader(data), sig, ring))
}

// TestLoadVerifierKeyring_overrideReplacesEmbedded verifies that setting the
// override env var excludes the embedded Elastic key from the ring.
func TestLoadVerifierKeyring_overrideReplacesEmbedded(t *testing.T) {
	kp := testKeyPair(t)
	f := writeTempFile(t, kp.publicArmor)
	t.Setenv(verifierGPGKeyringEnv, f)

	ring, err := LoadVerifierKeyring()
	require.NoError(t, err)

	embeddedKey, err := crypto.NewKeyFromArmored(string(elasticPublicKey))
	require.NoError(t, err)

	for _, k := range ring.GetKeys() {
		require.NotEqual(t, embeddedKey.GetFingerprint(), k.GetFingerprint(),
			"override env should replace the embedded key, not add to it")
	}
}
