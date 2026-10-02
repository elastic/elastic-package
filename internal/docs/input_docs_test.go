// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestRenderInputDocsQualifiedInputs(t *testing.T) {
	builtRoot := "testdata/built_dual_input"

	manifest, err := packages.ReadPackageManifestFromPackageRoot(builtRoot)
	require.NoError(t, err)

	withoutMapping, err := renderInputDocs(builtRoot, nil)
	require.NoError(t, err)
	assert.NotContains(t, withoutMapping, "<summary>logfile</summary>")

	rendered, err := renderInputDocs(builtRoot, inputTypesByName(manifest))
	require.NoError(t, err)
	assert.Contains(t, rendered, "<summary>logfile</summary>")
	assert.Equal(t, 1, strings.Count(rendered, "<summary>logfile</summary>"))
}
