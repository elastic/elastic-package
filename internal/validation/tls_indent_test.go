// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckTLSIndentInFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name: "correct_indent",
			content: `data_stream:
  vars:
    ssl: |-
      certificate: |
        {{{tls_cert "elastic-agent" indent=8}}}
      key: |
        {{{tls_key "elastic-agent" indent=8}}}
      verification_mode: none`,
			wantErr: false,
		},
		{
			name: "wrong_indent_cert",
			content: `data_stream:
  vars:
    ssl: |-
      certificate: |
        {{{tls_cert "elastic-agent" indent=4}}}`,
			wantErr: true,
			errMsg:  "indent=4 but tag starts at column 8",
		},
		{
			name: "wrong_indent_key",
			content: `    ssl: |-
      key: |
        {{{tls_key "elastic-agent" indent=12}}}`,
			wantErr: true,
			errMsg:  "indent=12 but tag starts at column 8",
		},
		{
			name: "no_indent_parameter",
			content: `    ssl: |-
      certificate: |
        {{{tls_cert "elastic-agent"}}}`,
			wantErr: false,
		},
		{
			name: "no_tls_helpers",
			content: `input: http_endpoint
data_stream:
  vars:
    listen_port: 8080`,
			wantErr: false,
		},
		{
			name: "static_pem_no_helpers",
			content: `    ssl: |-
      certificate: |
        -----BEGIN CERTIFICATE-----
        MIIDJj...
        -----END CERTIFICATE-----`,
			wantErr: false,
		},
		{
			name:    "indent_zero_at_column_zero",
			content: `{{{tls_cert "localhost" indent=0}}}`,
			wantErr: false,
		},
		{
			name:    "custom_domain",
			content: `        {{{tls_cert "securityhub.xxxx.amazonaws.cn" indent=8}}}`,
			wantErr: false,
		},
		{
			name:    "custom_domain_wrong_indent",
			content: `    {{{tls_cert "securityhub.xxxx.amazonaws.cn" indent=8}}}`,
			wantErr: true,
			errMsg:  "indent=8 but tag starts at column 4",
		},
		{
			name: "tls_ca_correct_indent",
			content: `    certificate_authorities:
      - |
        {{{tls_ca indent=8}}}`,
			wantErr: false,
		},
		{
			name: "tls_ca_wrong_indent",
			content: `    certificate_authorities:
      - |
        {{{tls_ca indent=4}}}`,
			wantErr: true,
			errMsg:  "indent=4 but tag starts at column 8",
		},
		{
			name:    "tls_ca_no_indent",
			content: `{{{tls_ca}}}`,
			wantErr: false,
		},
		{
			name:    "tls_ca_indent_zero_at_column_zero",
			content: `{{{tls_ca indent=0}}}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test-foo-config.yml")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			errs, err := checkTLSIndentInFile(path, dir)
			if err != nil {
				t.Fatalf("checkTLSIndentInFile() returned error: %v", err)
			}

			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatal("expected errors but got none")
				}
				joined := strings.Join(errs, "\n")
				if !strings.Contains(joined, tt.errMsg) {
					t.Errorf("error = %q; want substring %q", joined, tt.errMsg)
				}
			} else {
				if len(errs) > 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
			}
		})
	}
}

func TestValidateTLSHelperIndent_realFixtures(t *testing.T) {
	fixtures := []string{
		"../../test/packages/parallel/auth0_logsdb",
		"../../test/packages/parallel/ti_anomali_logsdb",
		"../../test/packages/parallel/ti_anomali",
		"../../test/packages/parallel/mock_service_tls",
	}
	for _, pkg := range fixtures {
		name := filepath.Base(pkg)
		t.Run(name, func(t *testing.T) {
			if _, err := os.Stat(pkg); err != nil {
				t.Skipf("fixture not found: %s", pkg)
			}
			if err := ValidateTLSHelperIndent(pkg); err != nil {
				t.Errorf("ValidateTLSHelperIndent(%s) = %v; want nil", name, err)
			}
		})
	}
}
