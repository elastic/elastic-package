// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kubectl

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed kube-state-metrics-single.yaml
var singleDefinitionFile string

//go:embed kube-state-metrics-multiple.yaml
var multipleDefinitionFiles string

func TestExtractResources_singleDefinition(t *testing.T) {
	r, err := extractResources([]byte(singleDefinitionFile))
	require.NoError(t, err)

	assert.Len(t, r, 1)
	assert.Equal(t, "Service", r[0].Kind)
	assert.Equal(t, "kube-state-metrics", r[0].Metadata.Name)
	assert.Equal(t, "kube-system", r[0].Metadata.Namespace)
}

func TestExtractResources_multipleDefinitions(t *testing.T) {
	r, err := extractResources([]byte(multipleDefinitionFiles))
	require.NoError(t, err)

	assert.Len(t, r, 5)
	assert.Equal(t, "ClusterRole", r[1].Kind)
	assert.Equal(t, "kube-state-metrics", r[1].Metadata.Name)
	assert.Empty(t, r[1].Metadata.Namespace)
}
