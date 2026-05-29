// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateServiceTLS(t *testing.T) {
	dir := t.TempDir()
	composeYAML := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeYAML, []byte(`services:
  mock_server:
    hostname: guardduty.xxxx.amazonaws.com
    image: mock:latest
  sidecar:
    image: alpine:latest
`), 0o644); err != nil {
		t.Fatal(err)
	}

	tlsDir := filepath.Join(dir, "tls")
	svcInfo := ServiceInfo{Name: "guardduty"}

	if err := generateServiceTLS([]string{composeYAML}, tlsDir, &svcInfo); err != nil {
		t.Fatalf("generateServiceTLS() = %v", err)
	}

	// Verify files were created.
	for _, name := range []string{"ca.crt", "mock_server.crt", "mock_server.key"} {
		path := filepath.Join(tlsDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s: %v", name, err)
		}
	}

	// Verify sidecar (no hostname) did not get certs.
	if _, err := os.Stat(filepath.Join(tlsDir, "sidecar.crt")); err == nil {
		t.Error("sidecar should not have a cert file")
	}

	// Verify CustomProperties contains the CA PEM.
	caPEM, ok := svcInfo.CustomProperties[tlsCAPEMProperty]
	if !ok {
		t.Fatal("CustomProperties missing TLS_CA_PEM")
	}
	caPEMStr := caPEM.(string)
	if !strings.Contains(caPEMStr, "BEGIN CERTIFICATE") {
		t.Errorf("TLS_CA_PEM does not look like PEM: %s", caPEMStr[:60])
	}

	// Verify the cert is valid for both the hostname and the alias.
	certData, err := os.ReadFile(filepath.Join(tlsDir, "mock_server.crt"))
	if err != nil {
		t.Fatal(err)
	}
	keyData, err := os.ReadFile(filepath.Join(tlsDir, "mock_server.key"))
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caPEMStr))

	for _, name := range []string{"guardduty.xxxx.amazonaws.com", "svc-guardduty"} {
		if _, err := leaf.Verify(x509.VerifyOptions{
			DNSName: name,
			Roots:   pool,
		}); err != nil {
			t.Errorf("cert.Verify(%q) = %v", name, err)
		}
	}
}

func TestGenerateServiceTLSNoHostname(t *testing.T) {
	dir := t.TempDir()
	composeYAML := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeYAML, []byte(`services:
  worker:
    image: alpine:latest
`), 0o644); err != nil {
		t.Fatal(err)
	}

	tlsDir := filepath.Join(dir, "tls")
	svcInfo := ServiceInfo{Name: "worker"}

	if err := generateServiceTLS([]string{composeYAML}, tlsDir, &svcInfo); err != nil {
		t.Fatalf("generateServiceTLS() = %v", err)
	}

	// No hostnames means no cert generation at all.
	if _, err := os.Stat(tlsDir); err == nil {
		t.Error("TLS directory should not be created when there are no hostnames")
	}
	if svcInfo.CustomProperties != nil {
		t.Error("CustomProperties should remain nil when there are no hostnames")
	}
}

func TestLoadServiceTLSCA(t *testing.T) {
	dir := t.TempDir()
	caPEM := "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----\n"
	if err := os.WriteFile(filepath.Join(dir, "ca.crt"), []byte(caPEM), 0o644); err != nil {
		t.Fatal(err)
	}

	svcInfo := ServiceInfo{}
	if err := loadServiceTLSCA(dir, &svcInfo); err != nil {
		t.Fatalf("loadServiceTLSCA() = %v", err)
	}

	got, ok := svcInfo.CustomProperties[tlsCAPEMProperty]
	if !ok {
		t.Fatal("CustomProperties missing TLS_CA_PEM")
	}
	if !strings.Contains(got.(string), "BEGIN CERTIFICATE") {
		t.Errorf("loaded CA PEM does not contain expected content")
	}
	if strings.HasSuffix(got.(string), "\n") {
		t.Error("loaded CA PEM should have trailing newline trimmed")
	}
}

func TestLoadServiceTLSCAMissingFile(t *testing.T) {
	dir := t.TempDir()
	svcInfo := ServiceInfo{}
	if err := loadServiceTLSCA(dir, &svcInfo); err != nil {
		t.Fatalf("loadServiceTLSCA() should not error on missing ca.crt: %v", err)
	}
	if svcInfo.CustomProperties != nil {
		t.Error("CustomProperties should remain nil when ca.crt is absent")
	}
}

func TestExtractComposeHostnames(t *testing.T) {
	dir := t.TempDir()
	composeYAML := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composeYAML, []byte(`services:
  api:
    hostname: api.example.com
    image: mock:latest
  worker:
    image: worker:latest
  db:
    hostname: db.internal
    image: postgres:14
`), 0o644); err != nil {
		t.Fatal(err)
	}

	hostnames, err := extractComposeHostnames([]string{composeYAML})
	if err != nil {
		t.Fatalf("extractComposeHostnames() = %v", err)
	}

	if len(hostnames) != 2 {
		t.Fatalf("got %d hostnames, want 2", len(hostnames))
	}
	if hostnames["api"] != "api.example.com" {
		t.Errorf("api hostname = %q, want %q", hostnames["api"], "api.example.com")
	}
	if hostnames["db"] != "db.internal" {
		t.Errorf("db hostname = %q, want %q", hostnames["db"], "db.internal")
	}
}
