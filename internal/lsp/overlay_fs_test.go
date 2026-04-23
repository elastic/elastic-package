// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverlayFSExposesVirtualFileMetadata(t *testing.T) {
	fsys := newOverlayFS(t.TempDir(), map[string]string{
		"manifest.yml": "title: buffer",
	})

	file, err := fsys.Open("manifest.yml")
	require.NoError(t, err)
	defer file.Close()

	data, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, "title: buffer", string(data))

	info, err := file.Stat()
	require.NoError(t, err)
	assert.Equal(t, "manifest.yml", info.Name())
	assert.Equal(t, int64(len("title: buffer")), info.Size())
	assert.Equal(t, fs.FileMode(0o444), info.Mode())
	assert.Equal(t, time.Time{}, info.ModTime())
	assert.False(t, info.IsDir())
	assert.Nil(t, info.Sys())
}
