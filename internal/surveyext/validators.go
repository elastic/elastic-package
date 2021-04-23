// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"fmt"
	"os"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
)

// PackageDoesNotExistValidator function checks if the package hasn't been already created.
func PackageDoesNotExistValidator(val interface{}) error {
	if baseDir, ok := val.(string); ok {
		_, err := os.Stat(baseDir)
		if err == nil {
			return fmt.Errorf(`package "%s" already exists`, baseDir)
		}
	}
	return errors.New("string type expected")
}

// SemverValidator function checks if the value is a correct semver.
func SemverValidator(val interface{}) error {
	if ver, ok := val.(string); ok {
		_, err := semver.NewVersion(ver)
		if err != nil {
			return errors.Wrap(err, "can't parse value as proper semver")
		}
	}
	return errors.New("string type expected")
}

// DefaultConstraintValue function returns a constraint
func DefaultConstraintValue() string {
	ver := semver.MustParse(install.DefaultStackVersion)
	v, _ := ver.SetPrerelease("")
	return "^" + v.String()
}

// ConstraintValidator function checks if the value is a correct version constraint.
func ConstraintValidator(val interface{}) error {
	if c, ok := val.(string); ok {
		_, err := semver.NewConstraint(c)
		if err != nil {
			return errors.Wrap(err, "can't parse value as proper constraint")
		}
	}
	return errors.New("string type expected")
}
