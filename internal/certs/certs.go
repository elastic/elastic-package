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
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Certificate contains the key and certificate for an issued certificate.
type Certificate struct {
	key    crypto.Signer
	cert   *x509.Certificate
	issuer *Certificate
}

// LoadCertificate loads a certificate and key from disk.
func LoadCertificate(certFile, keyFile string) (*Certificate, error) {
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
		return nil, fmt.Errorf("key of type %T cannot be used", privKey)
	}

	cert := &Certificate{
		key:  key,
		cert: chain[0],
	}

	if len(chain) > 1 {
		// This is an intermediate certificate, rebuild the full chain.
		c := cert
		for _, cert := range chain[1:] {
			c.issuer = &Certificate{
				// Parent keys are not known here, but that's ok
				// as these certs are only used for the cert chain.
				cert: cert,
			}
			c = c.issuer
		}
	}
	return cert, nil
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
	cert, err := LoadCertificate(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &Issuer{cert}, nil
}

func newCA(parent *Issuer) (*Issuer, error) {
	cert, err := New(true, false, parent)
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
	return New(false, false, i, opts...)
}

// IssueClient issues a certificate with the given options. This certificate
// can be used to configure a TLS client.
func (i *Issuer) IssueClient(opts ...Option) (*Certificate, error) {
	return New(false, true, i, opts...)
}

// NewSelfSignedCert issues a self-signed certificate with the given options.
// This certificate can be used to configure a TLS server.
func NewSelfSignedCert(opts ...Option) (*Certificate, error) {
	return New(false, false, nil, opts...)
}

// Option is a function that can modify a certificate template. To be used
// when issuing certificates.
type Option func(template *x509.Certificate)

// WithName is an option to configure the common and alternate DNS names of a certificate.
func WithName(name string) Option {
	return func(template *x509.Certificate) {
		template.Subject.CommonName = name
		if !slices.Contains(template.DNSNames, name) {
			template.DNSNames = append(template.DNSNames, name)
		}
	}
}

// New is the main helper to create a certificate, it is recommended to
// use the more specific ones for specific use cases.
func New(isCA, isClient bool, issuer *Issuer, opts ...Option) (*Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	sn, err := newSerialNumber()
	if err != nil {
		return nil, fmt.Errorf("failed to get a unique serial number: %w", err)
	}

	// Don't use a expiration time longer than 825 days.
	// See https://rahulkj.github.io/openssl,/certificates/2022/09/09/self-signed-certificates.html.
	const longTime = 800 * 24 * time.Hour
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
		// If the requester is a client we set clientAuth instead
	} else if isClient {
		template.ExtKeyUsage = []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		}

		// Include local hostname and ips as alternates in service certificates.
		template.DNSNames = []string{"localhost"}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{
			// Required for Chrome in OSX to show the "Proceed anyway" link.
			// https://stackoverflow.com/a/64309893/28855
			x509.ExtKeyUsageServerAuth,
		}

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

// WriteEnv writes the environment variables about the certificate in the given writer.
func (c *Certificate) WriteEnv(w io.Writer) error {
	fingerprint := c.Fingerprint()
	_, err := fmt.Fprintf(w, "%s=%s\n",
		"ELASTIC_PACKAGE_CA_TRUSTED_FINGERPRINT",
		strings.ToUpper(hex.EncodeToString(fingerprint)))
	return err
}

// Fingerprint returns the fingerprint of the certificate. The fingerprint
// of a CA can be used to verify certificates.
func (c *Certificate) Fingerprint() []byte {
	f := sha256.Sum256(c.cert.Raw)
	return f[:]
}

// WriteCertFile writes the PEM-encoded certifiacte in the given file.
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

// Verify verifies a certificate with the given verification options.
func (c *Certificate) Verify(options x509.VerifyOptions) error {
	err := checkExpectedCertUsage(c.cert)
	if err != nil {
		return err
	}
	_, err = c.cert.Verify(options)
	return err
}

func checkExpectedCertUsage(cert *x509.Certificate) error {
	expectedKeyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if cert.IsCA {
		expectedKeyUsage |= x509.KeyUsageCRLSign | x509.KeyUsageCertSign
	}

	if cert.KeyUsage&expectedKeyUsage != expectedKeyUsage {
		return fmt.Errorf("missing expected usage flags in certificate")
	}

	if !cert.IsCA {
		// Required for Chrome in OSX to show the "Proceed anyway" link.
		// https://stackoverflow.com/a/64309893/28855
		if !containsExtKeyUsage(cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth) {
			return fmt.Errorf("missing server auth key usage in certificate")
		}
	}

	return nil
}

func containsExtKeyUsage(us []x509.ExtKeyUsage, u x509.ExtKeyUsage) bool {
	for _, candidate := range us {
		if u == candidate {
			return true
		}
	}
	return false
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
