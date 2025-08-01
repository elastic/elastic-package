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
		pd := createPackageDescriptorForTest("integration", "^7.13.0")
		dd := createDataStreamDescriptorForTest()
		dd.Manifest.Type = "logs"

		createAndCheckDataStream(t, pd, dd, true)
	})
	t.Run("valid-metrics", func(t *testing.T) {
		pd := createPackageDescriptorForTest("integration", "^7.13.0")
		dd := createDataStreamDescriptorForTest()
		dd.Manifest.Type = "metrics"

		createAndCheckDataStream(t, pd, dd, true)
	})
	t.Run("missing-type", func(t *testing.T) {
		pd := createPackageDescriptorForTest("integration", "^7.13.0")
		dd := createDataStreamDescriptorForTest()
		dd.Manifest.Type = ""

		createAndCheckDataStream(t, pd, dd, false)
	})
}

func createDataStreamDescriptorForTest() DataStreamDescriptor {
	elasticsearch := &packages.Elasticsearch{
		IndexTemplate: &packages.ManifestIndexTemplate{
			Mappings: &packages.ManifestMappings{
				Subobjects: false,
			},
		},
	}

	return DataStreamDescriptor{
		Manifest: packages.DataStreamManifest{
			Name:  "go_unit_test_data_stream",
			Title: "Go Unit Test Data Stream",
			Type:  "logs",

			Elasticsearch: elasticsearch,
		},
	}
}

func createAndCheckDataStream(t *testing.T, pd PackageDescriptor, dd DataStreamDescriptor, valid bool) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	tempDir := makeInRepoBuildTempDir(t)
	require.NoError(t, err)

	os.Chdir(tempDir)
	defer func() {
		os.Chdir(wd)
	}()

	err = CreatePackage(pd)
	require.NoError(t, err)

	packageRoot := filepath.Join(tempDir, pd.Manifest.Name)
	dd.PackageRoot = packageRoot

	err = CreateDataStream(dd)
	require.NoError(t, err)

	checkPackage(t, pd.Manifest.Name, valid)
}
