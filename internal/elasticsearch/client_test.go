// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package elasticsearch_test

import (
	"bytes"
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
		Record   string
		Expected string
	}{
		{
			// To reproduce the scenario, just start the stack with 8.5 version.
			Record: "./testdata/elasticsearch-8-5-healthy",
		},
		{
			// To reproduce the scenario, start a project in serverless, and
			// replace the host in the urls with https://127.0.0.1:9200.
			Record: "./testdata/elasticsearch-serverless-healthy",
		},
		{
			// To reproduce the scenario, start the stack with 8.5 version and
			// limited disk space. If difficult to reproduce, manually modify
			// the recording using info from previous changesets.
			Record:   "./testdata/elasticsearch-8-5-red-out-of-disk",
			Expected: "cluster in unhealthy state: 33 indices reside on nodes that have run or are likely to run out of disk space, this can temporarily disable writing on these indices.",
		},
	}

	for _, c := range cases {
		t.Run(c.Record, func(t *testing.T) {
			client := test.NewClient(t, c.Record, nil)

			err := client.CheckHealth(t.Context())
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

func TestClusterInfo(t *testing.T) {
	client := test.NewClient(t, "./testdata/elasticsearch-9-info", nil)
	info, err := client.Info(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "9.0.0-SNAPSHOT", info.Version.Number)
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
