// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/elastic/elastic-package/internal/packages"
)

// findPackageRoot finds the integration package root for a given file path.
// Returns ErrPackageRootNotFound if the file is not inside a package.
func findPackageRoot(filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	return packages.FindPackageRootFrom(dir)
}

// uriToPath converts a file:// URI to a filesystem path.
func uriToPath(uri string) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme: %s", u.Scheme)
	}

	path := u.Path
	// On Windows, file URIs look like file:///C:/path, so we need to strip
	// the leading slash from the path.
	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") && len(path) > 2 && path[2] == ':' {
		path = path[1:]
	}

	return filepath.FromSlash(path), nil
}

// pathToURI converts a filesystem path to a file:// URI.
func pathToURI(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}
