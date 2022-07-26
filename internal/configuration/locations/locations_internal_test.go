// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package locations manages base file and directory locations from within the elastic-package config
package locations

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_configurationDir(t *testing.T) {
	userHome, err := os.UserHomeDir()
	assert.Nil(t, err)
	expected := filepath.Join(userHome, elasticPackageDir)

	actual, err := configurationDir()
	assert.Nil(t, err)

	assert.Equal(t, expected, actual)
}

func Test_configurationDirError(t *testing.T) {
	var env string
	// Copied from os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		env = "USERPROFILE"
	case "plan9":
		env = "home"
	default:
		env = "HOME"
	}
	homeEnv := os.Getenv(env)
	os.Unsetenv(env)

	_, err := configurationDir()
	assert.Error(t, err)

	os.Setenv(env, homeEnv)
}

func Test_configurationDirOverride(t *testing.T) {
	expected := "/tmp/foobar"
	os.Setenv(elasticPackageDataHome, expected)

	actual, err := configurationDir()
	assert.Nil(t, err)

	assert.Equal(t, expected, actual)
	os.Setenv(elasticPackageDataHome, "")
}
