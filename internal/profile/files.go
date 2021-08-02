// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"os"

	"github.com/pkg/errors"
)

// NewConfig is a generic function type to return a new Managed config
type NewConfig = func(profileName string, profilePath string) (*simpleFile, error)

// simpleFile defines a file that's managed by the profile system
// and doesn't require any  rendering
type simpleFile struct {
	name string
	path string
	body string
}

const profileStackPath = "stack"

// configfilesDiffer checks to see if a local configItem differs from the one it knows.
func (cfg simpleFile) configfilesDiffer() (bool, error) {
	changes, err := os.ReadFile(cfg.path)
	if err != nil {
		return false, errors.Wrapf(err, "error reading %s", KibanaConfigFile)
	}
	if string(changes) != cfg.body {
		return true, nil
	}
	return false, nil
}

// writeConfig writes the config item
func (cfg simpleFile) writeConfig() error {
	err := os.WriteFile(cfg.path, []byte(cfg.body), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing file failed (path: %s)", cfg.path)
	}
	return nil
}

// readConfig reads the config item, overwriting whatever exists in the fileBody.
func (cfg *simpleFile) readConfig() error {
	body, err := os.ReadFile(cfg.path)
	if err != nil {
		return errors.Wrapf(err, "reading filed failed (path: %s)", cfg.path)
	}
	cfg.body = string(body)
	return nil
}
