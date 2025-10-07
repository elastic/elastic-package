// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVarValue_MarshalJSON(t *testing.T) {
	t.Run("null", func(t *testing.T) {
		var vv VarValue
		data, err := json.Marshal(vv)
		require.NoError(t, err)
		assert.Equal(t, string(data), "null")
	})

	t.Run("scalar", func(t *testing.T) {
		vv := VarValue{
			scalar: "hello",
		}
		data, err := json.Marshal(vv)
		require.NoError(t, err)
		assert.Equal(t, string(data), `"hello"`)
	})

	t.Run("array", func(t *testing.T) {
		vv := VarValue{
			list: []interface{}{
				"hello",
				"world",
			},
		}

		data, err := json.Marshal(vv)
		require.NoError(t, err)
		assert.Equal(t, string(data), `["hello","world"]`)
	})
}

func TestDataStreamManifest_IndexTemplateName(t *testing.T) {
	cases := map[string]struct {
		dsm                       DataStreamManifest
		pkgName                   string
		expectedIndexTemplateName string
	}{
		"no_dataset": {
			DataStreamManifest{
				Name: "foo",
				Type: dataStreamTypeLogs,
			},
			"pkg",
			dataStreamTypeLogs + "-pkg.foo",
		},
		"no_dataset_hidden": {
			DataStreamManifest{
				Name:   "foo",
				Type:   dataStreamTypeLogs,
				Hidden: true,
			},
			"pkg",
			"." + dataStreamTypeLogs + "-pkg.foo",
		},
		"with_dataset": {
			DataStreamManifest{
				Name:    "foo",
				Type:    dataStreamTypeLogs,
				Dataset: "custom",
			},
			"pkg",
			dataStreamTypeLogs + "-custom",
		},
		"with_dataset_hidden": {
			DataStreamManifest{
				Name:    "foo",
				Type:    dataStreamTypeLogs,
				Dataset: "custom",
				Hidden:  true,
			},
			"pkg",
			"." + dataStreamTypeLogs + "-custom",
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			actualIndexTemplateName := test.dsm.IndexTemplateName(test.pkgName)
			require.Equal(t, test.expectedIndexTemplateName, actualIndexTemplateName)
		})
	}
}

func TestReadTransformDefinitionFile(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		packageManifest          string
		transformManifest        string
		createIngestPipelineFile bool
		ingestPipelineName       string
		expectedError            bool
		expectedErrorMessage     string
		expectedTransform        string
	}{
		"valid transform manifest with package version": {
			packageManifest: `
name: test-package
version: 1.2.3
`,
			createIngestPipelineFile: true,
			ingestPipelineName:       "my-pipeline",
			transformManifest: `
source:
  index: "logs-package.dataset"
dest:
  index: "logs-package_latest-index-1"
  pipeline: "{{ ingestPipelineName "my-pipeline" }}"
latest:
  unique_key:
    - event.dataset
`,
			expectedTransform: `
source:
  index: "logs-package.dataset"
dest:
  index: "logs-package_latest-index-1"
  pipeline: "1.2.3-my-pipeline"
latest:
  unique_key:
    - event.dataset
`,
			expectedError: false,
		},
		"invalid transform manifest without package version": {
			packageManifest: `
name: test-package
`,
			ingestPipelineName: "my-pipeline",
			transformManifest: `
source:
  index: "logs-package.dataset"
dest:
  index: "logs-package_latest-index-1"
  pipeline: "{{ ingestPipelineName "my-pipeline" }}"
latest:
  unique_key:
    - event.dataset
`,
			expectedError:        true,
			expectedErrorMessage: "package version is not defined in the package manifest",
		},
		"ingest_pipeline not exists": {
			packageManifest: `
name: test-package
version: 1.2.3
`,
			createIngestPipelineFile: false,
			ingestPipelineName:       "my-pipeline",
			transformManifest: `
source:
  index: "logs-package.dataset"
dest:
  index: "logs-package_latest-index-1"
  pipeline: "{{ ingestPipelineName "my-pipeline" }}"
latest:
  unique_key:
    - event.dataset
`,
			expectedError:        true,
			expectedErrorMessage: "destination ingest pipeline file my-pipeline.yml not found:",
		},
		"ingest_pipeline name empty": {
			packageManifest: `
name: test-package
version: 1.2.3
`,
			createIngestPipelineFile: false,
			ingestPipelineName:       "my-pipeline",
			transformManifest: `
source:
  index: "logs-package.dataset"
dest:
  index: "logs-package_latest-index-1"
  pipeline: "{{ ingestPipelineName "" }}"
latest:
  unique_key:
    - event.dataset
`,
			expectedError:        true,
			expectedErrorMessage: "error calling ingestPipelineName: ingest pipeline name is not define",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup temporary directory for the package
			packageDir := t.TempDir()
			packageManifestPath := filepath.Join(packageDir, "manifest.yml")
			err := os.WriteFile(packageManifestPath, []byte(tc.packageManifest), 0644)
			require.NoError(t, err)

			// Optionally create an ingest pipeline file
			if tc.createIngestPipelineFile {
				ingestPipelineDir := filepath.Join(packageDir, "elasticsearch", "ingest_pipeline")
				err = os.MkdirAll(ingestPipelineDir, 0755)
				require.NoError(t, err)
				ingestPipelinePath := filepath.Join(ingestPipelineDir, tc.ingestPipelineName+".yml")
				err = os.WriteFile(ingestPipelinePath, []byte(`---\nprocessors: {}\n`), 0644)
				require.NoError(t, err)
			}

			// Setup temporary directory for the transform
			transformDir := filepath.Join(packageDir, "elasticsearch", "transform", "latest")
			err = os.MkdirAll(transformDir, 0755)
			require.NoError(t, err)
			transformManifestPath := filepath.Join(transformDir, "transform.yml")
			err = os.WriteFile(transformManifestPath, []byte(tc.transformManifest), 0644)
			require.NoError(t, err)

			// Call the function under test
			contents, _, err := ReadTransformDefinitionFile(transformManifestPath, packageDir)
			if tc.expectedError {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErrorMessage)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, contents)

				assert.Equal(t, tc.expectedTransform, string(contents))
			}
		})
	}
}
