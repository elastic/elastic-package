// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/certs"
)

func initTLSCertificates(profilePath string) error {
	certsDir := filepath.Join(profilePath, "certs")
	certFile := filepath.Join(certsDir, "cert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")
	if err := verifyTLSCertificates(certFile, keyFile); err == nil {
		// Valid certificates are already present, nothing to do.
		return nil
	}

	err := os.MkdirAll(certsDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating directory for TSL certificates: %w", err)
	}

	cert, err := certs.NewSelfSignedCert()
	if err != nil {
		return fmt.Errorf("error initializing self-signed certificates")
	}
	err = cert.WriteCertFile(certFile)
	if err != nil {
		return err
	}
	err = cert.WriteKeyFile(keyFile)
	if err != nil {
		return err
	}

	return nil
}

func verifyTLSCertificates(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	if len(cert.Certificate) == 0 {
		return errors.New("certificate chain is empty")
	}

	leaf := cert.Certificate[0]
	parsed, err := x509.ParseCertificate(leaf)
	if err != nil {
		// This shouldn't happen because we have already loaded the certificate before.
		return err
	}

	// This is expected to be self-signed, so check with itself.
	pool := x509.NewCertPool()
	pool.AddCert(parsed)
	_, err = parsed.Verify(x509.VerifyOptions{
		Roots: pool,
	})
	if err != nil {
		return err
	}

	return nil
}
