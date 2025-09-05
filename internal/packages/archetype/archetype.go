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

	"github.com/elastic/elastic-package/internal/packages"
)

func renderResourceFile(templateBody string, data interface{}, targetPath string) error {
	funcs := template.FuncMap{
		"indent":     indent,
		"yamlString": packages.VarValueYamlString,
	}
	t := template.Must(template.New("template").Funcs(funcs).Delims("{[", "]}").Parse(templateBody))
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

// indent adds `numSpaces` to the beginning of each line in `s`.
func indent(s string, numSpaces int) string {
	lines := strings.Split(s, "\n")

	var b strings.Builder
	indent := strings.Repeat(" ", numSpaces)
	for _, line := range lines {
		fmt.Fprintf(&b, "%s%s\n", indent, line)
	}
	return b.String()
}
