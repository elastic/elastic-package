// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func renderResourceFile(templateBody string, data interface{}, targetPath string) error {
	t := template.Must(template.New("template").Funcs(template.FuncMap{"indent": Indent}).Parse(templateBody))
	var rendered bytes.Buffer
	err := t.Execute(&rendered, data)
	if err != nil {
		return fmt.Errorf("can't render package resource: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		return fmt.Errorf("can't create base directory: %w", err)
	}

	packageManifestPath := targetPath
	err = os.WriteFile(packageManifestPath, rendered.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("can't write resource file (path: %s): %w", packageManifestPath, err)
	}
	return nil
}

func decodeBase64Resource(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("can't decode encoded resource: %w", err)
	}
	return decoded, nil
}

func writeRawResourceFile(content []byte, targetPath string) error {
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		return fmt.Errorf("can't create base directory: %w", err)
	}

	packageManifestPath := targetPath
	err = os.WriteFile(packageManifestPath, content, 0644)
	if err != nil {
		return fmt.Errorf("can't write resource file (path: %s): %w", packageManifestPath, err)
	}
	return nil
}

//Indent adds `numSpaces` to the beginning of each line in `s`.
func Indent(s string, numSpaces int) string {
	lines := strings.Split(s, "\n")
	indent := ""
	for range numSpaces {
		indent = indent + " "
	}
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
