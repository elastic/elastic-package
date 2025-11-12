// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"path/filepath"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/certs"
)

type tlsService struct {
	Name     string
	IsClient bool
}

// tlsServices is the list of server TLS certificates that will be
// created in the given path.
var tlsServices = []tlsService{
	{Name: "elasticsearch"},
	{Name: "kibana"},
	{Name: "package-registry"},
	{Name: "fleet-server"},
	{Name: "logstash"},
	{Name: "elastic-agent", IsClient: true},
}

// tlsLocalServices is the list of server TLS certificates that will
// be created for local services when the stack is not local.
var tlsLocalServices = []tlsService{
	{Name: "elastic-agent", IsClient: true},
	{Name: "fleet-server"},
	{Name: "logstash"},
}

var (
	// CertificatesDirectory is the path to the certificates directory inside a profile.
	CertificatesDirectory = "certs"

	// CACertificateFile is the path to the CA certificate file inside a profile.
	CACertificateFile = filepath.Join(CertificatesDirectory, "ca-cert.pem")

	// CAKeyFile is the path to the CA key file inside a profile.
	CAKeyFile = filepath.Join(CertificatesDirectory, "ca-key.pem")

	// CAEnvFile is the path to the file with environment variables about the CA.
	CAEnvFile = filepath.Join(CertificatesDirectory, "ca.env")
)

// initTLSCertificates initializes all the certificates needed to run the services
// managed by elastic-package stack. It includes a CA, and a pair of keys and
// certificates for each service.
func initTLSCertificates(fileProvider string, profilePath string, tlsServices []tlsService) ([]resource.Resource, error) {
	certsDir := filepath.Join(profilePath, CertificatesDirectory)
	caCertFile := filepath.Join(profilePath, string(CACertificateFile))
	caKeyFile := filepath.Join(profilePath, string(CAKeyFile))
	envFile := filepath.Join(profilePath, string(CAEnvFile))

	var resources []resource.Resource

	ca, err := initCA(caCertFile, caKeyFile)
	if err != nil {
		return nil, err
	}
	resources, err = certWriteToResource(resources, fileProvider, profilePath, caCertFile, ca.WriteCert)
	if err != nil {
		return nil, err
	}
	resources, err = certWriteToResource(resources, fileProvider, profilePath, caKeyFile, ca.WriteKey)
	if err != nil {
		return nil, err
	}
	resources, err = certWriteToResource(resources, fileProvider, profilePath, envFile, ca.WriteEnv)
	if err != nil {
		return nil, err
	}

	for _, service := range tlsServices {
		certsDir := filepath.Join(certsDir, service.Name)
		caFile := filepath.Join(certsDir, "ca-cert.pem")
		certFile := filepath.Join(certsDir, "cert.pem")
		keyFile := filepath.Join(certsDir, "key.pem")
		cert, err := initServiceTLSCertificates(ca, caCertFile, certFile, keyFile, service)
		if err != nil {
			return nil, err
		}

		resources, err = certWriteToResource(resources, fileProvider, profilePath, certFile, cert.WriteCert)
		if err != nil {
			return nil, err
		}
		resources, err = certWriteToResource(resources, fileProvider, profilePath, keyFile, cert.WriteKey)
		if err != nil {
			return nil, err
		}

		// Write the CA also in the service directory, so only a directory needs to be mounted
		// for services that need to configure the CA to validate other services certificates.
		resources, err = certWriteToResource(resources, fileProvider, profilePath, caFile, ca.WriteCert)
		if err != nil {
			return nil, err
		}
	}

	return resources, nil
}

func certWriteToResource(resources []resource.Resource, fileProvider string, profilePath string, absPath string, write func(w io.Writer) error) ([]resource.Resource, error) {
	path, err := filepath.Rel(profilePath, absPath)
	if err != nil {
		return resources, err
	}

	var buf bytes.Buffer
	err = write(&buf)
	if err != nil {
		return resources, err
	}

	return append(resources, &resource.File{
		Provider:     fileProvider,
		Path:         path,
		CreateParent: true,
		Content:      resource.FileContentLiteral(buf.String()),
	}), nil
}

func initCA(certFile, keyFile string) (*certs.Issuer, error) {
	if err := verifyTLSCertificates(certFile, certFile, keyFile, tlsService{}); err == nil {
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
	return ca, nil
}

func initServiceTLSCertificates(ca *certs.Issuer, caCertFile string, certFile, keyFile string, service tlsService) (*certs.Certificate, error) {
	if err := verifyTLSCertificates(caCertFile, certFile, keyFile, service); err == nil {
		// Certificate already present and valid, load it.
		return certs.LoadCertificate(certFile, keyFile)
	}

	var cert *certs.Certificate
	var err error
	if service.IsClient {
		cert, err = ca.IssueClient(certs.WithName(service.Name))
		if err != nil {
			return nil, fmt.Errorf("error initializing certificate for %q", service.Name)
		}
	} else {
		cert, err = ca.Issue(certs.WithName(service.Name))
		if err != nil {
			return nil, fmt.Errorf("error initializing certificate for %q", service.Name)
		}
	}

	return cert, nil
}

func verifyTLSCertificates(caFile, certFile, keyFile string, service tlsService) error {
	cert, err := certs.LoadCertificate(certFile, keyFile)
	if err != nil {
		return err
	}

	certPool, err := certs.PoolWithCACertificate(caFile)
	if err != nil {
		return err
	}
	options := x509.VerifyOptions{
		Roots: certPool,
	}
	if service.Name != "" {
		options.DNSName = service.Name
	}

	// By default ExtKeyUsageServerAuth is add to KeyUsages
	// See https://github.com/golang/go/blob/master/src/crypto/x509/verify.go#L193-L195
	if service.IsClient {
		options.KeyUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	err = cert.Verify(options)
	if err != nil {
		return err
	}

	return nil
}
