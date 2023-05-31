// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package certs

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfSignedCertificate(t *testing.T) {
	const commonName = "someserver"
	cert, err := NewSelfSignedCert(WithName(commonName))
	require.NoError(t, err)

	address := testTLSServer(t, cert)
	testTLSClient(t, cert, commonName, address)
}

func TestCA(t *testing.T) {
	ca, err := NewCA()
	require.NoError(t, err)

	intermediate, err := ca.IssueIntermediate()
	require.NoError(t, err)

	const commonName = "elasticsearch"
	cert, err := intermediate.Issue(WithName(commonName))
	require.NoError(t, err)

	t.Run("validate server with root CA", func(t *testing.T) {
		address := testTLSServer(t, cert)
		t.Run("go-http client", func(t *testing.T) {
			testTLSClient(t, ca.Certificate, commonName, address)
		})
		t.Run("curl", func(t *testing.T) {
			testCurl(t, ca.Certificate, commonName, address)
		})
	})

	t.Run("validate server with intermediate CA", func(t *testing.T) {
		address := testTLSServer(t, cert)
		t.Run("go-http client", func(t *testing.T) {
			testTLSClient(t, intermediate.Certificate, commonName, address)
		})
		t.Run("curl", func(t *testing.T) {
			testCurl(t, intermediate.Certificate, commonName, address)
		})
	})
}

func testTLSServer(t *testing.T, cert *Certificate) string {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "cert.key")
	certFile := filepath.Join(tmpDir, "cert.pem")

	err := cert.WriteKeyFile(keyFile)
	require.NoError(t, err)

	err = cert.WriteCertFile(certFile)
	require.NoError(t, err)

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

	return listener.Addr().String()
}

func testTLSClient(t *testing.T, root *Certificate, commonName, address string) {
	caPool := x509.NewCertPool()
	caPool.AddCert(root.cert)
	client := &http.Client{
		Transport: &http.Transport{
			// Send all requests to the listener address.
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, address)
			},
			DialTLSContext: func(ctx context.Context, network, reqAddress string) (net.Conn, error) {
				var d tls.Dialer
				host, _, _ := net.SplitHostPort(reqAddress)
				d.Config = &tls.Config{
					ServerName: host,
					RootCAs:    caPool,
				}
				return d.DialContext(ctx, network, address)
			},
		},
	}

	resp, err := client.Get("https://" + commonName)
	require.NoError(t, err)
	defer resp.Body.Close()
	d, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(d))

}

func testCurl(t *testing.T, root *Certificate, commonName, address string) {
	_, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("curl not available")
	}

	caCert := filepath.Join(t.TempDir(), "ca-cert.pem")
	err = root.WriteCertFile(caCert)
	require.NoError(t, err)

	serverHost, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	require.NotNilf(t, net.ParseIP(serverHost), "%s expected to be an ip", serverHost)

	// Address to use in the request, hostname here must match name in certificate.
	reqAddress := net.JoinHostPort(commonName, port)

	args := []string{
		"-v",
		"--cacert", caCert,
		// Send requests to the listener address.
		"--resolve", reqAddress + ":" + serverHost,
		// Ignore check for revocation status when not available.
		"--ssl-revoke-best-effort",
		"https://" + reqAddress,
	}

	var buf bytes.Buffer
	cmd := exec.Command("curl", args...)
	cmd.Stderr = &buf
	cmd.Stdout = &buf

	err = cmd.Run()
	if !assert.NoError(t, err) {
		t.Log(buf.String())
	}
}
