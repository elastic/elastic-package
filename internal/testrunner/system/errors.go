package system

import (
	"errors"
)

// ErrNoSystemTests is an error caused when no system tests are found for a package.
var ErrNoSystemTests = errors.New("no system tests defined")
