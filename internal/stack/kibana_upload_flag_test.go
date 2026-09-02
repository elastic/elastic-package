// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
)

// TestSkipUploadPackageValidationGating verifies the version-only gate (no env var).
// Only 9.6.0-SNAPSHOT+ is enabled unconditionally; backport branches require the env var.
func TestSkipUploadPackageValidationGating(t *testing.T) {
	t.Setenv(KibanaSkipUploadPackageValidationEnvVar, "")

	cases := []struct {
		version string
		want    bool
	}{
		// 8.x — never enabled without env var
		{"8.0.0", false},
		{"8.18.0", false},
		{"8.19.0", false},
		{"8.19.9", false},
		{"8.19.10-SNAPSHOT", false},
		{"8.19.21", false},
		// 9.x pre-backport branches — never enabled without env var
		{"9.0.0", false},
		{"9.4.0", false},
		{"9.4.7-SNAPSHOT", false},
		{"9.5.0", false},
		{"9.5.3-SNAPSHOT", false},
		// 9.6.0-SNAPSHOT+ — always enabled (elastic/kibana#286094 merged on main)
		{"9.6.0-SNAPSHOT", true},
		{"9.6.0", true},
		{"9.6.1", true},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("version_%s", tc.version), func(t *testing.T) {
			got := renderAndCheck(t, tc.version)
			if got != tc.want {
				t.Errorf("version %s: skipUploadPackageValidation present=%v, want=%v", tc.version, got, tc.want)
			} else {
				t.Logf("version %s: skipUploadPackageValidation present=%v ✓", tc.version, got)
			}
		})
	}
}

// TestSkipUploadPackageValidationBackportEnvVar verifies that setting
// ELASTIC_PACKAGE_KIBANA_SKIP_UPLOAD_PACKAGE_VALIDATION=true enables the flag
// for backport patches at or above the known minimum, and not for patches below it.
func TestSkipUploadPackageValidationBackportEnvVar(t *testing.T) {
	t.Setenv(KibanaSkipUploadPackageValidationEnvVar, "true")

	cases := []struct {
		version string
		want    bool
	}{
		// Entirely outside the backport branches — env var has no effect
		{"8.18.0", false},
		{"9.0.0", false},
		{"9.0.0-SNAPSHOT", false},
		{"9.3.0", false},
		// 8.19.x: already-released patches (backport present but env var not needed for past releases)
		{"8.19.0", false},
		{"8.19.9", false},
		{"8.19.10-SNAPSHOT", false},
		{"8.19.10", false},
		{"8.19.21", false},
		// 8.19.x: current and future snapshots — env var activates the flag
		{"8.19.22-SNAPSHOT", true},
		// 9.4.x: patches before the backport (elastic/kibana#287671)
		{"9.4.0", false},
		{"9.4.6", false},
		{"9.4.6-SNAPSHOT", false},
		// 9.4.x: backport patch and later
		{"9.4.7-SNAPSHOT", true},
		{"9.4.7", true},
		// 9.5.x: patches before the backport (elastic/kibana#287672)
		{"9.5.0", false},
		{"9.5.0-SNAPSHOT", false},
		{"9.5.2", false},
		{"9.5.2-SNAPSHOT", false},
		// 9.5.x: backport patch and later
		{"9.5.3-SNAPSHOT", true},
		{"9.5.3", true},
		// 9.6.0-SNAPSHOT+ enabled by version gate regardless of env var
		{"9.6.0-SNAPSHOT", true},
		{"9.6.0", true},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("version_%s", tc.version), func(t *testing.T) {
			got := renderAndCheck(t, tc.version)
			if got != tc.want {
				t.Errorf("version %s (env=true): skipUploadPackageValidation present=%v, want=%v", tc.version, got, tc.want)
			} else {
				t.Logf("version %s (env=true): skipUploadPackageValidation present=%v ✓", tc.version, got)
			}
		})
	}
}

func renderAndCheck(t *testing.T, version string) bool {
	t.Helper()
	elasticPackagePath := t.TempDir()
	profilesPath := filepath.Join(elasticPackagePath, "profiles")
	t.Setenv("ELASTIC_PACKAGE_DATA_HOME", elasticPackagePath)

	if err := profile.CreateProfile(profile.Options{ProfilesDirPath: profilesPath, Name: "test"}); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	p, err := profile.LoadProfile("test")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	appConfig, err := install.Configuration()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := applyResources(p, appConfig, version, version); err != nil {
		t.Fatalf("applyResources: %v", err)
	}
	d, err := os.ReadFile(p.Path(ProfileStackPath, KibanaConfigFile))
	if err != nil {
		t.Fatalf("read kibana.yml: %v", err)
	}
	return strings.Contains(string(d), "skipUploadPackageValidation")
}
