// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/require"
)

func TestNewClient_invalidCertificateAuthorityPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-ca.pem")
	client, err := NewClient("https://example.test", CertificateAuthority(missing))
	require.Error(t, err)
	require.Nil(t, client)
	require.ErrorContains(t, err, "creating registry HTTP client")
	require.ErrorContains(t, err, "reading CA certificate")
}

func TestNewClient_invalidCertificateAuthorityPEM(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "not-a-cert.pem")
	require.NoError(t, os.WriteFile(badPath, []byte("this is not a PEM certificate block"), 0o600))

	client, err := NewClient("https://example.test", CertificateAuthority(badPath))
	require.Error(t, err)
	require.Nil(t, client)
	require.ErrorContains(t, err, "creating registry HTTP client")
	require.ErrorContains(t, err, "no certificate found")
}

func TestNewClient_tlsskipVerifyOption(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	client, err := NewClient(srv.URL, TLSSkipVerify())
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestDownloadPackage_unexpectedStatusDoesNotWriteZip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	t.Cleanup(srv.Close)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err)
	require.ErrorContains(t, err, "unexpected status code")

	_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "no zip should be written when the registry returns a non-OK status")
}

func TestDownloadPackage_writeFailureCleansUp(t *testing.T) {
	zipBytes := testAcmePackageZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/epr/acme/acme-1.0.0.zip" {
			http.NotFound(w, r)
			return
		}
		_, err := w.Write(zipBytes)
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	dest := t.TempDir()
	zipPath := filepath.Join(dest, "acme-1.0.0.zip")
	require.NoError(t, os.Mkdir(zipPath, 0o700))

	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err)
	require.ErrorContains(t, err, "writing package zip")

	_, statErr := os.Stat(zipPath)
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "partial zip should not remain after a write error")
}

func TestDownloadPackage_success(t *testing.T) {
	t.Setenv("ELASTIC_PACKAGE_DISABLE_VERIFY_PACKAGE_SIGNATURE", "true")

	zipBytes := testAcmePackageZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/epr/acme/acme-1.0.0.zip" {
			http.NotFound(w, r)
			return
		}
		_, err := w.Write(zipBytes)
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	zipPath, err := client.DownloadPackage("acme", "1.0.0", dest)
	require.NoError(t, err)
	require.FileExists(t, zipPath)
}

func TestDownloadPackage_signatureValid(t *testing.T) {
	kp := testKeyPair(t)
	zipBytes := testAcmePackageZip(t)
	sigBytes := signZip(t, kp, zipBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		case "/epr/acme/acme-1.0.0.zip.sig":
			_, err := w.Write(sigBytes)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	pubKeyFile := writeTempFile(t, kp.publicArmor)
	t.Setenv("ELASTIC_PACKAGE_VERIFIER_PUBLIC_KEYFILE", pubKeyFile)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	zipPath, err := client.DownloadPackage("acme", "1.0.0", dest)
	require.NoError(t, err)
	require.FileExists(t, zipPath)
}

func TestDownloadPackage_signatureMissing(t *testing.T) {
	kp := testKeyPair(t)
	zipBytes := testAcmePackageZip(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	pubKeyFile := writeTempFile(t, kp.publicArmor)
	t.Setenv("ELASTIC_PACKAGE_VERIFIER_PUBLIC_KEYFILE", pubKeyFile)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err)
	require.ErrorContains(t, err, "signature")

	_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed when signature is missing")
}

func TestDownloadPackage_signatureInvalid(t *testing.T) {
	kp := testKeyPair(t)
	zipBytes := testAcmePackageZip(t)
	// Sign different bytes to produce a bad signature.
	sigBytes := signZip(t, kp, []byte("not the zip"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		case "/epr/acme/acme-1.0.0.zip.sig":
			_, err := w.Write(sigBytes)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	pubKeyFile := writeTempFile(t, kp.publicArmor)
	t.Setenv("ELASTIC_PACKAGE_VERIFIER_PUBLIC_KEYFILE", pubKeyFile)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err)
	require.ErrorContains(t, err, "signature verification failed")

	_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed when signature is invalid")
}

func TestDownloadPackage_signatureFromWrongKey(t *testing.T) {
	signer := testKeyPair(t)
	verifier := testKeyPair(t)
	zipBytes := testAcmePackageZip(t)
	sigBytes := signZip(t, signer, zipBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		case "/epr/acme/acme-1.0.0.zip.sig":
			_, err := w.Write(sigBytes)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	// Override points at the verifier key, not the signer — verification must fail.
	pubKeyFile := writeTempFile(t, verifier.publicArmor)
	t.Setenv("ELASTIC_PACKAGE_VERIFIER_PUBLIC_KEYFILE", pubKeyFile)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err)
	require.ErrorContains(t, err, "signature verification failed")

	_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed when key does not match")
}

func TestDownloadPackage_verificationDisabled(t *testing.T) {
	t.Setenv("ELASTIC_PACKAGE_DISABLE_VERIFY_PACKAGE_SIGNATURE", "true")

	zipBytes := testAcmePackageZip(t)
	var sigRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		case "/epr/acme/acme-1.0.0.zip.sig":
			sigRequests.Add(1)
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	zipPath, err := client.DownloadPackage("acme", "1.0.0", dest)
	require.NoError(t, err)
	require.FileExists(t, zipPath)
	require.Equal(t, int32(0), sigRequests.Load(), ".zip.sig should never be requested when verification is disabled")
}

// TestDownloadPackage_defaultKeyIsEmbedded verifies that when no override key
// is set, the embedded Elastic key is used — and since the test zip is signed
// with a generated test key (not the real Elastic key), verification fails.
func TestDownloadPackage_defaultKeyIsEmbedded(t *testing.T) {
	t.Setenv("ELASTIC_PACKAGE_VERIFIER_PUBLIC_KEYFILE", "")

	kp := testKeyPair(t)
	zipBytes := testAcmePackageZip(t)
	sigBytes := signZip(t, kp, zipBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		case "/epr/acme/acme-1.0.0.zip.sig":
			_, err := w.Write(sigBytes)
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err, "test key should not verify against the embedded Elastic key")
	require.ErrorContains(t, err, "signature verification failed")

	_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed when verification fails")
}

// testKeyPairData holds a generated test RSA keypair.
type testKeyPairData struct {
	key         *crypto.Key
	publicArmor []byte
}

// testKeyPair generates an RSA-2048 test keypair.
func testKeyPair(t *testing.T) testKeyPairData {
	t.Helper()
	key, err := crypto.GenerateKey("Test", "test@example.com", "rsa", 2048)
	require.NoError(t, err)

	pubArmored, err := key.GetArmoredPublicKey()
	require.NoError(t, err)

	return testKeyPairData{key: key, publicArmor: []byte(pubArmored)}
}

// signZip produces an armored detached PGP signature over data.
func signZip(t *testing.T, kp testKeyPairData, data []byte) []byte {
	t.Helper()
	keyRing, err := crypto.NewKeyRing(kp.key)
	require.NoError(t, err)

	sig, err := keyRing.SignDetachedStream(bytes.NewReader(data))
	require.NoError(t, err)

	armored, err := sig.GetArmored()
	require.NoError(t, err)
	return []byte(armored)
}

// writeTempFile writes data to a temporary file and returns its path.
func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "key.asc")
	require.NoError(t, os.WriteFile(f, data, 0o600))
	return f
}

func testAcmePackageZip(t *testing.T) []byte {
	t.Helper()
	const (
		name    = "acme"
		version = "1.0.0"
	)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	manifestPath := fmt.Sprintf("%s/manifest.yml", name)
	w, err := zw.Create(manifestPath)
	require.NoError(t, err)
	_, err = fmt.Fprintf(w, "name: %s\nversion: %s\ntype: integration\n", name, version)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}
