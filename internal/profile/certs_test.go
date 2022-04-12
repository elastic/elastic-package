// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"crypto/tls"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSCertsInitialization(t *testing.T) {
	profilePath := t.TempDir()
	certFile := filepath.Join(profilePath, "certs", "cert.pem")
	keyFile := filepath.Join(profilePath, "certs", "key.pem")

	assert.Error(t, verifyTLSCertificates(certFile, keyFile))

	err := initTLSCertificates(profilePath)
	require.NoError(t, err)

	assert.NoError(t, verifyTLSCertificates(certFile, keyFile))
}

func TestSelfSignedCertificate(t *testing.T) {
	cert, err := NewSelfSignedCert()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "cert.key")
	certFile := filepath.Join(tmpDir, "cert.pem")

	err = cert.WriteKeyFile(keyFile)
	require.NoError(t, err)

	err = cert.WriteCertFile(certFile)
	require.NoError(t, err)

	serverAddr := testTLSServer(t, certFile, keyFile)
	client := testHttpClient()

	resp, err := client.Get(serverAddr)
	require.NoError(t, err)
	resp.Body.Close()
}

func testTLSServer(t *testing.T, certFile, keyFile string) string {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		server := &http.Server{
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		}
		server.ServeTLS(listener, certFile, keyFile)
	}()

	return "https://" + listener.Addr().String()
}

func testHttpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}
