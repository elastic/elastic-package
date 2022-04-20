// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSCertsInitialization(t *testing.T) {
	profilePath := t.TempDir()
	caCertFile := filepath.Join(profilePath, "certs", "ca-cert.pem")
	caKeyFile := filepath.Join(profilePath, "certs", "ca-key.pem")

	assert.Error(t, verifyTLSCertificates(caCertFile, caCertFile, caKeyFile, ""))

	err := initTLSCertificates(profilePath)
	require.NoError(t, err)

	assert.NoError(t, verifyTLSCertificates(caCertFile, caCertFile, caKeyFile, ""))

	for _, service := range tlsServices {
		t.Run(service, func(t *testing.T) {
			serviceCertFile := filepath.Join(profilePath, "certs", service, "cert.pem")
			serviceKeyFile := filepath.Join(profilePath, "certs", service, "key.pem")
			assert.NoError(t, verifyTLSCertificates(caCertFile, serviceCertFile, serviceKeyFile, service))
		})
	}

	t.Run("service certificate individually recreated", func(t *testing.T) {
		service := tlsServices[0]
		serviceCertFile := filepath.Join(profilePath, "certs", service, "cert.pem")
		serviceKeyFile := filepath.Join(profilePath, "certs", service, "key.pem")
		assert.NoError(t, verifyTLSCertificates(caCertFile, serviceCertFile, serviceKeyFile, service))

		// Remove the certificate.
		os.Remove(serviceCertFile)
		os.Remove(serviceKeyFile)
		assert.Error(t, verifyTLSCertificates(caCertFile, serviceCertFile, serviceKeyFile, service))

		// Check it is created again and is validated by the same CA.
		err := initTLSCertificates(profilePath)
		require.NoError(t, err)
		assert.NoError(t, verifyTLSCertificates(caCertFile, serviceCertFile, serviceKeyFile, service))
	})
}
