// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/environment"
)

func TestVerifyDetachedPGP_roundTrip(t *testing.T) {
	content := []byte("package-bytes-for-signature")
	passphrase := []byte("test-passphrase")

	priv, err := crypto.GenerateKey("Test Verify", "", "rsa", 2048)
	require.NoError(t, err)
	priv, err = priv.Lock(passphrase)
	require.NoError(t, err)
	unlocked, err := priv.Unlock(passphrase)
	require.NoError(t, err)
	t.Cleanup(func() { unlocked.ClearPrivateParams() })

	signRing, err := crypto.NewKeyRing(unlocked)
	require.NoError(t, err)

	sig, err := signRing.SignDetachedStream(bytes.NewReader(content))
	require.NoError(t, err)
	armoredSig, err := sig.GetArmored()
	require.NoError(t, err)

	pubArmored, err := unlocked.GetArmoredPublicKey()
	require.NoError(t, err)

	err = VerifyDetachedPGP(bytes.NewReader(content), []byte(armoredSig), []byte(pubArmored))
	require.NoError(t, err)
}

func TestVerifyDetachedPGP_wrongContent(t *testing.T) {
	content := []byte("original")
	other := []byte("tampered")
	passphrase := []byte("p")

	priv, err := crypto.GenerateKey("Test Wrong", "", "rsa", 2048)
	require.NoError(t, err)
	priv, err = priv.Lock(passphrase)
	require.NoError(t, err)
	unlocked, err := priv.Unlock(passphrase)
	require.NoError(t, err)
	t.Cleanup(func() { unlocked.ClearPrivateParams() })

	signRing, err := crypto.NewKeyRing(unlocked)
	require.NoError(t, err)
	sig, err := signRing.SignDetachedStream(bytes.NewReader(content))
	require.NoError(t, err)
	armoredSig, err := sig.GetArmored()
	require.NoError(t, err)
	pubArmored, err := unlocked.GetArmoredPublicKey()
	require.NoError(t, err)

	err = VerifyDetachedPGP(bytes.NewReader(other), []byte(armoredSig), []byte(pubArmored))
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

func TestPackageSignatureVerificationFromEnv(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "pub.asc")
	require.NoError(t, os.WriteFile(keyFile, []byte("not-a-real-key-but-present"), 0o600))

	prefix := environment.WithElasticPackagePrefix
	t.Run("unset", func(t *testing.T) {
		t.Setenv(prefix("VERIFY_PACKAGE_SIGNATURE"), "")
		t.Setenv(prefix("VERIFIER_PUBLIC_KEYFILE"), "")
		v, p, err := PackageSignatureVerificationFromEnv()
		require.NoError(t, err)
		require.False(t, v)
		require.Empty(t, p)
	})
	t.Run("false", func(t *testing.T) {
		t.Setenv(prefix("VERIFY_PACKAGE_SIGNATURE"), "false")
		t.Setenv(prefix("VERIFIER_PUBLIC_KEYFILE"), keyFile)
		v, p, err := PackageSignatureVerificationFromEnv()
		require.NoError(t, err)
		require.False(t, v)
		require.Empty(t, p)
	})
	t.Run("true_ok", func(t *testing.T) {
		t.Setenv(prefix("VERIFY_PACKAGE_SIGNATURE"), "true")
		t.Setenv(prefix("VERIFIER_PUBLIC_KEYFILE"), keyFile)
		v, p, err := PackageSignatureVerificationFromEnv()
		require.NoError(t, err)
		require.True(t, v)
		require.Equal(t, keyFile, p)
	})
	t.Run("true_missing_key_path", func(t *testing.T) {
		t.Setenv(prefix("VERIFY_PACKAGE_SIGNATURE"), "1")
		t.Setenv(prefix("VERIFIER_PUBLIC_KEYFILE"), "")
		_, _, err := PackageSignatureVerificationFromEnv()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "not set"))
	})
	t.Run("invalid_bool", func(t *testing.T) {
		t.Setenv(prefix("VERIFY_PACKAGE_SIGNATURE"), "maybe")
		t.Setenv(prefix("VERIFIER_PUBLIC_KEYFILE"), keyFile)
		_, _, err := PackageSignatureVerificationFromEnv()
		require.Error(t, err)
	})
	t.Run("true_missing_file", func(t *testing.T) {
		t.Setenv(prefix("VERIFY_PACKAGE_SIGNATURE"), "true")
		t.Setenv(prefix("VERIFIER_PUBLIC_KEYFILE"), filepath.Join(t.TempDir(), "nope.asc"))
		_, _, err := PackageSignatureVerificationFromEnv()
		require.Error(t, err)
	})
}
