// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// collectAndCopyPolicyTemplateFiles opens the input package at inputPkgPath,
// reads template names from its policy_templates manifest entries, copies each
// file from agent/input/<name> into destDir inside buildRoot with the prefix
// "<pkgName>-<name>", and returns the list of destination file names.
func collectAndCopyPolicyTemplateFiles(inputPkgPath, pkgName, destDir string, buildRoot *os.Root) ([]string, error) {
	inputPkgFS, closeFn, err := openPackageFS(inputPkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open input package %q: %w", inputPkgPath, err)
	}
	defer func() { _ = closeFn() }()

	manifestBytes, err := fs.ReadFile(inputPkgFS, packages.PackageManifestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read input package manifest: %w", err)
	}
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input package manifest: %w", err)
	}

	seen := make(map[string]bool)
	copiedNames := make([]string, 0)
	for _, pt := range manifest.PolicyTemplates {
		var names []string
		switch {
		case len(pt.TemplatePaths) > 0:
			names = pt.TemplatePaths
		case pt.TemplatePath != "":
			names = []string{pt.TemplatePath}
		}
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			content, err := fs.ReadFile(inputPkgFS, path.Join("agent", "input", name))
			if err != nil {
				return nil, fmt.Errorf("failed to read template %q from agent/input (declared in manifest): %w", name, err)
			}
			destName := pkgName + "-" + name
			if err := buildRoot.MkdirAll(destDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory %q: %w", destDir, err)
			}
			destPath := path.Join(destDir, destName)
			if err := buildRoot.WriteFile(destPath, content, 0644); err != nil {
				return nil, fmt.Errorf("failed to write template %q: %w", destName, err)
			}
			logger.Debugf("Copied input package template: %s -> %s", name, destName)
			copiedNames = append(copiedNames, destName)
		}
	}
	return copiedNames, nil
}
