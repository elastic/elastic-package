package cobraext

import "github.com/pkg/errors"

// FlagParsingError method wraps the original error with parsing error.
func FlagParsingError(err error, flagName string) error {
	return errors.Wrapf(err, "error parsing --%s flag", flagName)
}
