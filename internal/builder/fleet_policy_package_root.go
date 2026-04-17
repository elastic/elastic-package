// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/packages"
)

// FleetPolicyPackageRoot returns the directory used to read package and data_stream
// manifests when constructing Fleet PackagePolicy payloads (e.g. policy and system tests).
//
// Integrations with requires.input are "composable": policy_template input types and
// stream inputs are filled only on the built tree by requiredinputs.Bundle. Reading the
// source manifest would produce Fleet input keys that do not match the installed zip
// (e.g. template name plus empty type). Those integrations must use the built tree after
// BuildPackage; callers should run install/build first so that directory exists.
func FleetPolicyPackageRoot(sourcePackageRoot string) (string, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(sourcePackageRoot)
	if err != nil {
		return "", fmt.Errorf("read package manifest: %w", err)
	}
	if manifest.Type != "integration" {
		return sourcePackageRoot, nil
	}
	if manifest.Requires == nil || len(manifest.Requires.Input) == 0 {
		return sourcePackageRoot, nil
	}

	builtRoot, err := BuildPackagesDirectory(sourcePackageRoot, "")
	if err != nil {
		return "", fmt.Errorf("locate built package directory: %w", err)
	}
	builtManifest := filepath.Join(builtRoot, packages.PackageManifestFile)
	if _, err := os.Stat(builtManifest); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("built package manifest not found at %s (build or install the package before creating Fleet policies for composable integrations): %w", builtManifest, err)
		}
		return "", fmt.Errorf("stat built package manifest %s: %w", builtManifest, err)
	}
	return builtRoot, nil
}
