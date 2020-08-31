// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type formatter func(content []byte) ([]byte, bool, error)

var formatters = map[string]formatter{
	".json": jsonFormatter,
	".yaml": yamlFormatter,
	".yml":  yamlFormatter,
}

// Format method formats files inside of the integration directory.
func Format(packageRoot string, failFast bool) error {
	err := filepath.Walk(packageRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == "ingest_pipeline" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		err = formatFile(path, failFast)
		if err != nil {
			return errors.Wrapf(err, "formatting file failed (path: %s)", path)
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "walking through the integration files failed")
	}
	return nil
}

func formatFile(path string, failFast bool) error {
	file := filepath.Base(path)
	ext := filepath.Ext(file)

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "reading file content failed")
	}

	format, defined := formatters[ext]
	if !defined {
		return nil // no errors returned as we have few files that will be never formatted (png, svg, log, etc.)
	}

	newContent, alreadyFormatted, err := format(content)
	if err != nil {
		return errors.Wrap(err, "formatting file content failed")
	}

	if alreadyFormatted {
		return nil
	}

	if failFast {
		return fmt.Errorf("file is not formatted (path: %s)", path)
	}

	err = ioutil.WriteFile(path, newContent, 0755)
	if err != nil {
		return errors.Wrapf(err, "rewriting file failed (path: %s)", path)
	}
	return nil
}
