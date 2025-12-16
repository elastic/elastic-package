// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
)

func TestTransform(t *testing.T) {
	b, err := os.ReadFile("./testdata/system-navigation.json")
	require.NoError(t, err)

	var given common.MapStr
	err = json.Unmarshal(b, &given)
	require.NoError(t, err)

	ctx := &transformationContext{
		packageName: "system",
	}

	results, err := applyTransformations(ctx, []common.MapStr{given})
	require.NoError(t, err)
	require.Len(t, results, 1)

	result, err := json.MarshalIndent(&results[0], "", "  ")
	require.NoError(t, err)

	expected, err := os.ReadFile("./test/system-navigation.json-expected.json")
	require.NoError(t, err)

	require.Equal(t, string(expected), string(result))
}

func TestReadSharedTagsFile(t *testing.T) {

	t.Run("file exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := os.MkdirAll(filepath.Join(tmpDir, "kibana"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, "kibana", "tags.yml"), []byte(`- text: tag1
- text: tag2
- text: tag3
`), 0644)
		require.NoError(t, err)

		tags, err := readSharedTagsFile(tmpDir)
		require.NoError(t, err)
		require.Equal(t, []string{"tag1", "tag2", "tag3"}, tags)
	})

	t.Run("file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		tags, err := readSharedTagsFile(tmpDir)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := os.MkdirAll(filepath.Join(tmpDir, "kibana"), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, "kibana", "tags.yml"), []byte(`- text: tag1
- text
- text: tag3
`), 0644)
		require.NoError(t, err)

		_, err = readSharedTagsFile(tmpDir)
		require.Error(t, err)
	})
}
