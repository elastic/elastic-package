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

func SystemPoolWithCACertificate(path string) (*x509.CertPool, error) {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cert, _ := pem.Decode(d)
	if cert == nil || cert.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("no certificate found in %q", path)
	}

	ca, err := x509.ParseCertificate(cert.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing certificate found in %q: %w", path, err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("initializing root certificate pool: %w", err)
	}
	pool.AddCert(ca)

	return pool, nil
}
