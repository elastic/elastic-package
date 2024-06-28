// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/files"
)

func TestMigrationsFromLegacy(t *testing.T) {
	profileName := "default"
	homeDirName := "migration-home-legacy"
	homeDir := filepath.Join(t.TempDir(), homeDirName)
	profilesDirPath := filepath.Join(homeDir, "profiles")
	sourceDir := filepath.Join("testdata", homeDirName)
	err := files.CopyAll(sourceDir, homeDir)
	require.NoError(t, err)

	// Check some file from the original profile that will be later moved.
	assert.FileExists(t, filepath.Join(profilesDirPath, profileName, "stack", "snapshot.yml"))
	assert.NoFileExists(t, filepath.Join(profilesDirPath, profileName, "stack", "docker-compose.yml"))

	profile, err := loadProfile(profilesDirPath, profileName)
	t.Log(homeDir, profileName)
	require.NoError(t, err)

	dateCreated, err := time.Parse(dateFormat, "2024-05-15T12:18:58.505287578+02:00")
	require.NoError(t, err)
	expectedMeta := Metadata{
		Name:        profileName,
		DateCreated: dateCreated,
		Version:     "undefined",
	}
	assert.Equal(t, expectedMeta, profile.metadata)

	err = profile.migrate(currentVersion)
	require.NoError(t, err)

	// Check that the in-memory profile is updated.
	expectedMeta.Version = strconv.Itoa(currentVersion)
	assert.Equal(t, expectedMeta, profile.metadata)

	// Check some file that has been moved.
	assert.NoFileExists(t, filepath.Join(profilesDirPath, profileName, "stack", "snapshot.yml"))
	assert.FileExists(t, filepath.Join(profilesDirPath, profileName, "stack", "docker-compose.yml"))

	// Load it again to check that it is updated on disk too.
	profile, err = loadProfile(profilesDirPath, profileName)
	require.NoError(t, err)
	assert.Equal(t, expectedMeta, profile.metadata)
}
