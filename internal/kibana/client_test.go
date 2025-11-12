// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientWithTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hi!")
	}))

	caCertFile := writeCACertFile(t, server.Certificate())

	version := func(c *Client) {
		c.versionInfo = VersionInfo{Number: "8.0.0"}
		c.semver = semver.MustParse(c.versionInfo.Number)
	}

	t.Run("no TLS config, should fail", func(t *testing.T) {
		client, err := NewClient(version, Address(server.URL))
		require.NoError(t, err)

		_, _, err = client.get(t.Context(), "/")
		assert.Error(t, err)
	})

	t.Run("with CA", func(t *testing.T) {
		client, err := NewClient(version, Address(server.URL), CertificateAuthority(caCertFile))
		require.NoError(t, err)

		_, _, err = client.get(t.Context(), "/")
		assert.NoError(t, err)
	})

	t.Run("skip TLS verify", func(t *testing.T) {
		client, err := NewClient(version, Address(server.URL), TLSSkipVerify())
		require.NoError(t, err)

		_, _, err = client.get(t.Context(), "/")
		assert.NoError(t, err)
	})
}

func writeCACertFile(t *testing.T, cert *x509.Certificate) string {
	var d bytes.Buffer
	err := pem.Encode(&d, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
	require.NoError(t, err)

	caCertFile := filepath.Join(t.TempDir(), "ca.pem")
	err = os.WriteFile(caCertFile, d.Bytes(), 0644)
	require.NoError(t, err)

	return caCertFile
}
