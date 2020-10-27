// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package multierror

import (
	"fmt"
	"sort"
	"strings"
)

// Error is a multi-error representation.
type Error []error

// Unique selects only unique
func (me Error) Unique() Error {
	// Create copy of multi error array
	errs := me

	// Sort them first
	sort.Slice(errs, func(i, j int) bool {
		return sort.StringsAreSorted([]string{errs[i].Error(), errs[j].Error()})
	})

	// Select unique values
	var unique []error
	encountered := map[string]struct{}{}
	for _, err := range errs {
		if _, ok := encountered[err.Error()]; !ok {
			encountered[err.Error()] = struct{}{}
			unique = append(unique, err)
		}
	}
	return unique
}

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
