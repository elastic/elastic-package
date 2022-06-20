// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package elasticsearch

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientWithTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-elastic-product", "Elasticsearch")
	}))

	caCertFile := writeCACertFile(t, server.Certificate())

	t.Run("no TLS config, should fail", func(t *testing.T) {
		client, err := Client(OptionWithAddress(server.URL))
		require.NoError(t, err)

		_, err = client.Ping()
		assert.Error(t, err)
	})

	t.Run("with CA", func(t *testing.T) {
		client, err := Client(OptionWithAddress(server.URL), OptionWithCertificateAuthority(caCertFile))
		require.NoError(t, err)

		_, err = client.Ping()
		assert.NoError(t, err)
	})

	t.Run("skip TLS verify", func(t *testing.T) {
		client, err := Client(OptionWithAddress(server.URL), OptionWithSkipTLSVerify())
		require.NoError(t, err)

		_, err = client.Ping()
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
	err = ioutil.WriteFile(caCertFile, d.Bytes(), 0644)
	require.NoError(t, err)

	return caCertFile
}
