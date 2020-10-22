// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package multierror

import (
	"fmt"
	"strings"
)

// Error is a multi-error representation.
type Error []error

// Error combines a detailed report consisting of attached errors separated with new lines.
func (me Error) Error() string {
	if me == nil {
		return ""
	}

	strs := make([]string, len(me))
	for i, err := range me {
		strs[i] = fmt.Sprintf("[%d] %v", i, err)
	}
	return strings.Join(strs, "\n")
}
