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

func TestSkipUploadPackageValidationGating(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		// Should NOT have the flag (backport not yet landed in these versions)
		{"8.0.0", false},
		{"8.7.0", false},
		{"8.18.0", false},
		{"8.19.0", false},
		{"8.19.21", false},
		{"8.19.21-SNAPSHOT", false},
		{"9.0.0", false},
		{"9.4.0", false},
		{"9.4.6", false},
		{"9.4.6-SNAPSHOT", false},
		{"9.5.0", false},
		{"9.5.2", false},
		// Should have the flag (9.6 main — elastic/kibana#286094 merged here; backport follow-up pending)
		{"9.6.0-SNAPSHOT", true},
		{"9.6.0", true},
		{"9.6.1", true},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("version_%s", tc.version), func(t *testing.T) {
			elasticPackagePath := t.TempDir()
			profilesPath := filepath.Join(elasticPackagePath, "profiles")
			t.Setenv("ELASTIC_PACKAGE_DATA_HOME", elasticPackagePath)

			err := profile.CreateProfile(profile.Options{
				ProfilesDirPath: profilesPath,
				Name:            "test",
			})
			if err != nil {
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

			if err := applyResources(p, appConfig, tc.version, tc.version); err != nil {
				t.Fatalf("applyResources: %v", err)
			}

			d, err := os.ReadFile(p.Path(ProfileStackPath, KibanaConfigFile))
			if err != nil {
				t.Fatalf("read kibana.yml: %v", err)
			}

			got := strings.Contains(string(d), "skipUploadPackageValidation")
			if got != tc.want {
				lines := strings.Split(string(d), "\n")
				tail := lines
				if len(lines) > 20 {
					tail = lines[len(lines)-20:]
				}
				t.Errorf("version %s: skipUploadPackageValidation present=%v, want=%v\n\n--- kibana.yml tail ---\n%s",
					tc.version, got, tc.want, strings.Join(tail, "\n"))
			} else {
				t.Logf("version %s: skipUploadPackageValidation present=%v ✓", tc.version, got)
			}
		})
	}
}
