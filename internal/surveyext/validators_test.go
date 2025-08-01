// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackageDoesNotExistValidator_Exists(t *testing.T) {
	validator := Validator{Cwd: "testdata"}
	err := validator.PackageDoesNotExist("hello-world")
	require.Error(t, err)
}

func TestPackageDoesNotExistValidator_NotExists(t *testing.T) {
	validator := Validator{Cwd: "testdata"}
	err := validator.PackageDoesNotExist("lost-world")
	require.NoError(t, err)
}

func TestDataStreamDoesNotExistValidator_Exists(t *testing.T) {
	validator := Validator{Cwd: filepath.Join("testdata", "hello-world")}
	err := validator.DataStreamDoesNotExist("magic")
	require.Error(t, err)
}

func TestDataStreamDoesNotExistValidator_NotExists(t *testing.T) {
	validator := Validator{Cwd: filepath.Join("testdata", "hello-world")}
	err := validator.DataStreamDoesNotExist("no-magic")
	require.NoError(t, err)
}

func TestSemverValidator_Valid(t *testing.T) {
	err := Validator{}.Semver("1.2.3-FOOBAR")
	require.NoError(t, err)
}

func TestSemverValidator_Invalid(t *testing.T) {
	err := Validator{}.Semver("1.2.3.4")
	require.Error(t, err)
}

func TestConstraintValidator_Valid(t *testing.T) {
	err := Validator{}.Constraint("^1.2.3")
	require.NoError(t, err)
}

func TestConstraintValidator_Invalid(t *testing.T) {
	err := Validator{}.Constraint("+1.2.3")
	require.Error(t, err)
}

func TestGithubOwnerValidator_ValidUser(t *testing.T) {
	err := Validator{}.GithubOwner("mtojek")
	require.NoError(t, err)
}

func TestGithubOwnerValidator_InvalidUser(t *testing.T) {
	err := Validator{}.GithubOwner("mtojek%")
	require.Error(t, err)
}

func TestGithubOwnerValidator_ValidTeam(t *testing.T) {
	err := Validator{}.GithubOwner("elastic/integrations")
	require.NoError(t, err)
}

func TestGithubOwnerValidator_InvalidTeam(t *testing.T) {
	err := Validator{}.GithubOwner("elastic/integrations/123")
	require.Error(t, err)
}
