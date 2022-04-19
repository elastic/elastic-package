// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package certs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfSignedCertificate(t *testing.T) {
	cert, err := NewSelfSignedCert()
	require.NoError(t, err)

	testServerWithCertificate(t, cert, cert)
}

func TestCA(t *testing.T) {
	ca, err := NewCA()
	require.NoError(t, err)

	intermediate, err := ca.IssueIntermediate()
	require.NoError(t, err)

	cert, err := intermediate.Issue()
	require.NoError(t, err)

	t.Run("validate server with root CA", func(t *testing.T) {
		testServerWithCertificate(t, ca.Certificate, cert)
	})

	t.Run("validate server with intermediate CA", func(t *testing.T) {
		testServerWithCertificate(t, intermediate.Certificate, cert)
	})
}

func TestIsSelfSigned(t *testing.T) {
	t.Run("self-signed", func(t *testing.T) {
		c, err := NewSelfSignedCert()
		require.NoError(t, err)
		assert.True(t, isSelfSigned(c.cert))
	})

	t.Run("self-signed CA", func(t *testing.T) {
		c, err := NewCA()
		require.NoError(t, err)
		assert.True(t, isSelfSigned(c.cert))
	})

	t.Run("certificate signed by CA", func(t *testing.T) {
		ca, err := NewCA()
		require.NoError(t, err)
		c, err := ca.Issue()
		require.NoError(t, err)
		assert.False(t, isSelfSigned(c.cert))
	})

	t.Run("intermediate CA", func(t *testing.T) {
		ca, err := NewCA()
		require.NoError(t, err)
		c, err := ca.IssueIntermediate()
		assert.False(t, isSelfSigned(c.cert))
	})

	t.Run("certificate signed by intermediary CA", func(t *testing.T) {
		ca, err := NewCA()
		require.NoError(t, err)
		intermediate, err := ca.IssueIntermediate()
		c, err := intermediate.Issue()
		require.NoError(t, err)
		assert.False(t, isSelfSigned(c.cert))
	})
}

func testServerWithCertificate(t *testing.T, root *Certificate, cert *Certificate) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "cert.key")
	certFile := filepath.Join(tmpDir, "cert.pem")

	err := cert.WriteKeyFile(keyFile)
	require.NoError(t, err)

	err = cert.WriteCertFile(certFile)
	require.NoError(t, err)

	client := testTLSServer(t, root, certFile, keyFile)

	resp, err := client.Get("https://elasticsearch")
	require.NoError(t, err)
	defer resp.Body.Close()
	d, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(d))
}

func testTLSServer(t *testing.T, root *Certificate, certFile, keyFile string) *http.Client {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "ok")
			}),
		}
		server.ServeTLS(listener, certFile, keyFile)
	}()

	caPool := x509.NewCertPool()
	caPool.AddCert(root.cert)
	return &http.Client{
		Transport: &http.Transport{
			// Send all requests to the listener address.
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, listener.Addr().String())
			},
			DialTLSContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				var d tls.Dialer
				host, _, _ := net.SplitHostPort(address)
				d.Config = &tls.Config{
					ServerName: host,
					RootCAs:    caPool,
				}
				return d.DialContext(ctx, network, listener.Addr().String())
			},
		},
	}
}
