// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"path/filepath"

	"github.com/elastic/package-spec/v3/code/go/pkg/linkedfiles"
)

func IncludeLinkedFiles(fromDir, toDir string) ([]linkedfiles.Link, error) {
	root, err := linkedfiles.FindRepositoryRoot()
	if err != nil {
		return nil, err
	}

	fromRel, err := filepath.Rel(root.Name(), fromDir)
	if err != nil {
		return nil, err
	}
	toRel, err := filepath.Rel(root.Name(), toDir)
	if err != nil {
		return nil, err
	}

	return linkedfiles.IncludeLinkedFiles(root, fromRel, toRel)
}
