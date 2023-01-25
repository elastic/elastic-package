// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package multierror

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnique(t *testing.T) {
	errs := Error{
		fmt.Errorf("2"),
		fmt.Errorf("1"),
		fmt.Errorf("2"),
		fmt.Errorf("1"),
		fmt.Errorf("3"),
	}

	unique := errs.Unique()

	require.Len(t, unique, 3)
	require.Len(t, errs, 5)
}
