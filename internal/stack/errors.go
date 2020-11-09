// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import "fmt"

// UndefinedEnvError formats an error reported for undefined variable.
func UndefinedEnvError(envName string) error {
	return fmt.Errorf("undefined environment variable: %s", envName)
}
