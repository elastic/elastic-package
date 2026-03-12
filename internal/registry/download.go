// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/logger"
)

type packageMetadata struct {
	Download string `json:"download"`
}

// DownloadPackage fetches a package by name and version from the registry,
// saves the zip into destDir, and returns the path to the zip file.
func (c *Client) DownloadPackage(name, version, destDir string) (string, error) {
	metadataPath := fmt.Sprintf("/package/%s/%s", name, version)
	statusCode, body, err := c.get(metadataPath)
	if err != nil {
		return "", fmt.Errorf("fetching package metadata for %s-%s: %w", name, version, err)
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("fetching package metadata for %s-%s: status %d: %s", name, version, statusCode, body)
	}

	var meta packageMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", fmt.Errorf("parsing package metadata for %s-%s: %w", name, version, err)
	}
	if meta.Download == "" {
		return "", fmt.Errorf("package metadata for %s-%s has no download path", name, version)
	}

	logger.Debugf("Downloading package %s-%s from %s", name, version, meta.Download)
	statusCode, zipBytes, err := c.get(meta.Download)
	if err != nil {
		return "", fmt.Errorf("downloading package %s-%s: %w", name, version, err)
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("downloading package %s-%s: status %d", name, version, statusCode)
	}

	zipPath := filepath.Join(destDir, fmt.Sprintf("%s-%s.zip", name, version))
	if err := os.WriteFile(zipPath, zipBytes, 0644); err != nil {
		return "", fmt.Errorf("writing package zip %s-%s: %w", name, version, err)
	}

	logger.Debugf("Saved package %s-%s to %s", name, version, zipPath)
	return zipPath, nil
}
