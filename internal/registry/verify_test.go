// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// --- LoadVerifierPublicKey ---

func TestLoadVerifierPublicKey_embedded(t *testing.T) {
	t.Setenv(verifierPublicKeyfileEnv, "")

	key, err := LoadVerifierPublicKey()
	require.NoError(t, err)
	require.Equal(t, ElasticPublicKey(), key, "should return the embedded key when override is not set")
}

func TestLoadVerifierPublicKey_override(t *testing.T) {
	kp := testKeyPair(t)

	f := filepath.Join(t.TempDir(), "public.asc")
	require.NoError(t, os.WriteFile(f, kp.publicArmor, 0o600))
	t.Setenv(verifierPublicKeyfileEnv, f)

	key, err := LoadVerifierPublicKey()
	require.NoError(t, err)
	require.Equal(t, kp.publicArmor, key, "should return the override key file contents")
}

func TestLoadVerifierPublicKey_overrideMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nonexistent.asc")
	t.Setenv(verifierPublicKeyfileEnv, missing)

	_, err := LoadVerifierPublicKey()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), verifierPublicKeyfileEnv),
		"error should mention the env var name, got: %v", err)
}

// --- VerifyDetachedPGP ---

func TestVerifyDetachedPGP_success(t *testing.T) {
	kp := testKeyPair(t)
	data := []byte("package contents")
	sig := signZip(t, kp, data)

	err := VerifyDetachedPGP(bytes.NewReader(data), sig, kp.publicArmor)
	require.NoError(t, err)
}

func TestVerifyDetachedPGP_wrongKey(t *testing.T) {
	signer := testKeyPair(t)
	verifier := testKeyPair(t)

	data := []byte("package contents")
	sig := signZip(t, signer, data)

	err := VerifyDetachedPGP(bytes.NewReader(data), sig, verifier.publicArmor)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

func TestVerifyDetachedPGP_tamperedData(t *testing.T) {
	kp := testKeyPair(t)
	data := []byte("package contents")
	sig := signZip(t, kp, data)

	tampered := []byte("package TAMPERED")
	err := VerifyDetachedPGP(bytes.NewReader(tampered), sig, kp.publicArmor)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

func TestVerifyDetachedPGP_invalidSignatureArmor(t *testing.T) {
	kp := testKeyPair(t)
	err := VerifyDetachedPGP(bytes.NewReader([]byte("data")), []byte("not-a-pgp-signature"), kp.publicArmor)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading signature")
}
