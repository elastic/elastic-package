// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package certs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"
)

// Certificate contains the key and certificate for an issued certificate.
type Certificate struct {
	key    crypto.Signer
	cert   *x509.Certificate
	issuer *Certificate
}

// Issuer is a certificate that can issue other certificates.
type Issuer struct {
	*Certificate
}

// NewCA creates a new self-signed root CA.
func NewCA() (*Issuer, error) {
	return newCA(nil)
}

func newCA(parent *Issuer) (*Issuer, error) {
	cert, err := New(true, parent)
	if err != nil {
		return nil, err
	}
	return &Issuer{Certificate: cert}, nil
}

func (i *Issuer) IssueIntermediate() (*Issuer, error) {
	return newCA(i)
}

func (i *Issuer) Issue() (*Certificate, error) {
	return New(false, i)
}

func NewSelfSignedCert() (*Certificate, error) {
	return New(false, nil)
}

func New(isCA bool, issuer *Issuer) (*Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	const longTime = 100 * 24 * 365 * time.Hour
	template := x509.Certificate{
		Subject: pkix.Name{
			// TODO: Parameterize common names.
			CommonName: "elasticsearch",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(longTime),

		// TODO: Generate different serials?
		SerialNumber:          big.NewInt(1),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,

		// TODO: Parameterize this.
		DNSNames: []string{"elasticsearch"},
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCRLSign | x509.KeyUsageCertSign

		if issuer == nil {
			template.Subject.CommonName = "elastic-package CA"
		} else {
			template.Subject.CommonName = "intermediate elastic-package CA"
		}
	}

	// Self-signed unless an issuer has been received.
	var parent *x509.Certificate = &template
	var signer crypto.Signer = key
	var issuerCert *Certificate
	if issuer != nil {
		parent = issuer.cert
		signer = issuer.key
		issuerCert = issuer.Certificate
		template.Issuer = issuer.cert.Subject
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, parent, key.Public(), signer)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &Certificate{
		key:    key,
		cert:   cert,
		issuer: issuerCert,
	}, nil
}

// WriteKey writes the PEM-encoded key in the given writer.
func (c *Certificate) WriteKey(w io.Writer) error {
	keyPem, err := keyPemBlock(c.key)
	if err != nil {
		return fmt.Errorf("failed to encode key PEM block: %w", err)
	}

	return encodePem(w, keyPem)
}

// WriteKeyFile writes the PEM-encoded key in the given file.
func (c *Certificate) WriteKeyFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create key file %q: %w", path, err)
	}
	defer f.Close()

	return c.WriteKey(f)
}

// WriteCert writes the PEM-encoded certificate in the given writer.
func (c *Certificate) WriteCert(w io.Writer) error {
	for cert := c; cert != nil; cert = cert.issuer {
		err := encodePem(w, certPemBlock(cert.cert.Raw))
		if err != nil {
			return err
		}
	}

	return nil
}

// WriteCertFile writes the PEM-encoded certifiacte in the given file.
func (c *Certificate) WriteCertFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create cert file %q: %w", path, err)
	}
	defer f.Close()

	return c.WriteCert(f)
}

func certPemBlock(cert []byte) *pem.Block {
	const certificatePemType = "CERTIFICATE"
	return &pem.Block{
		Type:  certificatePemType,
		Bytes: cert,
	}
}

func keyPemBlock(key crypto.Signer) (*pem.Block, error) {
	const (
		ecPrivateKeyPemType  = "EC PRIVATE KEY"
		rsaPrivateKeyPemType = "RSA PRIVATE KEY"
	)
	switch key := key.(type) {
	case *rsa.PrivateKey:
		d := x509.MarshalPKCS1PrivateKey(key)
		return &pem.Block{
			Type:  rsaPrivateKeyPemType,
			Bytes: d,
		}, nil
	case *ecdsa.PrivateKey:
		d, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to encode EC private key: %w", err)
		}
		return &pem.Block{
			Type:  ecPrivateKeyPemType,
			Bytes: d,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported key type %T", key)
	}
}

func encodePem(w io.Writer, blocks ...*pem.Block) error {
	for _, block := range blocks {
		err := pem.Encode(w, block)
		if err != nil {
			return err
		}
	}
	return nil
}
