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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/common"
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

// LoadCA loads a CA certificate and key from disk.
func LoadCA(certFile, keyFile string) (*Issuer, error) {
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	if len(pair.Certificate) == 0 {
		return nil, fmt.Errorf("no certificates in %q?", certFile)
	}
	chain := make([]*x509.Certificate, len(pair.Certificate))
	for i := range pair.Certificate {
		cert, err := x509.ParseCertificate(pair.Certificate[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse #%d certificate loaded from %q", i, certFile)
		}
		chain[i] = cert
	}

	var key crypto.Signer
	switch privKey := pair.PrivateKey.(type) {
	case crypto.Signer:
		key = privKey
	default:
		return nil, fmt.Errorf("key of type %T cannot be used as CA", privKey)
	}

	ca := &Issuer{
		&Certificate{
			key:  key,
			cert: chain[0],
		},
	}

	if len(chain) > 1 {
		// This is an intermediate certificate, rebuild the full chain.
		c := ca.Certificate
		for _, cert := range chain[1:] {
			c.issuer = &Certificate{
				// Parent keys are not known here, but that's ok
				// as these certs are only used for the cert chain.
				cert: cert,
			}
			c = c.issuer
		}
	}
	return ca, nil
}

func newCA(parent *Issuer) (*Issuer, error) {
	cert, err := New(true, parent)
	if err != nil {
		return nil, err
	}
	return &Issuer{Certificate: cert}, nil
}

// IssueIntermediate issues an intermediate CA signed by the issuer.
func (i *Issuer) IssueIntermediate() (*Issuer, error) {
	return newCA(i)
}

// Issue issues a certificate with the given options. This certificate
// can be used to configure a TLS server.
func (i *Issuer) Issue(opts ...Option) (*Certificate, error) {
	return New(false, i, opts...)
}

// NewSelfSignedCert issues a self-signed certificate with the given options.
// This certificate can be used to configure a TLS server.
func NewSelfSignedCert(opts ...Option) (*Certificate, error) {
	return New(false, nil, opts...)
}

// Option is a function that can modify a certificate template. To be used
// when issuing certificates.
type Option func(template *x509.Certificate)

// WithName is an option to configure the common and alternate DNS names of a certificate.
func WithName(name string) Option {
	return func(template *x509.Certificate) {
		template.Subject.CommonName = name
		if !common.StringSliceContains(template.DNSNames, name) {
			template.DNSNames = append(template.DNSNames, name)
		}
	}
}

// New is the main helper to create a certificate, it is recommended to
// use the more specific ones for specific use cases.
func New(isCA bool, issuer *Issuer, opts ...Option) (*Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	sn, err := newSerialNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to get a unique serial number: %w", err)
	}

	const longTime = 100 * 24 * 365 * time.Hour
	template := x509.Certificate{
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(longTime),

		SerialNumber:          sn,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCRLSign | x509.KeyUsageCertSign

		if issuer == nil {
			template.Subject.CommonName = "elastic-package CA"
		} else {
			template.Subject.CommonName = "intermediate elastic-package CA"
		}
	} else {
		// Include local hostname and ips as alternates in service certificates.
		template.DNSNames = []string{"localhost"}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	}

	for _, opt := range opts {
		opt(&template)
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

func newSerialNumber() (*big.Int, error) {
	// This implementation attempts to get unique serial numbers
	// by getting random ones between 0 and 2^128.
	max := new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil)
	return rand.Int(rand.Reader, max)
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
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory for key file: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create key file %q: %w", path, err)
	}
	defer f.Close()

	return c.WriteKey(f)
}

// WriteCert writes the PEM-encoded certificate chain in the given writer.
func (c *Certificate) WriteCert(w io.Writer) error {
	for i := c; i != nil; i = i.issuer {
		err := encodePem(w, certPemBlock(i.cert.Raw))
		if err != nil {
			return err
		}
	}

	return nil
}

// WriteCertFile writes the PEM-encoded certificate in the given file.
func (c *Certificate) WriteCertFile(path string) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory for certificate file: %w", err)
	}
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
