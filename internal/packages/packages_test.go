// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestVarValueYamlString(t *testing.T) {
	t.Run("nil value returns empty", func(t *testing.T) {
		var vv VarValue
		assert.Equal(t, "", VarValueYamlString(vv, "default"))
	})

	t.Run("simple scalar", func(t *testing.T) {
		vv := VarValue{scalar: "10s"}
		assert.Equal(t, "default: 10s", VarValueYamlString(vv, "default"))
	})

	t.Run("scalar with special characters is quoted", func(t *testing.T) {
		vv := VarValue{scalar: "hello: world"}
		assert.Equal(t, "default: 'hello: world'", VarValueYamlString(vv, "default"))
	})

	t.Run("integer scalar", func(t *testing.T) {
		vv := VarValue{scalar: 42}
		assert.Equal(t, "default: 42", VarValueYamlString(vv, "default"))
	})

	t.Run("boolean scalar", func(t *testing.T) {
		vv := VarValue{scalar: true}
		assert.Equal(t, "default: true", VarValueYamlString(vv, "default"))
	})

	t.Run("list with default indent adds padding for template embedding", func(t *testing.T) {
		vv := VarValue{list: []interface{}{"a", "b", "c"}}
		result := VarValueYamlString(vv, "default")
		lines := strings.Split(result, "\n")
		require.Len(t, lines, 4)
		assert.Equal(t, "default:", lines[0])
		pad := strings.Repeat(" ", 4)
		for _, line := range lines[1:] {
			assert.True(t, strings.HasPrefix(line, pad), "continuation line %q should start with 4-space pad", line)
		}
	})

	t.Run("list with custom indent 6", func(t *testing.T) {
		vv := VarValue{list: []interface{}{"x", "y"}}
		result := VarValueYamlString(vv, "default", 6)
		lines := strings.Split(result, "\n")
		require.Len(t, lines, 3)
		assert.Equal(t, "default:", lines[0])
		pad := strings.Repeat(" ", 6)
		for _, line := range lines[1:] {
			assert.True(t, strings.HasPrefix(line, pad), "continuation line %q should start with 6-space pad", line)
		}
	})

	t.Run("list continuation lines are padded for template embedding", func(t *testing.T) {
		vv := VarValue{list: []interface{}{"/var/log/*.log", "/tmp/*.log"}}
		result := VarValueYamlString(vv, "default", 6)
		lines := strings.Split(result, "\n")
		require.Len(t, lines, 3)
		assert.Equal(t, "default:", lines[0])
		pad := strings.Repeat(" ", 6)
		for _, line := range lines[1:] {
			assert.True(t, strings.HasPrefix(line, pad), "continuation line %q should start with 6-space pad", line)
		}
	})

	t.Run("single element list", func(t *testing.T) {
		vv := VarValue{list: []interface{}{"only"}}
		result := VarValueYamlString(vv, "default")
		lines := strings.Split(result, "\n")
		require.Len(t, lines, 2)
		assert.Equal(t, "default:", lines[0])
		assert.True(t, strings.HasPrefix(lines[1], "    "), "continuation line should start with 4-space pad")
		assert.Contains(t, lines[1], "- only")
	})

	t.Run("multiline list embeds correctly in template at indent 6", func(t *testing.T) {
		// Simulates how the dataStream-manifest.yml.tmpl uses yamlString:
		//   "      {[ yamlString .Default "default" 6 ]}"
		// The template provides 6 leading spaces for the first line only.
		// The function must pad continuation lines so the full embedded
		// result is valid YAML. The YAML encoder indents list items by 6
		// (its SetIndent value), and the function prepends another 6 spaces
		// so continuation lines align at column 12 (matching the template's
		// 6-space base + the encoder's 6-space relative indent).
		vv := VarValue{list: []interface{}{"/var/log/*.log", "/tmp/*.log"}}
		result := VarValueYamlString(vv, "default", 6)

		templateIndent := "      "
		var embedded strings.Builder
		for i, line := range strings.Split(result, "\n") {
			if i == 0 {
				embedded.WriteString(templateIndent)
			}
			embedded.WriteString(line)
			embedded.WriteByte('\n')
		}

		expected := "      default:\n            - /var/log/*.log\n            - /tmp/*.log\n"
		assert.Equal(t, expected, embedded.String())
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
