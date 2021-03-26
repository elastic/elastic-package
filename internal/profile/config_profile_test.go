// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
package profile

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

const (
	profileName = "test_profile"
)

func TestNewProfile(t *testing.T) {

	elasticPackageDir, err := ioutil.TempDir("", "package")
	if err != nil {
		t.Fatalf("Error getting stack dir: %s", err)
	}
	t.Logf("writing to directory %s", elasticPackageDir)

	err = CreateProfile(elasticPackageDir, profileName, false)
	if err != nil {
		t.Fatalf("error creating profile: %s", err)
	}

}

func TestNewProfileFrom(t *testing.T) {

	elasticPackageDir, err := ioutil.TempDir("", "package")
	if err != nil {
		t.Fatalf("Error getting stack dir: %s", err)
	}
	t.Logf("writing to directory %s", elasticPackageDir)

	err = CreateProfile(elasticPackageDir, profileName, false)
	if err != nil {
		t.Fatalf("error creating profile: %s", err)
	}

	//update the profile to make sure we're properly copying everything

	testProfile, err := NewConfigProfile(elasticPackageDir, profileName)
	if err != nil {
		t.Fatalf("error creating profile %s", err)
	}
	pkgRegUpdated := &SimpleFile{
		FilePath: filepath.Join(testProfile.ProfilePath, string(PackageRegistryConfigFile)),
		FileBody: `package_paths:
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
	if err != nil {
		t.Fatalf("Error updated default profile: %s", err)
	}

	// actually create & check the new profile
	err = CreateProfileFrom(elasticPackageDir, "test_from", profileName)
	if err != nil {
		t.Fatalf("error copying profile: %s", err)
	}

}
