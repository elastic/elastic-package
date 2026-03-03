// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package registry

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"
)

type packageMetadata struct {
	Download string `json:"download"`
}

// DownloadPackage fetches a package by name and version from the registry,
// extracts it into destDir, and returns the path to the extracted package root.
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

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("%s-%s-*.zip", name, version))
	if err != nil {
		return "", fmt.Errorf("creating temp file for package %s-%s: %w", name, version, err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(zipBytes); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("writing package zip %s-%s: %w", name, version, err)
	}
	tmpFile.Close()

	pkgRoot, err := extractZipPackage(tmpPath, destDir)
	if err != nil {
		return "", fmt.Errorf("extracting package %s-%s: %w", name, version, err)
	}

	logger.Debugf("Extracted package %s-%s to %s", name, version, pkgRoot)
	return pkgRoot, nil
}

// extractZipPackage extracts a package zip archive into destDir. EPR archives
// contain a single top-level directory (e.g. "name-version/"). The function
// returns the path to that directory.
func extractZipPackage(zipPath, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	var pkgRoot string
	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		if strings.Contains(cleanName, "..") {
			return "", fmt.Errorf("zip entry %q contains path traversal", f.Name)
		}

		dest := filepath.Join(destDir, cleanName)

		if pkgRoot == "" {
			parts := strings.SplitN(filepath.ToSlash(cleanName), "/", 2)
			pkgRoot = filepath.Join(destDir, parts[0])
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return "", fmt.Errorf("creating directory %s: %w", dest, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return "", fmt.Errorf("creating parent directory for %s: %w", dest, err)
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening zip entry %s: %w", f.Name, err)
		}

		outFile, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return "", fmt.Errorf("creating file %s: %w", dest, err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			rc.Close()
			return "", fmt.Errorf("extracting %s: %w", f.Name, err)
		}
		outFile.Close()
		rc.Close()
	}

	if pkgRoot == "" {
		return "", fmt.Errorf("zip archive is empty")
	}
	return pkgRoot, nil
}
