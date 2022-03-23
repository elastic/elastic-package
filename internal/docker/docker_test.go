// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitDockerContainerIDs_OnWindows(t *testing.T) {
	b := []byte("aaa\r\nbbb\r\nccc\r\n")
	containerIDs := SplitDockerContainerIDs(b)
	assert.Equal(t, "aaa", containerIDs[0])
	assert.Equal(t, "bbb", containerIDs[1])
	assert.Equal(t, "ccc", containerIDs[2])
}

func TestSplitDockerContainerIDs_OnLinux(t *testing.T) {
	b := []byte("aaa\nbbb\nccc\n")
	containerIDs := SplitDockerContainerIDs(b)
	assert.Equal(t, "aaa", containerIDs[0])
	assert.Equal(t, "bbb", containerIDs[1])
	assert.Equal(t, "ccc", containerIDs[2])
}
