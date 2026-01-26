// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/profile"
)

func TestApplyResourcesWithCustomGeoipDir(t *testing.T) {
	const expectedGeoipPath = "/some/path/ingest-geoip"
	const profileName = "custom_geoip"

	elasticPackagePath := t.TempDir()
	profilesPath := filepath.Join(elasticPackagePath, "profiles")

	t.Setenv("ELASTIC_PACKAGE_DATA_HOME", elasticPackagePath)

	// Create profile.
	err := profile.CreateProfile(profile.Options{
		ProfilesDirPath: profilesPath,
		Name:            profileName,
	})
	require.NoError(t, err)

	// Write configuration to the profile.
	configPath := filepath.Join(profilesPath, profileName, profile.PackageProfileConfigFile)
	config := fmt.Sprintf("stack.geoip_dir: %q", expectedGeoipPath)
	err = os.WriteFile(configPath, []byte(config), 0644)
	require.NoError(t, err)

	p, err := profile.LoadProfile(profileName)
	require.NoError(t, err)
	t.Logf("Profile name: %s, path: %s", p.ProfileName, p.ProfilePath)

	// Smoke test to check that we are actually loading the profile we want and it has the setting.
	v := p.Config("stack.geoip_dir", "")
	require.Equal(t, expectedGeoipPath, v)

	// Now, apply resources and check that the variable has been used.
	stackVersion := "8.6.1"
	err = applyResources(p, stackVersion, stackVersion)
	require.NoError(t, err)

	d, err := os.ReadFile(p.Path(ProfileStackPath, ComposeFile))
	require.NoError(t, err)

	var composeFile struct {
		Services struct {
			Elasticsearch struct {
				Volumes []string `yaml:"volumes"`
			} `yaml:"elasticsearch"`
		} `yaml:"services"`
	}
	err = yaml.Unmarshal(d, &composeFile)
	require.NoError(t, err)

	volumes := composeFile.Services.Elasticsearch.Volumes
	expectedVolume := fmt.Sprintf("%s:/usr/share/elasticsearch/config/ingest-geoip", expectedGeoipPath)
	assert.Contains(t, volumes, expectedVolume)
}

func TestApplyResourcesWithDefaultPackageRegistryURL(t *testing.T) {
	const profileName = "custom_package_registry"
	const expectedEPR = "https://epr.elastic.co"

	elasticPackagePath := t.TempDir()
	profilesPath := filepath.Join(elasticPackagePath, "profiles")

	t.Setenv("ELASTIC_PACKAGE_DATA_HOME", elasticPackagePath)

	// Create profile.
	err := profile.CreateProfile(profile.Options{
		ProfilesDirPath: profilesPath,
		Name:            profileName,
	})
	require.NoError(t, err)

	p, err := profile.LoadProfile(profileName)
	require.NoError(t, err)
	t.Logf("Profile name: %s, path: %s", p.ProfileName, p.ProfilePath)

	// Ensure that the profile has no specific configuration for EPR proxy.
	v := p.Config(configElasticEPRProxyTo, "")
	require.Equal(t, "", v)

	// Now, apply resources and check that the variable has been used.
	stackVersion := "8.6.1"
	err = applyResources(p, stackVersion, stackVersion)
	require.NoError(t, err)

	d, err := os.ReadFile(p.Path(ProfileStackPath, DockerfilePackageRegistryFile))
	require.NoError(t, err)

	assert.Contains(t, string(d), fmt.Sprintf("ENV EPR_PROXY_TO=%s", expectedEPR))
}

func TestApplyResourcesWithCustomPackageRegistryURL(t *testing.T) {
	const profileName = "custom_package_registry"
	const expectedEPR = "http://localhost"

	elasticPackagePath := t.TempDir()
	profilesPath := filepath.Join(elasticPackagePath, "profiles")

	t.Setenv("ELASTIC_PACKAGE_DATA_HOME", elasticPackagePath)

	// Create profile.
	err := profile.CreateProfile(profile.Options{
		ProfilesDirPath: profilesPath,
		Name:            profileName,
	})
	require.NoError(t, err)

	// Write configuration to the profile.
	configPath := filepath.Join(profilesPath, profileName, profile.PackageProfileConfigFile)
	config := fmt.Sprintf("%s: %q", configElasticEPRProxyTo, expectedEPR)
	err = os.WriteFile(configPath, []byte(config), 0644)
	require.NoError(t, err)

	p, err := profile.LoadProfile(profileName)
	require.NoError(t, err)
	t.Logf("Profile name: %s, path: %s", p.ProfileName, p.ProfilePath)

	// Smoke test to check that we are actually loading the profile we want and it has the setting.
	v := p.Config(configElasticEPRProxyTo, "")
	require.Equal(t, expectedEPR, v)

	// Now, apply resources and check that the variable has been used.
	stackVersion := "8.6.1"
	err = applyResources(p, stackVersion, stackVersion)
	require.NoError(t, err)

	d, err := os.ReadFile(p.Path(ProfileStackPath, DockerfilePackageRegistryFile))
	require.NoError(t, err)

	assert.Contains(t, string(d), fmt.Sprintf("ENV EPR_PROXY_TO=%s", expectedEPR))
}

func TestSemverLessThan(t *testing.T) {
	b, err := semverLessThan("8.9.0", "8.10.0-SNAPSHOT")
	require.NoError(t, err)
	assert.True(t, b)

	b, err = semverLessThan("8.10.0-SNAPSHOT", "8.10.0")
	require.NoError(t, err)
	assert.True(t, b)
}

func TestIndent(t *testing.T) {
	s := indent(`-----BEGIN CERTIFICATE-----
MIIByDCCAW+gAwIBAgIRAKZ7t5czbExcLrfZnBchSzUwCgYIKoZIzj0EAwIwHTEb
MBkGA1UEAxMSZWxhc3RpYy1wYWNrYWdlIENBMB4XDTI0MDIxMzA5MjM0M1oXDTI2
MDQyMzA5MjM0M1owGDEWMBQGA1UEAxMNZWxhc3RpYy1hZ2VudDBZMBMGByqGSM49
AgEGCCqGSM49AwEHA0IABBv3HqeW3NWIfp408trMNvBiSIHv4Dahc+os52yXN5/b
ho1G3WGLj0WYErCzJbB4He18pCV4c0/33o/lEYW3JjijgZQwgZEwDgYDVR0PAQH/
BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwHwYDVR0j
BBgwFoAUw0L8p+5q7uZycR3T7xj5pyWOIU8wOwYDVR0RBDQwMoIJbG9jYWxob3N0
gg1lbGFzdGljLWFnZW50hwR/AAABhxAAAAAAAAAAAAAAAAAAAAABMAoGCCqGSM49
BAMCA0cAMEQCIFukH6qlkBvHkZAccsFZZtX4vHQ7foeNTQhursBMmynOAiA0wwwQ
vvG/LwXVsGCXgSJahuOLkBPOaX2N+oDdYt267A==
-----END CERTIFICATE-----`, "        ")

	exp :=
		`-----BEGIN CERTIFICATE-----
        MIIByDCCAW+gAwIBAgIRAKZ7t5czbExcLrfZnBchSzUwCgYIKoZIzj0EAwIwHTEb
        MBkGA1UEAxMSZWxhc3RpYy1wYWNrYWdlIENBMB4XDTI0MDIxMzA5MjM0M1oXDTI2
        MDQyMzA5MjM0M1owGDEWMBQGA1UEAxMNZWxhc3RpYy1hZ2VudDBZMBMGByqGSM49
        AgEGCCqGSM49AwEHA0IABBv3HqeW3NWIfp408trMNvBiSIHv4Dahc+os52yXN5/b
        ho1G3WGLj0WYErCzJbB4He18pCV4c0/33o/lEYW3JjijgZQwgZEwDgYDVR0PAQH/
        BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwHwYDVR0j
        BBgwFoAUw0L8p+5q7uZycR3T7xj5pyWOIU8wOwYDVR0RBDQwMoIJbG9jYWxob3N0
        gg1lbGFzdGljLWFnZW50hwR/AAABhxAAAAAAAAAAAAAAAAAAAAABMAoGCCqGSM49
        BAMCA0cAMEQCIFukH6qlkBvHkZAccsFZZtX4vHQ7foeNTQhursBMmynOAiA0wwwQ
        vvG/LwXVsGCXgSJahuOLkBPOaX2N+oDdYt267A==
        -----END CERTIFICATE-----`

	assert.Equal(t, exp, s)

	s = indent("\n", "        ")
	exp = "\n        "
	assert.Equal(t, exp, s)
}
