// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"
)

func renderResourceFile(templateBody string, data interface{}, targetPath string) error {
	t := template.Must(template.New("template").Parse(templateBody))
	var rendered bytes.Buffer
	err := t.Execute(&rendered, data)
	if err != nil {
		return errors.Wrap(err, "can't render package resource")
	}

	err = os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		return errors.Wrap(err, "can't create base directory")
	}

	packageManifestPath := targetPath
	err = os.WriteFile(packageManifestPath, rendered.Bytes(), 0644)
	if err != nil {
		return errors.Wrapf(err, "can't write resource file (path: %s)", packageManifestPath)
	}
	return nil
}

func decodeBase64Resource(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.Wrap(err, "can't decode encoded resource")
	}
	return decoded, nil
}

func writeRawResourceFile(content []byte, targetPath string) error {
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		return errors.Wrap(err, "can't create base directory")
	}

	packageManifestPath := targetPath
	err = os.WriteFile(packageManifestPath, content, 0644)
	if err != nil {
		return errors.Wrapf(err, "can't write resource file (path: %s)", packageManifestPath)
	}
	return nil
}
