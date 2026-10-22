// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/agentdeployer"
	"github.com/elastic/elastic-package/internal/profile"
)

func TestServiceLogsIndependentAgents_PackageName(t *testing.T) {
	// Folder name (apache_basic_license) differs from the manifest name (apache).
	t.Chdir(filepath.Join("..", "..", "test", "packages", "parallel", "apache_basic_license"))

	p := &profile.Profile{ProfilePath: t.TempDir()}
	logsDir := agentdeployer.ServiceLogsDir(p)
	for _, name := range []string{
		"agent-apache-access-11111",
		"agent-apache-error-22222",
		"agent-apache-33333", // without data stream
		"agent-nginx-access-44444",
		"agent-apache2-access-55555", // other package whose name starts as this one
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(logsDir, name), 0o755))
	}

	_, err := ServiceLogsIndependentAgents(p)
	require.NoError(t, err)

	// Only the directories of the package in the current directory are removed, found by its name in the manifest.
	assert.NoDirExists(t, filepath.Join(logsDir, "agent-apache-access-11111"))
	assert.NoDirExists(t, filepath.Join(logsDir, "agent-apache-error-22222"))
	assert.NoDirExists(t, filepath.Join(logsDir, "agent-apache-33333"))
	assert.DirExists(t, filepath.Join(logsDir, "agent-nginx-access-44444"))
	assert.DirExists(t, filepath.Join(logsDir, "agent-apache2-access-55555"))
}
