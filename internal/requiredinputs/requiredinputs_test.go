// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

type fakeEprClient struct {
	downloadPackageFunc func(packageName string, packageVersion string, tmpDir string) (string, error)
}

func (f *fakeEprClient) DownloadPackage(packageName string, packageVersion string, tmpDir string) (string, error) {
	if f.downloadPackageFunc != nil {
		return f.downloadPackageFunc(packageName, packageVersion, tmpDir)
	}
	return "", fmt.Errorf("download package not implemented")
}

func TestBundle_Success(t *testing.T) {
	fakeInputPath := createFakeInputHelper(t)
	fakeEprClient := &fakeEprClient{
		downloadPackageFunc: func(packageName string, packageVersion string, tmpDir string) (string, error) {
			return fakeInputPath, nil
		},
	}
	buildPackageRoot := t.TempDir()

	manifest := []byte(`name: test-package
version: 0.1.0
type: integration
requires:
  input:
    - package: sql
      version: 0.1.0
policy_templates:
  - inputs:
      - package: sql
      - type: logs
`)
	err := os.WriteFile(path.Join(buildPackageRoot, "manifest.yml"), manifest, 0644)
	require.NoError(t, err)

	resolver, err := NewRequiredInputsResolver(fakeEprClient)
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	_, err = os.ReadFile(path.Join(buildPackageRoot, "agent", "input", "sql-input.yml.hbs"))
	require.NoError(t, err)

	updatedManifestBytes, err := os.ReadFile(path.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	updatedManifest, err := packages.ReadPackageManifestBytes(updatedManifestBytes)
	require.NoError(t, err)
	require.Len(t, updatedManifest.Requires.Input, 1)
	require.Equal(t, "sql", updatedManifest.Requires.Input[0].Package)
	require.Equal(t, "0.1.0", updatedManifest.Requires.Input[0].Version)

	require.Equal(t, "sql", updatedManifest.PolicyTemplates[0].Inputs[0].Type)
	require.Empty(t, updatedManifest.PolicyTemplates[0].Inputs[0].Package)
	require.Len(t, updatedManifest.PolicyTemplates[0].Inputs[0].TemplatePaths, 1)
	require.Equal(t, "sql-input.yml.hbs", updatedManifest.PolicyTemplates[0].Inputs[0].TemplatePaths[0])

}

func TestBundle_NoManifest(t *testing.T) {
	fakeInputPath := createFakeInputHelper(t)
	fakeEprClient := &fakeEprClient{
		downloadPackageFunc: func(packageName string, packageVersion string, tmpDir string) (string, error) {
			return fakeInputPath, nil
		},
	}
	buildPackageRoot := t.TempDir()

	resolver, err := NewRequiredInputsResolver(fakeEprClient)
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to read package manifest")
}

func TestBundle_SkipNoIntegration(t *testing.T) {
	fakeInputPath := createFakeInputHelper(t)
	fakeEprClient := &fakeEprClient{
		downloadPackageFunc: func(packageName string, packageVersion string, tmpDir string) (string, error) {
			return fakeInputPath, nil
		},
	}
	buildPackageRoot := t.TempDir()

	manifest := []byte(`name: test-package
version: 0.1.0
type: input
`)
	err := os.WriteFile(path.Join(buildPackageRoot, "manifest.yml"), manifest, 0644)
	require.NoError(t, err)

	resolver, err := NewRequiredInputsResolver(fakeEprClient)
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)
}

func TestBundle_NoRequires(t *testing.T) {
	fakeEprClient := &fakeEprClient{
		downloadPackageFunc: func(packageName string, packageVersion string, tmpDir string) (string, error) {
			return "", fmt.Errorf("no download without requires")
		},
	}
	buildPackageRoot := t.TempDir()

	manifest := []byte(`name: test-package
version: 0.1.0
type: integration
policy_templates:
  - inputs:
      - type: logs
`)
	err := os.WriteFile(path.Join(buildPackageRoot, "manifest.yml"), manifest, 0644)
	require.NoError(t, err)

	resolver, err := NewRequiredInputsResolver(fakeEprClient)
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	updatedManifestBytes, err := os.ReadFile(path.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	updatedManifest, err := packages.ReadPackageManifestBytes(updatedManifestBytes)
	require.NoError(t, err)
	require.Nil(t, updatedManifest.Requires)
}
