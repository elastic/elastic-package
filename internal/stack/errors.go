// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"errors"
	"fmt"
)

// ErrUndefinedEnv is an error about an undefined environment variable for the current profile.
type ErrUndefinedEnv struct {
	EnvName string
}

// Error returns the error message for this error.
func (err *ErrUndefinedEnv) Error() string {
	return fmt.Sprintf("undefined environment variable: %s. If you have started the Elastic Stack using the elastic-package tool, "+
		`please load stack environment variables using '%s' or set their values manually`, err.EnvName, helpText(AutodetectShell()))
}

// UndefinedEnvError formats an error reported for undefined variable.
func UndefinedEnvError(envName string) error {
	return &ErrUndefinedEnv{EnvName: envName}
}

// ErrUnavailableStack is an error about an unavailable Elastic stack.
var ErrUnavailableStack = errors.New("Elastic stack unavailable, remember to start it with 'elastic-package stack up', or configure elastic-package with environment variables")
