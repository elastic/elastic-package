// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// RemoveContent method wipes out the directory content.
func RemoveContent(dir string) error {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "readdir failed (path: %s)", dir)
	}

	for _, fi := range fis {
		p := filepath.Join(dir, fi.Name())
		err = os.RemoveAll(p)
		if err != nil {
			return errors.Wrapf(err, "removing resource failed (path: %s)", p)
		}
	}
	return nil
}
