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
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/environment"
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

	t.Setenv(environment.WithElasticPackagePrefix("VERIFY_PACKAGE_SIGNATURE"), "")
	t.Setenv(environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE"), "")

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

	t.Setenv(environment.WithElasticPackagePrefix("VERIFY_PACKAGE_SIGNATURE"), "")
	t.Setenv(environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE"), "")

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

func TestDownloadPackage_withoutVerification(t *testing.T) {
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

	t.Setenv(environment.WithElasticPackagePrefix("VERIFY_PACKAGE_SIGNATURE"), "")
	t.Setenv(environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE"), "")

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	zipPath, err := client.DownloadPackage("acme", "1.0.0", dest)
	require.NoError(t, err)
	require.FileExists(t, zipPath)
}

func TestDownloadPackage_withVerification_success(t *testing.T) {
	zipBytes := testAcmePackageZip(t)
	passphrase := []byte("registry-test-pass")

	priv, err := crypto.GenerateKey("Registry Test", "", "rsa", 2048)
	require.NoError(t, err)
	priv, err = priv.Lock(passphrase)
	require.NoError(t, err)
	unlocked, err := priv.Unlock(passphrase)
	require.NoError(t, err)
	t.Cleanup(func() { unlocked.ClearPrivateParams() })

	signRing, err := crypto.NewKeyRing(unlocked)
	require.NoError(t, err)
	sig, err := signRing.SignDetachedStream(bytes.NewReader(zipBytes))
	require.NoError(t, err)
	armoredSig, err := sig.GetArmored()
	require.NoError(t, err)
	pubArmored, err := unlocked.GetArmoredPublicKey()
	require.NoError(t, err)

	pubPath := filepath.Join(t.TempDir(), "verify.pub.asc")
	require.NoError(t, os.WriteFile(pubPath, []byte(pubArmored), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		case "/epr/acme/acme-1.0.0.zip.sig":
			_, err := w.Write([]byte(armoredSig))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(environment.WithElasticPackagePrefix("VERIFY_PACKAGE_SIGNATURE"), "true")
	t.Setenv(environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE"), pubPath)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	zipPath, err := client.DownloadPackage("acme", "1.0.0", dest)
	require.NoError(t, err)
	require.FileExists(t, zipPath)
}

func TestDownloadPackage_withVerification_missingSignature(t *testing.T) {
	zipBytes := testAcmePackageZip(t)
	pubPath := filepath.Join(t.TempDir(), "verify.pub.asc")
	require.NoError(t, os.WriteFile(pubPath, []byte("x"), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/epr/acme/acme-1.0.0.zip" {
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(environment.WithElasticPackagePrefix("VERIFY_PACKAGE_SIGNATURE"), "true")
	t.Setenv(environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE"), pubPath)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed after failed verification")
}

func TestDownloadPackage_withVerification_badSignature(t *testing.T) {
	zipBytes := testAcmePackageZip(t)
	passphrase := []byte("a")

	priv, err := crypto.GenerateKey("Signer A", "", "rsa", 2048)
	require.NoError(t, err)
	priv, err = priv.Lock(passphrase)
	require.NoError(t, err)
	unlocked, err := priv.Unlock(passphrase)
	require.NoError(t, err)
	t.Cleanup(func() { unlocked.ClearPrivateParams() })
	signRing, err := crypto.NewKeyRing(unlocked)
	require.NoError(t, err)
	sig, err := signRing.SignDetachedStream(bytes.NewReader(zipBytes))
	require.NoError(t, err)
	armoredSig, err := sig.GetArmored()
	require.NoError(t, err)

	priv2, err := crypto.GenerateKey("Signer B", "", "rsa", 2048)
	require.NoError(t, err)
	priv2, err = priv2.Lock(passphrase)
	require.NoError(t, err)
	unlocked2, err := priv2.Unlock(passphrase)
	require.NoError(t, err)
	t.Cleanup(func() { unlocked2.ClearPrivateParams() })
	pubArmored, err := unlocked2.GetArmoredPublicKey()
	require.NoError(t, err)

	pubPath := filepath.Join(t.TempDir(), "b.pub.asc")
	require.NoError(t, os.WriteFile(pubPath, []byte(pubArmored), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/epr/acme/acme-1.0.0.zip":
			_, err := w.Write(zipBytes)
			require.NoError(t, err)
		case "/epr/acme/acme-1.0.0.zip.sig":
			_, err := w.Write([]byte(armoredSig))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(environment.WithElasticPackagePrefix("VERIFY_PACKAGE_SIGNATURE"), "true")
	t.Setenv(environment.WithElasticPackagePrefix("VERIFIER_PUBLIC_KEYFILE"), pubPath)

	dest := t.TempDir()
	client, err := NewClient(srv.URL)
	require.NoError(t, err)
	_, err = client.DownloadPackage("acme", "1.0.0", dest)
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(dest, "acme-1.0.0.zip"))
	require.True(t, errors.Is(statErr, fs.ErrNotExist), "zip should be removed after failed verification")
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
