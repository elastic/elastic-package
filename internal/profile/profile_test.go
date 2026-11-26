// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProfile(t *testing.T) {
	options := Options{
		ProfilesDirPath: t.TempDir(),
	}
	err := CreateProfile(options)
	require.NoError(t, err)
}

func TestCreateProfileWithSettings(t *testing.T) {
	t.Setenv("ELASTIC_PACKAGE_DATA_HOME", t.TempDir())

	err := CreateProfile(Options{
		Name: "base",
		Settings: map[string]string{
			"stack.logstash_enabled":     "true",
			"stack.elastic_subscription": "basic",
		},
	})
	require.NoError(t, err)

	base, err := LoadProfile("base")
	require.NoError(t, err)
	assert.Equal(t, "true", base.Config("stack.logstash_enabled", ""))
	assert.Equal(t, "basic", base.Config("stack.elastic_subscription", ""))

	// Settings of the profile used as base are kept, unless they are set again.
	err = CreateProfile(Options{
		Name:        "other",
		FromProfile: "base",
		Settings: map[string]string{
			"stack.elastic_subscription": "trial",
			"stack.apm_enabled":          "true",
		},
	})
	require.NoError(t, err)

	other, err := LoadProfile("other")
	require.NoError(t, err)
	assert.Equal(t, "true", other.Config("stack.logstash_enabled", ""))
	assert.Equal(t, "trial", other.Config("stack.elastic_subscription", ""))
	assert.Equal(t, "true", other.Config("stack.apm_enabled", ""))
}
