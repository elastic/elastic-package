// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package certs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/github/smimesign/fakeca"
)

// Certificate is a self-signed certificate.
type Certificate struct {
	identity *fakeca.Identity
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
	const longTime = 100 * 24 * 365 * time.Hour
	options := []fakeca.Option{
		fakeca.Subject(pkix.Name{
			// TODO: Parameterize this.
			CommonName: "elasticsearch",
		}),
		fakeca.NotBefore(time.Now()),
		fakeca.NotAfter(time.Now().Add(longTime)),
		fakeca.KeyUsage(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
	}
	if isCA {
		options = append(options, fakeca.IsCA)
	}
	if issuer != nil {
		options = append(options, fakeca.Issuer(issuer.identity))
	}
	identity := fakeca.New(options...)

	return &Certificate{
		identity: identity,
	}, nil
}

// WriteKey writes the PEM-encoded key in the given writer.
func (c *Certificate) WriteKey(w io.Writer) error {
	keyPem, err := keyPemBlock(c.identity.PrivateKey)
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
	certPem := certPemBlock(c.identity.Certificate.Raw)
	return encodePem(w, certPem)
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
