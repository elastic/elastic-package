// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package elasticsearch_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/test"
)

func TestClientWithTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-elastic-product", "Elasticsearch")
	}))

	caCertFile := writeCACertFile(t, server.Certificate())

	t.Run("no TLS config, should fail", func(t *testing.T) {
		client, err := elasticsearch.NewClient(elasticsearch.OptionWithAddress(server.URL))
		require.NoError(t, err)

		_, err = client.Ping()
		assert.Error(t, err)
	})

	t.Run("with CA", func(t *testing.T) {
		client, err := elasticsearch.NewClient(elasticsearch.OptionWithAddress(server.URL), elasticsearch.OptionWithCertificateAuthority(caCertFile))
		require.NoError(t, err)

		_, err = client.Ping()
		assert.NoError(t, err)
	})

	t.Run("skip TLS verify", func(t *testing.T) {
		client, err := elasticsearch.NewClient(elasticsearch.OptionWithAddress(server.URL), elasticsearch.OptionWithSkipTLSVerify())
		require.NoError(t, err)

		_, err = client.Ping()
		assert.NoError(t, err)
	})
}

func TestClusterHealth(t *testing.T) {
	cases := []struct {
		RecordDir string
		Expected  string
	}{
		{
			RecordDir: "./testdata/elasticsearch-8-5-healthy",
		},
		{
			RecordDir: "./testdata/elasticsearch-8-5-red-out-of-disk",
			Expected:  "cluster in unhealthy state: 33 indices reside on nodes that have run or are likely to run out of disk space, this can temporarily disable writing on these indices.",
		},
	}

	for _, c := range cases {
		t.Run(c.RecordDir, func(t *testing.T) {
			client := test.NewClient(t, c.RecordDir)

			err := client.CheckHealth(context.Background())
			if c.Expected != "" {
				if assert.Error(t, err) {
					assert.Equal(t, c.Expected, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
