// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
package profile

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	profileName = "test_profile"
)

func TestNewProfile(t *testing.T) {
	elasticPackageDir, err := ioutil.TempDir("", "package")
	defer cleanupProfile(t, elasticPackageDir)
	assert.NoError(t, err, "error creating tempdir")
	t.Logf("writing to directory %s", elasticPackageDir)

	options := Options{
		PackagePath:       elasticPackageDir,
		Name:              profileName,
		OverwriteExisting: false,
	}
	err = createProfile(options)
	assert.NoErrorf(t, err, "error creating profile %s", err)

}

func TestNewProfileFrom(t *testing.T) {
	elasticPackageDir, err := ioutil.TempDir("", "package")
	defer cleanupProfile(t, elasticPackageDir)
	assert.NoError(t, err, "error creating tempdir")
	t.Logf("writing to directory %s", elasticPackageDir)

	options := Options{
		PackagePath:       elasticPackageDir,
		Name:              profileName,
		OverwriteExisting: false,
	}
	err = createProfile(options)
	assert.NoErrorf(t, err, "error creating profile %s", err)

	//update the profile to make sure we're properly copying everything

	testProfile, err := NewConfigProfile(elasticPackageDir, profileName)
	assert.NoErrorf(t, err, "error creating profile %s", err)

	pkgRegUpdated := &simpleFile{
		path: filepath.Join(testProfile.ProfilePath, string(PackageRegistryConfigFile)),
		body: `package_paths:
		- /packages/testing
		- /packages/development
		- /packages/production
		- /packages/staging
		- /packages/snapshot
	  `,
	}
	t.Logf("updating profile %s", testProfile.ProfilePath)
	testProfile.configFiles[PackageRegistryConfigFile] = pkgRegUpdated
	err = testProfile.writeProfileResources()
	assert.NoErrorf(t, err, "error updating profile %s", err)

	// actually create & check the new profile
	option := Options{
		PackagePath: elasticPackageDir,
		Name:        "test_from",
		FromProfile: profileName,
	}
	err = createProfileFrom(option)
	assert.NoErrorf(t, err, "error copying updating profile %s", err)

}

func cleanupProfile(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	assert.NoErrorf(t, err, "Error cleaning up tempdir %s", dir)
}
