// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package licenses

import (
	_ "embed"
	"fmt"
	"io"
	"os"
)

const (
	// Licenses supported by package-spec, as specified in https://spdx.org/licenses/
	Apache20  = "Apache-2.0"
	Elastic20 = "Elastic-2.0"
)

//go:embed _static/Apache-2.0.txt
var apache20text []byte

//go:embed _static/Elastic-2.0.txt
var elastic20text []byte

func getText(license string) ([]byte, error) {
	switch license {
	case Apache20:
		return apache20text, nil
	case Elastic20:
		return elastic20text, nil
	}
	return nil, fmt.Errorf("unknown license %q", license)
}

// WriteText writes the text of a license to the given writer.
func WriteText(license string, w io.Writer) error {
	text, err := getText(license)
	if err != nil {
		return err
	}
	_, err = w.Write(text)
	if err != nil {
		return fmt.Errorf("failed to write license text: %w", err)
	}
	return nil
}

// WriteTextToFile writes the text of a license to a file in the given path.
func WriteTextToFile(license string, path string) error {
	w, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create license file (%q): %w", path, err)
	}
	defer w.Close()
	return WriteText(license, w)
}
