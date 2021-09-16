// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
)

var (
	githubOwnerRegexp = regexp.MustCompile(`^(([a-zA-Z0-9-]+)|([a-zA-Z0-9-]+\/[a-zA-Z0-9-]+))$`)
)

// PackageDoesNotExistValidator function checks if the package hasn't been already created.
func PackageDoesNotExistValidator(val interface{}) error {
	baseDir, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := os.Stat(baseDir)
	if err == nil {
		return fmt.Errorf(`package "%s" already exists`, baseDir)
	}
	return nil
}

// DataStreamDoesNotExistValidator function checks if the package doesn't contain the data stream.
func DataStreamDoesNotExistValidator(val interface{}) error {
	name, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	dataStreamDir := filepath.Join("data_stream", name)
	_, err := os.Stat(dataStreamDir)
	if err == nil {
		return fmt.Errorf(`data stream "%s" already exists`, name)
	}
	return nil
}

// SemverValidator function checks if the value is a correct semver.
func SemverValidator(val interface{}) error {
	ver, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := semver.NewVersion(ver)
	if err != nil {
		return errors.Wrap(err, "can't parse value as proper semver")
	}
	return nil
}

// ConstraintValidator function checks if the value is a correct version constraint.
func ConstraintValidator(val interface{}) error {
	c, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := semver.NewConstraint(c)
	if err != nil {
		return errors.Wrap(err, "can't parse value as proper constraint")
	}
	return nil
}

// GithubOwnerValidator function checks if the Github owner is valid (team or user)
func GithubOwnerValidator(val interface{}) error {
	githubOwner, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	if !githubOwnerRegexp.MatchString(githubOwner) {
		return fmt.Errorf("value doesn't match the regular expression (organization/group or username): %s", githubOwnerRegexp.String())
	}
	return nil
}
