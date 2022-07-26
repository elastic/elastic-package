// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"bytes"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/status"
)

var generateFlag = flag.Bool("generate", false, "Write golden files")

func fooPackage(version string) packages.PackageManifest {
	return packages.PackageManifest{
		Name:        "foo",
		Version:     version,
		Title:       "Foo",
		Description: "Foo integration",
	}
}

func TestStatusFormatAndPrint(t *testing.T) {
	cases := []struct {
		title     string
		pkgStatus *status.PackageStatus
		expected  string
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
					fooPackage("1.0.0"),
				},
			},
			expected: "./testdata/status-version-one-stage",
		},
		{
			title: "some versions",
			pkgStatus: &status.PackageStatus{
				Name: "foo",
				Production: []packages.PackageManifest{
					fooPackage("1.0.0"),
				},
				Staging: []packages.PackageManifest{
					fooPackage("1.1.0-beta1"),
				},
				Snapshot: []packages.PackageManifest{
					fooPackage("2.0.0-rc1"),
				},
			},
			expected: "./testdata/status-some-versions",
		},
		{
			title: "preview versions",
			pkgStatus: &status.PackageStatus{
				Name: "foo",
				Production: []packages.PackageManifest{
					fooPackage("0.9.0"),
				},
				Staging: []packages.PackageManifest{
					fooPackage("1.0.0-preview1"),
				},
				Snapshot: []packages.PackageManifest{
					fooPackage("1.0.0-preview5"),
				},
			},
			expected: "./testdata/status-preview-versions",
		},
		{
			title: "multiple versions in stage",
			pkgStatus: &status.PackageStatus{
				Name: "foo",
				Production: []packages.PackageManifest{
					fooPackage("1.0.0"),
					fooPackage("1.0.1"),
					fooPackage("1.0.2"),
				},
				Staging: []packages.PackageManifest{
					fooPackage("1.0.0"),
					fooPackage("1.0.1"),
					fooPackage("1.0.2"),
					fooPackage("1.1.0-beta1"),
				},
				Snapshot: []packages.PackageManifest{
					fooPackage("1.0.0"),
					fooPackage("1.0.1"),
					fooPackage("1.0.2"),
					fooPackage("1.1.0-beta1"),
					fooPackage("2.0.0-rc1"),
				},
			},
			expected: "./testdata/status-multiple-versions-in-stage",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			var buf bytes.Buffer
			err := print(c.pkgStatus, &buf)
			require.NoError(t, err)

			assertOutputWithFile(t, c.expected, buf.String())
		})
	}
}

func assertOutputWithFile(t *testing.T, path string, out string) {
	if *generateFlag {
		err := ioutil.WriteFile(path, []byte(out), 0644)
		require.NoError(t, err)
	}

	d, err := ioutil.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, string(d), out)
}
