// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackageDoesNotExistValidator_Exists(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	os.Chdir("testdata")
	defer os.Chdir(wd)

	err = PackageDoesNotExistValidator("hello-world")
	require.Error(t, err)
}

func TestPackageDoesNotExistValidator_NotExists(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	os.Chdir("testdata")
	defer os.Chdir(wd)

	err = PackageDoesNotExistValidator("lost-world")
	require.NoError(t, err)
}

func TestSemverValidator_Valid(t *testing.T) {
	err := SemverValidator("1.2.3-FOOBAR")
	require.NoError(t, err)
}

func TestSemverValidator_Invalid(t *testing.T) {
	err := SemverValidator("1.2.3.4")
	require.Error(t, err)
}

func TestConstraintValidator_Valid(t *testing.T) {
	err := ConstraintValidator("^1.2.3")
	require.NoError(t, err)
}

func TestConstraintValidator_Invalid(t *testing.T) {
	err := ConstraintValidator("+1.2.3")
	require.Error(t, err)
}

func TestGithubOwnerValidator_ValidUser(t *testing.T) {
	err := GithubOwnerValidator("mtojek")
	require.NoError(t, err)
}

func TestGithubOwnerValidator_InvalidUser(t *testing.T) {
	err := GithubOwnerValidator("mtojek%")
	require.Error(t, err)
}

func TestGithubOwnerValidator_ValidTeam(t *testing.T) {
	err := GithubOwnerValidator("elastic/integrations")
	require.NoError(t, err)
}

func TestGithubOwnerValidator_InvalidTeam(t *testing.T) {
	err := GithubOwnerValidator("elastic/integrations/123")
	require.Error(t, err)
}
