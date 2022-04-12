// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
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

	cert, err := NewSelfSignedCert()
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
		Subject: pkix.Name{
			// TODO: Parameterize this.
			CommonName: "elasticsearch",
		},
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
