// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/cobraext"
)

func TestFilterRegistry_Parse(t *testing.T) {

	t.Run("parse valid flags", func(t *testing.T) {
		cmd := &cobra.Command{}
		SetFilterFlags(cmd)
		cmd.Flags().Set(cobraext.FilterCategoriesFlagName, "security")
		cmd.Flags().Set(cobraext.FilterInputFlagName, "tcp")

		registry := NewFilterRegistry(2, "")
		err := registry.Parse(cmd)
		require.NoError(t, err)
		assert.Len(t, registry.filters, 2)
	})

	t.Run("parse no flags", func(t *testing.T) {
		cmd := &cobra.Command{}
		SetFilterFlags(cmd)

		registry := NewFilterRegistry(2, "")
		err := registry.Parse(cmd)
		require.NoError(t, err)
		assert.Empty(t, registry.filters)
	})
}

func TestFilterRegistry_Validate(t *testing.T) {
	t.Run("validate valid filters", func(t *testing.T) {
		registry := NewFilterRegistry(2, "")
		err := registry.Validate()
		assert.NoError(t, err)
	})
}

func TestFilterRegistry_Execute(t *testing.T) {
	// Use real test packages for execution test
	testPackagesPath, err := filepath.Abs("../../test/packages")
	require.NoError(t, err)

	categoryFlag := initCategoryFlag()
	categoryFlag.values = []string{"security"}

	t.Run("execute with real packages", func(t *testing.T) {
		registry := NewFilterRegistry(3, "")

		registry.filters = append(registry.filters, categoryFlag)

		filtered, errors := registry.Execute(testPackagesPath)

		// multierror.Error is empty
		require.Empty(t, errors)
		require.NotEmpty(t, filtered)

		for _, pkg := range filtered {
			assert.Contains(t, pkg.Manifest.Categories, "security")
		}
	})
}
