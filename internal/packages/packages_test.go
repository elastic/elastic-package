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

func TestDataStreamsForInput(t *testing.T) {
	t.Run("returns all matching data streams when policy template has no explicit list", func(t *testing.T) {
		// imported_mappings_tests has two data streams (first, second) both using
		// logfile input under a single policy template with no data_streams restriction.
		root, err := filepath.Abs("../../test/packages/other/imported_mappings_tests")
		require.NoError(t, err)

		pt := PolicyTemplate{Name: "sample"} // no DataStreams restriction

		got, err := DataStreamsForInput(root, pt, "logfile")
		require.NoError(t, err)
		names := make([]string, len(got))
		for i, ds := range got {
			names[i] = ds.Name
		}
		assert.ElementsMatch(t, []string{"first", "second"}, names)
	})

	t.Run("filters by policy template DataStreams list", func(t *testing.T) {
		// kubernetes has two policy templates (kubelet, kube-state-metrics) both
		// using kubernetes/metrics input, each scoped to different data streams.
		root, err := filepath.Abs("../../test/packages/with-kind/kubernetes")
		require.NoError(t, err)

		kubelet := PolicyTemplate{
			Name:        "kubelet",
			DataStreams: []string{"pod"},
		}
		got, err := DataStreamsForInput(root, kubelet, "kubernetes/metrics")
		require.NoError(t, err)
		names := make([]string, len(got))
		for i, ds := range got {
			names[i] = ds.Name
		}
		assert.Equal(t, []string{"pod"}, names)

		ksm := PolicyTemplate{
			Name:        "kube-state-metrics",
			DataStreams: []string{"state_pod", "state_cronjob"},
		}
		got, err = DataStreamsForInput(root, ksm, "kubernetes/metrics")
		require.NoError(t, err)
		names = make([]string, len(got))
		for i, ds := range got {
			names[i] = ds.Name
		}
		assert.ElementsMatch(t, []string{"state_pod", "state_cronjob"}, names)
	})

	t.Run("preserves dataset field for stream key building", func(t *testing.T) {
		// with_dataset has a data stream whose dataset field is overridden.
		root, err := filepath.Abs("../../test/packages/other/with_dataset")
		require.NoError(t, err)

		pt := PolicyTemplate{Name: "sample"}

		got, err := DataStreamsForInput(root, pt, "logfile")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "first", got[0].Name)
		assert.Equal(t, "with_dataset.overwritten_dataset.foo", got[0].Dataset)
	})

	t.Run("returns empty slice when no data stream matches the input type", func(t *testing.T) {
		root, err := filepath.Abs("../../test/packages/other/imported_mappings_tests")
		require.NoError(t, err)

		pt := PolicyTemplate{Name: "sample"}

		got, err := DataStreamsForInput(root, pt, "httpjson")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("returns empty slice when no data_stream directory exists", func(t *testing.T) {
		root := t.TempDir()
		pt := PolicyTemplate{Name: "sample"}

		got, err := DataStreamsForInput(root, pt, "logfile")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestReadTransformDefinitionFile(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		packageManifest                    string
		transformManifest                  string
		createIngestPipelineFile           bool
		createIngestPipelineFileDatastream bool
		ingestPipelineName                 string
		expectedError                      bool
		expectedErrorMessage               string
		expectedTransform                  string
	}{
		"valid transform manifest with package version": {
			packageManifest: `
name: test-package
version: 1.2.3
`,
			createIngestPipelineFile:           true,
			createIngestPipelineFileDatastream: false,
			ingestPipelineName:                 "my-pipeline",
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
			createIngestPipelineFile:           false,
			createIngestPipelineFileDatastream: false,
			ingestPipelineName:                 "my-pipeline",
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
			createIngestPipelineFile:           false,
			createIngestPipelineFileDatastream: false,
			ingestPipelineName:                 "my-pipeline",
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
			expectedErrorMessage: "destination ingest pipeline file my-pipeline.yml not found: incorrect version used in pipeline or unknown pipeline",
		},
		"ingest_pipeline name empty": {
			packageManifest: `
name: test-package
version: 1.2.3
`,
			createIngestPipelineFile:           false,
			createIngestPipelineFileDatastream: false,
			ingestPipelineName:                 "my-pipeline",
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
			expectedErrorMessage: "error calling ingestPipelineName: ingest pipeline name is empty",
		},
		"ingest_pipeline exists on data stream": {
			packageManifest: `
name: test-package
version: 1.2.3
`,
			createIngestPipelineFile:           false,
			createIngestPipelineFileDatastream: true,
			ingestPipelineName:                 "my-pipeline",
			transformManifest: `
source:
  index: "logs-package.dataset"
dest:
  index: "logs-package_latest-index-1"
  pipeline: "logs-test_package.test-{{ ingestPipelineName "my-pipeline" }}"
latest:
  unique_key:
    - event.dataset
`,
			expectedError: false,
			expectedTransform: `
source:
  index: "logs-package.dataset"
dest:
  index: "logs-package_latest-index-1"
  pipeline: "logs-test_package.test-1.2.3-my-pipeline"
latest:
  unique_key:
    - event.dataset
`,
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

			if tc.createIngestPipelineFileDatastream {
				ingestPipelineDir := filepath.Join(packageDir, "data_stream", "test", "elasticsearch", "ingest_pipeline")
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
