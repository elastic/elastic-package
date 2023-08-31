// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestDataStream(t *testing.T) {
	t.Run("valid-logs", func(t *testing.T) {
		pd := createPackageDescriptorForTest("integration")
		dd := createDataStreamDescriptorForTest()
		dd.Manifest.Type = "logs"

		err := createAndCheckDataStream(t, pd, dd)
		require.NoError(t, err)
	})
	t.Run("valid-metrics", func(t *testing.T) {
		pd := createPackageDescriptorForTest("integration")
		dd := createDataStreamDescriptorForTest()
		dd.Manifest.Type = "metrics"

		err := createAndCheckDataStream(t, pd, dd)
		require.NoError(t, err)
	})
	t.Run("missing-type", func(t *testing.T) {
		pd := createPackageDescriptorForTest("integration")
		dd := createDataStreamDescriptorForTest()
		dd.Manifest.Type = ""

		err := createAndCheckDataStream(t, pd, dd)
		require.Error(t, err)
	})
}

func createDataStreamDescriptorForTest() DataStreamDescriptor {
	return DataStreamDescriptor{
		Manifest: packages.DataStreamManifest{
			Name:  "go_unit_test_data_stream",
			Title: "Go Unit Test Data Stream",
			Type:  "logs",
		},
	}
}

func createAndCheckDataStream(t require.TestingT, pd PackageDescriptor, dd DataStreamDescriptor) error {
	wd, err := os.Getwd()
	require.NoError(t, err)

	tempDir, err := os.MkdirTemp("", "archetype-create-data-stream-")
	require.NoError(t, err)

	os.Chdir(tempDir)
	defer func() {
		os.Chdir(wd)
		os.RemoveAll(tempDir)
	}()

	err = CreatePackage(pd)
	require.NoError(t, err)

	packageRoot := filepath.Join(tempDir, pd.Manifest.Name)
	dd.PackageRoot = packageRoot

	err = CreateDataStream(dd)
	require.NoError(t, err)

	err = checkPackage(pd.Manifest.Name)
	return err
}
