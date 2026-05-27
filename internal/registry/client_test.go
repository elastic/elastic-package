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

func TestDownloadPackage(t *testing.T) {
	t.Run("unexpected status does not write zip", func(t *testing.T) {
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
	})

	t.Run("write failure cleans up", func(t *testing.T) {
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
	})

	t.Run("success", func(t *testing.T) {
		t.Setenv("ELASTIC_PACKAGE_VERIFIER_DISABLE", "true")

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
	})

	t.Run("signature valid", func(t *testing.T) {
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
		t.Setenv(verifierGPGKeyringEnv, pubKeyFile)

		dest := t.TempDir()
		client, err := NewClient(srv.URL)
		require.NoError(t, err)
		zipPath, err := client.DownloadPackage("acme", "1.0.0", dest)
		require.NoError(t, err)
		require.FileExists(t, zipPath)
	})

	t.Run("signature missing", func(t *testing.T) {
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
		t.Setenv(verifierGPGKeyringEnv, pubKeyFile)

		dest := t.TempDir()
		client, err := NewClient(srv.URL)
		require.NoError(t, err)
		_, err = client.DownloadPackage("acme", "1.0.0", dest)
		require.Error(t, err)
		require.ErrorContains(t, err, "unexpected status code 404")

		_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
		require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed when signature is missing")
	})

	t.Run("signature invalid", func(t *testing.T) {
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
		t.Setenv(verifierGPGKeyringEnv, pubKeyFile)

		dest := t.TempDir()
		client, err := NewClient(srv.URL)
		require.NoError(t, err)
		_, err = client.DownloadPackage("acme", "1.0.0", dest)
		require.Error(t, err)
		require.ErrorContains(t, err, "signature verification failed")

		_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
		require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed when signature is invalid")
	})

	t.Run("signature from wrong key", func(t *testing.T) {
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
		t.Setenv(verifierGPGKeyringEnv, pubKeyFile)

		dest := t.TempDir()
		client, err := NewClient(srv.URL)
		require.NoError(t, err)
		_, err = client.DownloadPackage("acme", "1.0.0", dest)
		require.Error(t, err)
		require.ErrorContains(t, err, "signature verification failed")

		_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
		require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed when key does not match")
	})

	t.Run("verification disabled", func(t *testing.T) {
		t.Setenv("ELASTIC_PACKAGE_VERIFIER_DISABLE", "true")

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
	})

	// When no override key is set the embedded Elastic key is used. Since the
	// test zip is signed with a generated key (not the real Elastic key),
	// verification must fail.
	t.Run("default key is embedded", func(t *testing.T) {
		t.Setenv(verifierGPGKeyringEnv, "")

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
	})

	// When the keyring file contains multiple keys, a signature from any of
	// them is accepted.
	t.Run("signature valid multi-key override", func(t *testing.T) {
		kpA := testKeyPair(t)
		kpB := testKeyPair(t)
		zipBytes := testAcmePackageZip(t)
		// Sign with key B only.
		sigBytes := signZip(t, kpB, zipBytes)

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

		// Keyring file contains both key A and key B.
		combined := make([]byte, 0, len(kpA.publicArmor)+1+len(kpB.publicArmor))
		combined = append(combined, kpA.publicArmor...)
		combined = append(combined, '\n')
		combined = append(combined, kpB.publicArmor...)
		t.Setenv(verifierGPGKeyringEnv, writeTempFile(t, combined))

		dest := t.TempDir()
		client, err := NewClient(srv.URL)
		require.NoError(t, err)
		zipPath, err := client.DownloadPackage("acme", "1.0.0", dest)
		require.NoError(t, err, "signature from key B should verify against a ring containing key A and key B")
		require.FileExists(t, zipPath)
	})

	// When ELASTIC_PACKAGE_VERIFIER_GPG_KEYRING is set the embedded Elastic
	// key is NOT trusted — a zip signed by a key not in the override ring must
	// fail verification.
	t.Run("override excludes embedded key", func(t *testing.T) {
		kpOverride := testKeyPair(t)
		kpSigner := testKeyPair(t)
		zipBytes := testAcmePackageZip(t)
		// Sign with kpSigner; the override keyring only contains kpOverride (different key).
		sigBytes := signZip(t, kpSigner, zipBytes)

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

		// Override contains only kpOverride — neither kpSigner nor the embedded key.
		t.Setenv(verifierGPGKeyringEnv, writeTempFile(t, kpOverride.publicArmor))

		dest := t.TempDir()
		client, err := NewClient(srv.URL)
		require.NoError(t, err)
		_, err = client.DownloadPackage("acme", "1.0.0", dest)
		require.Error(t, err, "override env should exclude the embedded key; signer key not in override ring")
		require.ErrorContains(t, err, "signature verification failed")

		_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
		require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed on verification failure")
	})
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
