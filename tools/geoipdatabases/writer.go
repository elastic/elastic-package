// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/maxmind/mmdbwriter"
)

// writer is responsible for writing test mmdb databases
// based on the provided data sources.
type writer struct {
	source string
	target string
}

// newWriter initializes a new test database writer struct.
func newWriter(source, target string) (*writer, error) {
	s := filepath.Clean(source)
	if _, err := os.Stat(s); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("source directory does not exist: %w", err)
	}

	t := filepath.Clean(target)
	//nolint:gosec // not security sensitive.
	if err := os.MkdirAll(t, os.ModePerm); err != nil {
		return nil, fmt.Errorf("creating target directory: %w", err)
	}

	return &writer{
		source: s,
		target: t,
	}, nil
}

func (w *writer) write(dbWriter *mmdbwriter.Tree, fileName string) error {
	outputFile, err := os.Create(filepath.Clean(filepath.Join(w.target, fileName)))
	if err != nil {
		return fmt.Errorf("creating mmdb file: %w", err)
	}
	defer outputFile.Close()

	if _, err := dbWriter.WriteTo(outputFile); err != nil {
		return fmt.Errorf("writing mmdb file: %w", err)
	}
	return nil
}
