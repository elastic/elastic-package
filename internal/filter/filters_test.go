// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/packages"
)

func loadTestPackages(t *testing.T) []packages.PackageDirNameAndManifest {
	t.Helper()
	testPackagesPath, err := filepath.Abs("../../test/packages")
	require.NoError(t, err)

	pkgs, err := packages.ReadAllPackageManifestsFromRepo(testPackagesPath, cobraext.FilterDepthFlagDefault, "")
	require.NoError(t, err)
	require.NotEmpty(t, pkgs, "no packages found in test/packages")

	return pkgs
}

func parseFlag(t *testing.T, f Filter, flagName, value string) {
	t.Helper()
	cmd := &cobra.Command{}
	f.Register(cmd)
	err := cmd.Flags().Set(flagName, value)
	require.NoError(t, err)
	err = f.Parse(cmd)
	require.NoError(t, err)
}

func TestCategoryFlag_Matches(t *testing.T) {
	pkgs := loadTestPackages(t)

	// Find a real category
	var realCategory string
	if len(pkgs[0].Manifest.Categories) > 0 {
		realCategory = pkgs[0].Manifest.Categories[0]
	} else {
		realCategory = "security" // Fallback
	}

	tests := []struct {
		name       string
		categories []string
		wantMatch  bool
	}{
		{"match existing", []string{realCategory}, true},
		{"no match random", []string{"random_category_xyz"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := initCategoryFlag()
			parseFlag(t, f, cobraext.FilterCategoriesFlagName, strings.Join(tt.categories, ","))

			matched := false
			for _, pkg := range pkgs {
				if f.Matches(pkg.DirName, pkg.Manifest) {
					matched = true
					break
				}
			}
			assert.Equal(t, tt.wantMatch, matched, "Category match expectation failed for %v", tt.categories)
		})
	}
}

func TestInputFlag_Matches(t *testing.T) {
	pkgs := loadTestPackages(t)

	// Find a real input (simplified extraction)
	realInput := "logfile" // Common input, fallback
	for _, pkg := range pkgs {
		for _, pt := range pkg.Manifest.PolicyTemplates {
			if pt.Input != "" {
				realInput = pt.Input
				break
			}
			for _, inp := range pt.Inputs {
				if inp.Type != "" {
					realInput = inp.Type
					break
				}
			}
		}
	}

	tests := []struct {
		name      string
		inputs    []string
		wantMatch bool
	}{
		{"match existing", []string{realInput}, true},
		{"no match random", []string{"random_input_xyz"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := initInputFlag()
			parseFlag(t, f, cobraext.FilterInputFlagName, strings.Join(tt.inputs, ","))

			matched := false
			for _, pkg := range pkgs {
				if f.Matches(pkg.DirName, pkg.Manifest) {
					matched = true
					break
				}
			}
			assert.Equal(t, tt.wantMatch, matched, "Input match expectation failed for %v", tt.inputs)
		})
	}
}

func TestCodeOwnerFlag_Matches(t *testing.T) {
	pkgs := loadTestPackages(t)

	realOwner := pkgs[0].Manifest.Owner.Github

	tests := []struct {
		name      string
		owner     []string
		wantMatch bool
	}{
		{"match existing", []string{realOwner}, true},
		{"no match random", []string{"random_owner_xyz"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := initCodeOwnerFlag()
			parseFlag(t, f, cobraext.FilterCodeOwnerFlagName, strings.Join(tt.owner, ","))

			matched := false
			for _, pkg := range pkgs {
				if f.Matches(pkg.DirName, pkg.Manifest) {
					matched = true
					break
				}
			}
			assert.Equal(t, tt.wantMatch, matched, "Owner match expectation failed for %v", tt.owner)
		})
	}
}

func TestPackageDirNameFlag_Matches(t *testing.T) {
	pkgs := loadTestPackages(t)
	realPkg := pkgs[0]

	tests := []struct {
		name      string
		dirNames  []string
		wantMatch bool
	}{
		{"match existing", []string{realPkg.DirName}, true},
		{"no match random", []string{"random_dirname_xyz"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := initPackageDirNameFlag()
			parseFlag(t, f, cobraext.FilterPackageDirNameFlagName, strings.Join(tt.dirNames, ","))

			matched := false
			for _, pkg := range pkgs {
				if f.Matches(pkg.DirName, pkg.Manifest) {
					matched = true
					break
				}
			}
			assert.Equal(t, tt.wantMatch, matched, "DirName match expectation failed for %v", tt.dirNames)
		})
	}
}

func TestPackageNameFlag_Matches(t *testing.T) {
	pkgs := loadTestPackages(t)
	realPkg := pkgs[0]

	tests := []struct {
		name      string
		pkgNames  []string
		wantMatch bool
	}{
		{"match existing", []string{realPkg.Manifest.Name}, true},
		{"no match random", []string{"random_pkgname_xyz"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := initPackageNameFlag()
			parseFlag(t, f, cobraext.FilterPackagesFlagName, strings.Join(tt.pkgNames, ","))

			matched := false
			for _, pkg := range pkgs {
				if f.Matches(pkg.DirName, pkg.Manifest) {
					matched = true
					break
				}
			}
			assert.Equal(t, tt.wantMatch, matched, "PackageName match expectation failed for %v", tt.pkgNames)
		})
	}
}

func TestPackageTypeFlag_Matches(t *testing.T) {
	pkgs := loadTestPackages(t)
	realType := pkgs[0].Manifest.Type

	tests := []struct {
		name      string
		pkgTypes  []string
		wantMatch bool
	}{
		{"match existing", []string{realType}, true},
		{"no match non existing", []string{"non_existing_type"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := initPackageTypeFlag()
			parseFlag(t, f, cobraext.FilterPackageTypeFlagName, strings.Join(tt.pkgTypes, ","))

			matched := false
			for _, pkg := range pkgs {
				if f.Matches(pkg.DirName, pkg.Manifest) {
					matched = true
					break
				}
			}
			assert.Equal(t, tt.wantMatch, matched, "PackageType match expectation failed for %v", tt.pkgTypes)
		})
	}
}

func TestSpecVersionFlag_Matches(t *testing.T) {
	pkgs := []packages.PackageDirNameAndManifest{
		{
			DirName: "test-pkg-1",
			Manifest: &packages.PackageManifest{
				SpecVersion: "3.2.1",
			},
		},
		{
			DirName: "test-pkg-2",
			Manifest: &packages.PackageManifest{
				SpecVersion: "3.2.2",
			},
		},
	}

	tests := []struct {
		name         string
		specVersions []string
		wantMatch    bool
	}{
		{"match existing", []string{"3.2.1"}, true},
		{"no match random", []string{"1.1.0"}, false},
		{"match operator", []string{">= 3.0.0"}, true},
		{"no match operator", []string{"< 3.0.0"}, false},
		{"match multiple", []string{">= 3.0.0", "<= 3.2.2"}, true},
		{"no match multiple", []string{"<= 3.0.0", "> 3.2.2"}, false},
		{"match minor", []string{"3.x"}, true},
		{"no match minor", []string{"3.1.x"}, false},
		{"match major", []string{"3.x"}, true},
		{"no match major", []string{"2.x"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := initSpecVersionFlag()
			parseFlag(t, f, cobraext.FilterSpecVersionFlagName, strings.Join(tt.specVersions, ","))

			matched := false
			for _, pkg := range pkgs {
				if f.Matches(pkg.DirName, pkg.Manifest) {
					matched = true
					break
				}
			}
			assert.Equal(t, tt.wantMatch, matched, "SpecVersion match expectation failed for %v", tt.specVersions)
		})
	}
}
