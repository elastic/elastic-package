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
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/certs"
)

// tlsServices is the list of server TLS certificates that will be
// created in the given path.
var tlsServices = []string{
	"elasticsearch",
	"kibana",
	"package-registry",
	"fleet-server",
}

// initTLSCertificates initializes all the certificates needed to run the services
// managed by elastic-package stack. It includes a CA, and a pair of keys and
// certificates for each service.
func initTLSCertificates(profilePath string) error {
	certsDir := filepath.Join(profilePath, "certs")
	caCertFile := filepath.Join(certsDir, "ca-cert.pem")
	caKeyFile := filepath.Join(certsDir, "ca-key.pem")

	ca, err := initCA(caCertFile, caKeyFile)
	if err != nil {
		return err
	}

	for _, service := range tlsServices {
		err := initServiceTLSCertificates(ca, caCertFile, certsDir, service)
		if err != nil {
			return err
		}
	}

	return nil
}

func initCA(certFile, keyFile string) (*certs.Issuer, error) {
	if err := verifyTLSCertificates(certFile, certFile, keyFile, ""); err == nil {
		// Valid CA is already present, load it to check service certificates.
		ca, err := certs.LoadCA(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("error loading CA: %w", err)
		}
		return ca, nil
	}
	ca, err := certs.NewCA()
	if err != nil {
		return nil, fmt.Errorf("error initializing self-signed CA")
	}
	err = ca.WriteCertFile(certFile)
	if err != nil {
		return nil, err
	}
	err = ca.WriteKeyFile(keyFile)
	if err != nil {
		return nil, err
	}
	return ca, nil
}

func initServiceTLSCertificates(ca *certs.Issuer, caCertFile string, certsDir, service string) error {
	certsDir = filepath.Join(certsDir, service)
	caFile := filepath.Join(certsDir, "ca-cert.pem")
	certFile := filepath.Join(certsDir, "cert.pem")
	keyFile := filepath.Join(certsDir, "key.pem")
	if err := verifyTLSCertificates(caCertFile, certFile, keyFile, service); err == nil {
		// Certificate already present and valid, nothing to do.
		return nil
	}

	cert, err := ca.Issue(certs.WithName(service))
	if err != nil {
		return fmt.Errorf("error initializing certificate for %q", service)
	}
	err = cert.WriteCertFile(certFile)
	if err != nil {
		return err
	}
	err = cert.WriteKeyFile(keyFile)
	if err != nil {
		return err
	}

	// Write the CA also in the service directory, so only a directory needs to be mounted
	// for services that need to configure the CA to validate other services certificates.
	err = ca.WriteCertFile(caFile)
	if err != nil {
		return err
	}

	return nil
}

func verifyTLSCertificates(caFile, certFile, keyFile, name string) error {
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
	options := x509.VerifyOptions{
		Roots: certPool,
	}
	if name != "" {
		options.DNSName = name
	}
	_, err = parsed.Verify(options)
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
