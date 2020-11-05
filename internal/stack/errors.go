package stack

import "fmt"

// UndefinedEnvError formats an error reported for undefined variable.
func UndefinedEnvError(envName string) error {
	return fmt.Errorf("undefined environment variable: %s", envName)
}