// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package changelog

import (
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
)

const (
	// PackageChangelogFile is the name of the package's changelog file.
	PackageChangelogFile = "changelog.yml"
)

// Entry represents an entry in a package changelog.yml file
type Entry struct {
	Description string `config:"description" json:"description" yaml:"description"`
	Type        string `config:"type" json:"type" yaml:"type"`
	Link        string `config:"link" json:"link" yaml:"link"`
}

// Revision represents an version in a package changelog.yml file
type Revision struct {
	Version string  `config:"version" json:"version" yaml:"version"`
	Changes []Entry `config:"changes" json:"changes" yaml:"changes"`
}

// ReadChangelogFromPackageRoot reads and parses the package changelog file for the given package.
func ReadChangelogFromPackageRoot(packageRoot string) ([]Revision, error) {
	return ReadChangelog(filepath.Join(packageRoot, PackageChangelogFile))
}

// ReadChangelog reads and parses the given package changelog file.
func ReadChangelog(path string) ([]Revision, error) {
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "reading file failed (path: %s)", path)
	}

	var c []Revision
	err = cfg.Unpack(&c)
	if err != nil {
		return nil, errors.Wrapf(err, "unpacking package changelog failed (path: %s)", path)
	}
	return c, nil
}
