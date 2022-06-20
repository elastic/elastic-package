// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package certs

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
)

// SystemPoolWithCACertificate returns a copy of the system pool, including the CA certificate
// in the given path.
func SystemPoolWithCACertificate(path string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("initializing root certificate pool: %w", err)
	}
	err = addCACertificateToPool(pool, path)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func addCACertificateToPool(pool *x509.CertPool, path string) error {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read certificate in %q: %w", path, err)
	}

	cert, _ := pem.Decode(d)
	if cert == nil || cert.Type != "CERTIFICATE" {
		return fmt.Errorf("no certificate found in %q", path)
	}

	ca, err := x509.ParseCertificate(cert.Bytes)
	if err != nil {
		return fmt.Errorf("parsing certificate found in %q: %w", path, err)
	}

	pool.AddCert(ca)

	return nil
}
