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
