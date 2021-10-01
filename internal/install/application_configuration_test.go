// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

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
