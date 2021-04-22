// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package profile

import (
	"io/ioutil"

	"github.com/pkg/errors"
)

// NewConfig is a generic function type to return a new Managed config
type NewConfig = func(profileName string, profilePath string) (*SimpleFile, error)

// SimpleFile defines a file that's managed by the profile system
// and doesn't require any  rendering
type SimpleFile struct {
	FileName string
	FilePath string
	FileBody string
}

// ConfigfilesDiffer checks to see if a local configItem differs from the one it knows.
func (cfg SimpleFile) ConfigfilesDiffer() (bool, error) {
	changes, err := ioutil.ReadFile(cfg.FilePath)
	if err != nil {
		return false, errors.Wrapf(err, "error reading %s", KibanaConfigFile)
	}
	if string(changes) != cfg.FileBody {
		return true, nil
	}
	return false, nil
}

// WriteConfig writes the config item
func (cfg SimpleFile) WriteConfig() error {
	err := ioutil.WriteFile(cfg.FilePath, []byte(cfg.FileBody), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing file failed (path: %s)", cfg.FilePath)
	}
	return nil
}

// ReadConfig reads the config item, overwriting whatever exists in the fileBody.
func (cfg *SimpleFile) ReadConfig() error {
	body, err := ioutil.ReadFile(cfg.FilePath)
	if err != nil {
		return errors.Wrapf(err, "reading filed failed (path: %s)", cfg.FilePath)
	}
	cfg.FileBody = string(body)
	return nil
}
