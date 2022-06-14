// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package certs

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolWithCACertificate(t *testing.T) {
	pool, err := PoolWithCACertificate("testdata/ca-cert.pem")
	require.NoError(t, err)

	verifyTestCertWithPool(t, pool)
}

func TestSystemPoolWithCACertificate(t *testing.T) {
	pool, err := SystemPoolWithCACertificate("testdata/ca-cert.pem")
	require.NoError(t, err)

	verifyTestCertWithPool(t, pool)
}

func verifyTestCertWithPool(t *testing.T, pool *x509.CertPool) {
	t.Helper()

	p, err := tls.LoadX509KeyPair("testdata/server-cert.pem", "testdata/server-key.pem")
	require.NoError(t, err)
	require.NotEmpty(t, p.Certificate)

	cert, err := x509.ParseCertificate(p.Certificate[0])
	require.NoError(t, err)

	opts := x509.VerifyOptions{
		// Test certificates were valid at this time.
		CurrentTime: time.Date(2022, 06, 10, 0, 0, 0, 0, time.UTC),
	}

	// Check that verification would fail with current system pool.
	opts.Roots, err = x509.SystemCertPool()
	require.NoError(t, err)
	_, err = cert.Verify(opts)
	require.Error(t, err, "this certificate is signed by custom authority, it should fail verification")

	// Now do the actual check.
	opts.Roots = pool
	_, err = cert.Verify(opts)
	assert.NoError(t, err)
}
