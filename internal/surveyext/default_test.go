// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package surveyext

import (
	"strings"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/require"
)

func TestDefaultConstraintValue(t *testing.T) {
	val := DefaultConstraintValue()

	_, err := semver.NewConstraint(val)
	require.NoError(t, err)
	require.False(t, strings.Contains(val, "-")) // No prerelease tag (for example: -SNAPSHOT)
}
