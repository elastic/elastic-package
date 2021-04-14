package kubectl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
