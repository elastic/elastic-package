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

// TestSkipUploadPackageValidationGating verifies which Kibana versions get
// xpack.fleet.internal.skipUploadPackageValidation automatically.
// Enabled unconditionally for 9.6.0-SNAPSHOT+ and for backport patches
// 8.19.22+, 9.4.7+, 9.5.4+.
func TestSkipUploadPackageValidationGating(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		// 8.x — disabled below the backport minimum
		{"8.0.0", false},
		{"8.18.0", false},
		{"8.19.0", false},
		{"8.19.9", false},
		{"8.19.10-SNAPSHOT", false},
		{"8.19.10", false},
		{"8.19.21", false},
		// 8.19.x backport (elastic/kibana#287670): enabled from 8.19.22-SNAPSHOT
		{"8.19.22-SNAPSHOT", true},
		// 9.x — disabled outside the backport branches
		{"9.0.0", false},
		{"9.0.0-SNAPSHOT", false},
		{"9.3.0", false},
		// 9.4.x before backport (elastic/kibana#287671)
		{"9.4.0", false},
		{"9.4.6", false},
		{"9.4.6-SNAPSHOT", false},
		// 9.4.x backport: enabled from 9.4.7-SNAPSHOT
		{"9.4.7-SNAPSHOT", true},
		{"9.4.7", true},
		// 9.5.x before backport (elastic/kibana#287672)
		{"9.5.0", false},
		{"9.5.0-SNAPSHOT", false},
		{"9.5.2", false},
		{"9.5.2-SNAPSHOT", false},
		{"9.5.3-SNAPSHOT", false},
		{"9.5.3", false},
		// 9.5.x backport: enabled from 9.5.4-SNAPSHOT
		{"9.5.4-SNAPSHOT", true},
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
