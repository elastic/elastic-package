// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Masterminds/semver/v3"
)

var (
	githubOwnerRegexp = regexp.MustCompile(`^(([a-zA-Z0-9-_]+)|([a-zA-Z0-9-_]+\/[a-zA-Z0-9-_]+))$`)

	packageNameRegexp    = regexp.MustCompile(`^[a-z0-9_]+$`)
	dataStreamNameRegexp = regexp.MustCompile(`^([a-z0-9]{2}|[a-z0-9][a-z0-9_]+[a-z0-9])$`)
)

type Validator struct {
	Cwd string
}

// PackageDoesNotExist function checks if the package hasn't been already created.
func (v Validator) PackageDoesNotExist(val interface{}) error {
	baseDir, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := os.Stat(filepath.Join(v.Cwd, baseDir))
	if err == nil {
		return fmt.Errorf(`package "%s" already exists`, baseDir)
	}
	return nil
}

// DataStreamDoesNotExist function checks if the package doesn't contain the data stream.
func (v Validator) DataStreamDoesNotExist(val interface{}) error {
	name, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	dataStreamDir := filepath.Join(v.Cwd, "data_stream", name)
	_, err := os.Stat(dataStreamDir)
	if err == nil {
		return fmt.Errorf(`data stream "%s" already exists`, name)
	}
	return nil
}

// Semver function checks if the value is a correct semver.
func (v Validator) Semver(val interface{}) error {
	ver, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := semver.NewVersion(ver)
	if err != nil {
		return fmt.Errorf("can't parse value as proper semver: %w", err)
	}
	return nil
}

// Constraint function checks if the value is a correct version constraint.
func (v Validator) Constraint(val interface{}) error {
	c, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}
	_, err := semver.NewConstraint(c)
	if err != nil {
		return fmt.Errorf("can't parse value as proper constraint: %w", err)
	}
	return nil
}

// GithubOwner function checks if the Github owner is valid (team or user)
func (v Validator) GithubOwner(val interface{}) error {
	githubOwner, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	if !githubOwnerRegexp.MatchString(githubOwner) {
		return fmt.Errorf("value doesn't match the regular expression (organization/group or username): %s", githubOwnerRegexp.String())
	}
	return nil
}

func (v Validator) PackageName(val interface{}) error {
	packageName, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	if !packageNameRegexp.MatchString(packageName) {
		return fmt.Errorf("value doesn't match the regular expression (package name): %s", packageNameRegexp.String())
	}
	return nil
}

func (v Validator) DataStreamName(val interface{}) error {
	dataStreamFolderName, ok := val.(string)
	if !ok {
		return errors.New("string type expected")
	}

	if !dataStreamNameRegexp.MatchString(dataStreamFolderName) {
		return fmt.Errorf("value doesn't match the regular expression (datastream name): %s", dataStreamNameRegexp.String())
	}
	return nil
}
