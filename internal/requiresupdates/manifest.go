// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"bytes"
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/elastic/elastic-package/internal/yamledit"
)

// setRequiresDependencyVersion updates the version of a package listed under requires.input or requires.content.
func setRequiresDependencyVersion(manifestBytes []byte, section, packageName, newVersion string) ([]byte, error) {
	doc, err := yamledit.NewDocumentBytes(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	seqPath := fmt.Sprintf("$.requires.%s", section)
	seqNode, err := doc.GetSequenceNode(seqPath)
	if err != nil {
		return nil, fmt.Errorf("manifest has no requires.%s block: %w", section, err)
	}

	idx := -1
	for i, v := range seqNode.Values {
		var item struct {
			Package string `yaml:"package"`
		}
		if err := yaml.NodeToValue(v, &item); err != nil {
			continue
		}
		if item.Package == packageName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("package %q not found under requires.%s", packageName, section)
	}

	if _, err = doc.SetKeyValue(fmt.Sprintf("%s[%d]", seqPath, idx), "version", newVersion, 0); err != nil {
		return nil, fmt.Errorf("updating version for package %q: %w", packageName, err)
	}

	var buf bytes.Buffer
	if _, err = doc.Write(&buf); err != nil {
		return nil, fmt.Errorf("writing manifest: %w", err)
	}
	return buf.Bytes(), nil
}
