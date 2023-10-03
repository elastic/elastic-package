// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"bytes"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/internal/packages/status"
)

var generateFlag = flag.Bool("generate", false, "Write golden files")

func fooPackage(version, kibanaVersion string) packages.PackageManifest {
	return packages.PackageManifest{
		Name:        "foo",
		Version:     version,
		Title:       "Foo",
		Description: "Foo integration",
		Owner: packages.Owner{
			Github: "team",
			Type:   "elastic",
		},
		Conditions: packages.Conditions{
			Kibana: packages.KibanaConditions{Version: kibanaVersion},
		},
		Categories: []string{
			"custom",
		},
	}
}

func TestStatusFormatAndPrint(t *testing.T) {
	localPackage := fooPackage("2.0.0-rc1", "^8.9.0")
	localPendingChanges := changelog.Revision{
		Version: "2.0.0-rc2",
		Changes: []changelog.Entry{
			changelog.Entry{
				Description: "New feature",
				Type:        "enhancement",
				Link:        "http:github.com/org/repo/pull/2",
			},
		},
	}

	cases := []struct {
		title           string
		pkgStatus       *status.PackageStatus
		extraParameters []string
		expected        string
	}{
		{
			title:     "no versions",
			pkgStatus: &status.PackageStatus{Name: "foo"},
			expected:  "./testdata/status-no-versions",
		},
		{
			title: "version-one-stage",
			pkgStatus: &status.PackageStatus{
				Name: "foo",
				Production: []packages.PackageManifest{
					fooPackage("1.0.0", "^8.8.0"),
				},
			},
			expected: "./testdata/status-version-one-stage",
		},
		{
			title: "beta versions",
			pkgStatus: &status.PackageStatus{
				Name: "foo",
				Production: []packages.PackageManifest{
					fooPackage("1.0.0", "^8.8.0"),
					fooPackage("1.1.0-beta1", "^8.8.0"),
				},
			},
			expected: "./testdata/status-beta-versions",
		},
		{
			title: "release candidate versions",
			pkgStatus: &status.PackageStatus{
				Name: "foo",
				Production: []packages.PackageManifest{
					fooPackage("1.0.0", "^8.8.0"),
					fooPackage("1.1.0-beta1", "^8.8.0"),
					fooPackage("2.0.0-rc1", "^8.8.0"),
				},
			},
			expected: "./testdata/status-release-candidate-versions",
		},
		{
			title: "preview versions",
			pkgStatus: &status.PackageStatus{
				Name: "foo",
				Production: []packages.PackageManifest{
					fooPackage("0.9.0", "^8.6.0"),
					fooPackage("1.0.0-preview1", "^8.8.0"),
					fooPackage("1.0.0-preview5", "^8.8.0"),
				},
			},
			expected: "./testdata/status-preview-versions",
		},
		{
			title: "local version stage",
			pkgStatus: &status.PackageStatus{
				Name:  "foo",
				Local: &localPackage,
				Production: []packages.PackageManifest{
					fooPackage("1.0.0", "^8.8.0"),
					fooPackage("1.0.1", "^8.8.0"),
					fooPackage("1.0.2", "^8.8.0"),
					fooPackage("1.1.0-beta1", "^8.8.0"),
				},
			},
			expected: "./testdata/status-local-version-stage",
		},
		{
			title: "local pending changes",
			pkgStatus: &status.PackageStatus{
				Name:           "foo",
				Local:          &localPackage,
				PendingChanges: &localPendingChanges,
				Production: []packages.PackageManifest{
					fooPackage("1.0.0", "^8.8.0"),
				},
			},
			expected: "./testdata/status-pending-changes",
		},
		{
			title: "extra parameters",
			pkgStatus: &status.PackageStatus{
				Name:  "foo",
				Local: &localPackage,
				Production: []packages.PackageManifest{
					fooPackage("1.0.0", "^8.8.0"),
				},
			},
			extraParameters: []string{
				"kibana.version",
				"categories",
			},
			expected: "./testdata/status-extra-parameters",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			var buf bytes.Buffer
			err := print(c.pkgStatus, &buf, c.extraParameters)
			require.NoError(t, err)

			assertOutputWithFile(t, c.expected, buf.String())
		})
	}
}

func assertOutputWithFile(t *testing.T, path string, out string) {
	if *generateFlag {
		err := os.WriteFile(path, []byte(out), 0644)
		require.NoError(t, err)
	}

	d, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, string(d), out)
}
