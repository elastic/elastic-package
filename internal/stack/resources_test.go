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

	os.Setenv("ELASTIC_PACKAGE_DATA_HOME", elasticPackagePath)

	// Create profile.
	err := profile.CreateProfile(profile.Options{
		// PackagePath is actually the profiles path, what is a bit counterintuitive.
		PackagePath: profilesPath,
		Name:        profileName,
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
	err = applyResources(p, "8.6.1")
	require.NoError(t, err)

	d, err := os.ReadFile(p.Path(ProfileStackPath, SnapshotFile))
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

func TestSemverLessThan(t *testing.T) {
	b, err := semverLessThan("8.9.0", "8.10.0-SNAPSHOT")
	require.NoError(t, err)
	assert.True(t, b)

	b, err = semverLessThan("8.10.0-SNAPSHOT", "8.10.0")
	require.NoError(t, err)
	assert.True(t, b)
}
