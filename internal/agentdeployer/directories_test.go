// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/profile"
)

func TestServiceLogsDirNames(t *testing.T) {
	p := &profile.Profile{ProfilePath: t.TempDir()}

	withDataStream, err := CreateServiceLogsDir(p, "apache", "access", "123")
	require.NoError(t, err)
	assert.Equal(t, "agent-apache-access-123", filepath.Base(withDataStream))
	assert.DirExists(t, withDataStream)

	withoutDataStream, err := CreateServiceLogsDir(p, "apache", "", "456")
	require.NoError(t, err)
	assert.Equal(t, "agent-apache-456", filepath.Base(withoutDataStream))

	// The glob of a package matches its directories, and not the ones of packages with a similar name.
	other, err := CreateServiceLogsDir(p, "apache_basic_license", "access", "789")
	require.NoError(t, err)

	matches, err := filepath.Glob(ServiceLogsDirGlobPackage(p, "apache"))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{withDataStream, withoutDataStream}, matches)
	assert.NotContains(t, matches, other)
}
