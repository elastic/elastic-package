// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/cobraext"
)

func TestServiceNameToBootUp(t *testing.T) {
	// Folder name (apache_basic_license) differs from the manifest name (apache).
	packageRoot := filepath.Join("..", "test", "packages", "parallel", "apache_basic_license")

	t.Run("default is the name in the manifest", func(t *testing.T) {
		name, err := serviceNameToBootUp("", packageRoot)
		require.NoError(t, err)
		assert.Equal(t, "apache", name)
	})

	t.Run("service flag", func(t *testing.T) {
		name, err := serviceNameToBootUp("apache_basic_license", packageRoot)
		require.NoError(t, err)
		assert.Equal(t, "apache_basic_license", name)
	})

	t.Run("manifest is not required with the service flag", func(t *testing.T) {
		name, err := serviceNameToBootUp("my_service", t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, "my_service", name)
	})

	t.Run("missing manifest without the service flag", func(t *testing.T) {
		_, err := serviceNameToBootUp("", t.TempDir())
		require.Error(t, err)
		assert.ErrorContains(t, err, "reading package manifest failed")
	})
}

func TestServiceUpCommandHasServiceFlag(t *testing.T) {
	up, _, err := setupServiceCommand().Find([]string{"up"})
	require.NoError(t, err)

	flag := up.Flags().Lookup(cobraext.ServiceFlagName)
	require.NotNil(t, flag, "the up command must define the service flag")
	assert.Empty(t, flag.DefValue)
}
