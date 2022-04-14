// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
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
