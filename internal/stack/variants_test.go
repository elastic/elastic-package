package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectStackVersion_NoVersion(t *testing.T) {
	var version string
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "default")
}

func TestSelectStackVersion_OlderStack(t *testing.T) {
	version := "7.14.99-SNAPSHOT"
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "default")
}

func TestSelectStackVersion_80(t *testing.T) {
	version := "8.0.33"
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "80")
}

func TestSelectStackVersion_81(t *testing.T) {
	version := "8.1.99-SNAPSHOT"
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "80")
}

func TestSelectStackVersion_82(t *testing.T) {
	version := "8.2.3"
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "8x")
}

func TestSelectStackVersion_82plus(t *testing.T) {
	version := "8.5.0-SNAPSHOT"
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "8x")
}

func TestSelectStackVersion_8dot(t *testing.T) {
	version := "8."
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "8x")
}

func TestSelectStackVersion_8(t *testing.T) {
	version := "8"
	selected := selectStackVersion(version)
	assert.Equal(t, selected, "default")
}
