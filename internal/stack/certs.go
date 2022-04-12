// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"
)

// SelfSignedCert is a self-signed certificate.
type SelfSignedCert struct {
	key  crypto.Signer
	cert []byte
}

func NewSelfSignedCert() (*SelfSignedCert, error) {
	key, err := createPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to create private key: %w", err)
	}

	cert, err := createCertificate(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	return &SelfSignedCert{
		key:  key,
		cert: cert,
	}, nil
}

// WriteKey writes the PEM-encoded key in the given writer.
func (c *SelfSignedCert) WriteKey(w io.Writer) error {
	keyPem, err := keyPemBlock(c.key)
	if err != nil {
		return fmt.Errorf("failed to encode key PEM block: %w", err)
	}

	return encodePem(w, keyPem)
}

// WriteKeyFile writes the PEM-encoded key in the given file.
func (c *SelfSignedCert) WriteKeyFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create key file %q: %w", path, err)
	}
	defer f.Close()

	return c.WriteKey(f)
}

// WriteCert writes the PEM-encoded certificate in the given writer.
func (c *SelfSignedCert) WriteCert(w io.Writer) error {
	certPem := certPemBlock(c.cert)
	return encodePem(w, certPem)
}

// WriteCertFile writes the PEM-encoded certifiacte in the given file.
func (c *SelfSignedCert) WriteCertFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create cert file %q: %w", path, err)
	}
	defer f.Close()

	return c.WriteCert(f)
}

func createPrivateKey() (crypto.Signer, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func createCertificate(key crypto.Signer) ([]byte, error) {
	const about100Years = 100 * 24 * 365 * time.Hour
	template := x509.Certificate{
		Subject:   pkix.Name{CommonName: "elastic-package self-signed"},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(about100Years),

		SerialNumber:          big.NewInt(1),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	// Self-signed.
	parent := template
	return x509.CreateCertificate(rand.Reader, &template, &parent, key.Public(), key)
}

func certPemBlock(cert []byte) *pem.Block {
	const certificatePemType = "CERTIFICATE"
	return &pem.Block{
		Type:  certificatePemType,
		Bytes: cert,
	}
}

func keyPemBlock(key crypto.Signer) (*pem.Block, error) {
	const ecPrivateKeyPemType = "EC PRIVATE KEY"
	switch key := key.(type) {
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
