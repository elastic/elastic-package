// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"github.com/Masterminds/semver"

	"github.com/elastic/elastic-package/internal/install"
)

// DefaultConstraintValue function returns a constraint
func DefaultConstraintValue() string {
	ver := semver.MustParse(install.DefaultStackVersion)
	v, _ := ver.SetPrerelease("")
	return "^" + v.String()
}
