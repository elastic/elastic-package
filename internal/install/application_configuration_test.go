package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectElasticAgentImageName_NoVersion(t *testing.T) {
	var version string
	selected := selectElasticAgentImageName(version)
	assert.Equal(t, selected, elasticAgentImageName)
}

func TestSelectElasticAgentImageName_OlderStack(t *testing.T) {
	version := "7.14.99-SNAPSHOT"
	selected := selectElasticAgentImageName(version)
	assert.Equal(t, selected, elasticAgentImageName)
}

func TestSelectElasticAgentImageName_FirstStackWithCompleteAgent(t *testing.T) {
	version := stackVersion715
	selected := selectElasticAgentImageName(version)
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}

func TestSelectElasticAgentImageName_NextStackWithAgentComplete(t *testing.T) {
	version := "7.16.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version)
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}
