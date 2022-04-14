// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/certs"
)

// TODO: Get this list from somewhere else?
var tlsServices = []string{
	"elasticsearch",
	"kibana",
	"package-registry",
	"fleet-server",
}

func initTLSCertificates(profilePath string) error {
	certsDir := filepath.Join(profilePath, "certs")
	caCertFile := filepath.Join(certsDir, "ca-cert.pem")
	caKeyFile := filepath.Join(certsDir, "ca-key.pem")
	if err := verifyTLSCertificates(caCertFile, caCertFile, caKeyFile); err == nil {
		// Valid certificates are already present, nothing to do.
		// TODO: Check also service certificates, and recreate individually.
		return nil
	}

	err := os.MkdirAll(certsDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating directory for TSL certificates: %w", err)
	}
	ca, err := certs.NewCA()
	if err != nil {
		return fmt.Errorf("error initializing self-signed certificates")
	}
	err = ca.WriteCertFile(caCertFile)
	if err != nil {
		return err
	}
	err = ca.WriteKeyFile(caKeyFile)
	if err != nil {
		return err
	}

	for _, service := range tlsServices {
		certsDir := filepath.Join(certsDir, service)
		certFile := filepath.Join(certsDir, "cert.pem")
		keyFile := filepath.Join(certsDir, "key.pem")
		err := os.MkdirAll(certsDir, 0755)
		if err != nil {
			return fmt.Errorf("error creating directory for TSL certificates: %w", err)
		}
		cert, err := ca.Issue()
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
	}

	return nil
}

func verifyTLSCertificates(caFile, certFile, keyFile string) error {
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

	certPool, err := certPoolWithCA(caFile)
	if err != nil {
		return err
	}
	_, err = parsed.Verify(x509.VerifyOptions{
		Roots: certPool,
	})
	if err != nil {
		return err
	}

	return nil
}

func certPoolWithCA(caFile string) (*x509.CertPool, error) {
	d, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	pem, _ := pem.Decode(d)
	if pem == nil || pem.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("no CA found in %s", caFile)
	}
	ca, err := x509.ParseCertificate(pem.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return pool, nil
}
