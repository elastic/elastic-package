// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DumpableJSONResource interface {
	Name() string
	JSON() []byte
}

func dumpJSONResource(dir string, object DumpableJSONResource) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}
	formatted, err := formatJSON(object.JSON())
	if err != nil {
		return fmt.Errorf("failed to format JSON object: %w", err)
	}
	path := filepath.Join(dir, object.Name()+".json")
	err = os.WriteFile(path, formatted, 0644)
	if err != nil {
		return fmt.Errorf("failed to dump object to file: %w", err)
	}
	return nil
}

func formatJSON(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	err := json.Indent(&buf, in, "", "  ")
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
