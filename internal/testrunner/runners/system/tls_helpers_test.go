// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"crypto/tls"
	"strings"
	"testing"

	"github.com/aymerick/raymond"

	"github.com/elastic/elastic-package/internal/servicedeployer"
)

func TestTLSHelpersBasicCertAndKey(t *testing.T) {
	result := execTLSTemplate(t, `{{{tls_cert "localhost"}}}
SEPARATOR
{{{tls_key "localhost"}}}`)

	parts := strings.SplitN(result, "SEPARATOR", 2)
	if len(parts) != 2 {
		t.Fatal("expected two sections separated by SEPARATOR")
	}
	certPEM, keyPEM := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

	for _, want := range []string{"-----BEGIN CERTIFICATE-----", "-----END CERTIFICATE-----"} {
		if !strings.Contains(certPEM, want) {
			t.Errorf("cert PEM missing %q", want)
		}
	}
	for _, want := range []string{"-----BEGIN EC PRIVATE KEY-----", "-----END EC PRIVATE KEY-----"} {
		if !strings.Contains(keyPEM, want) {
			t.Errorf("key PEM missing %q", want)
		}
	}

	// Verify the cert and key form a valid TLS pair.
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		t.Errorf("tls.X509KeyPair() = %v; want nil error", err)
	}
}

func TestTLSHelpersSameDomainConsistentPair(t *testing.T) {
	result := execTLSTemplate(t, "{{{tls_cert \"foo\"}}}\nSEPARATOR\n{{{tls_cert \"foo\"}}}")

	parts := strings.SplitN(result, "SEPARATOR", 2)
	if len(parts) != 2 {
		t.Fatal("expected two sections separated by SEPARATOR")
	}
	a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if a != b {
		t.Errorf("same domain produced different certs:\ngot[0]:\n%s\ngot[1]:\n%s", a, b)
	}
}

func TestTLSHelpersDifferentDomainsDifferentPairs(t *testing.T) {
	result := execTLSTemplate(t, "{{{tls_cert \"alpha\"}}}\nSEPARATOR\n{{{tls_cert \"beta\"}}}")

	parts := strings.SplitN(result, "SEPARATOR", 2)
	if len(parts) != 2 {
		t.Fatal("expected two sections separated by SEPARATOR")
	}
	a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if a == b {
		t.Error("different domains produced identical certs; want different")
	}
}

func TestTLSHelpersIndent(t *testing.T) {
	result := execTLSTemplate(t, `        {{{tls_cert "localhost" indent=8}}}`)

	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Fatalf("got %d lines; want at least 3 for multi-line PEM", len(lines))
	}
	for i, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "        ") {
			t.Errorf("line %d = %q; want 8-space prefix", i, line)
		}
	}
}

func TestTLSHelpersYAMLStructure(t *testing.T) {
	result := execTLSTemplate(t, `data_stream:
  vars:
    ssl: |-
      certificate: |
        {{{tls_cert "elastic-agent" indent=8}}}
      key: |
        {{{tls_key "elastic-agent" indent=8}}}
      verification_mode: none`)

	for _, want := range []string{"certificate: |", "verification_mode: none"} {
		if !strings.Contains(result, want) {
			t.Errorf("result missing %q", want)
		}
	}
	for i, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "BEGIN") || strings.Contains(line, "END") {
			if !strings.HasPrefix(line, "        ") {
				t.Errorf("line %d = %q; want 8-space prefix for PEM content", i, line)
			}
		}
	}
}

func TestTLSHelpersApplyServiceInfo(t *testing.T) {
	input := []byte(`    ssl: |-
      certificate: |
        {{{tls_cert "elastic-agent" indent=8}}}
      key: |
        {{{tls_key "elastic-agent" indent=8}}}
      verification_mode: none`)

	result, err := applyServiceInfo(input, servicedeployer.ServiceInfo{})
	if err != nil {
		t.Fatalf("applyServiceInfo() = %v; want nil error", err)
	}

	output := string(result)
	for _, want := range []string{
		"-----BEGIN CERTIFICATE-----",
		"-----BEGIN EC PRIVATE KEY-----",
		"verification_mode: none",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("applyServiceInfo result missing %q", want)
		}
	}
}

func execTLSTemplate(t *testing.T, text string) string {
	t.Helper()
	helpers := tlsHelpers()
	tmpl, err := raymond.Parse(text)
	if err != nil {
		t.Fatalf("raymond.Parse() = %v; want nil error", err)
	}
	tmpl.RegisterHelpers(helpers)
	result, err := tmpl.Exec(nil)
	if err != nil {
		t.Fatalf("tmpl.Exec() = %v; want nil error", err)
	}
	return result
}
