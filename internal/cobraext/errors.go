// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cobraext

import "fmt"

// FlagParsingError method wraps the original error with parsing error.
func FlagParsingError(err error, flagName string) error {
	return fmt.Errorf("error parsing --%s flag: %w", flagName, err)
}
